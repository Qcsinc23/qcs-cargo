# Database Schema Reference

Last verified: 2026-04-19

## Scope and Sources

This reference describes the active SQLite schema using:

- migration files in `sql/migrations/`
- sqlc schema files in `sql/schema/`

Practical rule:

- Migrations are the runtime DDL source of truth (tables, indexes, constraints, data fixes).
- `sql/schema` is the sqlc modeling source (query generation contracts).

Conventions used across the schema:

- IDs are stored as `TEXT` (UUID-style values).
- Timestamps are stored as `TEXT` unless otherwise noted.
- Booleans are stored as `INTEGER` (`0`/`1`).

## Auth and Session Tables

### `users`

Purpose: identity, account profile, and account lifecycle state.

Key columns:

- `id` (PK)
- `email`, `name`, `role`, `status`
- `password_hash` (optional password auth)
- `suite_code` (customer forwarding suite)
- `email_verified`, `email_verification_token`, `email_verification_sent_at`
- `created_at`, `updated_at`

Indexes and constraints:

- `idx_users_email` UNIQUE on `email`
- `idx_users_email_ci` UNIQUE on `lower(trim(email))` (case-insensitive uniqueness)
- `idx_users_suite_code` UNIQUE partial index on `suite_code` where non-null
- `idx_users_role_status` on `(role, status)`
- `idx_users_email_verification_token` on `email_verification_token`

Relationship notes:

- Referenced by many domain tables (`sessions`, `magic_links`, `password_resets`, etc.).
- Legacy verification fields remain on `users` for backward compatibility with earlier migrations. Active verification links are now stored in `email_verification_tokens`.

### `email_verification_tokens`

Purpose: one-time email verification token ledger. Multiple outstanding tokens may exist for the same user so older emails do not break when a resend or duplicate signup occurs.

Key columns:

- `id` (PK)
- `user_id`, `token_hash`, `used`, `expires_at`, `created_at`, `used_at`

Indexes and constraints:

- FK: `user_id -> users(id)`
- `token_hash` UNIQUE
- `idx_email_verification_tokens_user_id` on `user_id`
- `idx_email_verification_tokens_expires_at` on `expires_at`

### `sessions`

Purpose: refresh token session ledger.

Key columns:

- `id` (PK)
- `user_id`
- `refresh_token_hash`, `expires_at`, `created_at`

Indexes and constraints:

- FK: `user_id -> users(id)`

### `magic_links`

Purpose: passwordless login and one-time auth links.

Key columns:

- `id` (PK)
- `user_id`, `token_hash`, `redirect_to`, `used`, `expires_at`, `created_at`

Indexes and constraints:

- FK: `user_id -> users(id)`

### `password_resets`

Purpose: password reset token ledger.

Key columns:

- `id` (PK)
- `user_id`, `token_hash`, `used`, `expires_at`, `created_at`

Indexes and constraints:

- FK: `user_id -> users(id)`
- `idx_password_resets_user` on `user_id`

### `token_blacklist`

Purpose: revoked JWT/JTI tracking.

Key columns:

- `id` (PK)
- `token_jti`, `expires_at`, `created_at`

Indexes and constraints:

- `idx_token_blacklist_jti` on `token_jti`
- `idx_token_blacklist_expires` on `expires_at`

### `notification_prefs`

Purpose: per-user communication preferences.

Key columns:

- `id` (PK)
- `user_id`
- channel and event flags (`email_enabled`, `sms_enabled`, `push_enabled`, etc.)

Indexes and constraints:

- FK: `user_id -> users(id)`

### `sent_notifications`

Purpose: dedupe ledger for sent notifications.

Key columns:

- `id` (PK)
- `notification_type`, `resource_id`, `recipient_email`, `sent_at`

Indexes and constraints:

- UNIQUE composite constraint on `(notification_type, resource_id, recipient_email)`

## Forwarding, Locker, and Shipment Tables

### `destinations`

Purpose: destination catalog and pricing basis (`usd_per_lb`, transit range).

Key columns:

- `id` (PK)
- `name`, `code`, `capital`
- `usd_per_lb`, `transit_days_min`, `transit_days_max`
- `is_active`, `sort_order`

