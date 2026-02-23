-- name: ListRecipientsByUser :many
SELECT id, user_id, name, phone, destination_id, street, apt, city, delivery_instructions,
       is_default, use_count, created_at, updated_at
FROM recipients
WHERE user_id = ?
ORDER BY is_default DESC, name ASC;

-- name: GetRecipientByID :one
SELECT id, user_id, name, phone, destination_id, street, apt, city, delivery_instructions,
       is_default, use_count, created_at, updated_at
FROM recipients
WHERE id = ? AND user_id = ?;

-- name: CreateRecipient :one
INSERT INTO recipients (id, user_id, name, phone, destination_id, street, apt, city, delivery_instructions, is_default, use_count, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateRecipient :one
UPDATE recipients
SET name = ?, phone = ?, destination_id = ?, street = ?, apt = ?, city = ?, delivery_instructions = ?, is_default = ?, updated_at = ?
WHERE id = ? AND user_id = ?
RETURNING id;

-- name: DeleteRecipient :exec
DELETE FROM recipients
WHERE id = ? AND user_id = ?;
-- name: UnsetDefaultRecipients :exec
UPDATE recipients SET is_default = 0 WHERE user_id = ?;
