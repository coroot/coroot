package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddPostgresConnectTimeout(t *testing.T) {
	result, err := addPostgresConnectTimeout("postgres://user:password@localhost:5432/dbname")
	assert.NoError(t, err)
	assert.Equal(t, "postgres://user:password@localhost:5432/dbname?connect_timeout=30", result)

	result, err = addPostgresConnectTimeout("postgres://user:password@localhost:5432/dbname?connect_timeout=60")
	assert.NoError(t, err)
	assert.Equal(t, "postgres://user:password@localhost:5432/dbname?connect_timeout=60", result)

	result, err = addPostgresConnectTimeout("postgres://user:password@localhost:5432/dbname?sslmode=require")
	assert.NoError(t, err)
	assert.Equal(t, "postgres://user:password@localhost:5432/dbname?connect_timeout=30&sslmode=require", result)

	result, err = addPostgresConnectTimeout("postgres://user:password@localhost:5432/dbname?sslmode=require&connect_timeout=45")
	assert.NoError(t, err)
	assert.Equal(t, "postgres://user:password@localhost:5432/dbname?connect_timeout=45&sslmode=require", result)

	result, err = addPostgresConnectTimeout("host=localhost user=coroot password=secret dbname=coroot sslmode=require")
	assert.NoError(t, err)
	assert.Equal(t, "host=localhost user=coroot password=secret dbname=coroot sslmode=require connect_timeout=30", result)

	result, err = addPostgresConnectTimeout("host=localhost user=coroot password=secret dbname=coroot sslmode=require connect_timeout=60")
	assert.NoError(t, err)
	assert.Equal(t, "host=localhost user=coroot password=secret dbname=coroot sslmode=require connect_timeout=60", result)

	result, err = addPostgresConnectTimeout("postgres://invalid url with spaces")
	assert.Error(t, err)
}
