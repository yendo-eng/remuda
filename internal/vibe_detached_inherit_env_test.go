package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

type captureMultiplexer struct {
	start session.AgentStart
}

func (m *captureMultiplexer) Name() string { return string(session.MultiplexerTmux) }
func (m *captureMultiplexer) Start(start session.AgentStart) error {
	m.start = start
	return nil
}
func (m *captureMultiplexer) List() ([]session.SessionInfo, error) { return nil, nil }
func (m *captureMultiplexer) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m *captureMultiplexer) Attach(name string) error                          { return nil }
func (m *captureMultiplexer) ReadBuffer(name string, lines int) (string, error) { return "", nil }
func (m *captureMultiplexer) Send(name string, payload string, appendNewline bool) error {
	return nil
}
func (m *captureMultiplexer) Kill(name string) error { return nil }

func TestVibe_Detached_TmuxExportsInheritedEnvVars(t *testing.T) {
	t.Setenv("POSTMAN_API_KEY", "secret")
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
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
	require.NotEmpty(t, sm.start.Command)
	require.Contains(t, sm.start.Command, "'-e' 'POSTMAN_API_KEY'")
	require.NotContains(t, sm.start.Command, "export POSTMAN_API_KEY=")
	value, ok := envValue(sm.start.Env, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "secret", value)
}

func TestVibe_Detached_TmuxUnsetsMissingInheritedEnvVars(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
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
	require.NotEmpty(t, sm.start.Command)
	require.Contains(t, sm.start.Command, "'-e' 'POSTMAN_API_KEY'")
	require.NotContains(t, sm.start.Command, "unset POSTMAN_API_KEY")
	_, ok := envValue(sm.start.Env, "POSTMAN_API_KEY")
	require.False(t, ok)
}

func TestVibe_Detached_TmuxPreservesInheritedEnvWhitespaceAndEmpty(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("POSTMAN_API_KEY", "  spaced \nvalue\t")

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
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
	require.NotContains(t, sm.start.Command, "export POSTMAN_API_KEY=")
	value, ok := envValue(sm.start.Env, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "  spaced \nvalue\t", value)

	t.Setenv("POSTMAN_API_KEY", "")
	sm.start.Env = nil
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
	value, ok = envValue(sm.start.Env, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Empty(t, value)
}

func TestVibe_Detached_TmuxExportsImplicitAnthropicForClaude(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
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
	require.NotEmpty(t, sm.start.Command)
	require.Contains(t, sm.start.Command, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, sm.start.Command, "export ANTHROPIC_API_KEY=")
	value, ok := envValue(sm.start.Env, "ANTHROPIC_API_KEY")
	require.True(t, ok)
	require.Equal(t, "anthropic-secret", value)
}

func TestVibe_Detached_TmuxUnsetsImplicitAnthropicWhenMissingForClaude(t *testing.T) {
	t.Parallel()

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
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
	require.NotEmpty(t, sm.start.Command)
	require.Contains(t, sm.start.Command, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, sm.start.Command, "unset ANTHROPIC_API_KEY")
	_, ok := envValue(sm.start.Env, "ANTHROPIC_API_KEY")
	require.False(t, ok)
}

func TestVibe_DetachedEnvOverrideUsesStartEnvOnly(t *testing.T) {
	t.Parallel()

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		IO:          DefaultIO(),
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
	require.NotContains(t, sm.start.Command, "vibe-secret")
	require.NotContains(t, sm.start.Command, "OPENAI_API_KEY=vibe-secret")
	value, ok := envValue(sm.start.Env, "OPENAI_API_KEY")
	require.True(t, ok)
	require.Equal(t, "vibe-secret", value)
	value, ok = envValue(sm.start.Env, "CUSTOM_OVERRIDE")
	require.True(t, ok)
	require.Equal(t, "custom-secret", value)
}

func TestVibe_DetachedLaunchEnvDoesNotLeakIntoLaterLaunches(t *testing.T) {
	t.Parallel()

	sm := &captureMultiplexer{}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
		// NewRemuda and cli.NewContext both install a MutableProvider.
		Env: env.NewMutableProvider(env.StaticProvider{Values: map[string]string{
			"GH_TOKEN": "test-token", // avoid invoking `gh auth token`
		}}),
	}

	// First launch: claude yolo in a container, with a model and an override,
	// so every per-launch value is set.
	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "claude",
		Model:             "opus",
		Yolo:              true,
		Detached:          true,
		ExistingWorkspace: t.TempDir(),
		Container:         true,
		ContainerName:     "ghcr.io/acme/vibe-dev:latest",
		EnvOverrides:      map[string]string{"OPENAI_API_KEY": "first-launch-key"},
	})
	require.NoError(t, err)
	firstLaunchEnv := map[string]string{
		"IS_SANDBOX":     "1",
		"REMUDA_MODEL":   "opus",
		"OPENAI_API_KEY": "first-launch-key",
	}
	for key, want := range firstLaunchEnv {
		value, ok := envValue(sm.start.Env, key)
		require.True(t, ok, key)
		require.Equal(t, want, value, key)
	}

	// None of them are set for a later plain launch in the same Remuda.
	err = k.Vibe(context.Background(), VibeCommand{
		Agent:             "bash",
		Detached:          true,
		ExistingWorkspace: t.TempDir(),
	})
	require.NoError(t, err)
	for key := range firstLaunchEnv {
		_, ok := envValue(sm.start.Env, key)
		require.False(t, ok, "%s leaked from the first launch", key)
	}
}

// namedCaptureMultiplexer captures starts under another backend's name.
type namedCaptureMultiplexer struct {
	*captureMultiplexer
	name session.SupportedMultiplexer
}

func (m namedCaptureMultiplexer) Name() string { return string(m.name) }

func TestVibe_DetachedIsolatedSessionsForwardAllowlistedEnvOnly(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	t.Setenv("GH_TOKEN", "github-token")
	t.Setenv("POSTMAN_API_KEY", "passthrough-secret")
	t.Setenv("UNFORWARDED_SECRET", "do-not-forward")

	for _, name := range []session.SupportedMultiplexer{session.MultiplexerTmux, session.MultiplexerRex, session.MultiplexerHerdrCore} {
		t.Run(string(name), func(t *testing.T) {
			sm := namedCaptureMultiplexer{captureMultiplexer: &captureMultiplexer{}, name: name}
			k := Remuda{
				Git:         &fakeGit{},
				Multiplexer: sm,
				IO:          DefaultIO(),
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

			value, ok := envValue(sm.start.Env, "OPENAI_API_KEY")
			require.True(t, ok)
			require.Equal(t, "openai-secret", value)
			value, ok = envValue(sm.start.Env, "GH_TOKEN")
			require.True(t, ok)
			require.Equal(t, "github-token", value)
			value, ok = envValue(sm.start.Env, "POSTMAN_API_KEY")
			require.True(t, ok)
			require.Equal(t, "passthrough-secret", value)
			_, ok = envValue(sm.start.Env, "UNFORWARDED_SECRET")
			require.False(t, ok)
		})
	}
}
