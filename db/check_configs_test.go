package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCheckConfigsClosesRowsOnInvalidJSON(t *testing.T) {
	database, err := NewSqlite(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, database.DB().Close())
	})

	_, err = database.Exec(`CREATE TABLE check_configs (project_id TEXT, application_id TEXT, configs TEXT)`)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO check_configs VALUES ($1, $2, $3)`, "test", "default:Deployment:test", "{")
	require.NoError(t, err)

	_, err = database.GetCheckConfigs("test")
	require.Error(t, err)
	assert.Zero(t, database.DB().Stats().InUse, "the failed read must return its connection to the pool")
}
