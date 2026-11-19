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

func TestSessionResume(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		manifest       *internal.SessionManifest
		configYAML     string
		args           []string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "defaults_from_manifest",
			manifest: &internal.SessionManifest{
				Agent: "claude",
				Model: "claude-opus-4-8",
				Yolo:  true,
			},
			wantContains: []string{
				"claude --continue",
				"--model 'claude-opus-4-8'",
				"--dangerously-skip-permissions",
			},
		},
		{
			name: "explicit_profile_overrides_defined_fields",
			manifest: &internal.SessionManifest{
				Agent: "claude",
				Model: "claude-opus-4-8",
				Yolo:  true,
			},
			configYAML: `
version: 1
profiles:
  resume:
    agent: codex
    model: gpt-5.6-luna
`,
			args: []string{"--experiments", "session-manifest", "--profile", "resume"},
			wantContains: []string{
				"codex resume --last",
				"--model 'gpt-5.6-luna'",
				"--dangerously-bypass-approvals-and-sandbox",
			},
			wantNotContain: []string{"--dangerously-skip-permissions"},
		},
		{
			name: "explicit_profile_uses_manifest_agent_when_unset",
			manifest: &internal.SessionManifest{
				Agent: "claude",
				Model: "claude-opus-4-8",
				Yolo:  true,
			},
			configYAML: `
version: 1
profiles:
  resume:
    model: gpt-5.6-luna
`,
			args: []string{"--profile", "resume"},
			wantContains: []string{
				"claude --continue",
				"--model 'gpt-5.6-luna'",
				"--dangerously-skip-permissions",
			},
			wantNotContain: []string{"--model 'claude-opus-4-8'"},
		},
		{
			name: "explicit_flags_override_profile_and_manifest",
			manifest: &internal.SessionManifest{
				Agent: "claude",
				Model: "claude-opus-4-8",
				Yolo:  true,
			},
			configYAML: `
version: 1
profiles:
  resume:
    agent: claude
    model: claude-sonnet-4-6
    yolo: true
`,
			args: []string{"--profile", "resume", "--agent", "codex", "--model", "gpt-5.6-luna", "--no-yolo"},
			wantContains: []string{
				"codex resume --last",
				"--model 'gpt-5.6-luna'",
			},
			wantNotContain: []string{
				"--dangerously-skip-permissions",
				"--dangerously-bypass-approvals-and-sandbox",
			},
		},
		{
			name:         "defaults_without_manifest",
			wantContains: []string{"codex resume --last"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			workspace := filepath.Join(base, "owner", "repo", "wk")
			require.NoError(t, os.MkdirAll(workspace, 0o755))
			if tt.manifest != nil {
				writeTestManifest(t, workspace, *tt.manifest)
			}

			startCmd := runSessionResumeWithConfig(t, workspace, tt.configYAML, tt.args...)
			for _, want := range tt.wantContains {
				require.Contains(t, startCmd, want)
			}
			for _, notWant := range tt.wantNotContain {
				require.NotContains(t, startCmd, notWant)
			}
		})
	}
}
