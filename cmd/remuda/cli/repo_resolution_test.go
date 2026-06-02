package cli

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/github"
)

// These tests read the global repo alias registry via resolveRepoSelection; keep them serial.
// Kong leaves pointer fields nil when no flag/env/default is supplied.
func TestRepoOptionPointersRemainNilWhenUnset(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	_, err = parser.Parse([]string{"vibe", "--name", "wk", "hello"})
	require.NoError(t, err)
	require.Nil(t, cli.Vibe.Repo)
	require.Nil(t, cli.Vibe.RepoURL)
}

func TestResolveRepoSelectionFallsBackWhenUnset(t *testing.T) {
	selection, err := resolveRepoSelection(Context{}, nil, CloneRepoOption{}, RepoResolutionOptions{AllowFallback: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository is not configured")
	require.Equal(t, RepoSelection{}, selection)
}

func TestResolveRepoSelectionExplicitRepoFlag(t *testing.T) {
	installRepoResolutionAliases(t)

	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	kctx, err := parser.Parse([]string{"vibe", "--name", "wk", "--repo", "utils", "hello"})
	require.NoError(t, err)

	selection, err := resolveRepoSelection(Context{}, kctx, cli.Vibe.CloneRepoOption, RepoResolutionOptions{AllowFallback: true})
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
}

func TestResolveRepoSelectionExistingWorkspaceSkipsRepoURL(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "acme", "widgets", "feature-1")

	selection, err := resolveRepoSelection(Context{}, nil, CloneRepoOption{}, RepoResolutionOptions{
		ExistingWorkspace: workspace,
		ReposBaseDir:      base,
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceWorkspace, selection.Source)
	require.Empty(t, selection.RepoURL)
	require.Equal(t, "acme/widgets", selection.RepoSlug)
}

func TestResolveRepoSelectionExistingWorkspaceUsesExplicitRepo(t *testing.T) {
	installRepoResolutionAliases(t)

	base := t.TempDir()
	workspace := filepath.Join(base, "acme", "widgets", "feature-1")

	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	kctx, err := parser.Parse([]string{"vibe", "--in", workspace, "--repo", "utils", "hello"})
	require.NoError(t, err)

	selection, err := resolveRepoSelection(Context{}, kctx, cli.Vibe.CloneRepoOption, RepoResolutionOptions{
		ExistingWorkspace: workspace,
		ReposBaseDir:      base,
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
	require.Equal(t, "acme/utils", selection.RepoSlug)
}

func TestResolveRepoSelectionExistingWorkspaceUsesEnvBaseDirWhenOptionUnset(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "acme", "widgets", "feature-1")
	ctx := newTestContextWithEnv(t, EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	})

	selection, err := resolveRepoSelection(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		ExistingWorkspace: workspace,
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceWorkspace, selection.Source)
	require.Equal(t, "acme/widgets", selection.RepoSlug)
}

func TestResolveRepoSelectionRepoURLArgOverridesDefaults(t *testing.T) {
	defaultURL := "https://github.com/acme/widgets.git"
	explicitURL := "https://github.com/acme/utils.git"
	repo := CloneRepoOption{
		RepoURL: &defaultURL,
	}

	selection, err := resolveRepoSelection(Context{}, nil, repo, RepoResolutionOptions{
		AllowFallback: true,
		RepoURLArg:    explicitURL,
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, explicitURL, selection.RepoURL)
}

func TestResolveRepoSelectionExpandsShorthandRepoURLFlag(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	kctx, err := parser.Parse([]string{"vibe", "--name", "wk", "--repo-url", "github.com/acme/utils", "hello"})
	require.NoError(t, err)

	selection, err := resolveRepoSelection(Context{}, kctx, cli.Vibe.CloneRepoOption, RepoResolutionOptions{AllowFallback: true})
	require.NoError(t, err)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
	require.Equal(t, "acme/utils", selection.RepoSlug)
}

func TestResolveRepoSelectionExpandsShorthandRepoURLArg(t *testing.T) {
	selection, err := resolveRepoSelection(Context{}, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
		RepoURLArg:    "github.com/acme/utils",
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
	require.Equal(t, "acme/utils", selection.RepoSlug)
}

func installRepoResolutionAliases(t *testing.T) {
	t.Helper()
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"utils": "https://github.com/acme/utils.git",
	})
}
