package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/github"
)

// This test reads repo aliases via resolveRepoSelection; keep it serial.
func TestVibeCheckPRURLRepoInferenceBeatsDefaultRepoAlias(t *testing.T) {
	alias := "widgets"
	cmd := VibeCheckCmd{
		CloneRepoOption: CloneRepoOption{
			Repo: alias, // default alias via config/env
		},
		PRRef: "https://github.com/acme/tools/pull/123",
	}

	// Mirror the inference order used in VibeCheckCmd.run().
	sourceHint := RepoSourceUnspecified
	if prURLRepo := github.RepoURLFromPR(cmd.PRRef); prURLRepo != "" && cmd.RepoURL == "" {
		cmd.RepoURL = prURLRepo
		sourceHint = RepoSourceDerived
	}

	repoSelection, err := resolveRepoSelection(Context{}, cmd.CloneRepoOption, RepoResolutionOptions{
		AllowFallback: true,
		SourceHint:    sourceHint,
	})
	require.NoError(t, err)
	require.Equal(t, "https://github.com/acme/tools.git", repoSelection.RepoURL)
}

func TestDefaultReviewNameSanitizesSlashes(t *testing.T) {
	require.Equal(t, "feature-foo-code-review", defaultReviewName("", "feature/foo"))
	require.Equal(t, "feature-foo-code-review", defaultReviewName("", `feature\foo`))
	require.Equal(t, "already-code-review", defaultReviewName("", "already-code-review"))
}
