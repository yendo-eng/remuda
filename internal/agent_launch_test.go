package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestLaunchAgentSessionRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// workspace is under the test's directory, whose base subdirectory is
		// the repos base dir.
		workspace string
		// relativeBase is whether the repos base dir is given relative to the
		// working directory.
		relativeBase bool
		want         session.RepoSlug
	}{
		{name: "workspace in the repos base dir", workspace: "base/acme/widgets/feature-1", want: "acme/widgets"},
		{name: "workspace in a relative repos base dir", workspace: "base/acme/widgets/feature-1", relativeBase: true, want: "acme/widgets"},
		{name: "subdirectory of a workspace in the repos base dir", workspace: "base/acme/widgets/feature-1/pkg", want: "acme/widgets"},
		{name: "workspace outside the repos base dir", workspace: "home/u/src/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			workspace := filepath.Join(dir, tt.workspace)
			require.NoError(t, os.MkdirAll(workspace, 0o755))
			base := filepath.Join(dir, "base")
			if tt.relativeBase {
				wd, err := os.Getwd()
				require.NoError(t, err)
				base, err = filepath.Rel(wd, base)
				require.NoError(t, err)
			}
			capture := &captureEnvSession{}
			k := Remuda{
				Config:      Config{ReposBaseDir: base},
				Multiplexer: capture,
				IO:          DefaultIO(),
				Env:         env.StaticProvider{Values: map[string]string{"PATH": "/usr/bin:/bin"}},
			}

			_, err := k.launchAgentSession(agentLaunchCommand{
				Workspace: workspace,
				AgentName: "codex",
				Command:   "true",
				Detached:  true,
			})
			require.NoError(t, err)
			require.Equal(t, tt.want, capture.agentStart.Repo)
		})
	}
}
