-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = $1;

-- name: GetUserByPhoneNumber :one
SELECT * FROM "user" WHERE phone_number = $1;

-- name: UpdateUserPhoneNumber :exec
UPDATE "user" SET phone_number = $2, phone_verified = $3 WHERE id = $1;

-- name: UpdateUserSubscription :exec
UPDATE "user" SET account_type = $2, upgraded_at = $3, expired_at = $4
WHERE id = $1;

-- name: CreateUser :exec
INSERT INTO "user" (id, name, email) VALUES ($1, $2, $3);

-- name: DecrementCouponQuota :one
UPDATE coupon SET quota = quota - 1
WHERE code = $1 AND quota > 0 AND deleted_at IS NULL RETURNING quota;

-- name: IncrementCouponQuota :exec
UPDATE coupon SET quota = quota + 1 WHERE code = $1;

-- name: GetOrderByIDWithUser :one
SELECT 
  o.id AS order_id,
  o.payment_status,
  o.subscription_duration,
  u.id AS user_id,
  u.account_type,
  u.upgraded_at,
  u.expired_at
FROM "order" o JOIN "user" u ON o.user_id = u.id WHERE o.id = $1;

-- name: CreateOrder :exec
INSERT INTO "order" (
  id,
  user_id,
  transaction_id,
  coupon_code,
  amount,
  subscription_duration,
  payment_method,
  payment_url,
  expired_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdateOrderStatus :exec
UPDATE "order" SET payment_status = $2, paid_at = $3 WHERE id = $1;