package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/testutils"
)

func TestSessionResume_BuildsCommandAndArgsFromLauncher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		model   string
		args    []string
		command string
	}{
		{
			name:  "codex",
			model: "gpt-5.5",
			args: []string{
				"resume", "--last",
				"--dangerously-bypass-approvals-and-sandbox",
				"--dangerously-bypass-hook-trust",
				"--config", "shell_environment_policy.ignore_default_excludes=true",
				"--model", "gpt-5.5",
				"--config", "model_reasoning_effort=high",
			},
			command: "codex resume --last --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust --config shell_environment_policy.ignore_default_excludes=\"true\" --model 'gpt-5.5' --config model_reasoning_effort='high' -- 'continue from checkpoint'",
		},
		{
			name:  "claude",
			model: "claude-sonnet-4-6",
			args: []string{
				"--continue", "--model", "claude-sonnet-4-6",
				"--dangerously-skip-permissions", "--effort", "high",
			},
			command: "claude --continue --model 'claude-sonnet-4-6' --dangerously-skip-permissions --effort 'high' 'continue from checkpoint'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			workspace := filepath.Join(base, "org", "repo", "folder")
			require.NoError(t, os.MkdirAll(workspace, 0o755))

			capture := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
			k := Remuda{
				Config:      Config{ReposBaseDir: base},
				Multiplexer: capture,
				IO:          DefaultIO(),
			}

			err := k.SessionResume(context.Background(), SessionResumeCommand{
				Workspace:      workspace,
				Agent:          tt.name,
				Model:          tt.model,
				Detached:       true,
				Yolo:           true,
				ReasoningLevel: "high",
				Prompt:         "continue from checkpoint",
			})
			require.NoError(t, err)
			require.Equal(t, tt.args, capture.LastStart.Args)
			require.Contains(t, capture.LastStart.Command, tt.command)
			require.Equal(t, "continue from checkpoint", capture.LastStart.Prompt)
		})
	}
}

func TestSessionResume_StartsCodexDetachedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "codex resume --last")
	require.NotContains(t, cmd, "REMUDA_AGENT=")
	require.NotContains(t, cmd, "export BD_ACTOR=")
	require.NotContains(t, cmd, "export BEADS_DIR=")
	require.Contains(t, cmd, "sleep 3600")
	value, ok := envValue(mgr.StartEnv["org/repo/folder"], "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "codex", value)
	value, ok = envValue(mgr.StartEnv["org/repo/folder"], "BD_ACTOR")
	require.True(t, ok)
	require.Equal(t, "org/repo/folder", value)
}

func TestSessionResume_ClaudeStartsDetachedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "claude",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "claude --continue")
	require.NotContains(t, cmd, "codex resume --last")
	require.NotContains(t, cmd, "REMUDA_AGENT=")
	value, ok := envValue(mgr.StartEnv["org/repo/folder"], "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "claude", value)
}

func TestSessionResume_ClaudeModelAndPromptFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "claude",
		Model:     "claude-sonnet-4.6",
		Prompt:    "continue with tests",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "claude --continue")
	require.Contains(t, cmd, "--model 'claude-sonnet-4.6'")
	require.Contains(t, cmd, "'continue with tests'")
	require.NotContains(t, cmd, "REMUDA_MODEL")

	value, ok := envValue(mgr.StartEnv["org/repo/folder"], "REMUDA_MODEL")
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4.6", value)
}

func TestSessionResume_DetachedTmuxExportsImplicitAnthropicForClaudeContainer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace:     workspace,
		Agent:         "claude",
		Detached:      true,
		Container:     true,
		ContainerName: "ghcr.io/acme/vibe-dev:latest",
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, cmd, "export ANTHROPIC_API_KEY=")
	value, ok := envValue(mgr.StartEnv["org/repo/folder"], "ANTHROPIC_API_KEY")
	require.True(t, ok)
	require.Equal(t, "anthropic-secret", value)
}

func TestSessionResume_ContainerRequiresExplicitImage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
		Container: true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "container mode requires an explicit image")
	require.Empty(t, mgr.Started)
}

func TestSessionResume_UnsupportedAgentErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "opencode",
		Detached:  true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, `session resume unsupported for agent "opencode"`)
	require.Empty(t, mgr.Started)
}

func TestSessionResume_RefusesActiveWorkspace(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	mgr := &testutils.MockMultiplexer{
		MultiplexerName: "tmux",
		Sessions:        []testutils.Session{{SessionInfo: session.SessionInfo{Name: "org/repo/folder"}}},
	}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "active")
	require.ErrorContains(t, err, "org/repo/folder")
	require.Empty(t, mgr.Started)
}

func TestSessionResume_ValidatesWorkspaceEligibility(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "depth 3")
	require.Empty(t, mgr.Started)
}

func TestSessionResume_HerdrCoreCodexRunsWithoutDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: string(session.MultiplexerHerdrCore)}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Prompt:    "continue",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "codex resume --last '--no-daemon' -- 'continue'")
}

func TestSessionResume_CustomAgentCommandAppendsPrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "opencode",
		AgentCmd:  "my-resume",
		Prompt:    "pick this up",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "my-resume 'pick this up'")
	require.NotContains(t, cmd, "REMUDA_AGENT")

	value, ok := envValue(mgr.StartEnv["org/repo/folder"], "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "opencode", value)
}

func TestSessionResume_OpenAIKeyOverridesEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "from-env")

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &testutils.MockMultiplexer{MultiplexerName: "tmux"}
	k := Remuda{
		Config:      Config{ReposBaseDir: base},
		Multiplexer: mgr,
		IO:          DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace:    workspace,
		Detached:     true,
		EnvOverrides: map[string]string{"OPENAI_API_KEY": "override-key"},
	})
	require.NoError(t, err)

	cmd, ok := mgr.Started["org/repo/folder"]
	require.True(t, ok)
	require.NotContains(t, cmd, "OPENAI_API_KEY")

	startEnv, ok := mgr.StartEnv["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, startEnv, "OPENAI_API_KEY=override-key")
}
