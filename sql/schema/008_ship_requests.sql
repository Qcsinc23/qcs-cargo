CREATE TABLE ship_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    confirmation_code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    destination_id TEXT NOT NULL,
    recipient_id TEXT REFERENCES recipients(id),
    service_type TEXT NOT NULL CHECK (service_type IN ('standard', 'express', 'door_to_door')),
    consolidate INTEGER NOT NULL DEFAULT 1 CHECK (consolidate IN (0, 1)),
    special_instructions TEXT,
    subtotal REAL NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    service_fees REAL NOT NULL DEFAULT 0 CHECK (service_fees >= 0),
    insurance REAL NOT NULL DEFAULT 0 CHECK (insurance >= 0),
    discount REAL NOT NULL DEFAULT 0 CHECK (discount >= 0),
    total REAL NOT NULL DEFAULT 0 CHECK (total >= 0),
    payment_status TEXT CHECK (payment_status IS NULL OR payment_status IN ('pending', 'paid', 'failed', 'refunded')),
    stripe_payment_intent_id TEXT,
    customs_status TEXT,
    consolidated_weight_lbs REAL,
    staging_bay TEXT,
    manifest_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    status_constraint_guard INTEGER NOT NULL DEFAULT 1 CHECK (
        status IN (
            'draft',
            'pending_customs',
            'pending_payment',
            'paid',
            'processing',
            'staged',
            'shipped',
            'delivered',
            'cancelled',
            'expired'
        )
    )
);

CREATE TABLE ship_request_items (
    id TEXT PRIMARY KEY,
    ship_request_id TEXT NOT NULL REFERENCES ship_requests(id),
    locker_package_id TEXT NOT NULL UNIQUE REFERENCES locker_packages(id),
    customs_description TEXT,
    customs_value REAL,
    customs_quantity INTEGER,
    customs_hs_code TEXT,
    customs_country_of_origin TEXT,
    customs_weight_lbs REAL
);
