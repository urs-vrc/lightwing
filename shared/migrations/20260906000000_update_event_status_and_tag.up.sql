-- Convert event.status column to TEXT so enum value additions do not conflict with single-transaction migration executions
ALTER TABLE "event" ALTER COLUMN "status" DROP DEFAULT;
ALTER TABLE "event" ALTER COLUMN "status" TYPE TEXT USING "status"::text;
ALTER TABLE "event" ALTER COLUMN "status" SET DEFAULT 'DRAFT';

-- Safely add tag and deletedAt columns to event table
ALTER TABLE "event" ADD COLUMN IF NOT EXISTS "tag" TEXT NOT NULL DEFAULT 'OFFICIAL';
ALTER TABLE "event" ADD COLUMN IF NOT EXISTS "deletedAt" TIMESTAMP(6);

-- Backfill existing events tag to OFFICIAL
UPDATE "event" SET "tag" = 'OFFICIAL' WHERE "tag" IS NULL OR "tag" = '';

-- Migrate existing lifecycle status values (UNOFFICIAL, OFFICIAL -> PENDING)
UPDATE "event" SET "status" = 'PENDING' WHERE "status" IN ('UNOFFICIAL', 'OFFICIAL');
