-- Race members for a race.
-- name: ListRaceEventMembers :many
SELECT m."userId", COALESCE(u."vrchatUsername", u.name), u."classTier"
FROM "race_event_member" m JOIN "user" u ON u.id = m."userId"
WHERE m."raceEventId" = $1;

-- Max sequence for a race event (used by CreateRaceEvent).
-- name: GetMaxRaceSequence :one
SELECT MAX(sequence) FROM "race_event" WHERE "eventId" = $1;

-- Update race event sequence (used by ReorderRaceEvents).
-- name: UpdateRaceSequence :exec
UPDATE "race_event" SET sequence = $1 WHERE id = $2;

-- Create race event row.
-- name: InsertRaceEvent :exec
INSERT INTO "race_event" (id, "eventId", name, sequence, "distanceMeters", "trackType",
    location, "scoringType", grade, "classRestriction", "startsAt", "endsAt",
    "participantLimit", "createdAt", "updatedAt")
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- Update race event (used by UpdateRaceEvent).
-- name: UpdateRaceEvent :exec
UPDATE "race_event" SET name=$1, sequence=$2, "distanceMeters"=$3, "trackType"=$4,
    location=$5, "scoringType"=$6, grade=$7, "classRestriction"=$8, "startsAt"=$9,
    "endsAt"=$10, "participantLimit"=$11, "updatedAt"=$12 WHERE id=$13;

-- Delete race event.
-- name: DeleteRaceEvent :exec
DELETE FROM "race_event" WHERE id = $1;

-- Race member exists check (used by AddRaceEventMember).
-- name: RaceEventMemberExists :one
SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2);

-- Delete race event member (used by RemoveRaceEventMember, JoinRaceEvent).
-- name: DeleteRaceEventMember :exec
DELETE FROM "race_event_member" WHERE "raceEventId"=$1 AND "userId"=$2;

-- User class tier lookup (used by AddRaceEventMember, JoinRaceEvent).
-- name: GetUserClassTier :one
SELECT "classTier" FROM "user" WHERE id = $1;

-- Race event member count for limit checks.
-- name: GetRaceEventMemberCount :one
SELECT COUNT(*) FROM "race_event_member" WHERE "raceEventId" = $1;

-- Active race count for a user in an event (used by LeaveRaceEvent).
-- name: GetActiveRaceCountForUser :one
SELECT COUNT(*) FROM "race_event_member" m JOIN "race_event" r ON r.id = m."raceEventId"
WHERE m."userId"=$1 AND r."eventId"=$2;

-- Delete race event member by user and event (used by LeaveRaceEvent).
-- name: DeleteRaceEventMemberByEvent :exec
DELETE FROM "race_event_member" WHERE "userId"=$1 AND "raceEventId" IN
    (SELECT id FROM "race_event" WHERE "eventId"=$2);

-- Event member lookup.
-- name: EventMemberExists :one
SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2);

-- Create event member row.
-- name: InsertEventMember :exec
INSERT INTO "event_member" (id, "eventId", "userId", "createdAt")
VALUES ($1, $2, $3, $4) ON CONFLICT ("eventId", "userId") DO NOTHING;

-- Event member count (used by AddEventMember for participant limit).
-- name: GetEventMemberCount :one
SELECT COUNT(*) FROM "event_member" WHERE "eventId" = $1;

-- Event member for JoinEvent.
-- name: EventMemberExistsForJoin :one
SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2);

-- Delete all event members for a user (used by RemoveMemberFromEvent).
-- name: DeleteEventMembersByUser :exec
DELETE FROM "event_member" WHERE "eventId" = $1 AND "userId" = $2;

-- Delete event points entries for a user (used by RemoveMemberFromEvent).
-- name: DeleteEventPointsByUser :exec
DELETE FROM "event_points_entry" WHERE "eventId" = $1 AND "userId" = $2;

-- Delete event ladder entries for a user (used by RemoveMemberFromEvent).
-- name: DeleteEventLadderByUser :exec
DELETE FROM "event_ladder_entry" WHERE "eventId" = $1 AND "userId" = $2;

-- Event scheduling row lookup.
-- name: GetEventScheduleByID :one
SELECT id, "eventId", title, "startsAt", "endsAt", location, "createdAt"
FROM "event_schedule" WHERE id = $1;

-- Create event schedule row.
-- name: InsertEventSchedule :exec
INSERT INTO "event_schedule" (id, "eventId", title, "startsAt", "endsAt", location, "createdAt")
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- Signups lock update.
-- name: UpdateEventSignupsLocked :exec
UPDATE "event" SET "signupsLocked" = $1, "updatedAt" = $2 WHERE id = $3;

-- Event exists check.
-- name: EventExists :one
SELECT EXISTS(SELECT 1 FROM "event" WHERE id = $1);
