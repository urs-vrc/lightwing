-- Race members for a race.
-- name: ListRaceEventMembers :many
SELECT m."userId", COALESCE(u."vrchatUsername", u.name), u."classTier"
FROM "race_event_member" m JOIN "user" u ON u.id = m."userId"
WHERE m."raceEventId" = $1;

-- Max sequence for a race event (used by CreateRaceEvent).
-- name: GetMaxRaceSequence :one
SELECT COALESCE(MAX(sequence), 0)::integer FROM "race_event" WHERE "eventId" = $1;

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

-- Race member exists check (used by RequireMembershipForResult).
-- name: RaceMemberExists :one
SELECT EXISTS(SELECT 1 FROM "race_event_member" WHERE "raceEventId" = $1 AND "userId" = $2);

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

-- Race event member insert (used by Add/JoinRaceEventMember).
-- name: InsertRaceEventMember :exec
INSERT INTO "race_event_member" (id, "raceEventId", "userId") VALUES ($1, $2, $3);

-- Event member insert without timestamp (used by JoinRaceEvent auto-join).
-- name: InsertEventMemberSimple :exec
INSERT INTO "event_member" (id, "eventId", "userId") VALUES ($1, $2, $3);

-- Delete race results by user and event (used by RemoveMemberFromEvent).
-- name: DeleteRaceResultsByEvent :exec
DELETE FROM "race_result" WHERE "userId"=$1
 AND "raceEventId" IN (SELECT id FROM "race_event" WHERE "eventId"=$2);

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

-- Full event row (replaces eventColumns scans).
-- name: GetEventRow :one
SELECT id, name, description, "ownerType", "organizationId", "ownerUserId", status, tag, "deletedAt", "scoringType", "scoringRulesMode", "customScoringTables", "classRestriction", "granularParticipation", "signupsLocked", "scheduledAt", "participantLimit", "maxConcurrentRaceParticipations", "createdAt", "updatedAt"
FROM "event" WHERE id = $1;

-- Full race row (replaces raceEventColumns scans).
-- name: GetRaceEventRow :one
SELECT id, "eventId", name, sequence, "distanceMeters", "trackType", location, "scoringType", grade, "classRestriction", "startsAt", "endsAt", "participantLimit", "createdAt", "updatedAt"
FROM "race_event" WHERE id = $1;

-- Full race rows of an event (used by ListRaceEventsCore).
-- name: ListRaceEventRows :many
SELECT id, "eventId", name, sequence, "distanceMeters", "trackType", location, "scoringType", grade, "classRestriction", "startsAt", "endsAt", "participantLimit", "createdAt", "updatedAt"
FROM "race_event" WHERE "eventId" = $1 ORDER BY sequence ASC;

-- Standings seed rows (used by EnsureEventStandingsRow).
-- name: InsertPointsStandingsRow :exec
INSERT INTO "event_points_entry" (id, "eventId", "userId", points, "createdAt", "updatedAt")
VALUES ($1, $2, $3, 0, $4, $4) ON CONFLICT ("eventId", "userId") DO NOTHING;

-- name: InsertLadderStandingsRow :exec
INSERT INTO "event_ladder_entry" (id, "eventId", "userId", elo, "createdAt", "updatedAt")
VALUES ($1, $2, $3, $4, $5, $5) ON CONFLICT ("eventId", "userId") DO NOTHING;

-- Organization exists check (used by CreateEvent).
-- name: OrgExists :one
SELECT EXISTS(SELECT 1 FROM "organization" WHERE id = $1);

-- Event create.
-- name: CreateEvent :exec
INSERT INTO "event" (id, name, description, "ownerType", "organizationId", "ownerUserId",
  status, tag, "scoringType", "scoringRulesMode", "customScoringTables", "classRestriction",
  "granularParticipation", "scheduledAt", "participantLimit", "maxConcurrentRaceParticipations",
  "createdAt", "updatedAt")
VALUES ($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16);

-- Purge long-deleted events.
-- name: PurgeExpiredDeletedEvents :exec
DELETE FROM "event" WHERE status = 'PENDING_DELETION' AND "deletedAt" <= $1;

