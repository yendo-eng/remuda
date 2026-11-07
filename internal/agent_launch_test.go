package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
		AgentName:   "codex",
		Command:     "true",
		Detached:    true,
	})
	require.NoError(t, err)
	require.NotContains(t, sm.start.Command, "BEADS_DIR=")
	_, ok := envValue(sm.start.Env, "BEADS_DIR")
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
	value, ok := envValue(sm.start.Env, "BEADS_DIR")
	require.True(t, ok)
	require.Equal(t, "/tmp/explicit-beads", value)
}

func TestLaunchAgentSessionStartEnvPath(t *testing.T) {
	const hostPath = "/tmp/host/bin"
	t.Setenv("PATH", hostPath)

	tests := []struct {
		name     string
		values   map[string]string
		wantPath string
	}{
		{name: "host PATH when missing", wantPath: hostPath},
		{name: "provided PATH", values: map[string]string{"PATH": "/custom/bin"}, wantPath: "/custom/bin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sm := &captureMultiplexer{}
			k := Remuda{
				Multiplexer: sm,
				IO:          DefaultIO(),
				Env:         env.StaticProvider{Values: tt.values},
			}

			_, err := k.launchAgentSession(agentLaunchCommand{
				Workspace:   t.TempDir(),
				SessionName: "org/repo/folder",
				Command:     "true",
				Detached:    true,
			})
			require.NoError(t, err)
			value, ok := envValue(sm.start.Env, "PATH")
			require.True(t, ok)
			require.Equal(t, tt.wantPath, value)
		})
	}
}

func TestLaunchAgentSessionStartsContainer(t *testing.T) {
	t.Parallel()

	sm := &captureMultiplexer{}
	k := Remuda{
		Multiplexer: sm,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
		Env: env.StaticProvider{
			Values:  map[string]string{"GH_TOKEN": "test-token"}, // avoid invoking `gh auth token`
			HomeDir: t.TempDir(),
		},
	}
	workspace := t.TempDir()

	_, err := k.launchAgentSession(agentLaunchCommand{
		Workspace:      workspace,
		SessionName:    "org/repo/folder",
		AgentName:      "codex",
		Command:        "codex",
		Prompt:         "prompt",
		Detached:       true,
		Container:      true,
		ContainerImage: "ghcr.io/acme/vibe-dev:latest",
	})
	require.NoError(t, err)
	require.Equal(t, workspace, sm.start.Workspace)
	require.Equal(t, "prompt", sm.start.Prompt)
	require.True(t, sm.start.Container)
	require.Equal(t, []string{"docker", "run"}, sm.start.CommandArgv[:2])
	require.Contains(t, sm.start.Command, "'docker' 'run'")
}

func envValue(values []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range values {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	return "", false
}
