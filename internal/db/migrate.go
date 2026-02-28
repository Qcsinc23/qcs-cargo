package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const schemaMigrationsTable = "schema_migrations"

// RunMigrations executes all *.sql files in dir in lexicographic order on the global DB.
// Each file may contain "-- +goose Up" and "-- +goose Down"; only the Up block is executed.
func RunMigrations(dir string) error {
	return Migrate(DB(), dir)
}

// RunMigrationsDown executes down migrations in reverse lexicographic order on the
// global DB. If steps <= 0, all recorded migrations are rolled back.
func RunMigrationsDown(dir string, steps int) error {
	return RollbackMigrations(DB(), dir, steps)
}

// Migrate runs all *.sql migration files in dir against the given connection.
// Used by tests (e.g. testdata.NewTestDB) to migrate an in-memory DB.
func Migrate(conn *sql.DB, dir string) error {
	if err := ensureSchemaMigrationsTable(conn); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		applied, err := migrationRecorded(conn, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		up := extractGooseUp(string(raw))
		if up == "" {
			continue
		}
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		for i, stmt := range splitStatements(up) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				reconciled, recErr := migrationLooksAppliedBySchema(conn, name)
				if recErr != nil {
					return recErr
				}
				if reconciled {
					log.Printf("migrate: marking %s as already applied based on existing schema", name)
					if err := recordMigration(conn, name); err != nil {
						return err
					}
					goto nextMigration
				}
				return fmt.Errorf("migration %s statement %d: %w", name, i+1, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if err := recordMigration(conn, name); err != nil {
			return err
		}
	nextMigration:
	}
	return nil
}

// RollbackMigrations applies down blocks in reverse order for already-recorded
// migrations. If steps <= 0, all recorded migrations are rolled back.
func RollbackMigrations(conn *sql.DB, dir string, steps int) error {
	if err := ensureSchemaMigrationsTable(conn); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	rolledBack := 0
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		applied, err := migrationRecorded(conn, name)
		if err != nil {
			return err
		}
		if !applied {
			continue
		}

		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		down := extractGooseDown(string(raw))
		if strings.TrimSpace(down) == "" {
			return fmt.Errorf("migration %s has no down block", name)
		}

		for j, stmt := range splitStatements(down) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := conn.Exec(stmt); err != nil {
				return fmt.Errorf("rollback %s statement %d: %w", name, j+1, err)
			}
		}
		if err := deleteMigrationRecord(conn, name); err != nil {
			return err
		}
		rolledBack++
		if steps > 0 && rolledBack >= steps {
			break
		}
	}
	return nil
}

func ensureSchemaMigrationsTable(conn *sql.DB) error {
	_, err := conn.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	name TEXT PRIMARY KEY,
	applied_at TEXT NOT NULL
);`)
	return err
}

func migrationRecorded(conn *sql.DB, name string) (bool, error) {
	var one int
	err := conn.QueryRow("SELECT 1 FROM "+schemaMigrationsTable+" WHERE name = ? LIMIT 1", name).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func recordMigration(conn *sql.DB, name string) error {
	_, err := conn.Exec(
		"INSERT OR IGNORE INTO "+schemaMigrationsTable+"(name, applied_at) VALUES (?, ?)",
		name,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func deleteMigrationRecord(conn *sql.DB, name string) error {
	_, err := conn.Exec("DELETE FROM "+schemaMigrationsTable+" WHERE name = ?", name)
	return err
}

// migrationLooksAppliedBySchema is a safe fallback for older environments that
// had schema changes applied before migration tracking existed.
func migrationLooksAppliedBySchema(conn *sql.DB, name string) (bool, error) {
	switch name {
	case "20260221120000_initial_schema.sql":
		return tableExists(conn, "users")
	case "20260221130000_password_resets.sql":
		return tableExists(conn, "password_resets")
	case "20260221140000_users_password_hash.sql":
		return columnExists(conn, "users", "password_hash")
	case "20260222100000_warehouse_phase4.sql":
		a, err := columnExists(conn, "ship_requests", "consolidated_weight_lbs")
		if err != nil {
			return false, err
		}
		b, err := columnExists(conn, "ship_requests", "staging_bay")
		if err != nil {
			return false, err
		}
		c, err := columnExists(conn, "ship_requests", "manifest_id")
		if err != nil {
			return false, err
		}
		d, err := columnExists(conn, "locker_packages", "booking_id")
		if err != nil {
			return false, err
		}
		e, err := tableExists(conn, "warehouse_bays")
		if err != nil {
			return false, err
		}
		f, err := tableExists(conn, "warehouse_manifests")
		if err != nil {
			return false, err
		}
		g, err := tableExists(conn, "warehouse_manifest_ship_requests")
		if err != nil {
			return false, err
		}
		return a && b && c && d && e && f && g, nil
	default:
		return false, nil
	}
}

func tableExists(conn *sql.DB, table string) (bool, error) {
	var one int
	err := conn.QueryRow(
		"SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1",
		table,
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func columnExists(conn *sql.DB, table string, column string) (bool, error) {
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func quoteSQLiteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func splitStatements(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t+";")
		}
	}
	return out
}

func extractGooseUp(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inUp := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "-- +goose Up" {
			inUp = true
			continue
		}
		if inUp && strings.TrimSpace(line) == "-- +goose Down" {
			break
		}
		if inUp {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func extractGooseDown(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inDown := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "-- +goose Down" {
			inDown = true
			continue
		}
		if inDown {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
