package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"encore.dev"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"golang.org/x/oauth2"
)

// --- API Endpoint Input/Response Types ---

// SignInSocialResponse directs the client to the Discord authorization URL.
type SignInSocialResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

// GetSessionResponse mirrors the session shape returned by get-session and
// stored by the frontend in localStorage.
//
// Frontend stores: lightwing:session:token
// Sends:   Authorization: Bearer ***
type GetSessionResponse struct {
	Session SessionInfo `json:"session"`
	User    UserProfile `json:"user"`
}

// SessionInfo is the session portion of GetSessionResponse.
type SessionInfo struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

// SignInSocialParams provides the OAuth redirect URL for the callback.
type SignInSocialParams struct {
	// CallbackURL, if provided, indicates where the frontend expects to be
	// redirected after a successful sign-in callback.
	CallbackURL string `query:"callbackUrl"`
}

// --- Discord API Models ---

// discordAuthUser is the response from Discord's /users/@me endpoint.
type discordAuthUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
}

func discordUserAvatarURL(user *discordAuthUser) string {
	if user.Avatar == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", user.ID, user.Avatar)
}

// --- API Endpoints ---

// SignInSocial returns a redirect URL to start the Discord OAuth flow.
//
//encore:api public method=GET path=/auth/sign-in/social
func (s *Service) SignInSocial(ctx context.Context, p *SignInSocialParams) (*SignInSocialResponse, error) {
	redirectTo := p.CallbackURL
	if redirectTo == "" {
		redirectTo = "/auth"
	}

	if debugLoginEnabled() {
		sessionToken, serr := ensureDebugUserSession(ctx)
		if serr != nil {
			rlog.Error("failed to create debug session", "err", serr)
			return nil, &errs.Error{
				Code:    errs.Internal,
				Message: "failed to create debug session",
			}
		}
		targetURL := redirectTo
		parsed, err := url.Parse(targetURL)
		if err == nil {
			if parsed.Fragment != "" {
				parsed.Fragment += "&access_token=" + sessionToken
			} else {
				parsed.Fragment = "access_token=" + sessionToken
			}
			targetURL = parsed.String()
		}
		return &SignInSocialResponse{
			RedirectURL: targetURL,
		}, nil
	}

	state := generateState()
	if err := s.storeOAuthState(ctx, state, redirectTo, ""); err != nil {
		rlog.Error("failed to store OAuth state", "err", err)
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to initialize OAuth flow",
		}
	}

	conf := s.discordOAuthConfig("/api/auth/callback/discord")

	// The redirect target is stored server-side keyed by state (see
	// storeOAuthState); the wire state stays a pure random value so Discord
	// echoes it back verbatim.
	redirectURL := conf.AuthCodeURL(state, oauth2.SetAuthURLParam("prompt", "consent"))

	return &SignInSocialResponse{
		RedirectURL: redirectURL,
	}, nil
}

// discordOAuthConfig builds the Discord OAuth2 config for a callback path.
func (s *Service) discordOAuthConfig(callbackPath string) oauth2.Config {
	return oauth2.Config{
		ClientID:     s.secrets.DiscordAuthClientID,
		ClientSecret: s.secrets.DiscordAuthClientSecret,
		Endpoint:     discordEndpoint,
		RedirectURL:  encore.Meta().APIBaseURL.String() + callbackPath,
		Scopes:       []string{"identify"},
	}
}

// oauthCallbackURL returns the full URL of the OAuth callback endpoint.
func (s *Service) oauthCallbackURL() string {
	meta := encore.Meta()
	return meta.APIBaseURL.String() + "/api/auth/callback/discord"
}

// fetchDiscordUser fetches user info from Discord using OAuth2 token.
func fetchDiscordUser(ctx context.Context, conf oauth2.Config, token *oauth2.Token) (*discordAuthUser, error) {
	resp, err := conf.Client(ctx, token).Get("https://discordapp.com/api/users/@me")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord API returned non-200 status: %d", resp.StatusCode)
	}
	var discordUser discordAuthUser
	if err := json.NewDecoder(resp.Body).Decode(&discordUser); err != nil {
		return nil, err
	}
	return &discordUser, nil
}

