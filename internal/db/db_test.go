package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func openSQLiteConn(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	c, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	return c
}

func TestConnectAllowsReconnectAndResetsQueries(t *testing.T) {
	_ = Close()
	t.Cleanup(func() { _ = Close() })

	db1 := filepath.Join(t.TempDir(), "first.db")
	db2 := filepath.Join(t.TempDir(), "second.db")

	require.NoError(t, Connect("file:"+db1+"?_journal_mode=WAL"))
	conn1 := DB()
	q1 := Queries()

	require.NoError(t, Connect("file:"+db2+"?_journal_mode=WAL"))
	conn2 := DB()
	q2 := Queries()

	assert.NotSame(t, conn1, conn2)
	assert.NotSame(t, q1, q2)
	assert.Error(t, conn1.Ping(), "previous connection should be closed after reconnect")
	require.NoError(t, conn2.Ping())
}

func TestSetConnForTestReplacesConnectionAndQueries(t *testing.T) {
	_ = Close()
	t.Cleanup(func() { _ = Close() })

	conn1 := openSQLiteConn(t, "file:set_conn_for_test_1?mode=memory&cache=shared")
	conn2 := openSQLiteConn(t, "file:set_conn_for_test_2?mode=memory&cache=shared")
	t.Cleanup(func() {
		_ = conn1.Close()
		_ = conn2.Close()
	})

	SetConnForTest(conn1)
	q1 := Queries()

	SetConnForTest(conn2)
	q2 := Queries()

	assert.Same(t, conn2, DB())
	assert.NotSame(t, q1, q2)
	assert.Error(t, conn1.Ping(), "old test connection should be closed when replaced")
	require.NoError(t, conn2.Ping())
}

func TestCloseClearsGlobalConnectionState(t *testing.T) {
	_ = Close()

	conn := openSQLiteConn(t, "file:close_reset_test?mode=memory&cache=shared")
	SetConnForTest(conn)
	require.NoError(t, Close())

	assert.Panics(t, func() { DB() })
	assert.Panics(t, func() { Queries() })
}

