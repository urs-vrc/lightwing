package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"encore.dev/metrics"
	"encore.dev/rlog"
	"encore.dev/storage/cache"
	"encore.dev/beta/errs"
	"encore.app/shared"
	"golang.org/x/oauth2"
)

// Discord OAuth2 configuration.
// Source: ts-legacy/auth/auth.ts — discord OAuth provider with scope ["identify"].
var discordEndpoint = oauth2.Endpoint{
	AuthURL:  "https://discord.com/oauth2/authorize",
	TokenURL: "https://discord.com/api/oauth2/token",
}

// discordGuildID is the URS Discord guild ID.
const discordGuildID = "1482993434410225739"

// staffRoleNames is the set of Discord role names that grant SITE_ADMIN.
var staffRoleNames = map[string]bool{
	"Moderation Staff":                          true,
	"Lead Staff":                                true,
	"Event Staff":                               true,
	"Competitive Integrity Administration Staff": true,
}

// discordStaffRolesCacheKey is the singleton cache key for Discord staff role IDs.
const discordStaffRolesCacheKey = "discord-staff-role-ids"

// discordStaffRolesCacheVal is the cached value structure for Discord staff role IDs.
type discordStaffRolesCacheVal struct {
	IDs []string `json:"ids"`
}

// discordStaffRolesKey is the cache key type.
type discordStaffRolesKey struct {
	Key string
}

// actorCacheKey is the cache key for the Actor cache.
type actorCacheKey struct {
	Token string
}

// memberRoleCacheKey is the cache key for the member role cache.
type memberRoleCacheKey struct {
	Key string // "orgId:userId"
}

type cachedMemberRole struct {
	Role string
}

// --- Package-level Encore resources ---
// The database and cache cluster live in the shared package (mirroring
// ts-legacy/db.ts and ts-legacy/cache.ts); keyspaces stay service-local.

var actorCache = cache.NewStructKeyspace[actorCacheKey, Actor](shared.Cache, cache.KeyspaceConfig{
	KeyPattern:    "actor/:Token",
	DefaultExpiry: cache.ExpireIn(240 * time.Second),
})

var memberRoleCache = cache.NewStructKeyspace[memberRoleCacheKey, cachedMemberRole](shared.Cache, cache.KeyspaceConfig{
	KeyPattern:    "member-role/:Key",
	DefaultExpiry: cache.ExpireIn(180 * time.Second),
})

var discordRolesCache = cache.NewStructKeyspace[discordStaffRolesKey, discordStaffRolesCacheVal](shared.Cache, cache.KeyspaceConfig{
	KeyPattern:    "discord-staff-roles/:Key",
	DefaultExpiry: cache.ExpireIn(300 * time.Second),
})

// secrets is populated by Encore's runtime from the app's secret store
// (dashboard Secrets page or `encore secret set`). Field names must match the
// secret names exactly.
var secrets struct {
	DISCORD_AUTH_CLIENT_ID     string
	DISCORD_AUTH_CLIENT_SECRET string
	DISCORD_BOT_TOKEN          string
	SESSION_COOKIE_SECRET      string
}

// serviceSecrets is the auth service's resolved credentials: framework secret
// values first, plain environment variables as a fallback (self-hosted Docker
// runs outside Encore's secret injection).
type serviceSecrets struct {
	DiscordAuthClientID     string
	DiscordAuthClientSecret string
	DiscordBotToken         string
	AuthBaseURL             string
	// SessionCookieSecret signs the session cookie. Empty in local dev yields
	// an ephemeral key (sessions reset on restart) with a startup warning.
	SessionCookieSecret string
	ephemeralCookieKey  bool
}

func secretOrEnv(frameworkValue, envName string) string {
	if frameworkValue != "" {
		return frameworkValue
	}
	return os.Getenv(envName)
}

func loadSecrets() *serviceSecrets {
	s := &serviceSecrets{
		DiscordAuthClientID:     secretOrEnv(secrets.DISCORD_AUTH_CLIENT_ID, "DISCORD_AUTH_CLIENT_ID"),
		DiscordAuthClientSecret: secretOrEnv(secrets.DISCORD_AUTH_CLIENT_SECRET, "DISCORD_AUTH_CLIENT_SECRET"),
		DiscordBotToken:         secretOrEnv(secrets.DISCORD_BOT_TOKEN, "DISCORD_BOT_TOKEN"),
		AuthBaseURL:             os.Getenv("ENCORERUNTIME_API_BASE_URL"),
		SessionCookieSecret:     secretOrEnv(secrets.SESSION_COOKIE_SECRET, "SESSION_COOKIE_SECRET"),
	}
	if s.SessionCookieSecret == "" {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			panic(fmt.Sprintf("failed to generate ephemeral cookie secret: %v", err))
		}
		s.SessionCookieSecret = base64.RawURLEncoding.EncodeToString(b[:])
		s.ephemeralCookieKey = true
	}
	return s
}

// --- Metrics ---

