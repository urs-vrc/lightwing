package auth

import (
	"context"
	"strings"
	"testing"

)

// Test_slugify validates the slugify utility.
//
// Mirrors ts-legacy/lib/slugs.test.ts → "slugify"
func Test_slugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Hello   World", "hello-world"},
		{"Hello-World!", "hello-world"},
		{"  Hello  World  ", "hello-world"},
		{"Héllô Wörld", "hello-world"},           // accented characters stripped
		{"Héllô   Wörld 123", "hello-world-123"}, // multiple spaces + accented + numbers
		{"", ""},
		{"---", ""},                  // only separators → empty
		{"---Hello---", "hello"},     // leading/trailing dashes removed
		{"Hello World 123!", "hello-world-123"},
		{"foo__bar", "foo-bar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Test_isValidSlug validates team slug format.
//
// Mirrors ts-legacy/lib/slugs.test.ts → "isValidSlug"
func Test_isValidSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"abc", true},
		{"abc123", true},
		{"abc-123", true},
		{"a", false},        // too short (< 3)
		{"ab", false},       // too short (< 3)
		{"ab-", false},      // trailing dash
		{"-ab", false},      // leading dash
		{"a-b-c", true},
		{"ABC", false},      // uppercase
		{"hello!", false},   // invalid char
		{"admin", false},    // reserved
		{"api", false},      // reserved
		{"login", false},    // reserved
		{"auth", false},     // reserved
		{"profile", false},  // reserved
		{"dashboard", false}, // reserved
		{"settings", false},  // reserved
		{"help", false},     // reserved
		{"status", false},   // reserved
		{"a" + strings.Repeat("b", 24), false},  // too long (25 chars)
		{strings.Repeat("a", 24), true},           // 24 chars, exactly at limit
		{"hello-world-this-is-a-very-long-slug", false}, // > 24 chars
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := IsValidSlug(tt.slug)
			if got != tt.want {
				t.Errorf("IsValidSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}

// Test_isValidUserSlug validates user slug format.
//
// Mirrors ts-legacy/lib/slugs.test.ts → "isValidUserSlug"
func Test_isValidUserSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"abcd", true},
		{"abc123", true},
		{"a", false},         // too short (< 4)
		{"ab", false},        // too short (< 4)
		{"abc", false},       // too short (< 4)
		{"ABC", false},       // uppercase
		{"abc-123", false},   // hyphen not allowed
		{"admin", false},     // reserved
		{"users", false},     // reserved
		{"a" + strings.Repeat("b", 24), false}, // too long (25 chars)
		{strings.Repeat("a", 24), true},         // exactly at limit
		{"a-b-c", false},     // hyphen not allowed
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := IsValidUserSlug(tt.slug)
			if got != tt.want {
				t.Errorf("IsValidUserSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}

// Test_generateUniqueUserSlug validates unique user slug generation.
//
// Mirrors ts-legacy/lib/slugs.test.ts → "generateUniqueUserSlug"
func Test_generateUniqueUserSlug(t *testing.T) {
	ctx := context.Background()

	stdlibDB := db.Stdlib()

	// Insert a user holding slug "collisionuser".
	// Mirrors ts-legacy/auth/slugs.test.ts → "user slug collision resolution".
	_, err := db.Exec(ctx,
		`INSERT INTO "user" (id, name, email, image, "siteRole", "vrchatUsername", slug, "createdAt", "updatedAt")
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		"discord-user-1", "CollisionUser", "discord-user-1@discord.invalid", "",
		"USER", "", "collisionuser", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Same name for a different user resolves with a numeric suffix.
	slug, err := GenerateUniqueUserSlug(ctx, stdlibDB, "CollisionUser", "discord-user-2")
	if err != nil {
		t.Fatalf("GenerateUniqueUserSlug failed: %v", err)
	}
	if slug != "collisionuser2" {
		t.Errorf("GenerateUniqueUserSlug(\"CollisionUser\", \"discord-user-2\") = %q, want \"collisionuser2\"", slug)
	}
	if len(slug) > 24 {
		t.Errorf("slug %q exceeds 24 characters", slug)
	}

	// Multiple collisions keep incrementing the suffix.
	_, err = db.Exec(ctx,
		`INSERT INTO "user" (id, name, email, image, "siteRole", "vrchatUsername", slug, "createdAt", "updatedAt")
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		"discord-user-2", "CollisionUser", "discord-user-2@discord.invalid", "",
		"USER", "", "collisionuser2", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	slug2, err := GenerateUniqueUserSlug(ctx, stdlibDB, "CollisionUser", "discord-user-3")
	if err != nil {
		t.Fatalf("GenerateUniqueUserSlug failed: %v", err)
	}
	if slug2 != "collisionuser3" {
		t.Errorf("GenerateUniqueUserSlug(\"CollisionUser\", \"discord-user-3\") with collisions = %q, want \"collisionuser3\"", slug2)
	}
	if len(slug2) > 24 {
		t.Errorf("slug %q exceeds 24 characters", slug2)
	}
}

// Test_generateUniqueOrgSlug validates unique organization slug generation.
//
// Mirrors ts-legacy/lib/slugs.test.ts → "generateUniqueOrgSlug"
func Test_generateUniqueOrgSlug(t *testing.T) {
	ctx := context.Background()

	stdlibDB := db.Stdlib()

	// Insert an organization (unique ids: the test database is shared).
	_, err := db.Exec(ctx,
		`INSERT INTO "organization" (id, name, slug, "createdAt", "updatedAt")
		 VALUES ($1, $2, $3, $4, $5)`,
		"sg-org-1", "Sg Team", "sg-team", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert org: %v", err)
	}

	// Same name should produce a different slug with suffix
	slug, err := GenerateUniqueOrgSlug(ctx, stdlibDB, "Sg Team")
	if err != nil {
		t.Fatalf("GenerateUniqueOrgSlug failed: %v", err)
	}
	if slug != "sg-team-2" {
		t.Errorf("GenerateUniqueOrgSlug(\"Sg Team\") with collision = %q, want \"sg-team-2\"", slug)
	}

	// Different name should produce the slugified name
	slug2, err := GenerateUniqueOrgSlug(ctx, stdlibDB, "Sg New Team")
	if err != nil {
		t.Fatalf("GenerateUniqueOrgSlug failed: %v", err)
	}
	if slug2 != "sg-new-team" {
		t.Errorf("GenerateUniqueOrgSlug(\"Sg New Team\") = %q, want \"sg-new-team\"", slug2)
	}
}

// Test_isReservedSlug validates reserved slug detection.
func Test_isReservedSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"admin", true},
		{"api", true},
		{"events", true},
		{"users", true},
		{"teams", true},
		{"settings", true},
		{"login", true},
		{"auth", true},
		{"profile", true},
		{"onboarding", true},
		{"admin-panel", true},
		{"dashboard", true},
		{"help", true},
		{"support", true},
		{"status", true},
		{"my-team", false},
		{"alice", false},
		{"hello", false},
	}
	for _, tt := range tests {
		got := isReservedSlug(tt.slug)
		if got != tt.want {
			t.Errorf("isReservedSlug(%q) = %v, want %v", tt.slug, got, tt.want)
		}
	}
}
