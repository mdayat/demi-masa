-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = $1;

-- name: GetUserByPhoneNumber :one
SELECT * FROM "user" WHERE phone_number = $1;

-- name: UpdateUserPhoneNumber :exec
UPDATE "user" SET phone_number = $2, phone_verified = $3 WHERE id = $1;

-- name: UpdateUserSubs :exec
UPDATE "user" SET account_type = $2, upgraded_at = $3, expired_at = $4
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO "user" (id, name, email) VALUES ($1, $2, $3) RETURNING *;

-- name: DeleteUserByID :one
DELETE FROM "user" WHERE id = $1 RETURNING id;

-- name: GetSubsPlanByID :one
SELECT * FROM subscription_plan WHERE id = $1;

-- name: DecrementCouponQuota :one
UPDATE coupon SET quota = quota - 1
WHERE code = $1 AND quota > 0 AND deleted_at IS NULL RETURNING quota;

-- name: IncrementCouponQuota :exec
UPDATE coupon SET quota = quota + 1 WHERE code = $1;

-- name: GetTransactions :many
SELECT * FROM transaction;

-- name: GetTxByID :one
SELECT * FROM transaction WHERE id = $1;

-- name: GetTxWithSubsPlanByID :one
SELECT 
  t.id AS transaction_id,
  t.user_id,
  s.duration_in_seconds
FROM transaction t JOIN subscription_plan s ON t.subscription_plan_id = s.id WHERE t.id = $1;

-- name: CreateTx :exec
INSERT INTO transaction (id, user_id, subscription_plan_id, ref_id, coupon_code, payment_method, qr_url, expired_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateTxStatus :exec
UPDATE transaction SET status = $2, paid_at = $3 WHERE id = $1;