-- Event list count with optional filters (empty means absent).
-- name: CountEvents :one
SELECT COUNT(*) FROM "event" e
WHERE ($1::text = '' OR e."organizationId" = $1)
  AND ($2::text = '' OR e."classRestriction"::text = $2)
  AND ($3::text = '' OR e.tag = $3)
  AND ($4::text = '' OR e.status = $4)
  AND ($4::text != '' OR $5::boolean OR e.status != 'PENDING_DELETION');

-- Event list page with optional filters (empty means absent).
-- name: ListEvents :many
SELECT e.id, e.name, e.description, e."ownerType", e."organizationId", e."ownerUserId",
e.status, e.tag, e."deletedAt", e."scoringType", e."classRestriction", e."granularParticipation",
e."signupsLocked", e."scheduledAt", e."participantLimit", e."maxConcurrentRaceParticipations",
e."createdAt", e."updatedAt",
(SELECT COUNT(*) FROM "race_event" r WHERE r."eventId" = e.id),
(SELECT COUNT(*) FROM "event_member" m WHERE m."eventId" = e.id)
FROM "event" e
WHERE ($1::text = '' OR e."organizationId" = $1)
  AND ($2::text = '' OR e."classRestriction"::text = $2)
  AND ($3::text = '' OR e.tag = $3)
  AND ($4::text = '' OR e.status = $4)
  AND ($4::text != '' OR $5::boolean OR e.status != 'PENDING_DELETION')
ORDER BY e."createdAt" DESC
LIMIT NULLIF($6::int, 0) OFFSET $7::int;

-- Public event list count.
-- name: CountPublicEvents :one
SELECT COUNT(*) FROM "event" e WHERE e.status IN ('PENDING','ONGOING','CONCLUDED');

-- Public event list page.
-- name: ListPublicEvents :many
SELECT e.id, e.name, e.description, e."ownerType", e."organizationId", e."ownerUserId",
e.status, e.tag, e."deletedAt", e."scoringType", e."classRestriction", e."granularParticipation",
e."signupsLocked", e."scheduledAt", e."participantLimit", e."maxConcurrentRaceParticipations",
e."createdAt", e."updatedAt",
(SELECT COUNT(*) FROM "race_event" r WHERE r."eventId" = e.id),
(SELECT COUNT(*) FROM "event_member" m WHERE m."eventId" = e.id)
FROM "event" e
WHERE e.status IN ('PENDING','ONGOING','CONCLUDED')
ORDER BY e."createdAt" DESC LIMIT $1::int OFFSET $2::int;

-- Event administrators with display names.
-- name: ListEventAdmins :many
SELECT a."userId", COALESCE(u."vrchatUsername", u.name)
FROM "event_admin" a JOIN "user" u ON u.id = a."userId"
WHERE a."eventId" = $1;

-- Event admin add (idempotent) / remove.
-- name: InsertEventAdmin :exec
INSERT INTO "event_admin" (id, "eventId", "userId", "createdAt")
VALUES ($1, $2, $3, $4) ON CONFLICT ("eventId", "userId") DO NOTHING;

-- name: DeleteEventAdmin :exec
DELETE FROM "event_admin" WHERE "eventId" = $1 AND "userId" = $2;

-- Event status (used by DeleteEvent).
-- name: GetEventStatus :one
SELECT status FROM "event" WHERE id = $1;

-- Event deletion variants.
-- name: DeleteEventByID :exec
DELETE FROM "event" WHERE id = $1;

-- name: SoftDeleteEvent :exec
UPDATE "event" SET status = 'PENDING_DELETION', "deletedAt" = $1 WHERE id = $2;

-- name: RestoreEventStatus :exec
UPDATE "event" SET status = 'PENDING', "deletedAt" = NULL WHERE id = $1;

-- Races of an event for LoadEvent.
-- name: ListEventRaces :many
SELECT id, name, sequence, "distanceMeters", "trackType", location, "scoringType", grade, "classRestriction", "startsAt", "endsAt", "participantLimit"
FROM "race_event" WHERE "eventId" = $1 ORDER BY sequence ASC;

