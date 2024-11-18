-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = $1;

-- name: GetUserByPhoneNumber :one
SELECT * FROM "user" WHERE phone_number = $1;

-- name: UpdateUserPhoneNumber :exec
UPDATE "user" SET phone_number = $2, phone_verified = $3 WHERE id = $1;

-- name: CreateUser :exec
INSERT INTO "user" (id, name, email) VALUES ($1, $2, $3);