package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/session"
)

type noopGit struct{}

func (noopGit) Clone(repoURL, dir string) error                          { return nil }
func (noopGit) Pull(dir string) error                                    { return nil }
func (noopGit) WorktreeAdd(dir, branch string, args ...string) error     { return nil }
func (noopGit) WorktreeRemove(dir string, args ...string) error          { return nil }
func (noopGit) WorktreeLock(dir, worktree, reason string) error          { return nil }
func (noopGit) WorktreeUnlock(dir, worktree string) error                { return nil }
func (noopGit) Checkout(dir string, args ...string) error                { return nil }
func (noopGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (noopGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (noopGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = noopGit{}

func TestRun_WiresSessionManagerFlag(t *testing.T) {
	t.Parallel()

	for _, manager := range []string{"zellij", "herdr-tui", "rex", "herdr-core"} {
		manager := manager
		t.Run(manager, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			var errBuf bytes.Buffer
			k := internal.NewRemuda(internal.Config{ReposBaseDir: t.TempDir()})
			k.Git = noopGit{}
			k.IO = internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &errBuf}
			ctx := NewContext(context.Background(), k,
				WithEnv(EnvMap{}),
				WithHomeDir(t.TempDir()),
				WithWorkingDir(t.TempDir()),
			)
			ctx.MultiplexerFactory = func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
				return stubNamedMultiplexer{name: string(name)}
			}

			require.NoError(t, Run(ctx, []string{"--session-manager", manager, "session", "list"}))
			require.Contains(t, out.String(), "("+manager+")")
		})
	}
}

func TestRun_AggregatesMultiplexers(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	k := internal.NewRemuda(internal.Config{ReposBaseDir: t.TempDir()})
	k.Git = noopGit{}
	k.IO = internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &bytes.Buffer{}}
	ctx := NewContext(context.Background(), k,
		WithEnv(EnvMap{}),
		WithHomeDir(t.TempDir()),
		WithWorkingDir(t.TempDir()),
	)
	ctx.MultiplexerFactory = func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
		return stubNamedMultiplexer{
			name:     string(name),
			sessions: []session.SessionInfo{{Name: "org/repo/" + string(name)}},
		}
	}

	require.NoError(t, Run(ctx, []string{"--session-manager", "herdr-tui", "session", "list"}))
	require.Equal(t, "org/repo/tmux\norg/repo/zellij\norg/repo/herdr-tui\norg/repo/rex\norg/repo/herdr-core\n", out.String())
}

type stubNamedMultiplexer struct {
	name     string
	sessions []session.SessionInfo
}

func (m stubNamedMultiplexer) Name() string { return m.name }
func (m stubNamedMultiplexer) Start(start session.AgentStart) error {
	return nil
}
func (m stubNamedMultiplexer) List() ([]session.SessionInfo, error) {
	return m.sessions, nil
}
func (m stubNamedMultiplexer) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m stubNamedMultiplexer) Attach(name string) error { return nil }
func (m stubNamedMultiplexer) ReadBuffer(name string, lines int) (string, error) {
	return "", nil
}
func (m stubNamedMultiplexer) Send(name string, payload string, appendNewline bool) error {
	return nil
}
func (m stubNamedMultiplexer) Kill(name string) error { return nil }
