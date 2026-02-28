-- name: ListBookingsByUser :many
SELECT id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
       scheduled_date, time_slot, special_instructions,
       weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
       subtotal, discount, insurance, total,
       payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings
WHERE user_id = ?
ORDER BY scheduled_date DESC, created_at DESC;

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
WHERE date(scheduled_date) = date('now')
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
