package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionEditNameOrPickRequired(t *testing.T) {
	t.Parallel()
	parser, _ := testutils.RemudaParser(t)
	_, err := parser.Parse([]string{"session", "edit"})
	require.ErrorContains(t, err, "missing flags: --name=STRING or --pick")

	parser, cli := testutils.RemudaParser(t)
	_, err = parser.Parse([]string{"session", "edit", "--name", "org/repo/work"})
	require.NoError(t, err)
	require.Equal(t, "org/repo/work", cli.Session.Edit.Name)

	parser, cli = testutils.RemudaParser(t)
	_, err = parser.Parse([]string{"session", "edit", "--pick"})
	require.NoError(t, err)
	require.True(t, cli.Session.Edit.Pick)
}