Indexes and constraints:

- `idx_destinations_active_sort` on `(is_active, sort_order, id)`

Relationship notes:

- Used logically by `recipients`, `bookings`, `ship_requests`, `shipments`, and warehouse flows.
- Most destination links are not enforced as DB FKs in current schema.

### `recipients`

Purpose: saved delivery contacts per user.

Key columns:

- `id` (PK)
- `user_id`, `destination_id`
- address and delivery fields
- `is_default`, `use_count`

Indexes and constraints:

- FK: `user_id -> users(id)`
- `destination_id` is logical (no DB FK constraint currently).

### `locker_packages`

Purpose: inbound package inventory in warehouse locker flow.

Key columns:

- `id` (PK)
- `user_id`, `suite_code`
- dimensional and sender metadata
- `status`, `arrived_at`, `free_storage_expires_at`, `disposed_at`
- `booking_id` (added in warehouse phase)

Indexes and constraints:

- `idx_locker_packages_user_status` on `(user_id, status)`
- `idx_locker_packages_suite_code` on `suite_code`
- `idx_locker_packages_arrived_at` on `arrived_at`
- `idx_locker_packages_free_storage` on `(free_storage_expires_at, status)`
- `idx_locker_packages_status` on `status`
- FK: `user_id -> users(id)`

Relationship notes:

- `booking_id` is intended to link to `bookings(id)`; migration adds it with FK semantics.

### `service_requests`

Purpose: value-added service requests on locker packages.

Key columns:

- `id` (PK)
- `user_id`, `locker_package_id`
- `service_type`, `status`, `price`, lifecycle timestamps

Indexes and constraints:

- FK: `user_id -> users(id)`
- FK: `locker_package_id -> locker_packages(id)`
- `idx_service_requests_locker_status` on `(locker_package_id, status)`
- `idx_service_requests_status` on `status`

### `inbound_tracking`

Purpose: user-declared incoming parcels before warehouse intake.

Key columns:

- `id` (PK)
- `user_id`, `carrier`, `tracking_number`, `status`
- optional `locker_package_id`, `last_checked_at`

Indexes and constraints:

- FK: `user_id -> users(id)`
- `idx_inbound_tracking_user` on `user_id`
- `idx_inbound_tracking_number` on `tracking_number`

### `storage_fees`

Purpose: storage charge accrual records.

Key columns:

- `id` (PK)
- `user_id`, `locker_package_id`, `fee_date`, `amount`, `invoiced`, `invoice_id`

Indexes and constraints:

- FK: `user_id -> users(id)`
- FK: `locker_package_id -> locker_packages(id)`
- `idx_storage_fees_user_invoiced` on `(user_id, invoiced)`
- `idx_storage_fees_locker` on `locker_package_id`

### `unmatched_packages`

Purpose: warehouse receives that cannot be matched to a valid suite/user.

Key columns:

- `id` (PK)
- parcel identification fields
- `status`, `matched_user_id`, `resolution_notes`
- `received_at`, `resolved_at`

Indexes and constraints:

- `idx_unmatched_packages_status` on `status`
- `idx_unmatched_packages_received` on `received_at`

Relationship notes:

- `matched_user_id` is logical and not enforced as FK in current DDL.

### `bookings`

Purpose: booking transactions and pricing snapshot values.

Key columns:

- `id` (PK)
- `user_id`, `confirmation_code`, `status`, `service_type`
- `destination_id`, `recipient_id`
- schedule fields (`scheduled_date`, `time_slot`)
- pricing snapshot fields: `weight_lbs`, `length_in`, `width_in`, `height_in`, `value_usd`, `add_insurance`
- totals and payment fields

Indexes and constraints:

- FK: `user_id -> users(id)`
- `idx_bookings_user_status` on `(user_id, status)`
- `idx_bookings_scheduled` on `scheduled_date`
- `idx_bookings_confirmation` on `confirmation_code`

### `ship_requests`

Purpose: outbound forwarding request and fulfillment lifecycle.

Key columns:

