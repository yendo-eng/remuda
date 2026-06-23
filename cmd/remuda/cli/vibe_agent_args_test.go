package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/session"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

type captureStartSessionManager struct {
	startCmd string
}

func (m *captureStartSessionManager) Name() string { return string(session.SessionManagerTmux) }

func (m *captureStartSessionManager) Start(sessionName, command string) error {
	m.startCmd = command
	return nil
}

func (m *captureStartSessionManager) StartWithEnv(sessionName, command string, env []string) error {
	m.startCmd = command
	return nil
}

func (m *captureStartSessionManager) List() ([]session.SessionInfo, error) {
	return nil, nil
}

func (m *captureStartSessionManager) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}

func (m *captureStartSessionManager) Attach(name string) error { return nil }

func (m *captureStartSessionManager) ReadBuffer(name string, lines int) (string, error) {
	return "", nil
}

func (m *captureStartSessionManager) Send(name string, payload string, appendNewline bool) error {
	return nil
}

func (m *captureStartSessionManager) Kill(name string) error { return nil }

func runVibeWithConfig(t *testing.T, configYAML string, args ...string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0o644))

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	mgr := &captureStartSessionManager{}
	remuda := internal.NewRemuda(
		internal.Config{ReposBaseDir: base},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
	)

	ctx := NewContext(
		context.Background(),
		remuda,
		WithEnv(EnvMap{"REMUDA_CONFIG": configPath}),
		WithHomeDir(t.TempDir()),
		WithWorkingDir(t.TempDir()),
		WithSessionManagerFactory(func(name session.SupportedSessionManager, _ zerolog.Logger) session.SessionManager {
			return mgr
		}),
	)

	runArgs := []string{"vibe", "--in", workspace}
	runArgs = append(runArgs, args...)
	require.NoError(t, Run(ctx, runArgs))
	require.NotEmpty(t, mgr.startCmd)
	return mgr.startCmd
}

func TestVibe_AgentArgsFromGlobalConfig(t *testing.T) {
	startCmd := runVibeWithConfig(t, `
version: 1
defaults:
  agent_args:
    codex:
      - --global-arg
`, "hello")

	require.Contains(t, startCmd, "'--global-arg' -- 'hello'")
}

func TestVibe_AgentArgsPerRepoOverrideReplacesGlobalForAgent(t *testing.T) {
	startCmd := runVibeWithConfig(t, `
version: 1
defaults:
  agent_args:
    codex:
      - --global-arg
per_repo:
  owner/repo:
    defaults:
      agent_args:
        codex:
          - --repo-arg
`, "hello")

	require.Contains(t, startCmd, "'--repo-arg' -- 'hello'")
	require.NotContains(t, startCmd, "--global-arg")
}

func TestVibe_AgentArgsCLIFLagsAppendToConfig(t *testing.T) {
	startCmd := runVibeWithConfig(t, `
version: 1
defaults:
  agent_args:
    codex:
      - --config-arg
`, "--agent-arg=--cli-arg-1", "--agent-arg=--cli-arg-2", "hello")

	require.Contains(t, startCmd, "'--config-arg' '--cli-arg-1' '--cli-arg-2' -- 'hello'")
}

func TestVibe_AgentArgsIgnoredWhenAgentCmdSet(t *testing.T) {
	startCmd := runVibeWithConfig(t, `
version: 1
defaults:
  agent_args:
    codex:
      - --config-arg
`, "--agent-cmd", "echo", "--agent-arg=--cli-arg-1", "hello")

	require.Contains(t, startCmd, "&& echo 'hello'")
	require.NotContains(t, startCmd, "--config-arg")
	require.NotContains(t, startCmd, "--cli-arg-1")
}

func TestVibe_AgentArgsCLIVerbatimForCommaSpaceAndMetacharacters(t *testing.T) {
	startCmd := runVibeWithConfig(
		t,
		`version: 1`,
		"--agent-arg=--config=a,b",
		"--agent-arg=--label=hello world",
		"--agent-arg=--danger=$(echo hi);true",
		"hello",
	)

	require.Contains(t, startCmd, shellutil.SingleQuote("--config=a,b"))
	require.Contains(t, startCmd, shellutil.SingleQuote("--label=hello world"))
	require.Contains(t, startCmd, shellutil.SingleQuote("--danger=$(echo hi);true"))
	require.NotContains(t, startCmd, "'b'")
}
