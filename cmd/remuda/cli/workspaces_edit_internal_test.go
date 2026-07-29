package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestResolveEditorCommandOrder(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap{
		"REMUDA_EDITOR": "remuda-cmd",
		"VISUAL":        "visual-cmd",
		"EDITOR":        "editor-cmd",
	}
	cmd, err := cli.ResolveEditorCommand(env)
	require.NoError(t, err)
	require.Equal(t, "remuda-cmd", cmd)

	env["REMUDA_EDITOR"] = ""
	cmd, err = cli.ResolveEditorCommand(env)
	require.NoError(t, err)
	require.Equal(t, "visual-cmd", cmd)

	env["VISUAL"] = ""
	cmd, err = cli.ResolveEditorCommand(env)
	require.NoError(t, err)
	require.Equal(t, "editor-cmd", cmd)
}

func TestResolveEditorCommandMissing(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap{
		"REMUDA_EDITOR": "",
		"VISUAL":        "",
		"EDITOR":        "",
	}
	_, err := cli.ResolveEditorCommand(env)
	require.ErrorContains(t, err, "no editor configured")
}
