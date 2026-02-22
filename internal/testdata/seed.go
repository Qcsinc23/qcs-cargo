// Package testdata provides deterministic test fixtures for integration
// and E2E tests. All functions accept a *sql.DB and insert with ? placeholders for SQLite.
package testdata

import (
	"database/sql"
	"fmt"
	"time"
)

// Deterministic IDs for assertions.
const (
	CustomerAliceID  = "usr_alice_00000001"
	CustomerBobID    = "usr_bob_000000002"
	StaffWarehouseID = "usr_staff_0000001"
	AdminID          = "usr_admin_0000001"

	AliceSuiteCode = "QCS-A1B2C3"
	BobSuiteCode   = "QCS-D4E5F6"

	PkgAliceStored1 = "lpkg_alice_stor01"
	PkgAliceStored2 = "lpkg_alice_stor02"
	PkgAliceStored3 = "lpkg_alice_stor03"
	PkgAliceShipped = "lpkg_alice_ship01"
	PkgAliceService = "lpkg_alice_svc001"
	PkgAliceExpired = "lpkg_alice_exp001"
	PkgBobStored1   = "lpkg_bob_stored01"

	ShipReqAliceDraft   = "sreq_alice_draft1"
	ShipReqAlicePaid    = "sreq_alice_paid01"
	ShipReqAliceShipped = "sreq_alice_ship01"

	RecipientGeorgetown = "rcpt_georgetown01"
	RecipientKingston   = "rcpt_kingston001"

	BookingAlice1 = "bkg_alice_00001"
	UnmatchedPkg1 = "unmatch_00000001"
)

// SeedAll creates a complete test world. Call once at the start of an integration test suite.
func SeedAll(db *sql.DB) error {
	for _, s := range []func(*sql.DB) error{
		SeedUsers,
		SeedLockerPackagesAlice,
		SeedRecipients,
		SeedShipRequests,
		SeedServiceRequests,
	} {
		if err := s(db); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}
	return nil
}

// SeedUsers creates 4 users: 2 customers, 1 staff, 1 admin.
func SeedUsers(db *sql.DB) error {
	users := []struct {
		id, name, email, suiteCode, role string
	}{
		{CustomerAliceID, "Alice Johnson", "alice@test.com", AliceSuiteCode, "customer"},
		{CustomerBobID, "Bob Williams", "bob@test.com", BobSuiteCode, "customer"},
		{StaffWarehouseID, "Warehouse Staff", "staff@qcs-cargo.com", "", "staff"},
		{AdminID, "Admin User", "admin@qcs-cargo.com", "", "admin"},
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, u := range users {
		_, err := db.Exec(`
			INSERT INTO users (id, name, email, suite_code, role, email_verified, status, free_storage_days, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, 'active', 30, ?, ?)
		`, u.id, u.name, u.email, nullStr(u.suiteCode), u.role, now, now)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", u.name, err)
		}
	}
	return nil
}

// SeedLockerPackagesAlice creates packages in various lifecycle states.
func SeedLockerPackagesAlice(db *sql.DB) error {
	now := time.Now()
	packages := []struct {
		id, sender, status string
		arrivedDaysAgo    int
		weight            float64
	}{
		{PkgAliceStored1, "Amazon", "stored", 5, 2.5},
		{PkgAliceStored2, "Walmart", "stored", 25, 4.0},
		{PkgAliceStored3, "eBay", "stored", 32, 1.8},
		{PkgAliceShipped, "Nike", "shipped", 15, 3.2},
		{PkgAliceService, "Best Buy", "service_pending", 10, 6.0},
		{PkgAliceExpired, "Target", "expired", 65, 2.0},
	}
	for _, p := range packages {
		arrived := now.AddDate(0, 0, -p.arrivedDaysAgo)
		freeExpires := arrived.AddDate(0, 0, 30)
		arrivedStr := arrived.Format(time.RFC3339)
		freeStr := freeExpires.Format(time.RFC3339)
		nowStr := now.Format(time.RFC3339)
		_, err := db.Exec(`
			INSERT INTO locker_packages
				(id, user_id, suite_code, sender_name, weight_lbs, condition, status,
				 storage_bay, arrived_at, free_storage_expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'good', ?, 'A1', ?, ?, ?, ?)
		`, p.id, CustomerAliceID, AliceSuiteCode, p.sender, p.weight, p.status,
			arrivedStr, freeStr, arrivedStr, nowStr)
		if err != nil {
			return fmt.Errorf("seed locker package %s: %w", p.id, err)
		}
	}
	return nil
}

