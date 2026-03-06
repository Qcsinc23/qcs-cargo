-- name: ListShipRequestsByUser :many
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at
FROM ship_requests
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: GetShipRequestByID :one
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at
FROM ship_requests
WHERE id = ? AND user_id = ?;

-- name: GetShipRequestByStripePaymentIntentID :one
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at
FROM ship_requests
WHERE stripe_payment_intent_id = ?;

-- name: CreateShipRequest :one
INSERT INTO ship_requests (
    id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
    consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
    created_at, updated_at
) VALUES (
    ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
          consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
          payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at;

-- name: ListShipRequestItemsByShipRequestID :many
SELECT id, ship_request_id, locker_package_id, customs_description, customs_value,
       customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
FROM ship_request_items
WHERE ship_request_id = ?
ORDER BY id;

-- name: CreateShipRequestItem :one
INSERT INTO ship_request_items (
    id, ship_request_id, locker_package_id, customs_description, customs_value,
    customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, ship_request_id, locker_package_id, customs_description, customs_value,
          customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs;

-- name: UpdateShipRequestItemCustoms :exec
UPDATE ship_request_items
SET customs_description = ?, customs_value = ?, customs_quantity = ?,
    customs_hs_code = ?, customs_country_of_origin = ?, customs_weight_lbs = ?
WHERE id = ? AND ship_request_id = ?;

-- name: UpdateShipRequestCustomsStatus :exec
UPDATE ship_requests
SET customs_status = ?, status = ?, updated_at = ?
WHERE id = ? AND user_id = ?;

-- name: UpdateShipRequestPricing :exec
UPDATE ship_requests
SET subtotal = ?, service_fees = ?, insurance = ?, discount = ?, total = ?, updated_at = ?
WHERE id = ? AND user_id = ?;

-- name: UpdateShipRequestPaymentIntent :exec
UPDATE ship_requests
SET stripe_payment_intent_id = ?, updated_at = ?
WHERE id = ? AND user_id = ?;

-- name: UpdateShipRequestPaymentReconcile :exec
UPDATE ship_requests
SET payment_status = ?, status = ?, updated_at = ?
WHERE id = ? AND user_id = ?;

-- name: AdminListShipRequests :many
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at
FROM ship_requests
WHERE (? = '' OR status = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetShipRequestByIDOnly :one
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, consolidated_weight_lbs, staging_bay, manifest_id,
       created_at, updated_at
FROM ship_requests
WHERE id = ?;

-- name: AdminUpdateShipRequestStatus :exec
UPDATE ship_requests SET status = ?, updated_at = ? WHERE id = ?;

-- name: UpdateShipRequestConsolidatedWeight :exec
UPDATE ship_requests SET consolidated_weight_lbs = ?, updated_at = ? WHERE id = ?;

-- name: UpdateShipRequestStaged :exec
UPDATE ship_requests SET staging_bay = ?, manifest_id = ?, status = ?, updated_at = ? WHERE id = ?;

-- name: ListPaidShipRequestsByPaymentStatus :many
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, consolidated_weight_lbs, staging_bay, manifest_id,
       created_at, updated_at
FROM ship_requests
WHERE payment_status = 'paid'
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
-- name: GetActiveShipRequestCountByPackageID :one
SELECT COUNT(*) FROM ship_request_items sri
JOIN ship_requests sr ON sri.ship_request_id = sr.id
WHERE sri.locker_package_id = ?
  AND sr.status NOT IN ('delivered', 'cancelled', 'expired');
