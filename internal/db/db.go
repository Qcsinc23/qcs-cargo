package db

import (
	"database/sql"
	"log"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	mu   sync.RWMutex
	conn *sql.DB
)

// Connect opens the database and enables WAL mode (SQLite).
func Connect(databaseURL string) error {
	newConn, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return err
	}

	if _, pragmaErr := newConn.Exec("PRAGMA journal_mode=WAL"); pragmaErr != nil {
		log.Printf("db: failed to set PRAGMA journal_mode=WAL: %v", pragmaErr)
	}
	if _, pragmaErr := newConn.Exec("PRAGMA foreign_keys=ON"); pragmaErr != nil {
		log.Printf("db: failed to set PRAGMA foreign_keys=ON: %v", pragmaErr)
	}
	if _, pragmaErr := newConn.Exec("PRAGMA busy_timeout=5000"); pragmaErr != nil {
		log.Printf("db: failed to set PRAGMA busy_timeout=5000: %v", pragmaErr)
	}

	mu.Lock()
	oldConn := conn
	conn = newConn
	queries = nil
	mu.Unlock()

	if oldConn != nil && oldConn != newConn {
		_ = oldConn.Close()
	}
	return nil
}

// DB returns the global connection. Panics if Connect was not called.
func DB() *sql.DB {
	mu.RLock()
	c := conn
	mu.RUnlock()
	if c == nil {
		panic("db: Connect must be called first")
	}
	return c
}

// Close closes the global connection.
func Close() error {
	mu.Lock()
	oldConn := conn
	conn = nil
	queries = nil
	mu.Unlock()

	if oldConn == nil {
		return nil
	}
	return oldConn.Close()
}

// Ping checks connectivity.
func Ping() error {
	mu.RLock()
	c := conn
	mu.RUnlock()
	if c == nil {
		return nil
	}
	return c.Ping()
}

// SetConnForTest injects a connection for integration tests. Call before building
// the API app so handlers use this DB. Only use in tests; do not call in production.
func SetConnForTest(c *sql.DB) {
	mu.Lock()
	oldConn := conn
	conn = c
	queries = nil
	mu.Unlock()
	if oldConn != nil && oldConn != c {
		_ = oldConn.Close()
	}
}
