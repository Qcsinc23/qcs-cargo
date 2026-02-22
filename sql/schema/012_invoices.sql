CREATE TABLE invoices (
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

CREATE TABLE invoice_items (
    id TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL REFERENCES invoices(id),
    description TEXT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price REAL NOT NULL,
    total REAL NOT NULL
);
