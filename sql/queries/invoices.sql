-- name: ListInvoicesByUser :many
SELECT id, user_id, booking_id, ship_request_id, invoice_number, subtotal, tax, total,
       status, due_date, paid_at, notes, created_at
FROM invoices
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: GetInvoiceByID :one
SELECT id, user_id, booking_id, ship_request_id, invoice_number, subtotal, tax, total,
       status, due_date, paid_at, notes, created_at
FROM invoices
WHERE id = ? AND user_id = ?;

-- name: ListInvoiceItemsByInvoiceID :many
SELECT id, invoice_id, description, quantity, unit_price, total
FROM invoice_items
WHERE invoice_id = ?
ORDER BY id;
