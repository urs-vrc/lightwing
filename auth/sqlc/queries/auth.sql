-- Member role lookup (used by getMemberRole in rbac.go).
-- name: GetMemberRole :one
SELECT role FROM "member" WHERE "organizationId" = $1 AND "userId" = $2;

-- Event admin check (used by getEventActor in rbac.go).
-- name: EventAdminExists :one
SELECT EXISTS(SELECT 1 FROM "event_admin" WHERE "eventId" = $1 AND "userId" = $2);

-- User lookup by slug (used by Slug collision checks in slugs.go).
-- name: GetUserBySlug :one
SELECT id FROM "user" WHERE slug = $1;

-- User slug collision check (used by UpdateUser).
-- name: UserSlugExists :one
SELECT id FROM "user" WHERE slug = $1;

-- Organization lookup by slug (used by slugs.go).
-- name: GetOrgBySlug :one
SELECT id FROM "organization" WHERE slug = $1;

-- Discord account lookup (used by slugify in slugs.go).
-- name: GetDiscordAccountID :one
SELECT "accountId" FROM "account" WHERE "userId" = $1 AND "providerId" = 'discord' ORDER BY "updatedAt" DESC LIMIT 1;

-- Session delete (used by compat.go Logout).
-- name: DeleteSessionByToken :exec
DELETE FROM "session" WHERE "token" = $1;

-- Verification delete (used by compat.go).
-- name: DeleteVerificationByIdentifier :exec
DELETE FROM "verification" WHERE "identifier" = $1;

-- Update user placeholder (used by UpdateUser). Dynamic SQL is used in Go for
-- the column-list; this query is only a placeholder to verify the
-- table shape. Keep the raw db.Exec in users.go for now.
-- name: UpdateUserPlaceholder :exec
UPDATE "user" SET name = $1 WHERE id = $2;

-- First user id (bootstrap admin check).
-- name: GetFirstUserID :one
SELECT id FROM "user" ORDER BY "createdAt" ASC LIMIT 1;

-- name: GetUserSiteRole :one
SELECT "siteRole"::text FROM "user" WHERE id = $1;

-- name: UpdateUserSiteRole :exec
UPDATE "user" SET "siteRole" = $1 WHERE id = $2;

-- OAuth state store (id and identifier carry the same value).
-- name: StoreOAuthState :exec
INSERT INTO "verification" ("id", "identifier", "value", "expiresAt", "createdAt", "updatedAt")
VALUES (sqlc.arg('identifier'), sqlc.arg('identifier'), sqlc.arg('state_value'), sqlc.arg('expires_at'), sqlc.arg('now'), sqlc.arg('now'));

-- name: GetOAuthStateValue :one
SELECT "value" FROM "verification" WHERE "identifier" = $1 AND "expiresAt" > $2;

-- name: GetSessionExpiry :one
SELECT "expiresAt" FROM "session" WHERE "token" = $1;

-- Actor session join (used by resolveActor).
-- name: GetActorSession :one
SELECT s."userId", s."activeOrganizationId", u."siteRole"::text, s."expiresAt"
FROM "session" s
JOIN "user" u ON u.id = s."userId"
WHERE s."token" = $1;

-- Event ownership (used by getEventActor).
-- name: GetEventOwnership :one
SELECT "ownerType"::text, "ownerUserId", "organizationId" FROM "event" WHERE id = $1;

-- User name/slug (used by ensureUserSlug).
-- name: GetUserNameSlug :one
SELECT name, slug FROM "user" WHERE id = $1;

-- name: UpdateUserSlug :exec
UPDATE "user" SET slug = $1 WHERE id = $2;

-- Full user row for profiles.
-- name: GetUserProfileRow :one
SELECT id, name, email, image, slug, biography, "careerOverview",
       "vrchatUsername", "classTier", "siteRole"::text, "createdAt", "updatedAt"
FROM "user" WHERE id = $1;

-- Team affiliations for a batch of users.
-- name: ListTeamAffiliations :many
SELECT m."userId", o.id, o.name, o.slug, m.role
FROM "member" m
JOIN "organization" o ON o.id = m."organizationId"
WHERE m."userId" = ANY($1::text[])
ORDER BY o."createdAt" ASC;

