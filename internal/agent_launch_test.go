package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestLaunchAgentSessionSetsSharedBeadsDirWhenPresent(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	beadsDir := filepath.Join(base, "org", "repo", ".beads_worktree", ".beads")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(beadsDir, 0o755))

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		IO:      DefaultIO(),
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
	value, ok := envValue(sm.startEnv, "BEADS_DIR")
	require.True(t, ok)
	require.Equal(t, beadsDir, value)
}

func TestLaunchAgentSessionPreservesExplicitBeadsDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	beadsDir := filepath.Join(base, "org", "repo", ".beads_worktree", ".beads")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(beadsDir, 0o755))

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		IO:      DefaultIO(),
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
