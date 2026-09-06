package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func testAuthService() *Service {
	return &Service{secrets: &serviceSecrets{
		SessionCookieSecret:     "test-cookie-secret-at-least-32-bytes!",
		DiscordAuthClientID:     "test-client-id",
		DiscordAuthClientSecret: "test-client-secret",
	}}
}

// Debug bypass end-to-end through the SignInSocial endpoint: provisions the
// debug admin and returns redirectUrl with access_token fragment.
func Test_DebugSignInBypass(t *testing.T) {
	t.Setenv("LIGHTWING_DEBUG_LOGIN", "1")
	ctx := context.Background()
	svc := testAuthService()

	resp, err := svc.SignInSocial(ctx, &SignInSocialParams{
		CallbackURL: "http://localhost:5173/auth?redirect=/events",
	})
	if err != nil {
		t.Fatalf("SignInSocial failed: %v", err)
	}

	u, err := url.Parse(resp.RedirectURL)
	if err != nil {
		t.Fatalf("parse redirect url: %v", err)
	}

	if !strings.HasPrefix(u.String(), "http://localhost:5173/auth?redirect=/events#access_token=") {
		t.Errorf("redirectUrl = %q, want fragment with access_token", resp.RedirectURL)
	}

	token := strings.TrimPrefix(u.Fragment, "access_token=")
	if token == "" {
		t.Fatalf("access_token in fragment is empty")
	}

	// The token authenticates GetSession like a real Discord login.
	sessionResp, err := getSessionData(ctx, token, false)
	if err != nil {
		t.Fatalf("getSessionData status failed: %v", err)
	}

	if sessionResp.User.ID != debugUserID {
		t.Errorf("user id = %q, want %q", sessionResp.User.ID, debugUserID)
	}
	if sessionResp.User.SiteRole != string(SiteRoleSiteAdmin) {
		t.Errorf("siteRole = %q, want SITE_ADMIN", sessionResp.User.SiteRole)
	}
}

// Without the env var the endpoint keeps the Discord flow: it stores state
// and returns a Discord authorize URL.
func Test_DebugBypassDisabledKeepsDiscordFlow(t *testing.T) {
	t.Setenv("LIGHTWING_DEBUG_LOGIN", "")
	ctx := context.Background()
	svc := testAuthService()

	resp, err := svc.SignInSocial(ctx, &SignInSocialParams{
		CallbackURL: "http://localhost:5173/auth",
	})
	if err != nil {
		t.Fatalf("SignInSocial failed: %v", err)
	}

	if !strings.Contains(resp.RedirectURL, "discord.com") {
		t.Errorf("url = %q, want a Discord authorize URL", resp.RedirectURL)
	}
}

// ensureDebugUserSession provisions the admin and rotates the single session.
func Test_EnsureDebugUserSession(t *testing.T) {
	t.Setenv("LIGHTWING_DEBUG_LOGIN", "1")
	ctx := context.Background()

	token, err := ensureDebugUserSession(ctx)
	if err != nil {
		t.Fatalf("ensureDebugUserSession: %v", err)
	}
	actor, err := resolveActor(ctx, "Bearer "+token)
	if err != nil {
		t.Fatalf("resolveActor: %v", err)
	}
	if actor.UserID != debugUserID || !isSiteAdmin(actor.SiteRole) {
		t.Errorf("actor = %+v, want debug SITE_ADMIN", actor)
	}
	token2, err := ensureDebugUserSession(ctx)
	if err != nil {
		t.Fatalf("re-login: %v", err)
	}
	if token2 == token {
		t.Error("expected a fresh token on re-login")
	}
	var remaining int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM "session" WHERE "userId" = $1`, debugUserID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if remaining != 1 {
		t.Errorf("sessions = %d, want exactly 1", remaining)
	}
}
