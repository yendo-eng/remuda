package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

type repoAliasGroup struct {
	Primary string   `json:"primary"`
	URL     string   `json:"url"`
	Aliases []string `json:"aliases,omitempty"`
}

func TestRepoList(t *testing.T) {
	t.Parallel()

	t.Run("no repo aliases configured", func(t *testing.T) {
		t.Parallel()
		res := testutils.
			NewHarness(t).
			RunOK("repo", "list")

		require.Equal(t, "No repository aliases configured.\n", res.Stdout)
	})
}

func TestRepoListJSON(t *testing.T) {
	t.Parallel()

	res := testutils.
		NewHarness(t).
		RunOK("repo", "list", "--json")

	var got []repoAliasGroup
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got))

	require.Empty(t, got)
}
