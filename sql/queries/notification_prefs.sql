-- name: GetNotificationPrefsByUser :one
SELECT id, user_id, email_enabled, sms_enabled, push_enabled,
       on_package_arrived, on_storage_expiry, on_ship_updates, on_inbound_updates,
       daily_digest, created_at, updated_at
FROM notification_prefs
WHERE user_id = ?;

-- name: CreateNotificationPrefs :one
INSERT INTO notification_prefs (
    id, user_id, email_enabled, sms_enabled, push_enabled,
    on_package_arrived, on_storage_expiry, on_ship_updates, on_inbound_updates,
    daily_digest, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, email_enabled, sms_enabled, push_enabled,
          on_package_arrived, on_storage_expiry, on_ship_updates, on_inbound_updates,
          daily_digest, created_at, updated_at;

-- name: UpdateNotificationPrefs :exec
UPDATE notification_prefs
SET email_enabled = ?, sms_enabled = ?, push_enabled = ?,
    on_package_arrived = ?, on_storage_expiry = ?, on_ship_updates = ?, on_inbound_updates = ?,
    daily_digest = ?, updated_at = ?
WHERE user_id = ?;
