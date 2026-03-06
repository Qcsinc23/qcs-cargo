-- Core integrity guardrails for bookings and ship requests.
-- +goose Up
PRAGMA foreign_keys = OFF;

WITH ranked_booking_pi AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY stripe_payment_intent_id
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM bookings
    WHERE stripe_payment_intent_id IS NOT NULL
)
UPDATE bookings
SET stripe_payment_intent_id = NULL
WHERE id IN (SELECT id FROM ranked_booking_pi WHERE rn > 1);

UPDATE bookings
SET recipient_id = NULL
WHERE recipient_id IS NOT NULL
  AND recipient_id NOT IN (SELECT id FROM recipients);

UPDATE bookings
SET status = 'pending'
WHERE status NOT IN ('pending', 'confirmed', 'received', 'completed', 'cancelled');

UPDATE bookings
SET service_type = 'standard'
WHERE service_type NOT IN ('standard', 'express', 'door_to_door');

UPDATE bookings
SET time_slot = 'morning'
WHERE time_slot NOT IN ('morning', 'afternoon', 'evening');

UPDATE bookings
SET payment_status = NULL
WHERE payment_status IS NOT NULL
  AND payment_status NOT IN ('pending', 'paid', 'failed', 'refunded');

UPDATE bookings
SET
    weight_lbs = CASE WHEN weight_lbs < 0 THEN 0 ELSE weight_lbs END,
    length_in = CASE WHEN length_in < 0 THEN 0 ELSE length_in END,
    width_in = CASE WHEN width_in < 0 THEN 0 ELSE width_in END,
    height_in = CASE WHEN height_in < 0 THEN 0 ELSE height_in END,
    value_usd = CASE WHEN value_usd < 0 THEN 0 ELSE value_usd END,
    subtotal = CASE WHEN subtotal < 0 THEN 0 ELSE subtotal END,
    discount = CASE WHEN discount < 0 THEN 0 ELSE discount END,
    insurance = CASE WHEN insurance < 0 THEN 0 ELSE insurance END,
    total = CASE WHEN total < 0 THEN 0 ELSE total END,
    add_insurance = CASE WHEN add_insurance IN (0, 1) THEN add_insurance ELSE 0 END;