// Callback handles the Discord OAuth redirect. It exchanges the code for a token,
// fetches the user profile from Discord, upserts the user/account/session in the
// database, then redirects back to the frontend with the session token as a
// URL fragment (so the SPA can extract it).
//
//encore:api public raw method=GET path=/api/auth/callback/discord
func (s *Service) Callback(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "missing code or state parameter", http.StatusBadRequest)
		return
	}

	// New flows send a pure random state; the redirect target comes from the
	// stored OAuth state row. The split below tolerates pre-fix
	// <state>:<redirect> states still in flight.
	parts := strings.SplitN(state, ":", 2)
	var storedState, redirectTo string
	if len(parts) == 2 {
		storedState = parts[0]
		redirectTo = parts[1]
	} else {
		storedState = state
	}

	// Validate state against stored OAuth state
	if storedState != "" {
		redir, err := s.consumeOAuthState(ctx, storedState)
		if err != nil {
			rlog.Error("invalid OAuth state", "err", err)
			http.Error(w, "invalid or expired state parameter", http.StatusBadRequest)
			return
		}
		if redirectTo == "" {
			redirectTo = redir.Redirect
		}
	}
	if redirectTo == "" {
		redirectTo = "/auth"
	}
	// The frontend sends CallbackURL as a full URL; reduce to path-only
	// before prepending the configured frontend origin (also covers
	// in-flight state rows stored before this normalization existed).
	redirectTo = normalizeRedirectTarget(redirectTo)

	conf := s.discordOAuthConfig("/api/auth/callback/discord")

	// Exchange code for token
	token, err := conf.Exchange(ctx, code)
	if err != nil {
		rlog.Error("failed to exchange OAuth code", "err", err)
		oauthCallbackError.With(oauthCallbackErrorLabels{Error: "token_exchange"}).Add(1)
		http.Error(w, "failed to exchange OAuth code", http.StatusInternalServerError)
		return
	}

	// Fetch user info from Discord
	discordUser, err := fetchDiscordUser(ctx, conf, token)
	if err != nil {
		rlog.Error("failed to fetch Discord user", "err", err)
		oauthCallbackError.With(oauthCallbackErrorLabels{Error: "userinfo_fetch"}).Add(1)
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	// Upsert user + account + session
	sessionToken, err := upsertUserAndSession(ctx, s, token, discordUser)
	if err != nil {
		rlog.Error("failed to upsert user/session", "err", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Redirect back to the frontend with the session token in the URL fragment.
	// The SPA extracts the fragment and stores it in localStorage.
	frontendURL := strings.TrimSuffix(s.frontendBaseURL(), "/")
	parsedURL, err := url.Parse(frontendURL + redirectTo)
	if err != nil {
		http.Error(w, "invalid redirect URL", http.StatusInternalServerError)
		return
	}
	if parsedURL.Fragment != "" {
		parsedURL.Fragment += "&access_token=" + sessionToken
	} else {
		parsedURL.Fragment = "access_token=" + sessionToken
	}
	http.Redirect(w, req, parsedURL.String(), http.StatusFound)
}

// normalizeRedirectTarget reduces a redirect target to a safe same-origin
// path. The frontend sends CallbackURL as a full URL
// (<origin>/auth?redirect=...), but Callback prepends the configured
// frontend origin — so an absolute URL must be reduced to path+query here,
// otherwise the concatenated origin twice fails url.Parse with
// "invalid redirect URL". Anything not a same-origin path (including
// protocol-relative "//host/..." targets) falls back to "/auth".
func normalizeRedirectTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "/auth"
	}
	if u, err := url.Parse(target); err == nil && u.IsAbs() {
		path := u.EscapedPath()
		if path == "" {
			path = "/"
		}
		if u.RawQuery != "" {
			path += "?" + u.RawQuery
		}
		return path
	}
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		return "/auth"
	}
	if u, err := url.Parse(target); err != nil || u.IsAbs() {
		return "/auth"
	}
	return target
}