-- Race members of an event with display names (used by LoadEvent).
-- name: ListRaceMembersByEvent :many
SELECT m."raceEventId", m."userId", COALESCE(u."vrchatUsername", u.name), u."classTier"
FROM "race_event_member" m
JOIN "race_event" r ON r.id = m."raceEventId"
JOIN "user" u ON u.id = m."userId"
WHERE r."eventId" = $1;

-- Event members with display names (used by LoadEvent).
-- name: ListEventMembersByEvent :many
SELECT m."userId", COALESCE(u."vrchatUsername", u.name), u."classTier"
FROM "event_member" m JOIN "user" u ON u.id = m."userId"
WHERE m."eventId" = $1;

-- Schedules of an event (used by LoadEvent).
-- name: ListEventSchedules :many
SELECT id, title, "startsAt", "endsAt", location FROM "event_schedule"
WHERE "eventId" = $1 ORDER BY "startsAt" ASC;

-- Scoring overviews (used by LoadEvent).
-- name: ListPointsOverview :many
SELECT e."userId", COALESCE(u."vrchatUsername", u.name), e.points
FROM "event_points_entry" e JOIN "user" u ON u.id = e."userId"
WHERE e."eventId" = $1 ORDER BY e.points DESC;

-- name: ListLadderOverview :many
SELECT e."userId", COALESCE(u."vrchatUsername", u.name), e.elo, e.wins, e.losses
FROM "event_ladder_entry" e JOIN "user" u ON u.id = e."userId"
WHERE e."eventId" = $1 ORDER BY e.elo DESC;

-- Granularity flag (used by RequireMembershipForResult, ApplyAutoDeferrals).
-- name: GetEventGranularity :one
SELECT "granularParticipation" FROM "event" WHERE id = $1;

-- Event scoring rules (used by eventScoringRules, ApplyAutoDeferrals).
-- name: GetEventScoringRules :one
SELECT "scoringType", "scoringRulesMode", "customScoringTables" FROM "event" WHERE id = $1;

-- Race result list for one race.
-- name: ListRaceResults :many
SELECT id, "raceEventId", "userId", position, points, "gateNumber", "finishTime",
       margin, "passingOrder", "final3F", "resultStatus", "createdAt", "updatedAt"
FROM "race_result" WHERE "raceEventId" = $1
ORDER BY position ASC NULLS LAST, points DESC;

-- Race result upsert with RETURNING (COALESCE keeps omitted fields).
-- name: UpsertRaceResult :one
INSERT INTO "race_result" (id, "raceEventId", "userId", position, points,
  "gateNumber", "finishTime", margin, "passingOrder", "final3F", "resultStatus",
  "createdAt", "updatedAt")
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
ON CONFLICT ("raceEventId", "userId") DO UPDATE SET
  position = COALESCE($4, "race_result".position),
  points = $5,
  "gateNumber" = COALESCE($6, "race_result"."gateNumber"),
  "finishTime" = COALESCE($7, "race_result"."finishTime"),
  margin = COALESCE($8, "race_result".margin),
  "passingOrder" = COALESCE($9, "race_result"."passingOrder"),
  "final3F" = COALESCE($10, "race_result"."final3F"),
  "resultStatus" = COALESCE($11, "race_result"."resultStatus"),
  "updatedAt" = $12
RETURNING id, "raceEventId", "userId", position, points, "gateNumber", "finishTime",
       margin, "passingOrder", "final3F", "resultStatus", "createdAt", "updatedAt";

-- Explicit-null clears (COALESCE above cannot clear).
-- name: ClearRaceResultPosition :exec
UPDATE "race_result" SET position = NULL WHERE id = $1;

-- name: ClearRaceResultStatus :exec
UPDATE "race_result" SET "resultStatus" = NULL WHERE id = $1;

-- name: ClearRaceResultPositionAndStatus :exec
UPDATE "race_result" SET position = NULL, "resultStatus" = NULL WHERE id = $1;

-- Delete with affected count (used by DeleteRaceResult).
-- name: DeleteRaceResult :execrows
DELETE FROM "race_result" WHERE "raceEventId" = $1 AND "userId" = $2;

