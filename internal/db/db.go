package db

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	conn *sql.DB
	once sync.Once
)

// Connect opens the database and enables WAL mode (SQLite).
func Connect(databaseURL string) error {
	var err error
	once.Do(func() {
		conn, err = sql.Open("sqlite", databaseURL)
		if err != nil {
			return
		}
		_, _ = conn.Exec("PRAGMA journal_mode=WAL")
		_, _ = conn.Exec("PRAGMA foreign_keys=ON")
		_, _ = conn.Exec("PRAGMA busy_timeout=5000")
	})
	return err
}

// DB returns the global connection. Panics if Connect was not called.
func DB() *sql.DB {
	if conn == nil {
		panic("db: Connect must be called first")
	}
	return conn
}

// Close closes the global connection.
func Close() error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// Ping checks connectivity.
func Ping() error {
	if conn == nil {
		return nil
	}
	return conn.Ping()
}

// SetConnForTest injects a connection for integration tests. Call before building
// the API app so handlers use this DB. Only use in tests; do not call in production.
func SetConnForTest(c *sql.DB) {
	conn = c
	queries = nil
}
