package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
)

type repoAliasGroup struct {
	Primary string   `json:"primary"`
	URL     string   `json:"url"`
	Aliases []string `json:"aliases,omitempty"`
}

func TestRepoList(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: t.TempDir()}))

	res := h.RunOK("repo", "list")

	require.Equal(t, "No repository aliases configured.\n", res.Stdout)
}

func TestRepoListJSON(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: t.TempDir()}))

	res := h.RunOK("repo", "list", "--json")

	var got []repoAliasGroup
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &got))

	require.Empty(t, got)
}
