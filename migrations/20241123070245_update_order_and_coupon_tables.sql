-- Add value to enum type: "payment_status"
ALTER TYPE "payment_status" ADD VALUE 'paid' AFTER 'unpaid';
-- Modify "coupon" table
ALTER TABLE "coupon" ADD COLUMN "influencer_username" character varying(255) NOT NULL, ADD COLUMN "deleted_at" timestamptz NULL;
-- Modify "order" table
ALTER TABLE "order" ADD COLUMN "payment_url" character varying(255) NOT NULL, ADD COLUMN "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, ADD COLUMN "paid_at" timestamptz NULL;
