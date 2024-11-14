-- Create enum type "user_role"
CREATE TYPE "user_role" AS ENUM ('user', 'influencer');
-- Create "user" table
CREATE TABLE "user" (
  "id" character varying(255) NOT NULL,
  "name" character varying(255) NOT NULL,
  "email" character varying(255) NOT NULL,
  "phone_number" character varying(255) NULL,
  "role" "user_role" NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "user_email_key" UNIQUE ("email"),
  CONSTRAINT "user_phone_number_key" UNIQUE ("phone_number")
);
