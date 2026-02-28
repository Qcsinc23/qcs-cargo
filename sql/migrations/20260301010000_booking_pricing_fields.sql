-- +goose Up
ALTER TABLE bookings ADD COLUMN weight_lbs REAL NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN length_in REAL NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN width_in REAL NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN height_in REAL NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN value_usd REAL NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN add_insurance INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- Rebuild bookings table without pricing snapshot columns for SQLite compatibility.
PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS bookings__new_202603010100;
CREATE TABLE bookings__new_202603010100 (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    confirmation_code TEXT NOT NULL,
    status TEXT NOT NULL,
    service_type TEXT NOT NULL,
    destination_id TEXT NOT NULL,
    recipient_id TEXT,
    scheduled_date TEXT NOT NULL,
    time_slot TEXT NOT NULL,
    special_instructions TEXT,
    subtotal REAL NOT NULL DEFAULT 0,
    discount REAL NOT NULL DEFAULT 0,
    insurance REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    payment_status TEXT,
    stripe_payment_intent_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO bookings__new_202603010100 (
    id, user_id, confirmation_code, status, service_type, destination_id,
    recipient_id, scheduled_date, time_slot, special_instructions,
    subtotal, discount, insurance, total,
    payment_status, stripe_payment_intent_id, created_at, updated_at
)
SELECT
    id, user_id, confirmation_code, status, service_type, destination_id,
    recipient_id, scheduled_date, time_slot, special_instructions,
    subtotal, discount, insurance, total,
    payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings;

DROP TABLE bookings;
ALTER TABLE bookings__new_202603010100 RENAME TO bookings;

CREATE INDEX IF NOT EXISTS idx_bookings_user_status ON bookings(user_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_scheduled ON bookings(scheduled_date);
CREATE INDEX IF NOT EXISTS idx_bookings_confirmation ON bookings(confirmation_code);

PRAGMA foreign_keys = ON;
