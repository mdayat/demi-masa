-- Modify "order" table
ALTER TABLE "order" ADD COLUMN "subscription_duration" integer NOT NULL;
-- Rename a column from "expires_at" to "expired_at"
ALTER TABLE "user" RENAME COLUMN "expires_at" TO "expired_at";
