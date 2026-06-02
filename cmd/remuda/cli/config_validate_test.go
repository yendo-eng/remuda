package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestConfigValidateCmdParse(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"config", "validate"})
	require.NoError(t, err)
}

func TestConfigValidateCmd_NoConfigSucceeds(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	cmd := cli.ConfigValidateCmd{}
	require.NoError(t, cmd.Run(ctx))
}

func TestConfigValidateCmd_InvalidConfigFails(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("version: 999\n"), 0o644))

	cmd := cli.ConfigValidateCmd{}
	err := cmd.Run(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported config version")
}
