package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestDiscoverConfigFile_RemudaConfigStrict(t *testing.T) {
	t.Parallel()
	t.Run("missing override path errors (even if other config exists)", func(t *testing.T) {
		home := t.TempDir()

		legacy := filepath.Join(home, ".remuda", "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o755))
		require.NoError(t, os.WriteFile(legacy, []byte("version: 1\n"), 0o644))

		override := filepath.Join(home, "missing.yaml")
		env := cli.EnvMap{
			"REMUDA_CONFIG":   override,
			"XDG_CONFIG_HOME": "",
		}
		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

		_, err := cli.DiscoverConfigFile(ctx)
		require.Error(t, err)
		require.ErrorContains(t, err, "does not exist")
	})

	t.Run("directory override path errors", func(t *testing.T) {
		home := t.TempDir()

		overrideDir := filepath.Join(home, "configdir")
		require.NoError(t, os.MkdirAll(overrideDir, 0o755))
		env := cli.EnvMap{
			"REMUDA_CONFIG":   overrideDir,
			"XDG_CONFIG_HOME": "",
		}
		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

		_, err := cli.DiscoverConfigFile(ctx)
		require.Error(t, err)
		require.ErrorContains(t, err, "is a directory")
	})

	t.Run("tilde paths are expanded", func(t *testing.T) {
		home := t.TempDir()

		override := filepath.Join(home, "config.yaml")
		require.NoError(t, os.WriteFile(override, []byte("version: 1\n"), 0o644))
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "~/config.yaml",
			"XDG_CONFIG_HOME": "",
		}
		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

		cfg, err := cli.DiscoverConfigFile(ctx)
		require.NoError(t, err)
		require.True(t, cfg.Strict)
		require.Equal(t, override, cfg.Path)
		require.Equal(t, cli.ConfigFileSourceEnv, cfg.Source)
	})
}

func TestDiscoverConfigFile_DefaultSearchOrder(t *testing.T) {
	t.Parallel()
	t.Run("no config is non-error", func(t *testing.T) {
		home := t.TempDir()
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "",
			"XDG_CONFIG_HOME": "",
		}
		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

		cfg, err := cli.DiscoverConfigFile(ctx)
		require.NoError(t, err)
		require.Empty(t, cfg.Path)
		require.Equal(t, cli.ConfigFileSourceNone, cfg.Source)
	})

	t.Run("XDG fallback preferred over legacy when both exist", func(t *testing.T) {
		home := t.TempDir()
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "",
			"XDG_CONFIG_HOME": "",
		}

		xdgFallback := filepath.Join(home, ".config", "remuda", "config.yaml")
		legacy := filepath.Join(home, ".remuda", "config.yaml")

		require.NoError(t, os.MkdirAll(filepath.Dir(xdgFallback), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o755))
		require.NoError(t, os.WriteFile(xdgFallback, []byte("version: 1\n"), 0o644))
		require.NoError(t, os.WriteFile(legacy, []byte("version: 1\n"), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		cfg, err := cli.DiscoverConfigFile(ctx)
		require.NoError(t, err)
		require.Equal(t, xdgFallback, cfg.Path)
		require.Equal(t, cli.ConfigFileSourceXDG, cfg.Source)
	})

	t.Run("XDG_CONFIG_HOME is preferred when set", func(t *testing.T) {
		home := t.TempDir()

		xdgHome := filepath.Join(home, "xdg")
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "",
			"XDG_CONFIG_HOME": xdgHome,
		}

		xdgPath := filepath.Join(xdgHome, "remuda", "config.yaml")
		legacy := filepath.Join(home, ".remuda", "config.yaml")

		require.NoError(t, os.MkdirAll(filepath.Dir(xdgPath), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o755))
		require.NoError(t, os.WriteFile(xdgPath, []byte("version: 1\n"), 0o644))
		require.NoError(t, os.WriteFile(legacy, []byte("version: 1\n"), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		cfg, err := cli.DiscoverConfigFile(ctx)
		require.NoError(t, err)
		require.Equal(t, xdgPath, cfg.Path)
		require.Equal(t, cli.ConfigFileSourceXDG, cfg.Source)
	})

	t.Run("legacy is used when XDG path is missing", func(t *testing.T) {
		home := t.TempDir()

		xdgHome := filepath.Join(home, "xdg")
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "",
			"XDG_CONFIG_HOME": xdgHome,
		}

		legacy := filepath.Join(home, ".remuda", "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o755))
		require.NoError(t, os.WriteFile(legacy, []byte("version: 1\n"), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		cfg, err := cli.DiscoverConfigFile(ctx)
		require.NoError(t, err)
		require.Equal(t, legacy, cfg.Path)
		require.Equal(t, cli.ConfigFileSourceLegacy, cfg.Source)
	})
}

func TestDiscoverConfigFile_UsesContextEnvProvider(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	override := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(override, []byte("version: 1\n"), 0o644))

	env := cli.EnvMap{
		"REMUDA_CONFIG": override,
		"HOME":          home,
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	cfg, err := cli.DiscoverConfigFile(ctx)
	require.NoError(t, err)
	require.Equal(t, override, cfg.Path)
	require.Equal(t, cli.ConfigFileSourceEnv, cfg.Source)
}
