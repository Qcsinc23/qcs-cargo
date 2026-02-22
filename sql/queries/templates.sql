-- name: ListTemplatesByUser :many
SELECT id, user_id, name, service_type, destination_id, recipient_id, use_count, created_at
FROM templates
WHERE user_id = ?
ORDER BY name;

-- name: GetTemplateByID :one
SELECT id, user_id, name, service_type, destination_id, recipient_id, use_count, created_at
FROM templates
WHERE id = ? AND user_id = ?;

-- name: DeleteTemplate :exec
DELETE FROM templates WHERE id = ? AND user_id = ?;
