-- Modify "prayer" table
ALTER TABLE "prayer" DROP CONSTRAINT "unique_prayer", ADD CONSTRAINT "unique_prayer" UNIQUE ("user_id", "name", "year", "month", "day");