-- Discord callback login upsert (used by the OAuth callback tx).

-- name: GetDiscordUserID :one
SELECT "userId" FROM "account" WHERE "providerId" = 'discord' AND "accountId" = $1;

-- name: GetUserByID :one
SELECT id FROM "user" WHERE id = $1;

-- name: CountUsers :one
SELECT COUNT(*) FROM "user";

-- name: InsertUser :exec
INSERT INTO "user" (id, name, email, image, "siteRole", "vrchatUsername", slug, "createdAt", "updatedAt")
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdateUserOnLogin :exec
UPDATE "user" SET name = $1, email = $2, image = $3, "updatedAt" = $4
WHERE id = $5;

-- name: GetDiscordAccountRowID :one
SELECT id FROM "account" WHERE "userId" = $1 AND "providerId" = 'discord';

-- name: InsertDiscordAccount :exec
INSERT INTO "account" (id, "accountId", "providerId", "userId", "accessToken",
                        "refreshToken", "scope", "accessTokenExpiresAt", "createdAt", "updatedAt")
VALUES ($1, $2, 'discord', $3, $4, $5, $6, $7, $8, $8);

-- name: UpdateDiscordAccount :exec
UPDATE "account" SET "accessToken" = $1, "refreshToken" = $2, "scope" = $3,
                     "accessTokenExpiresAt" = $4, "updatedAt" = $5
WHERE id = $6;

-- name: GetMemberActiveOrg :one
SELECT "organizationId" FROM "member" WHERE "userId" = $1 ORDER BY "createdAt" ASC LIMIT 1;

-- name: DeleteSessionsByUser :exec
DELETE FROM "session" WHERE "userId" = $1;

-- name: InsertSession :exec
INSERT INTO "session" (id, "userId", token, "activeOrganizationId", "expiresAt", "createdAt", "updatedAt")
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- Debug login user insert (fixed SITE_ADMIN debug user).
-- name: InsertDebugUser :exec
INSERT INTO "user" (id, name, email, image, "siteRole", "vrchatUsername", slug, "createdAt", "updatedAt")
VALUES ($1, 'Debug User', 'debug-user@local.invalid', '', 'SITE_ADMIN', '', $2, $3, $3);

-- Profile update: nil leaves the column unchanged, except image-like
-- fields where non-nil (even empty, mapping to NULL) overwrites.
-- name: UpdateUserProfile :exec
UPDATE "user" SET
  "updatedAt" = sqlc.arg('updated_at'),
  name = COALESCE(sqlc.narg('name'), name),
  slug = COALESCE(sqlc.narg('slug'), slug),
  image = CASE WHEN sqlc.arg('image_set')::boolean THEN sqlc.narg('image_val') ELSE image END,
  biography = CASE WHEN sqlc.arg('biography_set')::boolean THEN sqlc.narg('biography_val') ELSE biography END,
  "careerOverview" = CASE WHEN sqlc.arg('career_set')::boolean THEN sqlc.narg('career_val') ELSE "careerOverview" END,
  "vrchatUsername" = CASE WHEN sqlc.arg('vrchat_set')::boolean THEN sqlc.narg('vrchat_val') ELSE "vrchatUsername" END
WHERE id = sqlc.arg('id');

-- User list count with optional ILIKE search (empty pattern matches all).
-- name: CountUsersBySearch :one
SELECT COUNT(*) FROM "user"
WHERE ($1::text = '' OR name ILIKE $1 ESCAPE '\' OR "vrchatUsername" ILIKE $1 ESCAPE '\' OR email ILIKE $1 ESCAPE '\' OR slug ILIKE $1 ESCAPE '\');

-- User list page with optional ILIKE search.
-- name: ListUserRows :many
SELECT id, name, email, image, slug, biography, "careerOverview",
       "vrchatUsername", "classTier", "siteRole"::text, "createdAt", "updatedAt"
FROM "user"
WHERE ($1::text = '' OR name ILIKE $1 ESCAPE '\' OR "vrchatUsername" ILIKE $1 ESCAPE '\' OR email ILIKE $1 ESCAPE '\' OR slug ILIKE $1 ESCAPE '\')
ORDER BY "createdAt" ASC
LIMIT NULLIF($2::int, 0) OFFSET $3::int;
