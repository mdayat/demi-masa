-- Modify "subscription_plan" table
ALTER TABLE "subscription_plan" DROP COLUMN "duration_in_seconds", ADD COLUMN "duration_in_months" smallint NOT NULL, ADD CONSTRAINT "subscription_plan_name_duration_in_months_key" UNIQUE ("name", "duration_in_months");
