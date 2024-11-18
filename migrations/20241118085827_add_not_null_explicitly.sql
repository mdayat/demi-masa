-- Modify "order" table
ALTER TABLE "order" ALTER COLUMN "payment_status" SET NOT NULL;
-- Modify "user" table
ALTER TABLE "user" ALTER COLUMN "phone_verified" SET NOT NULL, ALTER COLUMN "phone_verified" SET DEFAULT false, ALTER COLUMN "account_type" SET NOT NULL, ALTER COLUMN "created_at" SET NOT NULL;
