package forms

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/assert"
)

func newPrometheusConfig() db.IntegrationPrometheus {
	return db.IntegrationPrometheus{
		Url:            "http://prom:9090",
		BasicAuth:      &utils.BasicAuth{User: "u", Password: "p"},
		CustomHeaders:  []utils.Header{{Key: "k", Value: "v"}},
		RemoteWriteUrl: "http://prom:9090/api/v1/write",
	}
}

func TestIntegrationFormPrometheusGetMasked(t *testing.T) {
	project := &db.Project{Prometheus: newPrometheusConfig()}

	f := NewIntegrationForm(db.IntegrationTypePrometheus, nil, nil)
	f.Get(project, true)

	assert.Equal(t, newPrometheusConfig(), project.Prometheus)

	assertPrometheusMasked(t, f)
}

func TestIntegrationFormPrometheusGetMaskedGlobal(t *testing.T) {
	global := newPrometheusConfig()

	f := NewIntegrationForm(db.IntegrationTypePrometheus, nil, &global)
	f.Get(&db.Project{}, true)

	assert.Equal(t, newPrometheusConfig(), global)

	assertPrometheusMasked(t, f)
}

func assertPrometheusMasked(t *testing.T, f IntegrationForm) {
	t.Helper()
	pf := f.(*IntegrationFormPrometheus)
	assert.Equal(t, "<hidden>", pf.BasicAuth.User)
	assert.Equal(t, "<hidden>", pf.BasicAuth.Password)
	assert.Equal(t, "<hidden>", pf.CustomHeaders[0].Value)
}
