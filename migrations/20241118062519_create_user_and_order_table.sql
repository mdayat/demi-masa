-- Create enum type "account_type"
CREATE TYPE "account_type" AS ENUM ('free', 'premium');
-- Create enum type "payment_status"
CREATE TYPE "payment_status" AS ENUM ('unpaid', 'success', 'failed');
-- Create "user" table
CREATE TABLE "user" (
  "id" character varying(255) NOT NULL,
  "name" character varying(255) NOT NULL,
  "email" character varying(255) NOT NULL,
  "phone_number" character varying(255) NULL,
  "phone_verified" boolean NULL,
  "account_type" "account_type" NULL DEFAULT 'free',
  "upgraded_at" timestamptz NULL,
  "expires_at" timestamptz NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_email_key" UNIQUE ("email"),
  CONSTRAINT "user_phone_number_key" UNIQUE ("phone_number")
);
-- Create "order" table
CREATE TABLE "order" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "user_id" character varying(255) NOT NULL,
  "transaction_id" character varying(255) NOT NULL,
  "amount" integer NOT NULL,
  "payment_method" character varying(255) NOT NULL,
  "payment_status" "payment_status" NULL DEFAULT 'unpaid',
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_user" FOREIGN KEY ("user_id") REFERENCES "user" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
