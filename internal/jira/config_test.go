package jira

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAuthConfigEnvOverridesConfig(t *testing.T) {
	clearJiraEnv(t)

	configPath := writeRemudaConfig(t, t.TempDir(), `
version: 1
jira:
  endpoint: https://config.example.atlassian.net/
  user: config-user@example.com
  api_token: config-token
`)

	t.Setenv("REMUDA_JIRA_ENDPOINT", "  https://env.example.atlassian.net/// ")
	t.Setenv("REMUDA_JIRA_USER", "env-user@example.com")
	t.Setenv("REMUDA_JIRA_API_TOKEN", "env-api-token")

	got, err := loadAuthConfig(configPath, false, os.LookupEnv, os.ReadFile)
	require.NoError(t, err)

	assert.Equal(t, "https://env.example.atlassian.net", got.Endpoint)
	assert.Equal(t, "env-user@example.com", got.User)
	assert.Equal(t, "env-api-token", got.Token)
}


func TestLoadAuthConfigUsesConfigWhenEnvMissing(t *testing.T) {
	clearJiraEnv(t)

	configPath := writeRemudaConfig(t, t.TempDir(), `
version: 1
jira:
  endpoint: https://config-only.example.atlassian.net/
  user: config-only@example.com
  api_token: config-only-token
`)

	got, err := loadAuthConfig(configPath, false, os.LookupEnv, os.ReadFile)
	require.NoError(t, err)

	assert.Equal(t, "https://config-only.example.atlassian.net", got.Endpoint)
	assert.Equal(t, "config-only@example.com", got.User)
	assert.Equal(t, "config-only-token", got.Token)
}

func TestLoadAuthConfigAllowsEmptyConfigTokenWhenEnvProvidesToken(t *testing.T) {
	clearJiraEnv(t)

	configPath := writeRemudaConfig(t, t.TempDir(), `
version: 1
jira:
  endpoint: https://config-only.example.atlassian.net/
  user: config-only@example.com
  api_token: ""
`)
	t.Setenv("REMUDA_JIRA_API_TOKEN", "env-token")

	got, err := loadAuthConfig(configPath, false, os.LookupEnv, os.ReadFile)
	require.NoError(t, err)

	assert.Equal(t, "https://config-only.example.atlassian.net", got.Endpoint)
	assert.Equal(t, "config-only@example.com", got.User)
	assert.Equal(t, "env-token", got.Token)
}

func TestLoadAuthConfigMissingFieldsReturnsActionableError(t *testing.T) {
	clearJiraEnv(t)

	configPath := writeRemudaConfig(t, t.TempDir(), `
version: 1
jira:
  endpoint: https://only-endpoint.example.atlassian.net
`)

	_, err := loadAuthConfig(configPath, false, os.LookupEnv, os.ReadFile)
	require.Error(t, err)

	assert.ErrorContains(t, err, "missing Jira configuration fields: user, token")
	assert.ErrorContains(t, err, "REMUDA_JIRA_ENDPOINT")
	assert.ErrorContains(t, err, "REMUDA_JIRA_USER")
	assert.ErrorContains(t, err, "REMUDA_JIRA_API_TOKEN")
	assert.ErrorContains(t, err, "jira.endpoint, jira.user, jira.api_token")
	assert.ErrorContains(t, err, "Config path:")
	assert.NotContains(t, err.Error(), "token=")
}

func TestLoadAuthConfigStrictConfigPathMissingReturnsError(t *testing.T) {
	clearJiraEnv(t)

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := loadAuthConfig(missing, true, os.LookupEnv, os.ReadFile)
	require.Error(t, err)
	assert.ErrorContains(t, err, "read Remuda config")
	assert.ErrorContains(t, err, missing)
}

func TestDiscoverRemudaConfigPathUsesXDGThenLegacy(t *testing.T) {
	t.Run("uses XDG config when present", func(t *testing.T) {
		home := t.TempDir()
		xdgHome := filepath.Join(t.TempDir(), "xdg")
		configPath := filepath.Join(xdgHome, configPathSuffix)
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
		require.NoError(t, os.WriteFile(configPath, []byte("version: 1\n"), 0o644))

		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", xdgHome)
		t.Setenv("REMUDA_CONFIG", "")

		path, strict, err := discoverRemudaConfigPath(os.LookupEnv, os.UserHomeDir)
		require.NoError(t, err)
		require.False(t, strict)
		require.Equal(t, configPath, path)
	})

	t.Run("falls back to legacy path when xdg missing", func(t *testing.T) {
		home := t.TempDir()
		legacyPath := filepath.Join(home, legacyConfigPathSuffix)
		require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0o755))
		require.NoError(t, os.WriteFile(legacyPath, []byte("version: 1\n"), 0o644))

		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("REMUDA_CONFIG", "")

		path, strict, err := discoverRemudaConfigPath(os.LookupEnv, os.UserHomeDir)
		require.NoError(t, err)
		require.False(t, strict)
		require.Equal(t, legacyPath, path)
	})
}

func TestDiscoverRemudaConfigPathExpandsStrictOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REMUDA_CONFIG", "~/configs/custom.yaml")
	t.Setenv("XDG_CONFIG_HOME", "")

	path, strict, err := discoverRemudaConfigPath(os.LookupEnv, os.UserHomeDir)
	require.NoError(t, err)
	require.True(t, strict)
	require.Equal(t, filepath.Join(home, "configs", "custom.yaml"), path)
}

func clearJiraEnv(t *testing.T) {
	t.Helper()
	t.Setenv("REMUDA_JIRA_ENDPOINT", "")
	t.Setenv("REMUDA_JIRA_USER", "")
	t.Setenv("REMUDA_JIRA_API_TOKEN", "")
	t.Setenv("REMUDA_JIRA_TOKEN", "")
}

func writeRemudaConfig(t *testing.T, root string, content string) string {
	t.Helper()
	configPath := filepath.Join(root, "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))
	return configPath
}
