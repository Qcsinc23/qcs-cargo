// Package testdata provides a test database helper that creates
// a fresh in-memory SQLite database with all migrations applied.
package testdata

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
)

// migrationsDir returns the path to sql/migrations, from repo root or package dir.
func migrationsDir() string {
	for _, rel := range []string{"sql/migrations", "../sql/migrations", "../../sql/migrations"} {
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
	}
	return "sql/migrations"
}

// NewTestDB creates a fresh in-memory SQLite database with all
// migrations applied. The database is closed when the test ends.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	// Enable pragmas for compatibility with migrations
	_, _ = conn.Exec("PRAGMA foreign_keys=ON")
	_, _ = conn.Exec("PRAGMA busy_timeout=5000")
	dir := migrationsDir()
	if err := db.Migrate(conn, dir); err != nil {
		conn.Close()
		t.Fatalf("failed to migrate test db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// NewSeededDB creates a test database with all seed data loaded.
func NewSeededDB(t *testing.T) *sql.DB {
	t.Helper()
	conn := NewTestDB(t)
	if err := SeedAll(conn); err != nil {
		t.Fatalf("failed to seed test db: %v", err)
	}
	return conn
}
