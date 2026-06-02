package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/github"
)

// These tests mutate the global repo alias registry; keep them serial.
func TestCanonicalAliasKeys_EmptyCatalog(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)

	require.Empty(t, canonicalAliasKeys())
}

func TestRepoAliasGroups_EmptyCatalog(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)

	groups := repoAliasGroups()
	require.NotNil(t, groups)
	require.Empty(t, groups)
}

func TestCanonicalAliasKeys_SortsConfiguredAliases(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"zz":    "https://github.com/acme/alpha.git",
		"alpha": "https://github.com/acme/alpha.git",
		"gamma": "https://github.com/acme/gamma.git",
	})

	require.Equal(t, []string{"alpha", "gamma"}, canonicalAliasKeys())
}

func TestInitialRepoAliasSelection_NoAliasesDefaultsToCustomURL(t *testing.T) {
	require.Equal(t, sentinelCustomURL, initialRepoAliasSelection(nil, "", ""))
}

func TestInitialRepoAliasSelection_PrefersKnownAlias(t *testing.T) {
	choices := []string{"alpha", "gamma"}
	require.Equal(t, "gamma", initialRepoAliasSelection(choices, "gamma", ""))
}

func TestInitialRepoAliasSelection_CanonicalizesShortAlias(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"wg":      "https://github.com/acme/widgets.git",
		"widgets": "https://github.com/acme/widgets.git",
		"remuda":  "https://github.com/acme/remuda.git",
	})

	choices := canonicalAliasKeys()
	require.Equal(t, []string{"remuda", "widgets"}, choices)
	require.Equal(t, "widgets", initialRepoAliasSelection(choices, "wg", ""))
}

func TestInitialRepoAliasSelection_CanonicalizesAliasCaseVariant(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"wg":      "https://github.com/acme/widgets.git",
		"widgets": "https://github.com/acme/widgets.git",
		"remuda":  "https://github.com/acme/remuda.git",
	})

	choices := canonicalAliasKeys()
	require.Equal(t, []string{"remuda", "widgets"}, choices)
	require.Equal(t, "widgets", initialRepoAliasSelection(choices, "WG", ""))
}

func TestInitialRepoAliasSelection_UsesCustomWhenURLPrefilled(t *testing.T) {
	choices := []string{"alpha", "gamma"}
	require.Equal(t, sentinelCustomURL, initialRepoAliasSelection(choices, "unknown", "https://github.com/acme/tools.git"))
}

func TestInitialRepoAliasSelection_FallsBackToFirstAliasForUnknownAlias(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"wg":      "https://github.com/acme/widgets.git",
		"widgets": "https://github.com/acme/widgets.git",
		"remuda":  "https://github.com/acme/remuda.git",
	})

	choices := canonicalAliasKeys()
	require.Equal(t, []string{"remuda", "widgets"}, choices)
	require.Equal(t, "remuda", initialRepoAliasSelection(choices, "unknown-alias", ""))
}
