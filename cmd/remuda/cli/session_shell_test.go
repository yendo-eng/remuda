package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionShellNameOrPickRequired(t *testing.T) {
	t.Parallel()
	// no name or pick
	parser, _ := testutils.RemudaParser(t)
	_, err := parser.Parse([]string{"session", "shell"})
	require.ErrorContains(t, err, "missing flags: --name=STRING or --pick")

	// with name
	parser, cli := testutils.RemudaParser(t)
	_, err = parser.Parse([]string{"session", "shell", "--name", "org/repo"})
	require.NoError(t, err)
	require.Equal(t, "org/repo", cli.Session.Shell.Name)

	// with pick
	parser, cli = testutils.RemudaParser(t)
	_, err = parser.Parse([]string{"session", "shell", "--pick"})
	require.NoError(t, err)
	require.True(t, cli.Session.Shell.Pick)
}
