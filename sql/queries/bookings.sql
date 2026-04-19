-- name: ListBookingsByUser :many
-- Pass 3 HIGH-07: real SQL LIMIT/OFFSET pagination so the handler can
-- bound the result set in the database instead of fetching the full
-- user history and slicing the result in Go.
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE user_id = ?
ORDER BY scheduled_date DESC, created_at DESC
LIMIT ? OFFSET ?;

-- name: CountBookingsByUser :one
SELECT COUNT(*) FROM bookings WHERE user_id = ?;

-- name: GetBookingByID :one
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE id = ? AND user_id = ?;

-- name: CreateBooking :one
INSERT INTO bookings (
    id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
    scheduled_date, time_slot, special_instructions,
    weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
    subtotal, discount, insurance, total, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
          scheduled_date, time_slot, special_instructions,
          weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
          subtotal, discount, insurance, total,
          payment_status, stripe_payment_intent_id, created_at, updated_at;

-- name: UpdateBooking :one
UPDATE bookings
SET status = ?, special_instructions = ?,
    weight_lbs = ?, length_in = ?, width_in = ?, height_in = ?, value_usd = ?, add_insurance = ?,
    subtotal = ?, discount = ?, insurance = ?, total = ?, payment_status = ?, updated_at = ?
WHERE id = ? AND user_id = ?
RETURNING id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
          scheduled_date, time_slot, special_instructions,
          weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
          subtotal, discount, insurance, total,
          payment_status, stripe_payment_intent_id, created_at, updated_at;

-- name: DeleteBooking :exec
DELETE FROM bookings WHERE id = ? AND user_id = ?;

-- name: AdminListBookings :many
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
ORDER BY scheduled_date DESC, created_at DESC
LIMIT ? OFFSET ?;

-- name: ListBookingsToday :many
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE scheduled_date = date('now')
ORDER BY time_slot ASC, created_at ASC;

-- name: GetBookingByIDOnly :one
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE id = ?;

-- name: AdminListBookingsToday :many
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE scheduled_date = ?
ORDER BY time_slot, created_at
LIMIT 100;
-- name: UpdateBookingStatus :exec
UPDATE bookings SET status = ?, updated_at = ? WHERE id = ?;

-- name: AdminUpdateBookingStatus :exec
-- DEF (Pass 2.5 CRIT-01 / HIGH-01): admin-only lifecycle transition for
-- bookings. Customer-facing bookingUpdate cannot set status beyond
-- pending/cancelled or change payment_status at all. Mirrors the
-- UpdateShipRequestPaymentReconcileForAdmin pattern (id-only WHERE,
-- gated upstream by RequireAdmin in admin.go).
UPDATE bookings
SET status = ?, payment_status = ?, updated_at = ?
WHERE id = ?;

-- name: GetBookingByIDForAdmin :one
-- DEF (Pass 2.5 CRIT-01 / HIGH-01): admin-only read used to load a booking
-- without scoping by user_id, mirroring GetShipRequestByIDForAdmin. Column
-- list intentionally matches GetBookingByID exactly so handler code can use
-- the same gen.Booking row shape.
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE id = ?;
