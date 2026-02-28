package db

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMigrationsDir(t *testing.T) string {
	t.Helper()
	for _, rel := range []string{"sql/migrations", "../sql/migrations", "../../sql/migrations"} {
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
	}
	t.Fatalf("sql/migrations directory not found from test cwd")
	return ""
}

func TestMigrateAndRollbackAllMigrations(t *testing.T) {
	conn, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	defer conn.Close()

	dir := testMigrationsDir(t)
	require.NoError(t, Migrate(conn, dir))

	usersExists, err := tableExists(conn, "users")
	require.NoError(t, err)
	assert.True(t, usersExists)

	// Seed representative rows to validate rollback works on non-empty schemas.
	_, err = conn.Exec(`
		INSERT INTO users (id, name, email, role, free_storage_days, email_verified, status, created_at, updated_at)
		VALUES ('usr_seed_1', 'Seed User', 'seed@example.com', 'customer', 30, 1, 'active', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`)
	require.NoError(t, err)
	_, err = conn.Exec(`
		INSERT INTO recipients (id, user_id, name, destination_id, street, city, created_at, updated_at)
		VALUES ('rcpt_seed_1', 'usr_seed_1', 'Seed Recipient', 'guyana', '123 Main St', 'Georgetown', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`)
	require.NoError(t, err)

	require.NoError(t, RollbackMigrations(conn, dir, 0))

	usersExists, err = tableExists(conn, "users")
	require.NoError(t, err)
	assert.False(t, usersExists)

	var count int
	err = conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