- `id` (PK)
- `user_id`, `confirmation_code`, `status`
- `destination_id`, `recipient_id`, `service_type`, `consolidate`
- cost and payment fields (`subtotal`, `service_fees`, `insurance`, `discount`, `total`, `payment_status`)
- warehouse fields (`consolidated_weight_lbs`, `staging_bay`, `manifest_id`)
- `status_constraint_guard` (constraint carrier column)

Indexes and constraints:

- FK: `user_id -> users(id)`
- status check constraint enforced via `status_constraint_guard` and `CHECK(status IN (...))`
- allowed statuses: `draft`, `pending_customs`, `pending_payment`, `paid`, `processing`, `staged`, `shipped`, `delivered`, `cancelled`, `expired`
- `idx_ship_requests_user_status` on `(user_id, status)`
- `idx_ship_requests_confirmation` on `confirmation_code`

### `ship_request_items`

Purpose: package-level customs/item lines linked to a ship request.

Key columns:

- `id` (PK)
- `ship_request_id`, `locker_package_id`
- customs declaration fields

Indexes and constraints:

- FK: `ship_request_id -> ship_requests(id)`
- FK: `locker_package_id -> locker_packages(id)`
- `idx_ship_request_items_ship_request` on `ship_request_id`
- `idx_ship_request_items_locker` on `locker_package_id`

### `shipments`

Purpose: shipment records and final-mile tracking status.

Key columns:

- `id` (PK)
- `destination_id`, `manifest_id`, `ship_request_id`
- `tracking_number`, `status`, weight and delivery timestamps

Indexes and constraints:

- FK: `ship_request_id -> ship_requests(id)`
- `idx_shipments_destination_status` on `(destination_id, status)`
- `idx_shipments_tracking` on `tracking_number`
- `idx_shipments_ship_request` on `ship_request_id`

### `warehouse_bays`

Purpose: warehouse bay catalog and occupancy counters.

Key columns:

- `id` (PK)
- `name`, `zone`, `destination_id`, `capacity`, `current_count`

### `warehouse_manifests`

Purpose: warehouse-level manifest lifecycle.

Key columns:

- `id` (PK)
- `destination_id`, `status`, `created_at`, `updated_at`

### `warehouse_manifest_ship_requests`

Purpose: join table linking manifests and ship requests.

Key columns:

- `manifest_id`, `ship_request_id`

Indexes and constraints:

- composite PK `(manifest_id, ship_request_id)`
- FK: `manifest_id -> warehouse_manifests(id)`
- FK: `ship_request_id -> ship_requests(id)`

### `invoices` and `invoice_items`

Purpose: billing records for bookings and ship requests.

Key columns:

- `invoices`: `id` (PK), `user_id`, optional `booking_id`, optional `ship_request_id`, totals, status
- `invoice_items`: `id` (PK), `invoice_id`, `description`, quantity and pricing

Indexes and constraints:

- FK: `invoices.user_id -> users(id)`
- FK: `invoice_items.invoice_id -> invoices(id)`

### `templates`

Purpose: reusable request presets.

Key columns:

- `id` (PK)
- `user_id`, `name`, `service_type`, `destination_id`, `recipient_id`, `use_count`

Indexes and constraints:

- FK: `user_id -> users(id)`

### `admin_activity`

Purpose: admin action audit log.

Key columns:

- `id` (PK)
- `actor_id`, `action`, `entity_type`, optional `entity_id`, optional `details`, `created_at`

Indexes and constraints:

- FK: `actor_id -> users(id)`

### `blog_posts`

Purpose: CMS-style admin content table.

Key columns:

- `id` (PK)
- `slug`, `title`, `excerpt`, `content_md`, `category`, `status`, publication timestamps

Indexes and constraints:

- `slug` is UNIQUE

## Observability Tables

### `observability_events`

Purpose: unified event stream for error tracking, analytics, performance, and business metrics.

Key columns:

- `id` (PK)
- `category`, `event_name`
- optional context: `user_id`, `request_id`, `path`, `method`, `status_code`
- metric values: `duration_ms`, `value`
- `metadata_json`, `created_at`

Indexes and constraints:

- CHECK constraint: `category IN ('error', 'analytics', 'performance', 'business')`
- `idx_observability_events_category_created` on `(category, created_at)`
- `idx_observability_events_event_created` on `(event_name, created_at)`
- `idx_observability_events_user_created` on `(user_id, created_at)`
- `idx_observability_events_path_created` on `(path, created_at)`

