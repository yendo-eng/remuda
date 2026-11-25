package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionResume(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap(testutils.ProcessEnvMap())
	env["REMUDA_ISOLATION"] = "false"

	setup := func(t *testing.T) (string, *testutils.MockMultiplexer, internal.Remuda) {
		baseDir := t.TempDir()
		mgr := &testutils.MockMultiplexer{}
		k := internal.NewRemuda(internal.Config{ReposBaseDir: baseDir})
		k.Multiplexer = mgr
		k.Jira = jira.Mock{}
		k.Docker = &docker.Mock{Running: false}
		return baseDir, mgr, k
	}

	t.Run("resumes inactive workspace by path", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "codex resume --last")
		require.NotContains(t, sess.CommandRan, "REMUDA_AGENT=")
		require.NotContains(t, sess.CommandRan, "export BD_ACTOR=")
		require.NotContains(t, sess.CommandRan, "export BEADS_DIR=")
		require.Contains(t, sess.CommandRan, "sleep 3600")
		value, ok := sessionEnvValue(sess.StartEnv, "REMUDA_AGENT")
		require.True(t, ok)
		require.Equal(t, "codex", value)
		value, ok = sessionEnvValue(sess.StartEnv, "BD_ACTOR")
		require.True(t, ok)
		require.Equal(t, "org/repo/folder", value)
	})

	t.Run("resumes workspace with claude when agent is configured", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		claudeEnv := cli.EnvMap(testutils.ProcessEnvMap())
		claudeEnv["REMUDA_ISOLATION"] = "false"
		claudeEnv["REMUDA_AGENT"] = "claude"

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(claudeEnv), cli.WithHomeDir(claudeEnv["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "claude --continue")
		require.NotContains(t, sess.CommandRan, "codex resume --last")
		value, ok := sessionEnvValue(sess.StartEnv, "REMUDA_AGENT")
		require.True(t, ok)
		require.Equal(t, "claude", value)
	})

	t.Run("openai api key override uses start env only", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", "--openai-api-key", "resume-secret", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.NotContains(t, sess.CommandRan, "resume-secret")
		require.NotContains(t, sess.CommandRan, "OPENAI_API_KEY=resume-secret")
		value, ok := sessionEnvValue(sess.StartEnv, "OPENAI_API_KEY")
		require.True(t, ok)
		require.Equal(t, "resume-secret", value)
	})

	t.Run("does not leak openai key into session start command", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		keyEnv := cli.EnvMap(testutils.ProcessEnvMap())
		keyEnv["REMUDA_ISOLATION"] = "false"
		keyEnv["OPENAI_API_KEY"] = "sk-live-never-show"

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(keyEnv), cli.WithHomeDir(keyEnv["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.NotContains(t, sess.CommandRan, "sk-live-never-show")
		require.NotContains(t, sess.CommandRan, "OPENAI_API_KEY=")
	})

	t.Run("resumes workspace with explicit model and prompt", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{
			"session", "resume",
			"--agent", "claude",
			"--model", "claude-sonnet-4.6",
			workspace,
			"continue with changelog updates",
		})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "claude --continue")
		require.Contains(t, sess.CommandRan, "--model 'claude-sonnet-4.6'")
		require.Contains(t, sess.CommandRan, "'continue with changelog updates'")
		require.NotContains(t, sess.CommandRan, "REMUDA_MODEL")
		value, ok := sessionEnvValue(sess.StartEnv, "REMUDA_MODEL")
		require.True(t, ok)
		require.Equal(t, "claude-sonnet-4.6", value)
	})

	t.Run("returns clear error for unsupported built-in resume agent", func(t *testing.T) {
		baseDir, _, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", "--agent", "opencode", workspace})
		require.Error(t, err)
		require.ErrorContains(t, err, `session resume unsupported for agent "opencode"`)
	})

	t.Run("refuses active workspace", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(workspace, 0o755))

		require.NoError(t, mgr.Start(session.AgentStart{SessionName: "org/repo/folder", Command: "echo active"}))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", workspace})
		require.Error(t, err)
		require.ErrorContains(t, err, "refuse to resume")

		list, lerr := mgr.List()
		require.NoError(t, lerr)
		require.Len(t, list, 1)
	})

	t.Run("validates workspace eligibility", func(t *testing.T) {
		baseDir, _, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo")
		require.NoError(t, os.MkdirAll(workspace, 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", workspace})
		require.Error(t, err)
		require.ErrorContains(t, err, "depth 3")
	})

	t.Run("pick applies per_repo selected profile", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))
		tty, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tty.Close() })
		k.IO = internal.IO{In: tty, Out: tty, Err: tty}

		binDir := t.TempDir()
		fzfPath := filepath.Join(binDir, "fzf")
		require.NoError(t, os.WriteFile(fzfPath, []byte("#!/bin/sh\nread -r line1\nprintf '%s\\n' \"$line1\"\n"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: codex
profiles:
  review:
    agent: claude
per_repo:
  org/repo:
    profile: review
`), 0o644))

		pickEnv := cli.EnvMap(testutils.ProcessEnvMap())
		pickEnv["REMUDA_ISOLATION"] = "false"
		pickEnv["REMUDA_CONFIG"] = configPath
		pickEnv["PATH"] = binDir + string(os.PathListSeparator) + pickEnv["PATH"]

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(pickEnv), cli.WithHomeDir(pickEnv["HOME"]))
		err = cli.Run(ctx, []string{"session", "resume", "--pick"})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "claude --continue")
		require.NotContains(t, sess.CommandRan, "codex resume --last")
		value, ok := sessionEnvValue(sess.StartEnv, "REMUDA_AGENT")
		require.True(t, ok)
		require.Equal(t, "claude", value)
	})

	t.Run("pick applies per_repo isolation defaults", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		k.Docker = &docker.Mock{Running: true}
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))
		tty, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tty.Close() })
		k.IO = internal.IO{In: tty, Out: tty, Err: tty}

		binDir := t.TempDir()
		fzfPath := filepath.Join(binDir, "fzf")
		require.NoError(t, os.WriteFile(fzfPath, []byte("#!/bin/sh\nread -r line1\nprintf '%s\\n' \"$line1\"\n"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
per_repo:
  org/repo:
    defaults:
      isolation:
        enabled: true
        image: ghcr.io/acme/vibe-dev:latest
        opts:
          - --memory=2g
`), 0o644))

		pickEnv := cli.EnvMap(testutils.ProcessEnvMap())
		delete(pickEnv, "REMUDA_ISOLATION")
		pickEnv["REMUDA_CONFIG"] = configPath
		pickEnv["GH_TOKEN"] = "test-token"
		pickEnv["PATH"] = binDir + string(os.PathListSeparator) + pickEnv["PATH"]

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(pickEnv), cli.WithHomeDir(pickEnv["HOME"]))
		err = cli.Run(ctx, []string{"session", "resume", "--pick"})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "'docker' 'run' '--rm' '-it'")
		require.Contains(t, sess.CommandRan, "ghcr.io/acme/vibe-dev:latest")
		require.Contains(t, sess.CommandRan, "--memory=2g")
	})

	t.Run("pick preserves explicit agent flag over per_repo profile", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))
		tty, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tty.Close() })
		k.IO = internal.IO{In: tty, Out: tty, Err: tty}

		binDir := t.TempDir()
		fzfPath := filepath.Join(binDir, "fzf")
		require.NoError(t, os.WriteFile(fzfPath, []byte("#!/bin/sh\nread -r line1\nprintf '%s\\n' \"$line1\"\n"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
profiles:
  review:
    agent: claude
per_repo:
  org/repo:
    profile: review
`), 0o644))

		pickEnv := cli.EnvMap(testutils.ProcessEnvMap())
		pickEnv["REMUDA_ISOLATION"] = "false"
		pickEnv["REMUDA_CONFIG"] = configPath
		pickEnv["PATH"] = binDir + string(os.PathListSeparator) + pickEnv["PATH"]

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(pickEnv), cli.WithHomeDir(pickEnv["HOME"]))
		err = cli.Run(ctx, []string{"session", "resume", "--pick", "--agent", "codex"})
		require.NoError(t, err)

		// The per_repo overlay selects the "review" profile (agent: claude),
		// discovered only after --pick resolves the workspace. The explicit
		// --agent flag must survive that re-resolution unchanged.
		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "codex resume --last")
		require.NotContains(t, sess.CommandRan, "claude --continue")
		value, ok := sessionEnvValue(sess.StartEnv, "REMUDA_AGENT")
		require.True(t, ok)
		require.Equal(t, "codex", value)
	})

	t.Run("pick respects session.prune.ignore patterns", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		beadsWorktree := filepath.Join(baseDir, "org", "repo", ".beads_worktree")
		require.NoError(t, os.MkdirAll(workspace, 0o755))
		require.NoError(t, os.MkdirAll(beadsWorktree, 0o755))

		tty, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tty.Close() })
		k.IO = internal.IO{In: tty, Out: tty, Err: tty}

		binDir := t.TempDir()
		fzfPath := filepath.Join(binDir, "fzf")
		require.NoError(t, os.WriteFile(fzfPath, []byte("#!/bin/sh\nread -r line1\nprintf '%s\\n' \"$line1\"\n"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
session:
  prune:
    ignore:
      - org/repo/.beads_worktree
`), 0o644))

		pickEnv := cli.EnvMap(testutils.ProcessEnvMap())
		pickEnv["REMUDA_ISOLATION"] = "false"
		pickEnv["REMUDA_CONFIG"] = configPath
		pickEnv["PATH"] = binDir + string(os.PathListSeparator) + pickEnv["PATH"]

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(pickEnv), cli.WithHomeDir(pickEnv["HOME"]))
		err = cli.Run(ctx, []string{"session", "resume", "--pick"})
		require.NoError(t, err)

		require.NotNil(t, mgr.FindSession("org/repo/folder"))
		require.Nil(t, mgr.FindSession("org/repo/.beads_worktree"))
	})

	t.Run("isolation resume fails when image is missing", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		k.Docker = &docker.Mock{Running: true}
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", "--isolation", workspace})
		require.Error(t, err)
		require.ErrorContains(t, err, "isolation mode requires an explicit image")

		sessions, listErr := mgr.List()
		require.NoError(t, listErr)
		require.Empty(t, sessions)
	})

	t.Run("isolation resume uses configured image without --isolation-image", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		k.Docker = &docker.Mock{Running: true}
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  isolation:
    image: ghcr.io/acme/vibe-dev:latest
`), 0o644))

		containerEnv := cli.EnvMap(testutils.ProcessEnvMap())
		containerEnv["REMUDA_ISOLATION"] = "false"
		containerEnv["REMUDA_CONFIG"] = configPath
		containerEnv["GH_TOKEN"] = "test-token"

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(containerEnv), cli.WithHomeDir(containerEnv["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", "--isolation", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "ghcr.io/acme/vibe-dev:latest")
		require.Contains(t, sess.CommandRan, "'docker' 'run' '--rm' '-it'")
	})
}

func TestSessionResumeRejectsOutdatedManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{
			name: "experiment manifest with top-level container",
			data: `{"schema_version":1,"agent":"codex","container":{"enabled":true}}`,
		},
		{
			name: "manifest with defaults container",
			data: `{"schema_version":2,"defaults":{"Container":{"Enabled":true}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			workspaceRoot := t.TempDir()
			workspace := filepath.Join(workspaceRoot, "org", "repo", "wk")
			testutils.InitWorkspace(t, workspace)
			manifestPath := filepath.Join(workspace, internal.SessionManifestFileName)
			require.NoError(t, os.WriteFile(manifestPath, []byte(tt.data), 0o600))

			h := testutils.NewHarness(t,
				testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
				testutils.WithMultiplexer(&testutils.MockMultiplexer{}),
				testutils.WithDocker(&docker.Mock{Running: true}),
			)

			res := h.Run("session", "resume", workspace)
			require.Error(t, res.Err)
			require.ErrorContains(t, res.Err, "unsupported schema version")
			require.ErrorContains(t, res.Err, "delete it to resume with config defaults")
		})
	}
}
