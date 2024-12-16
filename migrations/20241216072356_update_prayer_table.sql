-- Modify "prayer" table
ALTER TABLE "prayer" DROP COLUMN "time", DROP COLUMN "time_zone", ADD CONSTRAINT "unique_prayer" UNIQUE ("name", "year", "month", "day");
