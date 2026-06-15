package cli_test

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestWorkspacesRemoveCmdParse_WithAbsoluteTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "remove", "/tmp/workspace"})
	require.NoError(t, err)
}

func TestWorkspacesRemoveCmdParse_WithIdentifierTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "remove", "acme/widgets/feature-1"})
	require.NoError(t, err)
}

func TestWorkspacesRemoveCmdParse_WithMultipleTargets(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{
		"workspaces", "remove",
		"/tmp/workspace-1",
		"acme/widgets/feature-2",
	})
	require.NoError(t, err)
}

func TestWorkspacesRemoveCmdParse_WithForce(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "remove", "--force", "/tmp/workspace"})
	require.NoError(t, err)
}

func TestWorkspacesRemoveCmdParse_RejectsRelativeTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "remove", "./workspace"})
	require.Error(t, err)
	require.ErrorContains(t, err, "expected absolute path or org/repo/workspace identifier")
}

func TestWorkspacesRemoveCmdParse_RejectsUnsupportedTildeUserTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "remove", "~other/repo/ws"})
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported tilde path")
}
