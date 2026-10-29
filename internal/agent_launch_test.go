package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestValidateMultiplexerLaunchRejectsUnsupportedHerdrTUIAgentCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agentCmd  string
		wantError bool
	}{
		{name: "agent command", agentCmd: "custom-agent", wantError: true},
		{name: "built-in agent"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateMultiplexerLaunch(session.NewHerdrTUI(), tt.agentCmd)
			if tt.wantError {
				var unsupported session.UnsupportedAgentCommandError
				require.ErrorAs(t, err, &unsupported)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSessionAgentArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manager  session.SupportedMultiplexer
		detached bool
		agent    string
		want     []string
	}{
		{name: "codex in herdr-core", manager: session.MultiplexerHerdrCore, detached: true, agent: "codex", want: []string{"--no-daemon"}},
		{name: "codex run inline", manager: session.MultiplexerHerdrCore, agent: "codex"},
		{name: "claude in herdr-core", manager: session.MultiplexerHerdrCore, detached: true, agent: "claude"},
		{name: "codex in tmux", manager: session.MultiplexerTmux, detached: true, agent: "codex"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, sessionAgentArgs(session.NewMultiplexer(tt.manager), tt.detached, tt.agent))
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
