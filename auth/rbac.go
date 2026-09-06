package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"encore.dev/beta/errs"
	"encore.dev/storage/cache"

	"encore.app/auth/sqlc"
)

// Actor represents the authenticated caller resolved from a session token.
type Actor struct {
	UserID               string
	ActiveOrganizationID string
	SiteRole             SiteRoleName
}

// actorCacheTTL is the conservative default TTL for cached actors (4 min).
const actorCacheTTL = 240 * time.Second

// resolveActor authenticates a caller from a session token supplied via the
// Authorization: Bearer *** header. The session table is queried to identify
// the caller, and the user's global siteRole is loaded alongside.
// Results are cached for up to 4 minutes (or the remaining session lifetime,
// whichever is shorter).
//
// Mirrors ts-legacy/auth/rbac.ts resolveActor
func resolveActor(ctx context.Context, authorization string) (*Actor, error) {
	token := strings.TrimPrefix(authorization, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "missing session token"}
	}

	// Try cache first (cache may be nil in test mode — skip if so)
	if actorCache != nil {
		cached, err := actorCache.Get(ctx, actorCacheKey{Token: token})
		if err == nil {
			return &cached, nil
		}
	}

	var userID, siteRole string
	var activeOrgID sql.NullString
	var expiresAt time.Time

	row, err := q().GetActorSession(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "invalid or expired session"}
	}
	if err != nil {
		return nil, err
	}
	userID, activeOrgID, siteRole, expiresAt = row.UserId, row.ActiveOrganizationId, row.USiteRole, row.ExpiresAt

	if !expiresAt.After(time.Now()) {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "invalid or expired session"}
	}

	actor := Actor{
		UserID:   userID,
		SiteRole: SiteRoleName(siteRole),
	}
	if activeOrgID.Valid {
		actor.ActiveOrganizationID = activeOrgID.String
	}

	// Cache the result with a TTL capped at the remaining session lifetime.
	if actorCache != nil {
		remaining := time.Until(expiresAt)
		ttlSeconds := remaining.Seconds() - 5
		if ttlSeconds > 0 {
			ttl := time.Duration(min(actorCacheTTL, time.Duration(ttlSeconds)*time.Second))
			_ = actorCache.With(cache.ExpireIn(ttl)).Set(ctx, actorCacheKey{Token: token}, actor)
		}
	}

	return &actor, nil
}

// getMemberRole looks up a user's role within an organization.
// Cached for 3 minutes.
//
// Mirrors ts-legacy/auth/rbac.ts getMemberRole
func getMemberRole(ctx context.Context, organizationId, userId string) (string, error) {
	key := organizationId + ":" + userId
	if memberRoleCache != nil {
		cached, err := memberRoleCache.Get(ctx, memberRoleCacheKey{Key: key})
		if err == nil {
			return cached.Role, nil
		}
	}

	role, err := q().GetMemberRole(ctx, sqlc.GetMemberRoleParams{
		OrganizationId: organizationId,
		UserId:         userId,
	})
	if errors.Is(err, sql.ErrNoRows) {
		role = ""
	} else if err != nil {
		return "", err
	}

	// Cache even if role is empty (null)
	if memberRoleCache != nil {
		_ = memberRoleCache.Set(ctx, memberRoleCacheKey{Key: key}, cachedMemberRole{Role: role})
	}
	return role, nil
}

// requirePermission authenticates the caller and asserts they hold a role in
// organizationId that grants action on resource, reusing the shared RBAC matrix.
// Site administrators short-circuit the org-scoped check (absolute control).
// Returns the resolved actor and their role on success.
//
// Mirrors ts-legacy/auth/rbac.ts requirePermission
func requirePermission(ctx context.Context, authorization string, organizationId string, resource Resource, action Action) (*Actor, string, error) {
	actor, err := resolveActor(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if isSiteAdmin(actor.SiteRole) {
		return actor, string(actor.SiteRole), nil
	}
	role, err := getMemberRole(ctx, organizationId, actor.UserID)
	if err != nil {
		return nil, "", err
	}
	if role == "" || !roleHasPermission(role, resource, action) {
		return nil, "", &errs.Error{
			Code:    errs.PermissionDenied,
			Message: "role is not permitted to " + string(action) + " " + string(resource),
		}
	}
	return actor, role, nil
}

// requireSiteAdmin asserts the caller holds the global SITE_ADMIN role.
//
// Mirrors ts-legacy/auth/rbac.ts requireSiteAdmin
func requireSiteAdmin(ctx context.Context, authorization string) (*Actor, error) {
	actor, err := resolveActor(ctx, authorization)
	if err != nil {
		return nil, err
	}
	if !isSiteAdmin(actor.SiteRole) {
		return nil, &errs.Error{
			Code:    errs.PermissionDenied,
			Message: "site administrator privilege required",
		}
	}
	return actor, nil
}

// requireEventPermission authorizes an action on a specific event.
// Resolution order:
//  1. SITE_ADMIN → absolute control.
//  2. user-owned → the owning user has full control.
//  3. org-owned → reuse the RBAC matrix against the owning organization.
//  4. explicit EventAdmin row (either ownership kind).
// Otherwise the request is denied.
//
// Mirrors ts-legacy/auth/rbac.ts requireEventPermission
func requireEventPermission(ctx context.Context, authorization string, eventId string, action Action) (*Actor, error) {
	actor, err := resolveActor(ctx, authorization)
	if err != nil {
		return nil, err
	}
	if isSiteAdmin(actor.SiteRole) {
		return actor, nil
	}

	// Load the event to check ownership. The non-applicable owner column is
	// NULL (user-owned events have no organizationId and vice versa).
	ownership, err := q().GetEventOwnership(ctx, eventId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return nil, err
	}
	ownerType, ownerUserID, organizationID := ownership.OwnerType, ownership.OwnerUserId, ownership.OrganizationId

	// Check user-owned
	if ownerType == string(EventOwnerTypeUser) {
		if ownerUserID.Valid && ownerUserID.String == actor.UserID {
			return actor, nil
		}
	} else if organizationID.Valid && organizationID.String != "" {
		// Check org-owned: reuse RBAC matrix
		role, err := getMemberRole(ctx, organizationID.String, actor.UserID)
		if err == nil && role != "" && roleHasPermission(role, ResourceEvent, action) {
			return actor, nil
		}
	}

	// Check explicit EventAdmin row
	eventAdminExists, err := q().EventAdminExists(ctx, sqlc.EventAdminExistsParams{
		EventId: eventId,
		UserId:  actor.UserID,
	})
	if err != nil {
		return nil, err
	}
	if eventAdminExists {
		return actor, nil
	}

	return nil, &errs.Error{
		Code:    errs.PermissionDenied,
		Message: "not permitted to " + string(action) + " this event",
	}
}

// min returns the smaller of two durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
