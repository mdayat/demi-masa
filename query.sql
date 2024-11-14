-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = $1;

-- name: CreateUser :exec
INSERT INTO "user" (id, name, email, role) VALUES ($1, $2, $3, $4);