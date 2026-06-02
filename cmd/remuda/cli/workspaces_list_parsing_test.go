package cli_test

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestWorkspacesListCmdParse_WithActive(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "list", "--active"})
	require.NoError(t, err)
}

func TestWorkspacesListCmdParse_RejectsActiveAndInactive(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"workspaces", "list", "--active", "--inactive"})
	require.Error(t, err)
	require.ErrorContains(t, err, "flags --active and --inactive cannot be used together")
}
