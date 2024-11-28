-- Create enum type "account_type"
CREATE TYPE "account_type" AS ENUM ('FREE', 'PREMIUM');
-- Create enum type "transaction_status"
CREATE TYPE "transaction_status" AS ENUM ('UNPAID', 'PAID', 'FAILED', 'EXPIRED', 'REFUND');
-- Create "coupon" table
CREATE TABLE "coupon" (
  "code" character varying(255) NOT NULL,
  "influencer_username" character varying(255) NOT NULL,
  "quota" smallint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("code")
);
-- Create "subscription_plan" table
CREATE TABLE "subscription_plan" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" character varying(255) NOT NULL,
  "price" integer NOT NULL,
  "duration_in_seconds" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "subscription_plan_name_duration_in_seconds_key" UNIQUE ("name", "duration_in_seconds")
);
-- Create "user" table
CREATE TABLE "user" (
  "id" character varying(255) NOT NULL,
  "name" character varying(255) NOT NULL,
  "email" character varying(255) NOT NULL,
  "phone_number" character varying(255) NULL,
  "phone_verified" boolean NOT NULL DEFAULT false,
  "account_type" "account_type" NOT NULL DEFAULT 'FREE',
  "upgraded_at" timestamptz NULL,
  "expired_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_email_key" UNIQUE ("email"),
  CONSTRAINT "user_phone_number_key" UNIQUE ("phone_number")
);
-- Create "transaction" table
CREATE TABLE "transaction" (
  "id" uuid NOT NULL,
  "user_id" character varying(255) NOT NULL,
  "subscription_plan_id" uuid NOT NULL,
  "ref_id" character varying(255) NOT NULL,
  "coupon_code" character varying(255) NULL,
  "payment_method" character varying(255) NOT NULL,
  "qr_url" character varying(255) NOT NULL,
  "status" "transaction_status" NOT NULL DEFAULT 'UNPAID',
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "paid_at" timestamptz NULL,
  "expired_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_coupon" FOREIGN KEY ("coupon_code") REFERENCES "coupon" ("code") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "fk_subscription_plan" FOREIGN KEY ("subscription_plan_id") REFERENCES "subscription_plan" ("id") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "fk_user" FOREIGN KEY ("user_id") REFERENCES "user" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
