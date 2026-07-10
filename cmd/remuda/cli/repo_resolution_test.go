package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/github"
)

// contextWithExplicitFlags builds a Context that reports the given flags as
// explicitly set on the command line.
func contextWithExplicitFlags(ctx Context, names ...string) Context {
	explicit := map[string]bool{}
	for _, name := range names {
		explicit[name] = true
	}
	ctx.inv = &invocation{rs: &flagResolution{explicit: explicit}}
	return ctx
}

// These tests read the global repo alias registry via resolveRepoSelection; keep them serial.
func TestResolveRepoSelectionFallsBackWhenUnset(t *testing.T) {
	selection, err := resolveRepoSelection(Context{}, CloneRepoOption{}, RepoResolutionOptions{AllowFallback: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository is not configured")
	require.Equal(t, RepoSelection{}, selection)
}

func TestResolveRepoSelectionExplicitRepoFlag(t *testing.T) {
	installRepoResolutionAliases(t)

	ctx := contextWithExplicitFlags(Context{}, "repo")
	selection, err := resolveRepoSelection(ctx, CloneRepoOption{Repo: "utils"}, RepoResolutionOptions{AllowFallback: true})
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
}

func TestResolveRepoSelectionExistingWorkspaceSkipsRepoURL(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "acme", "widgets", "feature-1")

	selection, err := resolveRepoSelection(Context{}, CloneRepoOption{}, RepoResolutionOptions{
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

	ctx := contextWithExplicitFlags(Context{}, "repo")
	selection, err := resolveRepoSelection(ctx, CloneRepoOption{Repo: "utils"}, RepoResolutionOptions{
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

	selection, err := resolveRepoSelection(ctx, CloneRepoOption{}, RepoResolutionOptions{
		ExistingWorkspace: workspace,
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceWorkspace, selection.Source)
	require.Equal(t, "acme/widgets", selection.RepoSlug)
}

func TestResolveRepoSelectionRepoURLArgOverridesDefaults(t *testing.T) {
	repo := CloneRepoOption{
		RepoURL: "https://github.com/acme/widgets.git",
	}

	selection, err := resolveRepoSelection(Context{}, repo, RepoResolutionOptions{
		AllowFallback: true,
		RepoURLArg:    "https://github.com/acme/utils.git",
	})
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
}

func TestResolveRepoSelectionExpandsShorthandRepoURLFlag(t *testing.T) {
	ctx := contextWithExplicitFlags(Context{}, "repo-url")
	selection, err := resolveRepoSelection(ctx, CloneRepoOption{RepoURL: "github.com/acme/utils"}, RepoResolutionOptions{AllowFallback: true})
	require.NoError(t, err)
	require.Equal(t, "https://github.com/acme/utils.git", selection.RepoURL)
	require.Equal(t, "acme/utils", selection.RepoSlug)
}

func TestResolveRepoSelectionExpandsShorthandRepoURLArg(t *testing.T) {
	selection, err := resolveRepoSelection(Context{}, CloneRepoOption{}, RepoResolutionOptions{
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
