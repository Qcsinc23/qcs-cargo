-- QCS Cargo initial schema per PRD Section 5. UUIDs as TEXT, timestamps as ISO8601.
-- +goose Up
-- Core
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT,
    role TEXT NOT NULL DEFAULT 'customer',
    avatar_url TEXT,
    suite_code TEXT,
    address_street TEXT,
    address_city TEXT,
    address_state TEXT,
    address_zip TEXT,
    storage_plan TEXT NOT NULL DEFAULT 'free',
    free_storage_days INTEGER NOT NULL DEFAULT 30,
    email_verified INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_suite_code ON users(suite_code);
CREATE INDEX IF NOT EXISTS idx_users_role_status ON users(role, status);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    refresh_token_hash TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS magic_links (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL,
    redirect_to TEXT,
    used INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS recipients (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    phone TEXT,
    destination_id TEXT NOT NULL,
    street TEXT NOT NULL,
    apt TEXT,
    city TEXT NOT NULL,
    delivery_instructions TEXT,
    is_default INTEGER NOT NULL DEFAULT 0,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Parcel forwarding (locker)
CREATE TABLE IF NOT EXISTS locker_packages (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    suite_code TEXT NOT NULL,
    tracking_inbound TEXT,
    carrier_inbound TEXT,
    sender_name TEXT,
    sender_address TEXT,
    weight_lbs REAL,
    length_in REAL,
    width_in REAL,
    height_in REAL,
    arrival_photo_url TEXT,
    condition TEXT,
    storage_bay TEXT,
    status TEXT NOT NULL DEFAULT 'stored',
    arrived_at TEXT,
    free_storage_expires_at TEXT,
    disposed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_locker_packages_user_status ON locker_packages(user_id, status);
CREATE INDEX IF NOT EXISTS idx_locker_packages_suite_code ON locker_packages(suite_code);
CREATE INDEX IF NOT EXISTS idx_locker_packages_arrived_at ON locker_packages(arrived_at);
CREATE INDEX IF NOT EXISTS idx_locker_packages_free_storage ON locker_packages(free_storage_expires_at, status);

CREATE TABLE IF NOT EXISTS locker_photos (
    id TEXT PRIMARY KEY,
    locker_package_id TEXT NOT NULL REFERENCES locker_packages(id),
    photo_url TEXT NOT NULL,
    photo_type TEXT NOT NULL,
    taken_by TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ship_requests (
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
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ship_requests_user_status ON ship_requests(user_id, status);
CREATE INDEX IF NOT EXISTS idx_ship_requests_confirmation ON ship_requests(confirmation_code);

CREATE TABLE IF NOT EXISTS ship_request_items (
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
CREATE INDEX IF NOT EXISTS idx_ship_request_items_ship_request ON ship_request_items(ship_request_id);
CREATE INDEX IF NOT EXISTS idx_ship_request_items_locker ON ship_request_items(locker_package_id);

CREATE TABLE IF NOT EXISTS service_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    locker_package_id TEXT NOT NULL REFERENCES locker_packages(id),
    service_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    notes TEXT,
    completed_by TEXT,
    price REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_service_requests_locker_status ON service_requests(locker_package_id, status);
CREATE INDEX IF NOT EXISTS idx_service_requests_status ON service_requests(status);

CREATE TABLE IF NOT EXISTS inbound_tracking (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    carrier TEXT NOT NULL,
    tracking_number TEXT NOT NULL,
    retailer_name TEXT,
    expected_items TEXT,
    status TEXT NOT NULL DEFAULT 'tracking',
    locker_package_id TEXT,
    last_checked_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_inbound_tracking_user ON inbound_tracking(user_id);
CREATE INDEX IF NOT EXISTS idx_inbound_tracking_number ON inbound_tracking(tracking_number);

CREATE TABLE IF NOT EXISTS storage_fees (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    locker_package_id TEXT NOT NULL REFERENCES locker_packages(id),
    fee_date TEXT NOT NULL,
    amount REAL NOT NULL,
    invoiced INTEGER NOT NULL DEFAULT 0,
    invoice_id TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_storage_fees_user_invoiced ON storage_fees(user_id, invoiced);
CREATE INDEX IF NOT EXISTS idx_storage_fees_locker ON storage_fees(locker_package_id);

CREATE TABLE IF NOT EXISTS unmatched_packages (
    id TEXT PRIMARY KEY,
    carrier TEXT,
    tracking_number TEXT,
    label_text TEXT,
    photo_url TEXT,
    weight_lbs REAL,
    status TEXT NOT NULL DEFAULT 'pending',
    matched_user_id TEXT,
    resolution_notes TEXT,
    received_at TEXT NOT NULL,
    resolved_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_unmatched_packages_status ON unmatched_packages(status);
CREATE INDEX IF NOT EXISTS idx_unmatched_packages_received ON unmatched_packages(received_at);

-- Bookings & shipping
CREATE TABLE IF NOT EXISTS bookings (
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
CREATE INDEX IF NOT EXISTS idx_bookings_user_status ON bookings(user_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_scheduled ON bookings(scheduled_date);
CREATE INDEX IF NOT EXISTS idx_bookings_confirmation ON bookings(confirmation_code);

CREATE TABLE IF NOT EXISTS shipments (
    id TEXT PRIMARY KEY,
    destination_id TEXT NOT NULL,
    manifest_id TEXT,
    ship_request_id TEXT,
    tracking_number TEXT,
    status TEXT NOT NULL,
    total_weight REAL,
    package_count INTEGER,
    carrier TEXT,
    estimated_delivery TEXT,
    actual_delivery TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shipments_destination_status ON shipments(destination_id, status);
CREATE INDEX IF NOT EXISTS idx_shipments_tracking ON shipments(tracking_number);
CREATE INDEX IF NOT EXISTS idx_shipments_ship_request ON shipments(ship_request_id);

CREATE TABLE IF NOT EXISTS manifests (
    id TEXT PRIMARY KEY,
    manifest_number TEXT NOT NULL,
    destination_id TEXT NOT NULL,
    carrier TEXT NOT NULL,
    flight_number TEXT,
    departure_date TEXT,
    total_pieces INTEGER,
    total_weight REAL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS packages (
    id TEXT PRIMARY KEY,
    booking_id TEXT,
    shipment_id TEXT,
    locker_package_id TEXT REFERENCES locker_packages(id),
    tracking_number TEXT,
    weight_estimated REAL,
    weight_actual REAL,
    length REAL,
    width REAL,
    height REAL,
    declared_value REAL,
    contents TEXT,
    condition TEXT,
    warehouse_location TEXT,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_packages_booking ON packages(booking_id);
CREATE INDEX IF NOT EXISTS idx_packages_shipment ON packages(shipment_id);
CREATE INDEX IF NOT EXISTS idx_packages_tracking ON packages(tracking_number);

CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    booking_id TEXT,
    ship_request_id TEXT,
    invoice_number TEXT NOT NULL,
    subtotal REAL NOT NULL DEFAULT 0,
    tax REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    due_date TEXT,
    paid_at TEXT,
    notes TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS invoice_items (
    id TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL REFERENCES invoices(id),
    description TEXT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price REAL NOT NULL,
    total REAL NOT NULL
);

-- Operations
CREATE TABLE IF NOT EXISTS exceptions (
    id TEXT PRIMARY KEY,
    package_id TEXT,
    booking_id TEXT,
    locker_package_id TEXT,
    type TEXT NOT NULL,
    priority TEXT,
    severity TEXT,
    description TEXT NOT NULL,
    resolution TEXT,
    status TEXT NOT NULL,
    customer_notified INTEGER NOT NULL DEFAULT 0,
    reported_by TEXT,
    resolved_by TEXT,
    created_at TEXT NOT NULL,
    resolved_at TEXT
);

CREATE TABLE IF NOT EXISTS weight_discrepancies (
    id TEXT PRIMARY KEY,
    package_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id),
    estimated_weight REAL NOT NULL,
    actual_weight REAL NOT NULL,
    difference REAL NOT NULL,
    additional_cost REAL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS communications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    subject TEXT,
    content TEXT NOT NULL,
    sent_by TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS activity_log (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    description TEXT,
    ip_address TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activity_log_resource ON activity_log(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at);

CREATE TABLE IF NOT EXISTS warehouse_bays (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    zone TEXT,
    destination_id TEXT,
    capacity INTEGER NOT NULL DEFAULT 0,
    current_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS templates (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    service_type TEXT NOT NULL,
    destination_id TEXT NOT NULL,
    recipient_id TEXT,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at TEXT NOT NULL,
    updated_by TEXT
);

CREATE TABLE IF NOT EXISTS notification_prefs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    email_enabled INTEGER NOT NULL DEFAULT 1,
    sms_enabled INTEGER NOT NULL DEFAULT 0,
    push_enabled INTEGER NOT NULL DEFAULT 1,
    on_package_arrived INTEGER NOT NULL DEFAULT 1,
    on_storage_expiry INTEGER NOT NULL DEFAULT 1,
    on_ship_updates INTEGER NOT NULL DEFAULT 1,
    on_inbound_updates INTEGER NOT NULL DEFAULT 1,
    daily_digest TEXT NOT NULL DEFAULT 'off',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS notification_prefs;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS warehouse_bays;
DROP TABLE IF EXISTS activity_log;
DROP TABLE IF EXISTS communications;
DROP TABLE IF EXISTS weight_discrepancies;
DROP TABLE IF EXISTS exceptions;
DROP TABLE IF EXISTS invoice_items;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS packages;
DROP TABLE IF EXISTS manifests;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS storage_fees;
DROP TABLE IF EXISTS inbound_tracking;
DROP TABLE IF EXISTS service_requests;
DROP TABLE IF EXISTS ship_request_items;
DROP TABLE IF EXISTS ship_requests;
DROP TABLE IF EXISTS locker_photos;
DROP TABLE IF EXISTS locker_packages;
DROP TABLE IF EXISTS unmatched_packages;
DROP TABLE IF EXISTS recipients;
DROP TABLE IF EXISTS magic_links;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
