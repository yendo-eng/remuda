package github_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/github"
)

func TestParseRepo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url      string
		wantOrg  string
		wantRepo string
	}{
		{
			url:      "https://github.com/acme/platform.git",
			wantOrg:  "acme",
			wantRepo: "platform",
		},
		{
			url:      "git@github.com:acme/platform.git",
			wantOrg:  "acme",
			wantRepo: "platform",
		},
		{
			url:      "/tmp/repos/acme/widgets.git",
			wantOrg:  "acme",
			wantRepo: "widgets",
		},
	}
	for _, tc := range cases {
		org, repo, err := github.ParseRepo(tc.url)
		require.NoError(t, err)
		require.Equal(t, tc.wantOrg, org)
		require.Equal(t, tc.wantRepo, repo)
	}

	_, _, err := github.ParseRepo("repo-only")
	require.Error(t, err)
}

func TestExpandRepoAlias(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)

	_, ok := github.ExpandRepoAlias("acme")
	require.False(t, ok, "expected failure expanding unknown alias")

	github.MergeRepoAliases(map[string]string{
		"Acme": "https://github.com/acme/widgets.git",
	})

	got, ok := github.ExpandRepoAlias("acme")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/widgets.git", got)

	got, ok = github.ExpandRepoAlias("ACME")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/widgets.git", got)
}

func TestRepoSlugFromURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"https://github.com/acme/widgets.git":  "acme/widgets",
		"https://github.com/acme/widgets":      "acme/widgets",
		"git@github.com:acme/platform.git":     "acme/platform",
		"ssh://git@github.com/acme/vision.git": "acme/vision",
		"acme/utils":                           "acme/utils",
	}
	for in, want := range cases {
		got, err := github.RepoSlugFromURL(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got)
	}

	_, err := github.RepoSlugFromURL("")
	require.Error(t, err)
	_, err = github.RepoSlugFromURL("invalid-url")
	require.Error(t, err)
}

func TestValidateRepoURL(t *testing.T) {
	t.Parallel()
	valid := []string{
		"https://github.com/acme/widgets.git",
		"http://github.com/acme/widgets",
		"ssh://git@github.com/acme/vision.git",
		"git@github.com:acme/platform.git",
		"  https://github.com/acme/utils.git  ",
	}
	for _, input := range valid {
		require.NoError(t, github.ValidateRepoURL(input), input)
	}

	invalid := []string{
		"",
		"  ",
		"acme/widgets",
		"file:///tmp/repo.git",
		"not-a-url",
	}
	for _, input := range invalid {
		require.Error(t, github.ValidateRepoURL(input), input)
	}
}

func TestExpandRepoURL(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"github.com/acme/widgets":         "https://github.com/acme/widgets.git",
		"github.com/acme/widgets.git":     "https://github.com/acme/widgets.git",
		"github.com/acme/widgets/":        "https://github.com/acme/widgets.git",
		"  github.com/acme/widgets  ":     "https://github.com/acme/widgets.git",
		"https://github.com/acme/widgets": "https://github.com/acme/widgets",
		"git@github.com:acme/widgets.git": "git@github.com:acme/widgets.git",
		"acme/widgets":                    "acme/widgets",
	}

	for input, want := range cases {
		require.Equal(t, want, github.ExpandRepoURL(input), input)
	}
}
