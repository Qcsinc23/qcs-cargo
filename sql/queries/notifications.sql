-- name: CreateSentNotification :exec
INSERT INTO sent_notifications (id, notification_type, resource_id, recipient_email, sent_at)
VALUES (?, ?, ?, ?, ?);

-- name: CheckSentNotification :one
SELECT COUNT(*) FROM sent_notifications
WHERE notification_type = ? AND resource_id = ? AND recipient_email = ?;