-- Auto-deferral source rows.
-- name: ListAutoDeferralInputs :many
SELECT res.id, res."raceEventId", r.grade, res."userId", res.position,
       res.points, res."resultStatus", u."classTier"
FROM "race_result" res
JOIN "race_event" r ON r.id = res."raceEventId"
JOIN "user" u ON u.id = res."userId"
WHERE r."eventId" = $1;

-- Event member ids (used by ApplyAutoDeferrals).
-- name: ListEventMemberIDs :many
SELECT "userId" FROM "event_member" WHERE "eventId" = $1;

-- Race member pairs for an event (used by ApplyAutoDeferrals).
-- name: ListRaceMemberPairs :many
SELECT m."raceEventId", m."userId" FROM "race_event_member" m
JOIN "race_event" r ON r.id = m."raceEventId" WHERE r."eventId" = $1;

-- Deferred result insert.
-- name: InsertDeferredResult :exec
INSERT INTO "race_result" (id, "raceEventId", "userId", points, "resultStatus", "createdAt", "updatedAt")
VALUES ($1, $2, $3, 0, 'DEFERRED', $4, $4);

-- Deferred result mark.
-- name: MarkResultDeferred :exec
UPDATE "race_result" SET "resultStatus" = 'DEFERRED', points = 0 WHERE id = $1;

-- Deferred result restore.
-- name: RestoreDeferredResult :exec
UPDATE "race_result" SET "resultStatus" = NULL, points = $1 WHERE id = $2;

-- Stored result rows for points recomputation.
-- name: ListStoredResultRows :many
SELECT res.id, res."userId", res.position, res.points, res."resultStatus", r.grade
FROM "race_result" res JOIN "race_event" r ON r.id = res."raceEventId"
WHERE r."eventId" = $1;

-- Race result points update (used by recomputeEventPoints).
-- name: UpdateRaceResultPoints :exec
UPDATE "race_result" SET points = $1 WHERE id = $2;

-- Max per-user race enrollment (used by UpdateEvent capacity checks).
-- name: GetMaxRaceEnrollment :one
SELECT COALESCE(MAX(c), 0)::integer FROM (
   SELECT COUNT(*) AS c FROM "race_event_member" m
   JOIN "race_event" r ON r.id = m."raceEventId"
   WHERE r."eventId" = $1 GROUP BY m."userId"
 ) t;

-- Event update: tri-state columns use (clear, set, value) so explicit-null
-- clearing survives without dynamic SQL. Plain optional columns use COALESCE.
-- name: UpdateEvent :exec
UPDATE "event" SET
  "updatedAt" = sqlc.arg('updated_at'),
  name = COALESCE(sqlc.narg('name'), name),
  tag = COALESCE(sqlc.narg('tag'), tag),
  description = CASE WHEN sqlc.arg('desc_set')::boolean THEN sqlc.narg('desc_val') ELSE description END,
  "scoringRulesMode" = COALESCE(sqlc.narg('mode'), "scoringRulesMode"),
  "customScoringTables" = CASE WHEN sqlc.arg('ct_clear')::boolean THEN NULL WHEN sqlc.arg('ct_set')::boolean THEN sqlc.arg('ct_val') ELSE "customScoringTables" END,
  "classRestriction" = CASE WHEN sqlc.arg('class_set')::boolean THEN sqlc.narg('class_val') ELSE "classRestriction" END,
  "scheduledAt" = CASE WHEN sqlc.arg('sched_set')::boolean THEN sqlc.narg('sched_val') ELSE "scheduledAt" END,
  "participantLimit" = CASE WHEN sqlc.arg('pl_clear')::boolean THEN NULL ELSE COALESCE(sqlc.narg('pl_val'), "participantLimit") END,
  "maxConcurrentRaceParticipations" = CASE WHEN sqlc.arg('mc_clear')::boolean THEN NULL ELSE COALESCE(sqlc.narg('mc_val'), "maxConcurrentRaceParticipations") END
WHERE id = sqlc.arg('id');

-- Race ids for an event.
-- name: ListRaceIDs :many
SELECT id FROM "race_event" WHERE "eventId" = $1;

