-- Create enum type "prayer_status"
CREATE TYPE "prayer_status" AS ENUM ('ON_TIME', 'LATE', 'MISSED');
-- Create "ibadah_list" table
CREATE TABLE "ibadah_list" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "user_id" character varying(255) NOT NULL,
  "name" character varying(255) NOT NULL,
  "description" text NOT NULL,
  "checked" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_user_ibadah_list" FOREIGN KEY ("user_id") REFERENCES "user" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
-- Create "prayer" table
CREATE TABLE "prayer" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "user_id" character varying(255) NOT NULL,
  "name" character varying(255) NOT NULL,
  "time" bigint NOT NULL,
  "time_zone" "indonesia_time_zone" NOT NULL,
  "status" "prayer_status" NOT NULL DEFAULT 'MISSED',
  "year" smallint NOT NULL,
  "month" smallint NOT NULL,
  "day" smallint NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_user_prayer" FOREIGN KEY ("user_id") REFERENCES "user" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
-- Modify "transaction" table
ALTER TABLE "transaction" DROP CONSTRAINT "fk_user", ADD
 CONSTRAINT "fk_user_transaction" FOREIGN KEY ("user_id") REFERENCES "user" ("id") ON UPDATE CASCADE ON DELETE CASCADE;
