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
func (noopGit) WorktreeMove(dir, src, dst string) error                  { return nil }
func (noopGit) Checkout(dir string, args ...string) error                { return nil }
func (noopGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (noopGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (noopGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = noopGit{}

func TestRun_WiresSessionManagerFlag(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	workDir := t.TempDir()
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
		WithHomeDir(homeDir),
		WithWorkingDir(workDir),
		WithSessionManagerFactory(func(name session.SupportedSessionManager, _ zerolog.Logger) session.SessionManager {
			return stubNamedSessionManager{name: string(name)}
		}),
	)

	require.NoError(t, Run(ctx, []string{"--session-manager", "zellij", "session", "list"}))
	require.Contains(t, out.String(), "(zellij)")
}

type stubNamedSessionManager struct {
	name string
}

func (m stubNamedSessionManager) Name() string { return m.name }
func (m stubNamedSessionManager) Start(sessionName, command string) error {
	return nil
}
func (m stubNamedSessionManager) List() ([]session.SessionInfo, error) {
	return nil, nil
}
func (m stubNamedSessionManager) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m stubNamedSessionManager) Attach(name string) error { return nil }
func (m stubNamedSessionManager) ReadBuffer(name string, lines int) (string, error) {
	return "", nil
}
func (m stubNamedSessionManager) Send(name string, payload string, appendNewline bool) error {
	return nil
}
func (m stubNamedSessionManager) Kill(name string) error { return nil }
