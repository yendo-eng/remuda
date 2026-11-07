package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
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

			err := validateMultiplexerLaunch(session.NewHerdrTUI(logging.DefaultLogger()), tt.agentCmd)
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

			require.Equal(t, tt.want, sessionAgentArgs(session.NewMultiplexer(tt.manager, logging.DefaultLogger()), tt.detached, tt.agent))
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
		Agent:       agentlauncher.Codex("", false, ""),
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
		Agent:       agentlauncher.Codex("", false, ""),
		Command:     "true",
		Detached:    true,
	})
	require.NoError(t, err)
	value, ok := envValue(sm.startEnv, "BEADS_DIR")
	require.True(t, ok)
	require.Equal(t, "/tmp/explicit-beads", value)
}

// containerEnvLauncher declares container env names that are not in
// isolatedSessionEnvAllowlist.
type containerEnvLauncher struct {
	agentlauncher.AgentLauncher
}

func (containerEnvLauncher) ContainerEnv() map[string]string {
	return map[string]string{"HOST_VALUE": "", "LAUNCH_VALUE": "set"}
}

func TestLaunchAgentSessionForwardsContainerEnvToIsolatedSessions(t *testing.T) {
	t.Parallel()

	sm := &captureMultiplexer{}
	k := Remuda{
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
		Env: env.StaticProvider{Values: map[string]string{
			"PATH":       "/usr/bin:/bin",
			"GH_TOKEN":   "test-token", // avoid invoking `gh auth token`
			"HOST_VALUE": "from-host",
		}},
	}

	_, err := k.launchAgentSession(agentLaunchCommand{
		Workspace:      t.TempDir(),
		Agent:          containerEnvLauncher{agentlauncher.Custom("true")},
		Command:        "true",
		Detached:       true,
		Container:      true,
		ContainerImage: "img",
	})
	require.NoError(t, err)
	require.Contains(t, sm.startCmd, "'-e' 'HOST_VALUE'")
	require.Contains(t, sm.startCmd, "'-e' 'LAUNCH_VALUE'")
	value, ok := envValue(sm.startEnv, "HOST_VALUE")
	require.True(t, ok)
	require.Equal(t, "from-host", value)
	value, ok = envValue(sm.startEnv, "LAUNCH_VALUE")
	require.True(t, ok)
	require.Equal(t, "set", value)
}