// getSessionOutcomeLabels tracks the (cookie present? × session resolved?) cross-tab
// on /get-session to diagnose cross-origin session-resolution issues.
type getSessionOutcomeLabels struct {
	HasCookie  bool
	HasSession bool
}

// oauthCallbackErrorLabels tracks OAuth callback error codes.
type oauthCallbackErrorLabels struct {
	Error string
}

var (
	getSessionOutcome  = metrics.NewCounterGroup[getSessionOutcomeLabels, uint64]("auth_get_session_outcome", metrics.CounterConfig{})
	oauthCallbackError = metrics.NewCounterGroup[oauthCallbackErrorLabels, uint64]("auth_oauth_callback_error", metrics.CounterConfig{})
)

// Service is the auth service.
//encore:service
type Service struct {
	secrets *serviceSecrets
}

// initService is called by Encore's runtime.
func initService() (*Service, error) {
	s := &Service{
		secrets: loadSecrets(),
	}
	if s.secrets.ephemeralCookieKey {
		rlog.Warn("SESSION_COOKIE_SECRET is unset; using an ephemeral cookie key (sessions reset on restart). Set the secret for stable sessions.")
	}
	return s, nil
}

// Shutdown is called during graceful shutdown.
func (s *Service) Shutdown(force context.Context) {
	// no-op; resources (db, cache) are managed by Encore runtime
}

// --- Discord integration ---

// getDiscordStaffRoleIds returns the set of Discord role IDs that grant staff status.
// Cached for 5 minutes.
//
// Mirrors ts-legacy/auth/auth.ts getDiscordStaffRoleIds
func (s *Service) getDiscordStaffRoleIds(ctx context.Context) (map[string]bool, error) {
	cached, err := discordRolesCache.Get(ctx, discordStaffRolesKey{Key: discordStaffRolesCacheKey})
	if err == nil {
		result := make(map[string]bool, len(cached.IDs))
		for _, id := range cached.IDs {
			result[id] = true
		}
		return result, nil
	}
	// cache.Miss is expected — fall through to fetch from Discord

	roles, err := s.fetchDiscordGuildRoles(ctx)
	if err != nil {
		rlog.Error("failed to fetch Discord guild roles; returning empty staff roles", "err", err)
		return map[string]bool{}, nil
	}

	staffIds := make([]string, 0, len(roles))
	staffSet := make(map[string]bool)
	for _, role := range roles {
		if staffRoleNames[role.Name] {
			staffIds = append(staffIds, role.ID)
			staffSet[role.ID] = true
		}
	}

	_ = discordRolesCache.Set(ctx, discordStaffRolesKey{Key: discordStaffRolesCacheKey}, discordStaffRolesCacheVal{IDs: staffIds})
	return staffSet, nil
}

// discordRole represents a Discord guild role.
type discordRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) fetchDiscordGuildRoles(ctx context.Context) ([]discordRole, error) {
	if s.secrets.DiscordBotToken == "" {
		return nil, errors.New("Discord bot token is not set")
	}
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://discord.com/api/v10/guilds/"+discordGuildID+"/roles", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+s.secrets.DiscordBotToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch guild roles: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("discord API returned status %d: %s", resp.StatusCode, string(body))
	}
	var roles []discordRole
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, fmt.Errorf("failed to decode guild roles: %w", err)
	}
	return roles, nil
}

// discordGuildMember represents a Discord guild member.
type discordGuildMember struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// getDiscordGuildMember fetches a user's guild membership from Discord.
//
// Mirrors ts-legacy/auth/auth.ts getDiscordGuildMember
func (s *Service) getDiscordGuildMember(ctx context.Context, userID string) (*discordGuildMember, error) {
	rlog.Info(fmt.Sprintf("Checking Discord guild membership for user ID: %s", userID))

	if s.secrets.DiscordBotToken == "" {
		return nil, errors.New("Discord bot token is not set")
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://discord.com/api/v10/guilds/"+discordGuildID+"/members/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+s.secrets.DiscordBotToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch guild member: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		rlog.Info(fmt.Sprintf("User ID %s is NOT a member of the Discord guild", userID))
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	var member discordGuildMember
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil, fmt.Errorf("failed to decode guild member: %w", err)
	}

	rlog.Info(fmt.Sprintf("User ID %s is a member of the Discord guild", userID))
	return &member, nil
}

// isBootstrapSiteAdminUser checks if the given user is the first user in the
// database (who is automatically granted SITE_ADMIN).
//
// Mirrors ts-legacy/auth/auth.ts isBootstrapSiteAdminUser
func (s *Service) isBootstrapSiteAdminUser(ctx context.Context, userId string) (bool, error) {
	var firstUserId string
	err := db.QueryRow(ctx,
		`SELECT id FROM "user" ORDER BY "createdAt" ASC LIMIT 1`,
	).Scan(&firstUserId)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to fetch first user: %w", err)
	}
	return firstUserId == userId, nil
}