DROP TABLE IF EXISTS bookings__new_202603061200;
CREATE TABLE bookings__new_202603061200 (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    confirmation_code TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed', 'received', 'completed', 'cancelled')),
    service_type TEXT NOT NULL CHECK (service_type IN ('standard', 'express', 'door_to_door')),
    destination_id TEXT NOT NULL,
    recipient_id TEXT REFERENCES recipients(id),
    scheduled_date TEXT NOT NULL,
    time_slot TEXT NOT NULL CHECK (time_slot IN ('morning', 'afternoon', 'evening')),
    special_instructions TEXT,
    weight_lbs REAL NOT NULL DEFAULT 0 CHECK (weight_lbs >= 0),
    length_in REAL NOT NULL DEFAULT 0 CHECK (length_in >= 0),
    width_in REAL NOT NULL DEFAULT 0 CHECK (width_in >= 0),
    height_in REAL NOT NULL DEFAULT 0 CHECK (height_in >= 0),
    value_usd REAL NOT NULL DEFAULT 0 CHECK (value_usd >= 0),
    add_insurance INTEGER NOT NULL DEFAULT 0 CHECK (add_insurance IN (0, 1)),
    subtotal REAL NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount REAL NOT NULL DEFAULT 0 CHECK (discount >= 0),
    insurance REAL NOT NULL DEFAULT 0 CHECK (insurance >= 0),
    total REAL NOT NULL DEFAULT 0 CHECK (total >= 0),
    payment_status TEXT CHECK (payment_status IS NULL OR payment_status IN ('pending', 'paid', 'failed', 'refunded')),
    stripe_payment_intent_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO bookings__new_202603061200 (
    id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
    scheduled_date, time_slot, special_instructions,
    weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
    subtotal, discount, insurance, total,
    payment_status, stripe_payment_intent_id, created_at, updated_at
)
SELECT
    id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
    scheduled_date, time_slot, special_instructions,
    weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
    subtotal, discount, insurance, total,
    payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings;

DROP TABLE bookings;
ALTER TABLE bookings__new_202603061200 RENAME TO bookings;

CREATE INDEX IF NOT EXISTS idx_bookings_user_status ON bookings(user_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_scheduled ON bookings(scheduled_date);
CREATE INDEX IF NOT EXISTS idx_bookings_confirmation ON bookings(confirmation_code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_stripe_payment_intent_id
ON bookings(stripe_payment_intent_id)
WHERE stripe_payment_intent_id IS NOT NULL;

WITH ranked_ship_pi AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY stripe_payment_intent_id
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM ship_requests
    WHERE stripe_payment_intent_id IS NOT NULL
)
UPDATE ship_requests
SET stripe_payment_intent_id = NULL
WHERE id IN (SELECT id FROM ranked_ship_pi WHERE rn > 1);

UPDATE ship_requests
SET recipient_id = NULL
WHERE recipient_id IS NOT NULL
  AND recipient_id NOT IN (SELECT id FROM recipients);

UPDATE ship_requests
SET service_type = 'standard'
WHERE service_type NOT IN ('standard', 'express', 'door_to_door');

UPDATE ship_requests
SET payment_status = NULL
WHERE payment_status IS NOT NULL
  AND payment_status NOT IN ('pending', 'paid', 'failed', 'refunded');

UPDATE ship_requests
SET
    subtotal = CASE WHEN subtotal < 0 THEN 0 ELSE subtotal END,
    service_fees = CASE WHEN service_fees < 0 THEN 0 ELSE service_fees END,
    insurance = CASE WHEN insurance < 0 THEN 0 ELSE insurance END,
    discount = CASE WHEN discount < 0 THEN 0 ELSE discount END,
    total = CASE WHEN total < 0 THEN 0 ELSE total END,
    consolidate = CASE WHEN consolidate IN (0, 1) THEN consolidate ELSE 1 END;

DROP TABLE IF EXISTS ship_requests__new_202603061200;
CREATE TABLE ship_requests__new_202603061200 (
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

INSERT INTO ship_requests__new_202603061200 (
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    consolidated_weight_lbs, staging_bay, manifest_id,
    created_at, updated_at, status_constraint_guard
)
SELECT
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    consolidated_weight_lbs, staging_bay, manifest_id,
    created_at, updated_at, status_constraint_guard
FROM ship_requests;

DROP TABLE ship_requests;
ALTER TABLE ship_requests__new_202603061200 RENAME TO ship_requests;

CREATE INDEX IF NOT EXISTS idx_ship_requests_user_status ON ship_requests(user_id, status);
CREATE INDEX IF NOT EXISTS idx_ship_requests_confirmation ON ship_requests(confirmation_code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ship_requests_stripe_payment_intent_id
ON ship_requests(stripe_payment_intent_id)
WHERE stripe_payment_intent_id IS NOT NULL;

DELETE FROM ship_request_items
WHERE rowid IN (
    SELECT rowid
    FROM (
        SELECT
            rowid,
            ROW_NUMBER() OVER (
                PARTITION BY locker_package_id
                ORDER BY rowid ASC
            ) AS rn
        FROM ship_request_items
    )
    WHERE rn > 1
);

DROP TABLE IF EXISTS ship_request_items__new_202603061200;
CREATE TABLE ship_request_items__new_202603061200 (
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

INSERT INTO ship_request_items__new_202603061200 (
    id, ship_request_id, locker_package_id, customs_description, customs_value,
    customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
)
SELECT
    id, ship_request_id, locker_package_id, customs_description, customs_value,
    customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
FROM ship_request_items;

DROP TABLE ship_request_items;
ALTER TABLE ship_request_items__new_202603061200 RENAME TO ship_request_items;

CREATE INDEX IF NOT EXISTS idx_ship_request_items_ship_request ON ship_request_items(ship_request_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ship_request_items_locker ON ship_request_items(locker_package_id);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_ship_request_items_locker;
DROP INDEX IF EXISTS idx_ship_requests_stripe_payment_intent_id;
DROP INDEX IF EXISTS idx_bookings_stripe_payment_intent_id;

DROP TABLE IF EXISTS ship_request_items__old_202603061200;
CREATE TABLE ship_request_items__old_202603061200 (
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

INSERT INTO ship_request_items__old_202603061200 (
    id, ship_request_id, locker_package_id, customs_description, customs_value,
    customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
)
SELECT
    id, ship_request_id, locker_package_id, customs_description, customs_value,
    customs_quantity, customs_hs_code, customs_country_of_origin, customs_weight_lbs
FROM ship_request_items;

DROP TABLE ship_request_items;
ALTER TABLE ship_request_items__old_202603061200 RENAME TO ship_request_items;
CREATE INDEX IF NOT EXISTS idx_ship_request_items_ship_request ON ship_request_items(ship_request_id);
CREATE INDEX IF NOT EXISTS idx_ship_request_items_locker ON ship_request_items(locker_package_id);

DROP TABLE IF EXISTS ship_requests__old_202603061200;
CREATE TABLE ship_requests__old_202603061200 (
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

INSERT INTO ship_requests__old_202603061200 (
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    consolidated_weight_lbs, staging_bay, manifest_id,
    created_at, updated_at, status_constraint_guard
)
SELECT
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    consolidated_weight_lbs, staging_bay, manifest_id,
    created_at, updated_at, status_constraint_guard
FROM ship_requests;

DROP TABLE ship_requests;
ALTER TABLE ship_requests__old_202603061200 RENAME TO ship_requests;
CREATE INDEX IF NOT EXISTS idx_ship_requests_user_status ON ship_requests(user_id, status);
CREATE INDEX IF NOT EXISTS idx_ship_requests_confirmation ON ship_requests(confirmation_code);

DROP TABLE IF EXISTS bookings__old_202603061200;
CREATE TABLE bookings__old_202603061200 (
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
    weight_lbs REAL NOT NULL DEFAULT 0,
    length_in REAL NOT NULL DEFAULT 0,
    width_in REAL NOT NULL DEFAULT 0,
    height_in REAL NOT NULL DEFAULT 0,
    value_usd REAL NOT NULL DEFAULT 0,
    add_insurance INTEGER NOT NULL DEFAULT 0,
    subtotal REAL NOT NULL DEFAULT 0,
    discount REAL NOT NULL DEFAULT 0,
    insurance REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    payment_status TEXT,
    stripe_payment_intent_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO bookings__old_202603061200 (
    id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
    scheduled_date, time_slot, special_instructions,
    weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
    subtotal, discount, insurance, total,
    payment_status, stripe_payment_intent_id, created_at, updated_at
)
SELECT
    id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
    scheduled_date, time_slot, special_instructions,
    weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
    subtotal, discount, insurance, total,
    payment_status, stripe_payment_intent_id, created_at, updated_at
FROM bookings;

DROP TABLE bookings;
ALTER TABLE bookings__old_202603061200 RENAME TO bookings;
CREATE INDEX IF NOT EXISTS idx_bookings_user_status ON bookings(user_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_scheduled ON bookings(scheduled_date);
CREATE INDEX IF NOT EXISTS idx_bookings_confirmation ON bookings(confirmation_code);

PRAGMA foreign_keys = ON;
