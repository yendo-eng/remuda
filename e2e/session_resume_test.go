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
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
)

func TestSessionResume(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap(testutils.ProcessEnvMap())
	env["REMUDA_CONTAINER"] = "false"

	setup := func(t *testing.T) (string, *testutils.MockSessionManager, internal.Remuda) {
		baseDir := t.TempDir()
		mgr := &testutils.MockSessionManager{}
		k := internal.NewRemuda(
			internal.Config{ReposBaseDir: baseDir},
			git.NewShellGit(),
			mgr,
			jira.Mock{},
			&docker.Mock{Running: false},
			github.NewGhCLI(),
		)
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
		require.Contains(t, sess.CommandRan, "REMUDA_AGENT='codex'")
		require.Contains(t, sess.CommandRan, "export BD_ACTOR='org/repo/folder'")
		require.NotContains(t, sess.CommandRan, "export BEADS_DIR=")
		require.Contains(t, sess.CommandRan, "sleep 3600")
	})

	t.Run("resumes workspace with claude when agent is configured", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		claudeEnv := cli.EnvMap(testutils.ProcessEnvMap())
		claudeEnv["REMUDA_CONTAINER"] = "false"
		claudeEnv["REMUDA_AGENT"] = "claude"

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(claudeEnv), cli.WithHomeDir(claudeEnv["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "claude --continue")
		require.Contains(t, sess.CommandRan, "REMUDA_AGENT='claude'")
		require.NotContains(t, sess.CommandRan, "codex resume --last")
	})

	t.Run("does not leak openai key into session start command", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		keyEnv := cli.EnvMap(testutils.ProcessEnvMap())
		keyEnv["REMUDA_CONTAINER"] = "false"
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
		require.Contains(t, sess.CommandRan, "REMUDA_MODEL='claude-sonnet-4.6'")
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

		require.NoError(t, mgr.Start("org/repo/folder", "echo active"))

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
		pickEnv["REMUDA_CONTAINER"] = "false"
		pickEnv["REMUDA_CONFIG"] = configPath
		pickEnv["PATH"] = binDir + string(os.PathListSeparator) + pickEnv["PATH"]

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(pickEnv), cli.WithHomeDir(pickEnv["HOME"]))
		err = cli.Run(ctx, []string{"session", "resume", "--pick"})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "claude --continue")
		require.Contains(t, sess.CommandRan, "REMUDA_AGENT='claude'")
		require.NotContains(t, sess.CommandRan, "codex resume --last")
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
		pickEnv["REMUDA_CONTAINER"] = "false"
		pickEnv["REMUDA_CONFIG"] = configPath
		pickEnv["PATH"] = binDir + string(os.PathListSeparator) + pickEnv["PATH"]

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(pickEnv), cli.WithHomeDir(pickEnv["HOME"]))
		err = cli.Run(ctx, []string{"session", "resume", "--pick"})
		require.NoError(t, err)

		require.NotNil(t, mgr.FindSession("org/repo/folder"))
		require.Nil(t, mgr.FindSession("org/repo/.beads_worktree"))
	})

	t.Run("container resume fails when image is missing", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		k.Docker = &docker.Mock{Running: true}
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", "--container", workspace})
		require.Error(t, err)
		require.ErrorContains(t, err, "container mode requires an explicit image")

		sessions, listErr := mgr.List()
		require.NoError(t, listErr)
		require.Empty(t, sessions)
	})

	t.Run("container resume uses configured image without --container-name", func(t *testing.T) {
		baseDir, mgr, k := setup(t)
		k.Docker = &docker.Mock{Running: true}
		workspace := filepath.Join(baseDir, "org", "repo", "folder")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  container:
    image: ghcr.io/acme/vibe-dev:latest
`), 0o644))

		containerEnv := cli.EnvMap(testutils.ProcessEnvMap())
		containerEnv["REMUDA_CONTAINER"] = "false"
		containerEnv["REMUDA_CONFIG"] = configPath
		containerEnv["GH_TOKEN"] = "test-token"

		ctx := cli.NewContext(t.Context(), k, cli.WithEnv(containerEnv), cli.WithHomeDir(containerEnv["HOME"]))
		err := cli.Run(ctx, []string{"session", "resume", "--container", workspace})
		require.NoError(t, err)

		sess := mgr.FindSession("org/repo/folder")
		require.NotNil(t, sess)
		require.Contains(t, sess.CommandRan, "ghcr.io/acme/vibe-dev:latest")
		require.Contains(t, sess.CommandRan, "docker run --rm -it")
	})
}
