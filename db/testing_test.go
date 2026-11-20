package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// forEachDB runs f against a freshly migrated SQLite database and, if COROOT_TEST_POSTGRES_DSN is set,
// against a Postgres database isolated in its own schema.
func forEachDB(t *testing.T, f func(t *testing.T, db *DB)) {
	t.Run("sqlite", func(t *testing.T) {
		db, err := NewSqlite(t.TempDir(), false)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.DB().Close() })
		require.NoError(t, db.Migrate())
		f(t, db)
	})
	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("COROOT_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("COROOT_TEST_POSTGRES_DSN is not set")
		}
		admin, err := sql.Open("postgres", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { _ = admin.Close() })
		schema := fmt.Sprintf("coroot_test_%d", time.Now().UnixNano())
		_, err = admin.Exec("CREATE SCHEMA " + schema)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE") })

		db, err := NewPostgres(withSearchPath(t, dsn, schema))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.DB().Close() })
		require.NoError(t, db.Migrate())
		f(t, db)
	})
}

func withSearchPath(t *testing.T, dsn, schema string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String()
	}
	return dsn + " search_path=" + schema
}

func createTestProject(t *testing.T, db *DB, id ProjectId) {
	_, err := db.db.Exec("INSERT INTO project (id, name) VALUES ($1, $2)", id, string(id))
	require.NoError(t, err)
}
