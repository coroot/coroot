package db

import (
	"fmt"
	"sync"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const concurrency = 20

func runConcurrently(n int, f func(i int) error) []error {
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f(i)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func TestSetSetting(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *DB) {
		require.NoError(t, db.SetSetting("s", "v1"))
		require.NoError(t, db.SetSetting("s", "v2"))
		var v string
		require.NoError(t, db.GetSetting("s", &v))
		assert.Equal(t, "v2", v)

		for round := 0; round < 5; round++ {
			name := fmt.Sprintf("concurrent-%d", round)
			for _, err := range runConcurrently(concurrency, func(i int) error { return db.SetSetting(name, i) }) {
				require.NoError(t, err)
			}
			var got int
			require.NoError(t, db.GetSetting(name, &got))
		}
	})
}

func TestSaveApplicationSettingConcurrently(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *DB) {
		createTestProject(t, db, "p1")
		for round := 0; round < 5; round++ {
			appId := model.NewApplicationId("", "default", model.ApplicationKindDeployment, fmt.Sprintf("app-%d", round))
			errs := runConcurrently(concurrency, func(i int) error {
				if i%2 == 0 {
					return db.SaveApplicationSetting("p1", appId, &model.ApplicationSettingsProfiling{Service: "profiling"})
				}
				return db.SaveApplicationSetting("p1", appId, &model.ApplicationSettingsTracing{Service: "tracing"})
			})
			for _, err := range errs {
				require.NoError(t, err)
			}
			s, err := db.GetApplicationSettings("p1", appId)
			require.NoError(t, err)
			require.NotNil(t, s)
			// concurrent updates of different fields must not overwrite each other
			require.NotNil(t, s.Profiling)
			require.NotNil(t, s.Tracing)
			assert.Equal(t, "profiling", s.Profiling.Service)
			assert.Equal(t, "tracing", s.Tracing.Service)
		}
	})
}

func TestSaveApplicationSettingUnsupportedType(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *DB) {
		createTestProject(t, db, "p1")
		appId := model.NewApplicationId("", "default", model.ApplicationKindDeployment, "app")
		require.Error(t, db.SaveApplicationSetting("p1", appId, "unsupported"))
		s, err := db.GetApplicationSettings("p1", appId)
		require.NoError(t, err)
		assert.Nil(t, s, "a failed save must not leave a row behind")
	})
}

func TestUserApiKeys(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *DB) {
		id, err := db.AddServiceAccount("sa", "Service Account", rbac.RoleViewer)
		require.NoError(t, err)

		descriptions := func() []string {
			keys, err := db.GetUserApiKeys(id)
			require.NoError(t, err)
			var res []string
			for _, k := range keys {
				res = append(res, k.Description)
			}
			return res
		}

		require.NoError(t, db.SetUserApiKeys(id, []ApiKey{{Key: "k1", Description: "one"}, {Key: "k2", Description: "two"}}))
		assert.ElementsMatch(t, []string{"one", "two"}, descriptions())

		require.NoError(t, db.SetUserApiKeys(id, []ApiKey{{Key: "k2", Description: "two"}, {Key: "k3", Description: "three"}}))
		assert.ElementsMatch(t, []string{"two", "three"}, descriptions())

		// a conflicting key must leave the existing keys untouched
		err = db.SetUserApiKeys(id, []ApiKey{{Key: "k4", Description: "dup"}, {Key: "k5", Description: "dup"}})
		assert.ErrorIs(t, err, ErrConflict)
		assert.ElementsMatch(t, []string{"two", "three"}, descriptions())

		u, err := db.GetUserByApiKey("k2")
		require.NoError(t, err)
		assert.Equal(t, id, u.Id)

		require.NoError(t, db.DeleteUser(id))
		_, err = db.GetUserByApiKey("k2")
		assert.ErrorIs(t, err, ErrNotFound)
		var n int
		require.NoError(t, db.db.QueryRow("SELECT count(*) FROM user_api_keys WHERE user_id = $1", id).Scan(&n))
		assert.Equal(t, 0, n)
	})
}