-- Race id + sequence list (used by ReorderRaceEvents).
-- name: ListRaceIDSequences :many
SELECT id, sequence FROM "race_event" WHERE "eventId" = $1 ORDER BY sequence ASC;

-- Result holder ids for a race (used by removedResultUsers).
-- name: ListResultUserIDs :many
SELECT "userId" FROM "race_result" WHERE "raceEventId" = $1;

-- Bulk delete results absent from the payload.
-- name: DeleteAbsentRaceResults :exec
DELETE FROM "race_result" WHERE "raceEventId" = $1 AND "userId" = ANY($2::text[]);

-- Points upsert (used by SetEventPoints).
-- name: UpsertEventPointsEntry :exec
INSERT INTO "event_points_entry" (id, "eventId", "userId", points, "createdAt", "updatedAt")
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT ("eventId", "userId") DO UPDATE SET points = $4, "updatedAt" = $5;

-- Ladder elo lookup (used by getOrCreateLadderElo).
-- name: GetLadderElo :one
SELECT elo FROM "event_ladder_entry" WHERE "eventId" = $1 AND "userId" = $2;

-- Ladder entry insert (1200 default, idempotent).
-- name: InsertLadderEntry :exec
INSERT INTO "event_ladder_entry" (id, "eventId", "userId", elo, "createdAt")
VALUES ($1, $2, $3, 1200, $4)
ON CONFLICT ("eventId", "userId") DO NOTHING;

-- Ladder match result updates.
-- name: RecordLadderWin :exec
UPDATE "event_ladder_entry" SET elo = $1, wins = wins + 1 WHERE "eventId" = $2 AND "userId" = $3;

-- name: RecordLadderLoss :exec
UPDATE "event_ladder_entry" SET elo = $1, losses = losses + 1 WHERE "eventId" = $2 AND "userId" = $3;

-- Event tag update.
-- name: UpdateEventTag :exec
UPDATE "event" SET tag = $1 WHERE id = $2;

-- Event status updates (with/without soft-deletion timestamp).
-- name: UpdateEventStatusWithDeletion :exec
UPDATE "event" SET status = $1, "deletedAt" = $2 WHERE id = $3;

-- name: UpdateEventStatus :exec
UPDATE "event" SET status = $1, "deletedAt" = NULL WHERE id = $2;

-- User exists check (used by SetUserClass).
-- name: UserExists :one
SELECT EXISTS(SELECT 1 FROM "user" WHERE id = $1);

-- User class tier assignment.
-- name: UpdateUserClassTier :exec
UPDATE "user" SET "classTier" = $1 WHERE id = $2;

-- Eligible events listing (used by ListEligibleEvents).
-- name: ListEligibleEvents :many
SELECT id, name, "organizationId", "classRestriction" FROM "event" ORDER BY "createdAt" DESC;

-- Eligible races for one event (used by ListEligibleEvents).
-- name: ListEligibleRaces :many
SELECT id, name, sequence, grade, "classRestriction" FROM "race_event" WHERE "eventId" = $1 ORDER BY sequence ASC;

-- Datasets scoped by event.
-- name: ListDatasetsByEvent :many
SELECT id, "eventId", source, rows, status, "importedAt", "createdAt", "updatedAt"
FROM "dataset" WHERE "eventId" = $1 ORDER BY "createdAt" DESC;

-- Dataset create with RETURNING.
-- name: CreateDataset :one
INSERT INTO "dataset" (id, "eventId", source, rows, status, "importedAt", "createdAt", "updatedAt")
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING id, "eventId", source, rows, status, "importedAt", "createdAt", "updatedAt";

-- Dataset fetch.
-- name: GetDatasetByID :one
SELECT id, "eventId", source, rows, status, "importedAt", "createdAt", "updatedAt"
FROM "dataset" WHERE id = $1 AND "eventId" = $2;

-- Dataset status update with RETURNING.
-- name: UpdateDatasetStatus :one
UPDATE "dataset" SET status = $1, "importedAt" = $2 WHERE id = $3
RETURNING id, "eventId", source, rows, status, "importedAt", "createdAt", "updatedAt";