Relationship notes:

- `user_id` and `request_id` are correlation fields, not DB-enforced FKs.

## Relationship Summary

High-value FK links:

- `users -> sessions | magic_links | password_resets | recipients | locker_packages | service_requests | inbound_tracking | storage_fees | bookings | invoices | templates | notification_prefs | admin_activity`
- `locker_packages -> service_requests | ship_request_items | storage_fees`
- `ship_requests -> ship_request_items | shipments | warehouse_manifest_ship_requests`
- `warehouse_manifests <-> ship_requests` via `warehouse_manifest_ship_requests`
- `invoices -> invoice_items`

Logical (app-level) links without strict FK in current DDL include many `destination_id` and selected optional reference columns (`recipient_id`, `matched_user_id`, observability correlation IDs).

## Migration and Down-Migration Policy Notes

Migration system characteristics:

- Goose-style migrations use explicit `-- +goose Up` and `-- +goose Down` blocks.
- Many `Down` paths are implemented and include table rebuild patterns for SQLite column removal compatibility.

SQLite rebuild pattern used in `Down`:

1. `PRAGMA foreign_keys = OFF`
2. create replacement table
3. copy data with `INSERT INTO ... SELECT ...`
4. drop old table and rename new table
5. recreate indexes
6. `PRAGMA foreign_keys = ON`

Operational caveats:

- `20260301030000_inc_011_014_db_hardening.sql` performs dedupe/normalization for `suite_code` and email before creating stricter unique indexes.
- That migration explicitly notes these data cleanup steps are not fully reversible in `Down`.

## Wave 11 and post-wave 11 tables

Migrations under `sql/migrations/2026030106*` and later add the following operational tables. They are documented at this summary level; column inventories live in the migration files themselves.

### Security and compliance

- `user_mfa` — per-user MFA factor metadata (method, secret hash, status).
- `api_keys` — hashed machine-auth API keys with rotation/revocation lifecycle.
- `ip_access_rules` — IP allow/deny rules enforced for `X-API-Key` requests.
- `feature_flags` — runtime feature toggle registry.
- `cookie_consents` — versioned per-user cookie consent capture.
- `resource_versions` — generic version-history ledger for compliance/security mutations (feature flags, GDPR requests, API key lifecycle, cookie consent updates, recipient restore actions).
- `auth_request_log` — per-account auth-throttle bucket for `services.CheckAndRecordAuthRequest`. Bounded at 1000 rows per bucket.

### Parcel features (parcel-plus)

- `assisted_purchase_requests` — customer-submitted assisted purchase orders.
- `parcel_consolidation_previews` — saved consolidation preview snapshots.
- `customs_preclearance_docs` — customs document metadata per ship request.
- `delivery_signatures` — delivery signature capture (image bytes, captured_at).
- `loyalty_ledger` — per-user loyalty point ledger.
- `data_import_jobs` — recipient (and other) import job audit metadata.

### Notifications and outbound mail

- `in_app_notifications` — per-user in-app notification feed (drives `/api/v1/notifications` and the SSE stream).
- `push_subscriptions` — Web Push endpoint registrations. UNIQUE on `(endpoint)` (post-`20260422120000` migration) to prevent cross-account re-registration.
- `outbound_emails` — durable outbound email queue. Workers claim rows with conditional `UPDATE ... WHERE status='pending' AND id=?` (`:execrows`) so multi-worker / multi-replica processing is race-safe. The supervisor reaps `in_progress` rows older than 5 minutes back to `pending`.

### Moderation

- `moderation_items` — admin moderation queue.

## Legacy tables from initial migration

`20260221120000_initial_schema.sql` also created the following tables. Some are still actively used at runtime (`locker_photos`, `activity_log`, `settings`, `packages`, `manifests`, `exceptions`, `weight_discrepancies`, `communications`); they are listed here because they are outside the active `sql/schema` sqlc model set used for generated query models. Handlers that read these tables use raw SQL.

- `activity_log`
- `communications`
- `exceptions`
- `locker_photos`
- `manifests`
- `packages`
- `settings`
- `weight_discrepancies`
