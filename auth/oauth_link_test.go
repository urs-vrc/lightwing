package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// Regression test for the broken Discord authorize link: redirect_uri must be
// the path registered with Discord (/api/auth/callback/discord), and state
// must be a pure random value — the frontend redirect target travels via the
// stored OAuth state row, not in the wire state.
func Test_SignInSocialDiscordLinkShape(t *testing.T) {
	t.Setenv("LIGHTWING_DEBUG_LOGIN", "")
	ctx := context.Background()
	svc := testAuthService()

	const frontendCallback = "https://lightwing-canary.urs.deno.net/auth?redirect=%2F"
	resp, err := svc.SignInSocial(ctx, &SignInSocialParams{
		CallbackURL: frontendCallback,
	})
	if err != nil {
		t.Fatalf("SignInSocial failed: %v", err)
	}

	authURL, err := url.Parse(resp.RedirectURL)
	if err != nil {
		t.Fatalf("parse redirect url: %v", err)
	}
	q := authURL.Query()

	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		t.Fatalf("authorize url %q has no redirect_uri", resp.RedirectURL)
	}
	ru, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirect_uri: %v", err)
	}
	if ru.Path != "/api/auth/callback/discord" {
		t.Errorf("redirect_uri path = %q, want %q", ru.Path, "/api/auth/callback/discord")
	}

	state := q.Get("state")
	if len(state) != 64 || strings.Trim(state, "0123456789abcdef") != "" {
		t.Errorf("state = %q, want 64-char hex with no embedded redirect", state)
	} else if strings.Contains(state, ":") || strings.Contains(state, "http") {
		t.Errorf("state = %q, must not embed the frontend URL", state)
	}

	// The callback recovers the frontend target from the stored row.
	var stored string
	err = db.QueryRow(ctx,
		`SELECT "value" FROM "verification" WHERE "identifier" = $1`,
		"oauth_state:"+state,
	).Scan(&stored)
	if err != nil {
		t.Fatalf("stored OAuth state missing for %q: %v", state, err)
	}
	if !strings.Contains(stored, frontendCallback) {
		t.Errorf("stored state value = %q, want it to contain %q", stored, frontendCallback)
	}
}