// syncSiteRoleFromDiscordMembership checks a user's Discord guild membership
// and updates their site role if they have staff privileges (or are the bootstrap admin).
//
// Mirrors ts-legacy/auth/auth.ts syncSiteRoleFromDiscordMembership
func (s *Service) syncSiteRoleFromDiscordMembership(ctx context.Context, userId string) error {
	if s.secrets.DiscordBotToken == "" {
		rlog.Warn("Discord bot token is not set; skipping site role sync")
		return nil
	}

	// Look up the Discord account ID for this user.
	var accountId string
	err := db.QueryRow(ctx,
		`SELECT "accountId" FROM "account" WHERE "userId" = $1 AND "providerId" = 'discord' ORDER BY "updatedAt" DESC LIMIT 1`,
		userId,
	).Scan(&accountId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User has no Discord account linked — they can't be staff.
			return nil
		}
		return fmt.Errorf("failed to fetch discord account: %w", err)
	}

	member, err := s.getDiscordGuildMember(ctx, accountId)
	if err != nil {
		rlog.Warn("failed to fetch Discord guild member", "err", err, "user_id", userId)
		return nil
	}
	if member == nil {
		return &errs.Error{
			Code:    errs.FailedPrecondition,
			Message: "you must be a member of the URS Discord server",
		}
	}

	staffRoleIds, err := s.getDiscordStaffRoleIds(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve staff role IDs: %w", err)
	}

	isBootstrap, err := s.isBootstrapSiteAdminUser(ctx, userId)
	if err != nil {
		return fmt.Errorf("failed to check bootstrap admin: %w", err)
	}

	// Check if the member has any staff role
	hasStaffRole := false
	for _, roleID := range member.Roles {
		if staffRoleIds[roleID] {
			hasStaffRole = true
			break
		}
	}

	nextSiteRole := string(SiteRoleUser)
	if isBootstrap || hasStaffRole {
		nextSiteRole = string(SiteRoleSiteAdmin)
	}

	// Get current site role
	var currentSiteRole string
	err = db.QueryRow(ctx,
		`SELECT "siteRole" FROM "user" WHERE id = $1`, userId,
	).Scan(&currentSiteRole)
	if err != nil {
		return fmt.Errorf("failed to fetch current site role: %w", err)
	}

	if currentSiteRole != nextSiteRole {
		rlog.Info(fmt.Sprintf("Updating site role for user ID %s from %s to %s", userId, currentSiteRole, nextSiteRole))
		_, err = db.Exec(ctx,
			`UPDATE "user" SET "siteRole" = $1 WHERE id = $2`,
			nextSiteRole, userId,
		)
		if err != nil {
			return fmt.Errorf("failed to update site role: %w", err)
		}
	} else {
		rlog.Info(fmt.Sprintf("No site role change needed for user ID %s; current role is %s", userId, currentSiteRole))
	}

	return nil
}

// generateSessionToken creates a cryptographically random session token.
// Mirrors the pattern from better-auth's session token generation.
func generateSessionToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateID generates a random UUID-like string for IDs.
func generateID() string {
	return shared.NewID()
}

// generateState creates a random OAuth state parameter for CSRF protection.
func generateState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// oauthRedirect carries the OAuth landing targets through the state row.
type oauthRedirect struct {
	Redirect      string `json:"redirect"`
	ErrorRedirect string `json:"error_redirect,omitempty"`
}

// storeOAuthState stores the OAuth state and redirect targets in the
// verification table for later validation on callback (single-use, 5 min).
// Mirrors TS storeStateStrategy: "database" (state stored in verification table).
func (s *Service) storeOAuthState(ctx context.Context, state, redirect, errorRedirect string) error {
	value, err := json.Marshal(oauthRedirect{Redirect: redirect, ErrorRedirect: errorRedirect})
	if err != nil {
		return fmt.Errorf("failed to encode OAuth state: %w", err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(ctx,
		`INSERT INTO "verification" ("id", "identifier", "value", "expiresAt", "createdAt", "updatedAt")
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		"oauth_state:"+state, "oauth_state:"+state, string(value),
		now.Add(5*time.Minute), now, now,
	)
	return err
}

// consumeOAuthState retrieves and deletes the stored OAuth state, returning
// the redirect targets.
func (s *Service) consumeOAuthState(ctx context.Context, state string) (oauthRedirect, error) {
	var raw string
	err := db.QueryRow(ctx,
		`SELECT "value" FROM "verification" WHERE "identifier" = $1 AND "expiresAt" > $2`,
		"oauth_state:"+state, time.Now().UTC(),
	).Scan(&raw)
	if err != nil {
		return oauthRedirect{}, err
	}
	// Delete the state (single use)
	_, _ = db.Exec(ctx, `DELETE FROM "verification" WHERE "identifier" = $1`, "oauth_state:"+state)
	var redir oauthRedirect
	if err := json.Unmarshal([]byte(raw), &redir); err != nil {
		// Plain-string rows predate the JSON envelope; treat as redirect only.
		redir = oauthRedirect{Redirect: raw}
	}
	return redir, nil
}
