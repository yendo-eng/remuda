package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/session"
)

func writeTestManifest(t *testing.T, workspace string, manifest internal.SessionManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, internal.SessionManifestFileName), data, 0o644))
}

func runSessionResume(t *testing.T, workspace string, args ...string) string {
	t.Helper()

	base := filepath.Dir(filepath.Dir(filepath.Dir(workspace)))
	mgr := &captureStartSessionManager{}
	remuda := internal.NewRemuda(
		internal.Config{ReposBaseDir: base},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
	)

	ctx := NewContext(
		context.Background(),
		remuda,
		WithHomeDir(t.TempDir()),
		WithWorkingDir(t.TempDir()),
		WithSessionManagerFactory(func(name session.SupportedSessionManager, _ zerolog.Logger) session.SessionManager {
			return mgr
		}),
	)

	runArgs := append([]string{"session", "resume", workspace}, args...)
	require.NoError(t, Run(ctx, runArgs))
	require.NotEmpty(t, mgr.startCmd)
	return mgr.startCmd
}

func TestSessionResume_DefaultsFromManifestWhenExperimentEnabled(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	writeTestManifest(t, workspace, internal.SessionManifest{
		Agent: "claude",
		Model: "claude-opus-4-8",
		Yolo:  true,
	})

	startCmd := runSessionResume(t, workspace, "--experiments", "session-manifest")
	require.Contains(t, startCmd, "claude --continue")
	require.Contains(t, startCmd, "--model 'claude-opus-4-8'")
	require.Contains(t, startCmd, "--dangerously-skip-permissions")
}

func TestSessionResume_ExplicitFlagsWinOverManifest(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	writeTestManifest(t, workspace, internal.SessionManifest{
		Agent: "claude",
		Model: "claude-opus-4-8",
		Yolo:  true,
	})

	startCmd := runSessionResume(t, workspace, "--experiments", "session-manifest", "--agent", "codex", "--no-yolo")
	require.Contains(t, startCmd, "codex resume --last")
	require.NotContains(t, startCmd, "--dangerously-skip-permissions")
	require.NotContains(t, startCmd, "--dangerously-bypass-approvals-and-sandbox")
}

func TestSessionResume_IgnoresManifestWhenExperimentDisabled(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	writeTestManifest(t, workspace, internal.SessionManifest{
		Agent: "claude",
		Model: "claude-opus-4-8",
		Yolo:  true,
	})

	startCmd := runSessionResume(t, workspace)
	require.Contains(t, startCmd, "codex resume --last")
}
