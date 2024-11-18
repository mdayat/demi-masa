-- Create "coupon" table
CREATE TABLE "coupon" (
  "code" character varying(255) NOT NULL,
  "quota" smallint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("code")
);
-- Modify "order" table
ALTER TABLE "order" ADD COLUMN "coupon_code" character varying(255) NULL, ADD
 CONSTRAINT "fk_coupon" FOREIGN KEY ("coupon_code") REFERENCES "coupon" ("code") ON UPDATE CASCADE ON DELETE CASCADE;
