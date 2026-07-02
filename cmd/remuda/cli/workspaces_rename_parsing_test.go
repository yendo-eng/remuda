package cli_test

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestWorkspacesRenameCmdParse_WithAbsoluteTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "rename", "/tmp/workspace", "new-name"})
	require.NoError(t, err)
}

func TestWorkspacesRenameCmdParse_WithIdentifierTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "rename", "acme/widgets/feature-1", "feature-2"})
	require.NoError(t, err)
}

func TestWorkspacesRenameCmdParse_RejectsRelativeTarget(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "rename", "./workspace", "new-name"})
	require.Error(t, err)
	require.ErrorContains(t, err, "expected absolute path or org/repo/workspace identifier")
}

func TestWorkspacesRenameCmdParse_RejectsNestedNewName(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "rename", "/tmp/workspace", "nested/name"})
	require.Error(t, err)
	require.ErrorContains(t, err, "must be a single workspace name")
}
