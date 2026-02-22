-- Matches bookings table from migrations (PRD 5.3 / 2.12).
CREATE TABLE bookings (
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
