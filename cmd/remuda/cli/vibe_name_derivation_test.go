package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveWorkspaceNameFromJiraWithoutFetchedIssues(t *testing.T) {
	t.Parallel()

	name, ok, err := deriveWorkspaceNameFromJira(Context{}, "", nil)
	require.NoError(t, err)
	require.Empty(t, name)
	require.False(t, ok)
}
