-- name: GetUsersByTimeZone :many
SELECT
  u.id,
  u.phone_number,
  u.account_type,
  u.time_zone
FROM "user" u WHERE u.time_zone = $1;

-- name: GetUserPrayerByID :one
SELECT
  u.phone_number,
  u.account_type,
  u.time_zone
FROM "user" u WHERE u.id = $1;

-- name: GetUserPhoneByID :one
SELECT u.phone_number FROM "user" u WHERE u.id = $1;

-- name: UpdateUserSubs :exec
UPDATE "user" SET account_type = $2 WHERE id = $1;

-- name: RemoveCheckedTask :exec
DELETE FROM task WHERE checked = TRUE;

-- name: UpdatePrayersToMissed :exec
UPDATE prayer SET status = 'MISSED' WHERE status IS NULL AND (day < $1 OR month < $2 OR year < $3);
