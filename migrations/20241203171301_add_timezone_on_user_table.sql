-- Create enum type "indonesia_time_zone"
CREATE TYPE "indonesia_time_zone" AS ENUM ('Asia/Jakarta', 'Asia/Makassar', 'Asia/Jayapura');
-- Modify "user" table
ALTER TABLE "user" ADD COLUMN "time_zone" "indonesia_time_zone" NULL;
