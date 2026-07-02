package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestWorkspacesListCmd(t *testing.T) {
	t.Parallel()

	t.Run("lists managed workspaces and excludes repo cache dir", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		active := filepath.Join(base, "org", "repo", "active")
		inactive := filepath.Join(base, "org", "repo", "inactive")
		beadsWorktree := filepath.Join(base, "org", "repo", ".beads_worktree")
		require.NoError(t, os.MkdirAll(active, 0o755))
		require.NoError(t, os.MkdirAll(inactive, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(base, "org", "repo", ".repo_cache"), 0o755))
		require.NoError(t, os.MkdirAll(beadsWorktree, 0o755))

		ctx, out := newWorkspacesListContext(
			base,
			[]session.SessionInfo{{Name: "org/repo/active"}},
			nil,
		)

		err := (WorkspacesListCmd{}).Run(ctx)
		require.NoError(t, err)

		lines := nonEmptyLines(out.String())
		require.ElementsMatch(t, []string{active, inactive, beadsWorktree}, lines)
		require.NotContains(t, out.String(), ".repo_cache")
	})

	t.Run("inactive only mode filters active workspaces", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		active := filepath.Join(base, "org", "repo", "active")
		inactive := filepath.Join(base, "org", "repo", "inactive")
		require.NoError(t, os.MkdirAll(active, 0o755))
		require.NoError(t, os.MkdirAll(inactive, 0o755))

		ctx, out := newWorkspacesListContext(
			base,
			[]session.SessionInfo{{Name: "org/repo/active"}},
			nil,
		)

		err := (WorkspacesListCmd{Inactive: true}).Run(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{inactive}, nonEmptyLines(out.String()))
	})

	t.Run("active only mode filters inactive workspaces", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		active := filepath.Join(base, "org", "repo", "active")
		inactive := filepath.Join(base, "org", "repo", "inactive")
		require.NoError(t, os.MkdirAll(active, 0o755))
		require.NoError(t, os.MkdirAll(inactive, 0o755))

		ctx, out := newWorkspacesListContext(
			base,
			[]session.SessionInfo{{Name: "org/repo/active"}},
			nil,
		)

		err := (WorkspacesListCmd{Active: true}).Run(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{active}, nonEmptyLines(out.String()))
	})

	t.Run("configured ignore patterns are excluded", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		keep := filepath.Join(base, "org", "repo", "keep")
		ignore := filepath.Join(base, "org", "repo", "ignore")
		require.NoError(t, os.MkdirAll(keep, 0o755))
		require.NoError(t, os.MkdirAll(ignore, 0o755))

		ignorePatterns := []string{"org/repo/ignore"}
		cfg := &configfile.V1{
			Version: 1,
			Workspaces: &configfile.WorkspacesV1{
				Ignore: &ignorePatterns,
			},
		}
		ctx, out := newWorkspacesListContext(base, nil, cfg)

		err := (WorkspacesListCmd{}).Run(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{keep}, nonEmptyLines(out.String()))
	})

	t.Run("inactive mode applies workspace ignore patterns", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		keep := filepath.Join(base, "org", "repo", "keep")
		ignore := filepath.Join(base, "org", "repo", "ignore")
		active := filepath.Join(base, "org", "repo", "active")
		require.NoError(t, os.MkdirAll(keep, 0o755))
		require.NoError(t, os.MkdirAll(ignore, 0o755))
		require.NoError(t, os.MkdirAll(active, 0o755))

		ignorePatterns := []string{"org/repo/ignore"}
		cfg := &configfile.V1{
			Version: 1,
			Workspaces: &configfile.WorkspacesV1{
				Ignore: &ignorePatterns,
			},
		}
		ctx, out := newWorkspacesListContext(
			base,
			[]session.SessionInfo{{Name: "org/repo/active"}},
			cfg,
		)

		err := (WorkspacesListCmd{Inactive: true}).Run(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{keep}, nonEmptyLines(out.String()))
	})

	t.Run("active mode applies workspace ignore patterns", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		keep := filepath.Join(base, "org", "repo", "keep")
		ignore := filepath.Join(base, "org", "repo", "ignore")
		inactive := filepath.Join(base, "org", "repo", "inactive")
		require.NoError(t, os.MkdirAll(keep, 0o755))
		require.NoError(t, os.MkdirAll(ignore, 0o755))
		require.NoError(t, os.MkdirAll(inactive, 0o755))

		ignorePatterns := []string{"org/repo/ignore"}
		cfg := &configfile.V1{
			Version: 1,
			Workspaces: &configfile.WorkspacesV1{
				Ignore: &ignorePatterns,
			},
		}
		ctx, out := newWorkspacesListContext(
			base,
			[]session.SessionInfo{
				{Name: "org/repo/keep"},
				{Name: "org/repo/ignore"},
			},
			cfg,
		)

		err := (WorkspacesListCmd{Active: true}).Run(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{keep}, nonEmptyLines(out.String()))
	})

	t.Run("session.prune.ignore config does not affect workspace listing", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		keep := filepath.Join(base, "org", "repo", "keep")
		ignore := filepath.Join(base, "org", "repo", "ignore")
		require.NoError(t, os.MkdirAll(keep, 0o755))
		require.NoError(t, os.MkdirAll(ignore, 0o755))

		pruneIgnore := []string{"org/repo/ignore"}
		cfg := &configfile.V1{
			Version: 1,
			Session: &configfile.SessionV1{
				Prune: &configfile.SessionPruneV1{
					Ignore: &pruneIgnore,
				},
			},
		}
		ctx, out := newWorkspacesListContext(base, nil, cfg)

		err := (WorkspacesListCmd{}).Run(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{keep, ignore}, nonEmptyLines(out.String()))
	})
}

func newWorkspacesListContext(base string, sessions []session.SessionInfo, cfg *configfile.V1) (Context, *bytes.Buffer) {
	var out bytes.Buffer

	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: base},
		stubGit{},
		stubSessionManager{sessions: sessions},
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{
			In:  strings.NewReader(""),
			Out: &out,
			Err: io.Discard,
		}),
	)
	ctx := NewContext(context.Background(), k)
	ctx.ConfigFile = cfg
	return ctx, &out
}

func nonEmptyLines(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

type stubGit struct{}

func (g stubGit) Clone(repoURL, dir string) error                          { return nil }
func (g stubGit) Pull(dir string) error                                    { return nil }
func (g stubGit) WorktreeAdd(dir, branch string, args ...string) error     { return nil }
func (g stubGit) WorktreeRemove(dir string, args ...string) error          { return nil }
func (g stubGit) WorktreeMove(dir, src, dst string) error                  { return nil }
func (g stubGit) Checkout(dir string, args ...string) error                { return nil }
func (g stubGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (g stubGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (g stubGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = stubGit{}

type stubSessionManager struct {
	sessions []session.SessionInfo
}

func (m stubSessionManager) Name() string                            { return "stub" }
func (m stubSessionManager) Start(sessionName, command string) error { return nil }
func (m stubSessionManager) List() ([]session.SessionInfo, error)    { return m.sessions, nil }
func (m stubSessionManager) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m stubSessionManager) Attach(name string) error                          { return nil }
func (m stubSessionManager) ReadBuffer(name string, lines int) (string, error) { return "", nil }
func (m stubSessionManager) Send(name string, payload string, appendNewline bool) error {
	return nil
}
func (m stubSessionManager) Kill(name string) error { return nil }
