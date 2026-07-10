package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestConfigValidateCmd_NoConfigSucceeds(t *testing.T) {
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	require.NoError(t, loadConfigThroughCLI(t, env, home))
}

func TestConfigValidateCmd_InvalidConfigFails(t *testing.T) {
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("version: 999\n"), 0o644))

	err := loadConfigThroughCLI(t, env, home)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported config version")
}
