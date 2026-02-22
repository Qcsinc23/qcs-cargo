package testdata

import (
	"testing"
)

func TestNewSeededDB(t *testing.T) {
	db := NewSeededDB(t)
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	if err != nil {
		t.Fatalf("query users: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 users, got %d", n)
	}
	err = db.QueryRow("SELECT COUNT(*) FROM locker_packages WHERE user_id = ?", CustomerAliceID).Scan(&n)
	if err != nil {
		t.Fatalf("query locker_packages: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 locker packages for Alice, got %d", n)
	}
}
