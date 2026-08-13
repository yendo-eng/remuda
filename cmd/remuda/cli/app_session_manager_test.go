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
func (noopGit) Checkout(dir string, args ...string) error                { return nil }
func (noopGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (noopGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (noopGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = noopGit{}

func TestRun_WiresSessionManagerFlag(t *testing.T) {
	t.Parallel()

	for _, manager := range []string{"zellij", "herdr"} {
		manager := manager
		t.Run(manager, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			var errBuf bytes.Buffer
			k := internal.NewRemuda(
				internal.Config{ReposBaseDir: t.TempDir()},
				noopGit{},
				nil,
				nil,
				nil,
				nil,
				internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &errBuf}),
			)
			ctx := NewContext(context.Background(), k,
				WithEnv(EnvMap{}),
				WithHomeDir(t.TempDir()),
				WithWorkingDir(t.TempDir()),
				WithMultiplexerFactory(func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
					return stubNamedMultiplexer{name: string(name)}
				}),
			)

			require.NoError(t, Run(ctx, []string{"--session-manager", manager, "session", "list"}))
			require.Contains(t, out.String(), "("+manager+")")
		})
	}
}

type stubNamedMultiplexer struct {
	name string
}

func (m stubNamedMultiplexer) Name() string { return m.name }
func (m stubNamedMultiplexer) Start(sessionName, command string) error {
	return nil
}
func (m stubNamedMultiplexer) List() ([]session.SessionInfo, error) {
	return nil, nil
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
