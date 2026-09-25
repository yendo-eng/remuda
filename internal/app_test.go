package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestConfigFromEnvDefaultsToHomeRemuda(t *testing.T) {
	t.Setenv("REMUDA_REPOS_BASE_DIR", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := ConfigFromEnv()
	require.Equal(t, filepath.Join(tmpHome, ".remuda", "repos"), cfg.ReposBaseDir)
}

func TestConfigFromEnvRespectsOverride(t *testing.T) {
	overrideDir := t.TempDir()
	t.Setenv("REMUDA_REPOS_BASE_DIR", overrideDir)

	cfg := ConfigFromEnv()
	require.Equal(t, overrideDir, cfg.ReposBaseDir)
}

func TestConfigFromEnvWithProvider_DefaultsToProviderHome(t *testing.T) {
	provider := env.StaticProvider{
		Values:  map[string]string{},
		HomeDir: t.TempDir(),
	}

	cfg := ConfigFromEnvWithProvider(provider)
	require.Equal(t, filepath.Join(provider.HomeDir, ".remuda", "repos"), cfg.ReposBaseDir)
}

func TestConfigFromEnvWithProvider_UsesFallbackWhenHomeUnavailable(t *testing.T) {
	provider := env.StaticProvider{
		Values:  map[string]string{},
		HomeErr: env.ErrHomeDirUnavailable,
	}

	cfg := ConfigFromEnvWithProvider(provider)
	require.Equal(t, "./repos", cfg.ReposBaseDir)
}
