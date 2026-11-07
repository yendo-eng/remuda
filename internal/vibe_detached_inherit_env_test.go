package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/testutils"
)

func TestVibe_DetachedExportsInheritedEnvVars(t *testing.T) {
	t.Setenv("POSTMAN_API_KEY", "secret")
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
	require.NotEmpty(t, sm.LastStart.Command)
	require.Contains(t, sm.LastStart.Command, "'-e' 'POSTMAN_API_KEY'")
	require.NotContains(t, sm.LastStart.Command, "export POSTMAN_API_KEY=")
	value, ok := envValue(sm.LastStart.Env, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "secret", value)
	require.Contains(t, sm.LastStart.ForwardEnv, "POSTMAN_API_KEY")
}

func TestVibe_DetachedUnsetsMissingInheritedEnvVars(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
	require.NotEmpty(t, sm.LastStart.Command)
	require.Contains(t, sm.LastStart.Command, "'-e' 'POSTMAN_API_KEY'")
	require.NotContains(t, sm.LastStart.Command, "unset POSTMAN_API_KEY")
	_, ok := envValue(sm.LastStart.Env, "POSTMAN_API_KEY")
	require.False(t, ok)
}

func TestVibe_DetachedPreservesInheritedEnvWhitespaceAndEmpty(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("POSTMAN_API_KEY", "  spaced \nvalue\t")

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
	require.NotContains(t, sm.LastStart.Command, "export POSTMAN_API_KEY=")
	value, ok := envValue(sm.LastStart.Env, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Equal(t, "  spaced \nvalue\t", value)

	t.Setenv("POSTMAN_API_KEY", "")
	sm.LastStart.Env = nil
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
	value, ok = envValue(sm.LastStart.Env, "POSTMAN_API_KEY")
	require.True(t, ok)
	require.Empty(t, value)
}

func TestVibe_DetachedExportsImplicitAnthropicForClaude(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
	require.NotEmpty(t, sm.LastStart.Command)
	require.Contains(t, sm.LastStart.Command, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, sm.LastStart.Command, "export ANTHROPIC_API_KEY=")
	value, ok := envValue(sm.LastStart.Env, "ANTHROPIC_API_KEY")
	require.True(t, ok)
	require.Equal(t, "anthropic-secret", value)
}

func TestVibe_DetachedUnsetsImplicitAnthropicWhenMissingForClaude(t *testing.T) {
	t.Parallel()

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
	require.NotEmpty(t, sm.LastStart.Command)
	require.Contains(t, sm.LastStart.Command, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, sm.LastStart.Command, "unset ANTHROPIC_API_KEY")
	_, ok := envValue(sm.LastStart.Env, "ANTHROPIC_API_KEY")
	require.False(t, ok)
}

func TestVibe_DetachedEnvOverrideUsesStartEnvOnly(t *testing.T) {
	t.Parallel()

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
	require.NotContains(t, sm.LastStart.Command, "vibe-secret")
	require.NotContains(t, sm.LastStart.Command, "OPENAI_API_KEY=vibe-secret")
	value, ok := envValue(sm.LastStart.Env, "OPENAI_API_KEY")
	require.True(t, ok)
	require.Equal(t, "vibe-secret", value)
	value, ok = envValue(sm.LastStart.Env, "CUSTOM_OVERRIDE")
	require.True(t, ok)
	require.Equal(t, "custom-secret", value)
	require.Subset(t, sm.LastStart.ForwardEnv, []string{"OPENAI_API_KEY", "CUSTOM_OVERRIDE"})
}

func TestVibe_DetachedLaunchEnvDoesNotLeakIntoLaterLaunches(t *testing.T) {
	t.Parallel()

	sm := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
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
		value, ok := envValue(sm.LastStart.Env, key)
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
		_, ok := envValue(sm.LastStart.Env, key)
		require.False(t, ok, "%s leaked from the first launch", key)
	}
}