// frontendBaseURL returns the configured frontend origin: framework secret
// first (settable via `encore secret set LIGHTWING_FRONTEND_URL`), plain
// environment as fallback, local-dev default otherwise.
func (s *Service) frontendBaseURL() string {
	if s != nil && s.secrets != nil && s.secrets.FrontendBaseURL != "" {
		return s.secrets.FrontendBaseURL
	}
	if v := os.Getenv("LIGHTWING_FRONTEND_URL"); v != "" {
		return v
	}
	return "http://localhost:3000"
}

// sessionLifetime matches session expiry (30 days).
const sessionLifetime = 30 * 24 * time.Hour

func upsertUserAndSession(ctx context.Context, svc *Service, token *oauth2.Token, discordUser *discordAuthUser) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(sessionLifetime)
	sessionToken := generateSessionToken()

	email := discordUser.ID + "@discord.invalid"
	displayName := discordUser.Username
	avatarURL := discordUserAvatarURL(discordUser)

	userID := ""
	err := db.QueryRow(ctx,
		`SELECT "userId" FROM "account" WHERE "providerId" = 'discord' AND "accountId" = $1`,
		discordUser.ID,
	).Scan(&userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to look up discord account: %w", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var existingUserID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM "user" WHERE id = $1`, userID,
	).Scan(&existingUserID)

	if errors.Is(err, sql.ErrNoRows) || userID == "" {
		var userCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM "user"`).Scan(&userCount); err != nil {
			return "", fmt.Errorf("failed to count users: %w", err)
		}
		siteRole := string(SiteRoleUser)
		if userCount == 0 {
			siteRole = string(SiteRoleSiteAdmin)
		}
		if userID == "" {
			userID = generateID()
		}
		slug, err := GenerateUniqueUserSlug(db.Stdlib(), displayName, userID)
		if err != nil {
			return "", fmt.Errorf("failed to generate user slug: %w", err)
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO "user" (id, name, email, image, "siteRole", "vrchatUsername", slug, "createdAt", "updatedAt")
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			userID, displayName, email, nullIfEmpty(avatarURL),
			siteRole, "", slug, now, now,
		)
		if err != nil {
			return "", fmt.Errorf("failed to insert user: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to check existing user: %w", err)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE "user" SET name = $1, email = $2, image = $3, "updatedAt" = $4
			 WHERE id = $5`,
			displayName, email, nullIfEmpty(avatarURL), now, userID,
		)
		if err != nil {
			return "", fmt.Errorf("failed to update user: %w", err)
		}
	}

	scope, _ := token.Extra("scope").(string)
	accessExpires := sql.NullTime{}
	if !token.Expiry.IsZero() {
		accessExpires = sql.NullTime{Time: token.Expiry.UTC(), Valid: true}
	}
	var existingAccountID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM "account" WHERE "userId" = $1 AND "providerId" = 'discord'`,
		userID,
	).Scan(&existingAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.Exec(ctx,
			`INSERT INTO "account" (id, "accountId", "providerId", "userId", "accessToken",
			                        "refreshToken", "scope", "accessTokenExpiresAt", "createdAt", "updatedAt")
			 VALUES ($1, $2, 'discord', $3, $4, $5, $6, $7, $8, $8)`,
			generateID(), discordUser.ID, userID, token.AccessToken,
			nullIfEmpty(token.RefreshToken), nullIfEmpty(scope), accessExpires, now,
		)
		if err != nil {
			return "", fmt.Errorf("failed to insert account: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to check existing account: %w", err)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE "account" SET "accessToken" = $1, "refreshToken" = $2, "scope" = $3,
			                     "accessTokenExpiresAt" = $4, "updatedAt" = $5
			 WHERE id = $6`,
			token.AccessToken, nullIfEmpty(token.RefreshToken), nullIfEmpty(scope),
			accessExpires, now, existingAccountID,
		)
		if err != nil {
			return "", fmt.Errorf("failed to update account: %w", err)
		}
	}

	var activeOrgID string
	err = tx.QueryRow(ctx,
		`SELECT "organizationId" FROM "member" WHERE "userId" = $1 ORDER BY "createdAt" ASC LIMIT 1`,
		userID,
	).Scan(&activeOrgID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to fetch active org: %w", err)
	}

	_, err = tx.Exec(ctx,
		`DELETE FROM "session" WHERE "userId" = $1`,
		userID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to delete old sessions: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO "session" (id, "userId", token, "activeOrganizationId", "expiresAt", "createdAt", "updatedAt")
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		generateID(), userID, sessionToken,
		sql.NullString{String: activeOrgID, Valid: activeOrgID != ""},
		expiresAt, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	go func() {
		backgroundCtx := context.Background()
		err := svc.syncSiteRoleFromDiscordMembership(backgroundCtx, userID)
		if err != nil {
			rlog.Error("failed to sync site role from Discord", "err", err)
		}
	}()

	return sessionToken, nil
}

