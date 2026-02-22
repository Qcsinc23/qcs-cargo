CREATE TABLE storage_fees (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    locker_package_id TEXT NOT NULL REFERENCES locker_packages(id),
    fee_date TEXT NOT NULL,
    amount REAL NOT NULL,
    invoiced INTEGER NOT NULL DEFAULT 0,
    invoice_id TEXT,
    created_at TEXT NOT NULL
);
