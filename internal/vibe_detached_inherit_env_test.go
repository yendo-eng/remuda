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
	startEnv  []string
}

func (m *captureSessionManager) Name() string { return string(session.SessionManagerTmux) }
func (m *captureSessionManager) Start(sessionName, command string) error {
	m.startName = sessionName
	m.startCmd = command
	return nil
}
func (m *captureSessionManager) StartWithEnv(sessionName, command string, envValues []string) error {
	m.startName = sessionName
	m.startCmd = command
	m.startEnv = append([]string{}, envValues...)
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
	require.Contains(t, sm.startCmd, "-e POSTMAN_API_KEY")
	require.NotContains(t, sm.startCmd, "export POSTMAN_API_KEY=")
	value, ok := envValue(sm.startEnv, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "secret", value)
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
	require.Contains(t, sm.startCmd, "-e POSTMAN_API_KEY")
	require.NotContains(t, sm.startCmd, "unset POSTMAN_API_KEY")
	_, ok := envValue(sm.startEnv, "POSTMAN_API_KEY")
	require.False(t, ok)
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
	require.NotContains(t, sm.startCmd, "export POSTMAN_API_KEY=")
	value, ok := envValue(sm.startEnv, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "  spaced \nvalue\t", value)

	t.Setenv("POSTMAN_API_KEY", "")
	sm.startEnv = nil
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
	value, ok = envValue(sm.startEnv, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Empty(t, value)
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
	require.Contains(t, sm.startCmd, "-e ANTHROPIC_API_KEY")
	require.NotContains(t, sm.startCmd, "export ANTHROPIC_API_KEY=")
	value, ok := envValue(sm.startEnv, "ANTHROPIC_API_KEY")
	require.True(t, ok)
	require.Equal(t, "anthropic-secret", value)
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
	require.Contains(t, sm.startCmd, "-e ANTHROPIC_API_KEY")
	require.NotContains(t, sm.startCmd, "unset ANTHROPIC_API_KEY")
	_, ok := envValue(sm.startEnv, "ANTHROPIC_API_KEY")
	require.False(t, ok)
}

func TestVibe_DetachedEnvOverrideUsesStartEnvOnly(t *testing.T) {
	t.Parallel()

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		IO:      DefaultIO(),
		Env: env.StaticProvider{Values: map[string]string{
			"PATH": "/usr/bin:/bin",
		}},
	}

	workspace := t.TempDir()

	err := k.Vibe(context.Background(), VibeCommand{
		AgentCmd:          "true",
		Detached:          true,
		ExistingWorkspace: workspace,
		EnvOverrides: map[string]string{
			"OPENAI_API_KEY":  "vibe-secret",
			"CUSTOM_OVERRIDE": "custom-secret",
		},
	})
	require.NoError(t, err)
	require.NotContains(t, sm.startCmd, "vibe-secret")
	require.NotContains(t, sm.startCmd, "OPENAI_API_KEY=vibe-secret")
	value, ok := envValue(sm.startEnv, "OPENAI_API_KEY")
	require.True(t, ok)
	require.Equal(t, "vibe-secret", value)
	value, ok = envValue(sm.startEnv, "CUSTOM_OVERRIDE")
	require.True(t, ok)
	require.Equal(t, "custom-secret", value)
}

func TestVibe_DetachedTmuxForwardsAllowlistedEnvOnly(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	t.Setenv("GH_TOKEN", "github-token")
	t.Setenv("POSTMAN_API_KEY", "passthrough-secret")
	t.Setenv("UNFORWARDED_SECRET", "do-not-forward")

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		IO:      DefaultIO(),
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:               "codex",
		AgentCmd:            "true",
		Prompt:              "ignored",
		Detached:            true,
		ExistingWorkspace:   t.TempDir(),
		ContainerInheritEnv: []string{"POSTMAN_API_KEY"},
	})
	require.NoError(t, err)

	value, ok := envValue(sm.startEnv, "OPENAI_API_KEY")
	require.True(t, ok)
	require.Equal(t, "openai-secret", value)
	value, ok = envValue(sm.startEnv, "GH_TOKEN")
	require.True(t, ok)
	require.Equal(t, "github-token", value)
	value, ok = envValue(sm.startEnv, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "passthrough-secret", value)
	_, ok = envValue(sm.startEnv, "UNFORWARDED_SECRET")
	require.False(t, ok)
}