// SeedRecipients creates 2 recipients: Georgetown (Guyana), Kingston (Jamaica).
func SeedRecipients(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	recipients := []struct {
		id, userID, name, destinationID, street, city string
	}{
		{RecipientGeorgetown, CustomerAliceID, "Georgetown Recipient", "guyana", "123 Main St", "Georgetown"},
		{RecipientKingston, CustomerAliceID, "Kingston Recipient", "jamaica", "456 Oak Ave", "Kingston"},
	}
	for _, r := range recipients {
		_, err := db.Exec(`
			INSERT INTO recipients (id, user_id, name, destination_id, street, city, is_default, use_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?, ?)
		`, r.id, r.userID, r.name, r.destinationID, r.street, r.city, now, now)
		if err != nil {
			return fmt.Errorf("seed recipient %s: %w", r.id, err)
		}
	}
	return nil
}

// SeedShipRequests creates 3 ship requests: draft, paid, shipped.
func SeedShipRequests(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	requests := []struct {
		id, userID, confirmationCode, status, destinationID, recipientID, serviceType string
		consolidate                                                                   int
		subtotal, total                                                               float64
	}{
		{ShipReqAliceDraft, CustomerAliceID, "QCS-DRAFT-001", "draft", "guyana", RecipientGeorgetown, "standard", 1, 0, 0},
		{ShipReqAlicePaid, CustomerAliceID, "QCS-PAID-001", "paid", "guyana", RecipientGeorgetown, "standard", 1, 17.50, 17.50},
		{ShipReqAliceShipped, CustomerAliceID, "QCS-SHIP-001", "shipped", "guyana", RecipientGeorgetown, "standard", 1, 17.50, 17.50},
	}
	for _, r := range requests {
		_, err := db.Exec(`
			INSERT INTO ship_requests
				(id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
				 consolidate, subtotal, service_fees, insurance, discount, total, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?, ?)
		`, r.id, r.userID, r.confirmationCode, r.status, r.destinationID, nullStr(r.recipientID), r.serviceType,
			r.consolidate, r.subtotal, r.total, now, now)
		if err != nil {
			return fmt.Errorf("seed ship request %s: %w", r.id, err)
		}
	}
	items := []struct{ id, shipReqID, lockerPkgID string }{
		{"sri_draft_1", ShipReqAliceDraft, PkgAliceStored1},
		{"sri_paid_1", ShipReqAlicePaid, PkgAliceStored2},
		{"sri_ship_1", ShipReqAliceShipped, PkgAliceStored2},
	}
	for _, it := range items {
		_, err := db.Exec(`
			INSERT INTO ship_request_items (id, ship_request_id, locker_package_id)
			VALUES (?, ?, ?)
		`, it.id, it.shipReqID, it.lockerPkgID)
		if err != nil {
			return fmt.Errorf("seed ship_request_item %s: %w", it.id, err)
		}
	}
	return nil
}

// SeedServiceRequests creates a pending photo request for PkgAliceService.
func SeedServiceRequests(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO service_requests (id, user_id, locker_package_id, service_type, status, price, created_at)
		VALUES (?, ?, ?, 'photo', 'pending', 5.00, ?)
	`, "sreq_photo_001", CustomerAliceID, PkgAliceService, now)
	if err != nil {
		return fmt.Errorf("seed service request: %w", err)
	}
	return nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
