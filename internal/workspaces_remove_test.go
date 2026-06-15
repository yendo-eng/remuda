package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/git"
)

func TestFilterInactiveWorkspaces_IgnorePatterns(t *testing.T) {
	base := t.TempDir()

	wsKeep := filepath.Join(base, "org", "repo", "keep")
	wsPrune := filepath.Join(base, "org", "repo", "prune")
	require.NoError(t, os.MkdirAll(wsKeep, 0o755))
	require.NoError(t, os.MkdirAll(wsPrune, 0o755))

	candidates := []string{wsKeep, wsPrune}
	active := map[string]struct{}{}

	inactive, err := filterInactiveWorkspaces(base, candidates, active, []string{"org/repo/keep"})
	require.NoError(t, err)
	require.Equal(t, []string{wsPrune}, inactive)
}

func TestFilterInactiveWorkspaces_InvalidIgnorePattern(t *testing.T) {
	base := t.TempDir()
	ws := filepath.Join(base, "org", "repo", "ws")
	require.NoError(t, os.MkdirAll(ws, 0o755))

	_, err := filterInactiveWorkspaces(base, []string{ws}, map[string]struct{}{}, []string{"["})
	require.Error(t, err)
}

type pruneSpyGit struct {
	worktreeRemoveCalls int
	worktreeRemoveArgs  []string
}

func (g *pruneSpyGit) Clone(repoURL, dir string) error                          { return nil }
func (g *pruneSpyGit) Pull(dir string) error                                    { return nil }
func (g *pruneSpyGit) WorktreeAdd(dir, branch string, args ...string) error     { return nil }
func (g *pruneSpyGit) Checkout(dir string, args ...string) error                { return nil }
func (g *pruneSpyGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (g *pruneSpyGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (g *pruneSpyGit) Branch(dir string, args ...string) error                  { return nil }
func (g *pruneSpyGit) WorktreeRemove(dir string, args ...string) error {
	g.worktreeRemoveCalls++
	g.worktreeRemoveArgs = append([]string(nil), args...)
	return nil
}

var _ git.Git = (*pruneSpyGit)(nil)

func TestPruneOneSession_SkipsWorktreeRemovalForFullClone(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "full")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".git"), 0o755))

	spy := &pruneSpyGit{}
	k := Remuda{
		Config: Config{ReposBaseDir: base},
		Git:    spy,
	}

	require.NoError(t, k.PruneOneSession(workspace, true, false, false))
	require.Equal(t, 0, spy.worktreeRemoveCalls)
	require.NoDirExists(t, workspace)
}

func TestPruneOneSession_RemovesLinkedWorktreeFromGit(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "linked")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".git"), []byte("gitdir: /tmp/fake"), 0o644))

	spy := &pruneSpyGit{}
	k := Remuda{
		Config: Config{ReposBaseDir: base},
		Git:    spy,
	}

	require.NoError(t, k.PruneOneSession(workspace, true, false, false))
	require.Equal(t, 1, spy.worktreeRemoveCalls)
	require.Equal(t, []string{workspace}, spy.worktreeRemoveArgs)
	require.NoDirExists(t, workspace)
}

func TestPruneOneSession_RemovesLinkedWorktreeFromGitWithForce(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "linked")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".git"), []byte("gitdir: /tmp/fake"), 0o644))

	spy := &pruneSpyGit{}
	k := Remuda{
		Config: Config{ReposBaseDir: base},
		Git:    spy,
	}

	require.NoError(t, k.PruneOneSession(workspace, true, false, true))
	require.Equal(t, 1, spy.worktreeRemoveCalls)
	require.Equal(t, []string{workspace, "--force"}, spy.worktreeRemoveArgs)
	require.NoDirExists(t, workspace)
}
