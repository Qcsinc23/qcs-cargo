-- name: ListUnmatchedPackages :many
SELECT id, carrier, tracking_number, label_text, photo_url, weight_lbs, status,
       matched_user_id, resolution_notes, received_at, resolved_at, created_at
FROM unmatched_packages
ORDER BY received_at DESC
LIMIT ? OFFSET ?;

-- name: GetUnmatchedPackageByID :one
SELECT id, carrier, tracking_number, label_text, photo_url, weight_lbs, status,
       matched_user_id, resolution_notes, received_at, resolved_at, created_at
FROM unmatched_packages
WHERE id = ?;

-- name: UpdateUnmatchedPackageStatus :exec
UPDATE unmatched_packages SET status = ?, matched_user_id = ?, resolution_notes = ?, resolved_at = ? WHERE id = ?;
