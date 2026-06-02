package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

type captureSessionManager struct {
	startName string
	startCmd  string
}

func (m *captureSessionManager) Name() string { return string(session.SessionManagerTmux) }
func (m *captureSessionManager) Start(sessionName, command string) error {
	m.startName = sessionName
	m.startCmd = command
	return nil
}
func (m *captureSessionManager) List() ([]session.SessionInfo, error) { return nil, nil }
func (m *captureSessionManager) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m *captureSessionManager) Attach(name string) error                          { return nil }
func (m *captureSessionManager) ReadBuffer(name string, lines int) (string, error) { return "", nil }
func (m *captureSessionManager) Send(name string, payload string, appendNewline bool) error {
	return nil
}
func (m *captureSessionManager) Kill(name string) error { return nil }

func TestVibe_Detached_TmuxExportsInheritedEnvVars(t *testing.T) {
	t.Setenv("POSTMAN_API_KEY", "secret")
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	workspace := t.TempDir()

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:               "bash",
		Prompt:              "ignored",
		Detached:            true,
		ExistingWorkspace:   workspace,
		Container:           true,
		ContainerName:       "ghcr.io/acme/vibe-dev:latest",
		ContainerInheritEnv: []string{"POSTMAN_API_KEY"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, sm.startCmd)
	require.Contains(t, sm.startCmd, "export POSTMAN_API_KEY='secret';")
}

func TestVibe_Detached_TmuxUnsetsMissingInheritedEnvVars(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	workspace := t.TempDir()

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:               "bash",
		Prompt:              "ignored",
		Detached:            true,
		ExistingWorkspace:   workspace,
		Container:           true,
		ContainerName:       "ghcr.io/acme/vibe-dev:latest",
		ContainerInheritEnv: []string{"POSTMAN_API_KEY"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, sm.startCmd)
	require.Contains(t, sm.startCmd, "unset POSTMAN_API_KEY;")
}

func TestVibe_Detached_TmuxPreservesInheritedEnvWhitespaceAndEmpty(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("POSTMAN_API_KEY", "  spaced \nvalue\t")

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	workspace := t.TempDir()

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:               "bash",
		Prompt:              "ignored",
		Detached:            true,
		ExistingWorkspace:   workspace,
		Container:           true,
		ContainerName:       "ghcr.io/acme/vibe-dev:latest",
		ContainerInheritEnv: []string{"POSTMAN_API_KEY"},
	})
	require.NoError(t, err)
	require.Contains(t, sm.startCmd, "export POSTMAN_API_KEY='  spaced \nvalue\t';")

	t.Setenv("POSTMAN_API_KEY", "")
	err = k.Vibe(context.Background(), VibeCommand{
		Agent:               "bash",
		Prompt:              "ignored",
		Detached:            true,
		ExistingWorkspace:   workspace,
		Container:           true,
		ContainerName:       "ghcr.io/acme/vibe-dev:latest",
		ContainerInheritEnv: []string{"POSTMAN_API_KEY"},
	})
	require.NoError(t, err)
	require.Contains(t, sm.startCmd, "export POSTMAN_API_KEY='';")
}

func TestVibe_Detached_TmuxExportsImplicitAnthropicForClaude(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	workspace := t.TempDir()

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "claude",
		Prompt:            "ignored",
		Detached:          true,
		ExistingWorkspace: workspace,
		Container:         true,
		ContainerName:     "ghcr.io/acme/vibe-dev:latest",
	})
	require.NoError(t, err)
	require.NotEmpty(t, sm.startCmd)
	require.Contains(t, sm.startCmd, "export ANTHROPIC_API_KEY='anthropic-secret';")
}

func TestVibe_Detached_TmuxUnsetsImplicitAnthropicWhenMissingForClaude(t *testing.T) {
	t.Parallel()

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
		Env: env.StaticProvider{Values: map[string]string{
			"GH_TOKEN": "test-token", // avoid invoking `gh auth token`
		}},
	}

	workspace := t.TempDir()

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "claude",
		Prompt:            "ignored",
		Detached:          true,
		ExistingWorkspace: workspace,
		Container:         true,
		ContainerName:     "ghcr.io/acme/vibe-dev:latest",
	})
	require.NoError(t, err)
	require.NotEmpty(t, sm.startCmd)
	require.Contains(t, sm.startCmd, "unset ANTHROPIC_API_KEY;")
}
