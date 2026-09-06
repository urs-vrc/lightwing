package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"encore.app/auth/sqlc"
)

// Local debug login bypass.
//
// Discord OAuth does not work on localhost, so local development needs a way
// to sign in without a provider. When the LIGHTWING_DEBUG_LOGIN environment
// variable is set (to any non-empty value), the browser sign-in endpoint
// (POST /api/auth/sign-in/social) provisions a fixed SITE_ADMIN "debug user",
// sets the normal session cookie, and returns the caller's callbackURL — the
// SPA navigates there and picks up the session through the regular
// get-session cookie flow. Same auth, no provider round-trip.
//
// NEVER set LIGHTWING_DEBUG_LOGIN in production: anyone could mint an admin
// session.

// debugUserID is the stable id of the local debug user.
const debugUserID = "debug-user-local"

// debugLoginEnabled reports whether the local debug login bypass is active.
func debugLoginEnabled() bool {
	return os.Getenv("LIGHTWING_DEBUG_LOGIN") != ""
}

// ensureDebugUserSession upserts the debug user (SITE_ADMIN, so local testing
// covers admin flows) and mints a fresh single active session for it.
func ensureDebugUserSession(ctx context.Context) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(sessionLifetime)
	sessionToken := generateSessionToken()

	stx, err := db.Stdlib().BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer stx.Rollback()
	qq := q().WithTx(stx)

	_, err = qq.GetUserByID(ctx, debugUserID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		slug, serr := GenerateUniqueUserSlug(ctx, db.Stdlib(), "Debug User", debugUserID)
		if serr != nil {
			return "", fmt.Errorf("failed to generate debug user slug: %w", serr)
		}
		if err := qq.InsertDebugUser(ctx, sqlc.InsertDebugUserParams{
			ID: debugUserID, Slug: nullIfEmpty(slug), CreatedAt: now,
		}); err != nil {
			return "", fmt.Errorf("failed to insert debug user: %w", err)
		}
	case err != nil:
		return "", fmt.Errorf("failed to look up debug user: %w", err)
	}

	activeOrgID, err := qq.GetMemberActiveOrg(ctx, debugUserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to fetch active org: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		activeOrgID = ""
	}

	if err := qq.DeleteSessionsByUser(ctx, debugUserID); err != nil {
		return "", fmt.Errorf("failed to delete old sessions: %w", err)
	}
	if err := qq.InsertSession(ctx, sqlc.InsertSessionParams{
		ID: generateID(), UserId: debugUserID, Token: sessionToken,
		ActiveOrganizationId: sql.NullString{String: activeOrgID, Valid: activeOrgID != ""},
		ExpiresAt: expiresAt, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return "", fmt.Errorf("failed to insert session: %w", err)
	}
	if err := stx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}
	return sessionToken, nil
}
