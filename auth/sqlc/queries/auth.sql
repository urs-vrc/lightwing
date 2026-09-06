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
