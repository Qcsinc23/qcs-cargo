-- name: ListAdminActivity :many
SELECT id, actor_id, action, entity_type, entity_id, details, created_at
FROM admin_activity
ORDER BY created_at DESC
LIMIT ?;

-- name: CreateAdminActivity :exec
INSERT INTO admin_activity (id, actor_id, action, entity_type, entity_id, details, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);
