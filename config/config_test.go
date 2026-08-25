package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalWithEnv(t *testing.T) {
	t.Setenv("TEST_API_KEY", "!random")
	t.Setenv("TEST_PASSWORD", `p#ss: *word"x`)
	t.Setenv("TEST_HOST", "myhost")
	t.Setenv("TEST_INTERVAL", "30s")

	data := []byte(`
projects:
  - name: default
    apiKeys:
      - key: ${TEST_API_KEY}
global_prometheus:
  url: http://$TEST_HOST:9090/prom
  refresh_interval: $TEST_INTERVAL
  password: ${TEST_PASSWORD}
`)
	cfg := NewConfig()
	require.NoError(t, unmarshalWithEnv(data, cfg))

	// values coming from the environment are used verbatim, even if they
	// contain YAML-special characters (see #692)
	require.Len(t, cfg.Projects, 1)
	require.Len(t, cfg.Projects[0].ApiKeys, 1)
	assert.Equal(t, "!random", cfg.Projects[0].ApiKeys[0].Key)
	assert.Equal(t, `p#ss: *word"x`, cfg.GlobalPrometheus.Password)

	// variables inside a larger scalar and in non-string fields still work
	assert.Equal(t, "http://myhost:9090/prom", cfg.GlobalPrometheus.Url)
	assert.Equal(t, "30s", cfg.GlobalPrometheus.RefreshInterval.String())
}

func TestUnmarshalWithEnvNoVariables(t *testing.T) {
	data := []byte(`
listen_address: ":8080"
projects:
  - name: default
    apiKeys:
      - key: static-key
`)
	cfg := NewConfig()
	require.NoError(t, unmarshalWithEnv(data, cfg))
	assert.Equal(t, ":8080", cfg.ListenAddress)
	assert.Equal(t, "static-key", cfg.Projects[0].ApiKeys[0].Key)
}

func TestUnmarshalWithEnvEmptyDocument(t *testing.T) {
	for _, data := range []string{"", "\n", "# only a comment\n"} {
		cfg := NewConfig()
		require.NoError(t, unmarshalWithEnv([]byte(data), cfg))
	}
}

func TestUnmarshalWithEnvUndefinedVariable(t *testing.T) {
	data := []byte(`
projects:
  - name: default
    apiKeys:
      - key: ${TEST_UNDEFINED_VAR}
`)
	cfg := NewConfig()
	require.NoError(t, unmarshalWithEnv(data, cfg))
	// undefined variables expand to an empty string, as before
	assert.Equal(t, "", cfg.Projects[0].ApiKeys[0].Key)
}

func TestLoadExpandsEnvInValues(t *testing.T) {
	// mirrors the coroot-operator flow: the config references a secret-backed
	// API key via an environment variable (see #692)
	t.Setenv("COROOT_CONFIG_SECRET_AB12CD34", "!random")

	f := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
projects:
  - name: default
    apiKeys:
      - key: ${COROOT_CONFIG_SECRET_AB12CD34}
        description: default
`)
	require.NoError(t, os.WriteFile(f, data, 0o600))
	prev := *configFile
	*configFile = f
	defer func() { *configFile = prev }()

	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.Projects, 1)
	require.Len(t, cfg.Projects[0].ApiKeys, 1)
	assert.Equal(t, "!random", cfg.Projects[0].ApiKeys[0].Key)
}
