package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestValidateMultiplexerLaunchRejectsUnsupportedHerdrModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agentCmd  string
		container bool
		want      error
	}{
		{name: "agent command", agentCmd: "custom-agent", want: session.UnsupportedAgentCommandError{}},
		{name: "container", container: true, want: session.UnsupportedContainerModeError{}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateMultiplexerLaunch(session.NewHerdr(), tt.agentCmd, tt.container)
			require.Error(t, err)
			require.IsType(t, tt.want, err)
		})
	}
}

func TestLaunchAgentSessionDoesNotInferSharedBeadsDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	beadsDir := filepath.Join(base, "org", "repo", ".beads_worktree", ".beads")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(beadsDir, 0o755))

	sm := &captureMultiplexer{}
	k := Remuda{
		Multiplexer: sm,
		IO:          DefaultIO(),
		Env: env.StaticProvider{Values: map[string]string{
			"PATH": "/usr/bin:/bin",
		}},
	}

	_, err := k.launchAgentSession(agentLaunchCommand{
		Workspace:   workspace,
		SessionName: "org/repo/folder",
		AgentName:   "codex",
		Command:     "true",
		Detached:    true,
	})
	require.NoError(t, err)
	require.NotContains(t, sm.startCmd, "BEADS_DIR=")
	_, ok := envValue(sm.startEnv, "BEADS_DIR")
	require.False(t, ok)
}

func TestLaunchAgentSessionPreservesExplicitBeadsDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	beadsDir := filepath.Join(base, "org", "repo", ".beads_worktree", ".beads")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(beadsDir, 0o755))

	sm := &captureMultiplexer{}
	k := Remuda{
		Multiplexer: sm,
		IO:          DefaultIO(),
		Env: env.StaticProvider{Values: map[string]string{
			"PATH":      "/usr/bin:/bin",
			"BEADS_DIR": "/tmp/explicit-beads",
		}},
	}

	_, err := k.launchAgentSession(agentLaunchCommand{
		Workspace:   workspace,
		SessionName: "org/repo/folder",
		AgentName:   "codex",
		Command:     "true",
		Detached:    true,
	})
	require.NoError(t, err)
	value, ok := envValue(sm.startEnv, "BEADS_DIR")
	require.True(t, ok)
	require.Equal(t, "/tmp/explicit-beads", value)
}
