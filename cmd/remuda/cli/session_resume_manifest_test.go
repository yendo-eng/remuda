package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/testutils"
)

func writeTestManifest(t *testing.T, workspace string, manifest internal.SessionManifest) {
	t.Helper()
	manifest.SchemaVersion = internal.SessionManifestSchemaVersion
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, internal.SessionManifestFileName), data, 0o644))
}

func sessionManifest(agent, model string, yolo bool) *internal.SessionManifest {
	reasoningLevel := ""
	agentCmd := ""
	usePrompts := []string{}
	usePosition := ""
	containerEnabled := false
	containerImage := ""
	containerOpts := []string{}
	containerInheritEnv := []string{}
	return &internal.SessionManifest{Defaults: configfile.DefaultsV1{
		Agent:              &agent,
		Model:              &model,
		ReasoningLevel:     &reasoningLevel,
		AgentCmd:           &agentCmd,
		AgentArgs:          map[string][]string{},
		UsePrompts:         &usePrompts,
		UsePromptsPosition: &usePosition,
		Yolo:               &yolo,
		Container: &configfile.ContainerV1{
			Enabled:    &containerEnabled,
			Image:      &containerImage,
			Opts:       &containerOpts,
			InheritEnv: &containerInheritEnv,
		},
	}}
}

func sessionManifestWithAllSettings() *internal.SessionManifest {
	manifest := sessionManifest("claude", "claude-opus-4-8", true)
	reasoningLevel := "high"
	agentCmd := ""
	usePrompts := []string{"make-pr"}
	usePosition := "after"
	containerEnabled := true
	containerImage := "manifest-image"
	containerOpts := []string{"--cpus=2"}
	containerInheritEnv := []string{"MANIFEST_TOKEN"}
	manifest.Defaults.ReasoningLevel = &reasoningLevel
	manifest.Defaults.AgentCmd = &agentCmd
	manifest.Defaults.UsePrompts = &usePrompts
	manifest.Defaults.UsePromptsPosition = &usePosition
	manifest.Defaults.Container = &configfile.ContainerV1{
		Enabled:    &containerEnabled,
		Image:      &containerImage,
		Opts:       &containerOpts,
		InheritEnv: &containerInheritEnv,
	}
	return manifest
}

func runSessionResumeWithConfig(t *testing.T, workspace, configYAML string, env EnvMap, args ...string) session.AgentStart {
	t.Helper()
	if env == nil {
		env = EnvMap{}
	}

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
	remuda.Docker = &docker.Mock{Running: true}

	ctx := NewContext(
		context.Background(),
		remuda,
		WithEnv(env),
		WithHomeDir(home),
		WithWorkingDir(t.TempDir()),
	)
	ctx.MultiplexerFactory = func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
		return mgr
	}

	runArgs := append([]string{"session", "resume", workspace}, args...)
	require.NoError(t, Run(ctx, runArgs))
	require.NotEmpty(t, mgr.LastStart.Command)
	return mgr.LastStart
}

func TestSessionResume(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		manifest       *internal.SessionManifest
		configYAML     string
		env            EnvMap
		args           []string
		wantContains   []string
		wantNotContain []string
		wantAgent      string
		wantContainer  bool
		wantForwardEnv []string
		wantNoEnv      []string
		promptAfter    string
	}{
		{
			name:     "defaults_from_manifest",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
			wantContains: []string{
				"claude --continue",
				"--model 'claude-opus-4-8'",
				"--dangerously-skip-permissions",
			},
		},
		{
			name:     "explicit_profile_overrides_manifest_fields",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
			configYAML: `
version: 1
profiles:
  resume:
    agent: codex
    model: gpt-5.6-luna
`,
			args: []string{"--profile", "resume"},
			wantContains: []string{
				"codex resume --last",
				"--model 'gpt-5.6-luna'",
				"--dangerously-bypass-approvals-and-sandbox",
			},
			wantNotContain: []string{"--model 'claude-opus-4-8'"},
		},
		{
			name:     "explicit_profile_uses_manifest_agent_when_unset",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
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
			name:     "environment_overrides_manifest",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
			env:      EnvMap{"REMUDA_MODEL": "gpt-5.6-luna"},
			wantContains: []string{
				"claude --continue",
				"--model 'gpt-5.6-luna'",
				"--dangerously-skip-permissions",
			},
			wantNotContain: []string{"--model 'claude-opus-4-8'"},
		},
		{
			name:     "environment_profile_overrides_manifest",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
			configYAML: `
version: 1
profiles:
  resume:
    model: gpt-5.6-luna
`,
			env: EnvMap{"REMUDA_PROFILE": "resume"},
			wantContains: []string{
				"claude --continue",
				"--model 'gpt-5.6-luna'",
				"--dangerously-skip-permissions",
			},
			wantNotContain: []string{"--model 'claude-opus-4-8'"},
		},
		{
			name:     "manifest_overrides_base_config",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
			configYAML: `
version: 1
defaults:
  agent: codex
  model: gpt-5.6-luna
  yolo: false
`,
			wantContains: []string{
				"claude --continue",
				"--model 'claude-opus-4-8'",
				"--dangerously-skip-permissions",
			},
			wantNotContain: []string{"--model 'gpt-5.6-luna'"},
		},
		{
			name:     "manifest_defaults_reach_resume_command",
			manifest: sessionManifestWithAllSettings(),
			configYAML: `
version: 1
defaults:
  agent: codex
  model: gpt-6-astra
  reasoning_level: low
  agent_cmd: echo from-base-config
  yolo: false
  use_prompts: [small-commits]
  use_prompts_position: before
  container:
    enabled: false
    image: base-image
    opts: [--memory=1g]
    inherit_env: [BASE_TOKEN]
`,
			args: []string{"continue from checkpoint"},
			wantContains: []string{
				"claude --continue",
				`--model '\''claude-opus-4-8'\''`,
				`--effort '\''high'\''`,
				"--dangerously-skip-permissions",
				"manifest-image",
				"--cpus=2",
			},
			wantNotContain: []string{"from-base-config", "base-image", "--memory=1g"},
			wantAgent:      "claude",
			wantContainer:  true,
			wantForwardEnv: []string{"MANIFEST_TOKEN"},
			wantNoEnv:      []string{"BASE_TOKEN"},
			promptAfter:    "When you finish the requested work, open a GitHub Pull Request",
		},
		{
			name:     "explicit_flags_override_profile_and_manifest",
			manifest: sessionManifest("claude", "claude-opus-4-8", true),
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

			start := runSessionResumeWithConfig(t, workspace, tt.configYAML, tt.env, tt.args...)
			for _, want := range tt.wantContains {
				require.Contains(t, start.Command, want)
			}
			for _, notWant := range tt.wantNotContain {
				require.NotContains(t, start.Command, notWant)
			}
			if tt.wantAgent != "" {
				require.Equal(t, tt.wantAgent, start.Agent)
			}
			if tt.wantContainer {
				require.True(t, start.Container)
			}
			for _, env := range tt.wantForwardEnv {
				require.Contains(t, start.ForwardEnv, env)
			}
			for _, env := range tt.wantNoEnv {
				require.NotContains(t, start.ForwardEnv, env)
			}
			if tt.promptAfter != "" {
				userPromptAt := strings.Index(start.Prompt, tt.args[0])
				savedPromptAt := strings.Index(start.Prompt, tt.promptAfter)
				require.Greater(t, userPromptAt, -1)
				require.Greater(t, savedPromptAt, userPromptAt)
			}
		})
	}
}
