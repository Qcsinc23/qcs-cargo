CREATE TABLE ship_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    confirmation_code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    destination_id TEXT NOT NULL,
    recipient_id TEXT,
    service_type TEXT NOT NULL,
    consolidate INTEGER NOT NULL DEFAULT 1,
    special_instructions TEXT,
    subtotal REAL NOT NULL DEFAULT 0,
    service_fees REAL NOT NULL DEFAULT 0,
    insurance REAL NOT NULL DEFAULT 0,
    discount REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    payment_status TEXT,
    stripe_payment_intent_id TEXT,
    customs_status TEXT,
    consolidated_weight_lbs REAL,
    staging_bay TEXT,
    manifest_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE ship_request_items (
    id TEXT PRIMARY KEY,
    ship_request_id TEXT NOT NULL REFERENCES ship_requests(id),
    locker_package_id TEXT NOT NULL REFERENCES locker_packages(id),
    customs_description TEXT,
    customs_value REAL,
    customs_quantity INTEGER,
    customs_hs_code TEXT,
    customs_country_of_origin TEXT,
    customs_weight_lbs REAL
);
