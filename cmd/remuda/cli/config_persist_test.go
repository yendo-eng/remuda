package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestPersistDefaultRepoSelection_WritesNewXDGConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	xdg := filepath.Join(tmp, "xdg")
	env := EnvMap{
		"XDG_CONFIG_HOME": xdg,
		"REMUDA_CONFIG":   "",
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	discovery, err := persistDefaultRepoSelection(ctx, "remuda", "")
	require.NoError(t, err)

	expected := filepath.Join(xdg, "remuda", "config.yaml")
	require.Equal(t, expected, discovery.Path)
	require.Equal(t, ConfigFileSourceXDG, discovery.Source)

	cfg := readConfigV1(t, expected)
	require.Equal(t, configfile.Version1, cfg.Version)
	require.NotNil(t, cfg.Repos)
	requireStringPtr(t, cfg.Repos.DefaultRepo, "remuda")
	require.Nil(t, cfg.Repos.DefaultRepoURL)
}

func TestPersistDefaultRepoSelection_PreservesExistingFields(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	xdg := filepath.Join(tmp, "xdg")
	configPath := filepath.Join(xdg, "remuda", "config.yaml")
	env := EnvMap{
		"XDG_CONFIG_HOME": xdg,
		"REMUDA_CONFIG":   "",
	}

	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`version: 1
repos:
  base_dir: /tmp/repos
  default_repo: legacy
  aliases:
    custom: https://github.com/acme/custom.git
defaults:
  agent: codex
`), 0o644))

	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	discovery, err := persistDefaultRepoSelection(ctx, "", "https://github.com/acme/new.git")
	require.NoError(t, err)
	require.Equal(t, configPath, discovery.Path)

	cfg := readConfigV1(t, configPath)
	require.NotNil(t, cfg.Defaults)
	requireStringPtr(t, cfg.Defaults.Agent, "codex")
	require.NotNil(t, cfg.Repos)
	requireStringPtr(t, cfg.Repos.BaseDir, "/tmp/repos")
	require.Equal(t, map[string]string{"custom": "https://github.com/acme/custom.git"}, cfg.Repos.Aliases)
	require.Nil(t, cfg.Repos.DefaultRepo)
	requireStringPtr(t, cfg.Repos.DefaultRepoURL, "https://github.com/acme/new.git")
}

func TestPersistDefaultRepoSelection_PreservesComments(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	xdg := filepath.Join(tmp, "xdg")
	configPath := filepath.Join(xdg, "remuda", "config.yaml")
	env := EnvMap{
		"XDG_CONFIG_HOME": xdg,
		"REMUDA_CONFIG":   "",
	}

	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`# top-level comment
version: 1
repos:
  # default repo comment
  default_repo: legacy
`), 0o644))

	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	_, err := persistDefaultRepoSelection(ctx, "", "https://github.com/acme/new.git")
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(updated), "# top-level comment")
	require.Contains(t, string(updated), "# default repo comment")
	require.Contains(t, string(updated), "default_repo_url: https://github.com/acme/new.git")
	require.NotContains(t, string(updated), "default_repo: legacy")
}

func TestPersistDefaultRepoSelection_WritesLegacyWhenOnlyConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	legacy := filepath.Join(tmp, ".remuda", "config.yaml")
	env := EnvMap{
		"REMUDA_CONFIG": "",
	}

	require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o755))
	require.NoError(t, os.WriteFile(legacy, []byte("version: 1\n"), 0o644))

	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	discovery, err := persistDefaultRepoSelection(ctx, "widgets", "")
	require.NoError(t, err)
	require.Equal(t, legacy, discovery.Path)
	require.Equal(t, ConfigFileSourceLegacy, discovery.Source)

	cfg := readConfigV1(t, legacy)
	require.NotNil(t, cfg.Repos)
	requireStringPtr(t, cfg.Repos.DefaultRepo, "widgets")
	require.Nil(t, cfg.Repos.DefaultRepoURL)
}

func TestPersistDefaultRepoSelection_CreatesMissingOverrideConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	override := filepath.Join(tmp, "override", "config.yaml")
	env := EnvMap{
		"REMUDA_CONFIG": override,
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	discovery, err := persistDefaultRepoSelection(ctx, "", "https://github.com/acme/custom.git")
	require.NoError(t, err)
	require.Equal(t, override, discovery.Path)
	require.True(t, discovery.Strict)
	require.Equal(t, ConfigFileSourceEnv, discovery.Source)

	cfg := readConfigV1(t, override)
	require.NotNil(t, cfg.Repos)
	require.Nil(t, cfg.Repos.DefaultRepo)
	requireStringPtr(t, cfg.Repos.DefaultRepoURL, "https://github.com/acme/custom.git")
}

func TestPersistDefaultRepoSelection_FailsOnOverrideDirectory(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	override := filepath.Join(tmp, "configdir")
	env := EnvMap{
		"REMUDA_CONFIG": override,
	}

	require.NoError(t, os.MkdirAll(override, 0o755))

	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	_, err := persistDefaultRepoSelection(ctx, "remuda", "")
	require.Error(t, err)
}

func TestPersistDefaultRepoSelection_FailsOnUnreadableOverrideFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	override := filepath.Join(tmp, "config.yaml")
	env := EnvMap{
		"REMUDA_CONFIG": override,
	}

	require.NoError(t, os.WriteFile(override, []byte("version: 1\n"), 0o600))
	require.NoError(t, os.Chmod(override, 0o000))
	t.Cleanup(func() {
		_ = os.Chmod(override, 0o600)
	})

	f, err := os.Open(override)
	if err == nil {
		_ = f.Close()
		t.Skip("filesystem permissions allow reading override file; cannot simulate unreadable file")
	}

	ctx := newTestContextWithEnv(t, env, WithHomeDir(tmp), WithWorkingDir(tmp))
	_, err = persistDefaultRepoSelection(ctx, "remuda", "")
	require.Error(t, err)
}

func readConfigV1(t *testing.T, path string) *configfile.V1 {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	cfg, err := configfile.ParseV1(data)
	require.NoError(t, err)
	return cfg
}

func requireStringPtr(t *testing.T, ptr *string, expected string) {
	t.Helper()
	require.NotNil(t, ptr)
	require.Equal(t, expected, *ptr)
}
