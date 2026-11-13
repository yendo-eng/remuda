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
	"github.com/yendo-eng/remuda/internal/testutils"
)

func writeTestManifest(t *testing.T, workspace string, manifest internal.SessionManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, internal.SessionManifestFileName), data, 0o644))
}

func runSessionResume(t *testing.T, workspace string, args ...string) string {
	return runSessionResumeWithConfig(t, workspace, "", args...)
}

func runSessionResumeWithConfig(t *testing.T, workspace, configYAML string, args ...string) string {
	t.Helper()

	base := filepath.Dir(filepath.Dir(filepath.Dir(workspace)))
	home := t.TempDir()
	if configYAML != "" {
		configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
		require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0o644))
	}
	mgr := &testutils.MockMultiplexer{MultiplexerName: string(session.MultiplexerTmux)}
	remuda := internal.NewRemuda(internal.Config{ReposBaseDir: base})
	remuda.Git = noopGit{}

	ctx := NewContext(
		context.Background(),
		remuda,
		WithEnv(EnvMap{}),
		WithHomeDir(home),
		WithWorkingDir(t.TempDir()),
	)
	ctx.MultiplexerFactory = func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
		return mgr
	}

	runArgs := append([]string{"session", "resume", workspace}, args...)
	require.NoError(t, Run(ctx, runArgs))
	require.NotEmpty(t, mgr.LastStart.Command)
	return mgr.LastStart.Command
}

func TestSessionResume_DefaultsFromManifest(t *testing.T) {
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
	require.Contains(t, startCmd, "claude --continue")
	require.Contains(t, startCmd, "--model 'claude-opus-4-8'")
	require.Contains(t, startCmd, "--dangerously-skip-permissions")
}

func TestSessionResume_ExplicitProfileWinsOverManifest(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	writeTestManifest(t, workspace, internal.SessionManifest{
		Agent: "claude",
		Model: "claude-opus-4-8",
		Yolo:  true,
	})

	startCmd := runSessionResumeWithConfig(t, workspace, `
version: 1
profiles:
  resume:
    agent: codex
    model: gpt-5.6-luna
    yolo: false
`, "--experiments", "session-manifest", "--profile", "resume")
	require.Contains(t, startCmd, "codex resume --last")
	require.Contains(t, startCmd, "--model 'gpt-5.6-luna'")
	require.NotContains(t, startCmd, "--dangerously-bypass-approvals-and-sandbox")
}

func TestSessionResume_ExplicitFlagsWinOverProfileAndManifest(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	writeTestManifest(t, workspace, internal.SessionManifest{
		Agent: "claude",
		Model: "claude-opus-4-8",
		Yolo:  true,
	})

	startCmd := runSessionResumeWithConfig(t, workspace, `
version: 1
profiles:
  resume:
    agent: claude
    model: claude-sonnet-4-6
    yolo: true
`, "--experiments", "session-manifest", "--profile", "resume", "--agent", "codex", "--model", "gpt-5.6-luna", "--no-yolo")
	require.Contains(t, startCmd, "codex resume --last")
	require.Contains(t, startCmd, "--model 'gpt-5.6-luna'")
	require.NotContains(t, startCmd, "--dangerously-skip-permissions")
	require.NotContains(t, startCmd, "--dangerously-bypass-approvals-and-sandbox")
}

func TestSessionResume_UsesDefaultsWithoutManifest(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	startCmd := runSessionResume(t, workspace)
	require.Contains(t, startCmd, "codex resume --last")
}