// --- API Endpoints ---

// GetSession returns the session and user profile for the authenticated caller.
//
//encore:api public
func (s *Service) GetSession(ctx context.Context) (*GetSessionResponse, error) {
	token := getAuthorizationToken(ctx)

	if token == "" {
		getSessionOutcome.With(getSessionOutcomeLabels{}).Add(1)
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "not authenticated"}
	}
	return getSessionData(ctx, token, false)
}

// getSessionData resolves a session token to the session + profile response.
// Split from the endpoint so tests can call it with an explicit token.
func getSessionData(ctx context.Context, token string, hasCookie bool) (*GetSessionResponse, error) {
	actor, err := resolveActor(ctx, "Bearer "+token)
	if err != nil {
		rlog.Error("resolveActor failed in GetSession", "err", err)
		getSessionOutcome.With(getSessionOutcomeLabels{HasCookie: hasCookie, HasSession: false}).Add(1)
		return nil, err
	}
	getSessionOutcome.With(getSessionOutcomeLabels{HasCookie: hasCookie, HasSession: true}).Add(1)

	profile, err := getUserProfile(ctx, actor.UserID)
	if err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: "failed to load user profile"}
	}

	var expiresAt time.Time
	err = db.QueryRow(ctx,
		`SELECT "expiresAt" FROM "session" WHERE "token" = $1`, token,
	).Scan(&expiresAt)
	if err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: "failed to load session"}
	}

	if profile.Slug == nil {
		if slug, err := ensureUserSlug(ctx, db, actor.UserID); err != nil {
			rlog.Error("failed to ensure user slug", "err", err)
		} else {
			profile.Slug = &slug
		}
	}

	return &GetSessionResponse{
		Session: SessionInfo{
			Token:     token,
			ExpiresAt: expiresAt.Format(time.RFC3339Nano),
		},
		User: *profile,
	}, nil
}

// SignOut deletes the caller's session.
//
//encore:api public
func (s *Service) SignOut(ctx context.Context) error {
	token := getAuthorizationToken(ctx)
	if token == "" {
		return nil
	}

	if err := signOutSession(ctx, token); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// signOutSession deletes a session row and drops its cached actor.
func signOutSession(ctx context.Context, token string) error {
	if actorCache != nil {
		_, _ = actorCache.Delete(ctx, actorCacheKey{Token: token})
	}
	_, err := db.Exec(ctx, `DELETE FROM "session" WHERE "token" = $1`, token)
	return err
}

// --- Token Extraction ---

// getAuthorizationToken extracts the Bearer token from the current request's
// Authorization header, available via encore.CurrentRequest().
func getAuthorizationToken(ctx context.Context) string {
	req := encore.CurrentRequest()
	if req == nil {
		return ""
	}
	h := req.Headers.Get("Authorization")
	if h == "" {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(h), "Bearer ")
}
