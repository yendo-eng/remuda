package e2e_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/prompts"
)

func getRemuda(t *testing.T) (internal.Remuda, *bytes.Buffer, *bytes.Buffer, cli.EnvMap) {
	reposDir := t.TempDir()

	k := internal.NewRemuda(
		internal.Config{
			ReposBaseDir: reposDir,
		},
		git.NewShellGit(),
		&testutils.MockSessionManager{},
		jira.Mock{},
		nil,
		nil,
	)

	stdout := bytes.NewBufferString("")
	stderr := bytes.NewBufferString("")

	return k, stdout, stderr, cli.EnvMap(testutils.ProcessEnvMap())
}

func TestVibeListPrompts(t *testing.T) {
	t.Parallel()
	k, stdout, stderr, env := getRemuda(t)
	env["REMUDA_PROMPTS_DIR"] = t.TempDir()
	args := []string{"prompts", "list"}
	err := cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), args)
	require.NoError(t, err)

	require.Equal(t,
		`Built-in prompts:
  small-commits     Encourage incremental changes and tight loops with git.
  make-pr           When you're done, open a GitHub PR using gh; assume you're already on a feature branc...
  update-docs       Keep documentation aligned with code changes.
  refactor-cohesion Improve cohesion and shared patterns while refactoring.
  minimal-change    Keep edits scoped to what the request strictly needs.
  prototype         Favor quick proofs of concept over production-hardening.
`, stdout.String())
}

func TestVibeShowPrompt(t *testing.T) {
	t.Parallel()
	k, stdout, stderr, env := getRemuda(t)
	env["REMUDA_PROMPTS_DIR"] = t.TempDir()
	args := []string{"prompts", "show", "small-commits"}
	err := cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), args)
	require.NoError(t, err)

	prompt, ok := prompts.Get("small-commits")
	require.True(t, ok, "expected to find built-in prompt small-commits")

	require.Equal(t, prompt.Content+"\n", stdout.String())
}

func TestVibeCustomPromptLifecycle(t *testing.T) {
	t.Parallel()
	customDir := t.TempDir()
	content := "please end your work by telling a joke"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "tell-jokes"), []byte(content), 0o644))

	k, stdout, stderr, env := getRemuda(t)
	env["REMUDA_PROMPTS_DIR"] = customDir
	args := []string{"prompts", "list"}
	require.NoError(t, cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), args))
	require.Contains(t, stdout.String(), "Built-in prompts:\n")
	require.Contains(t, stdout.String(), "\nCustom prompts:\n")
	require.Contains(t, stdout.String(), "Custom prompts:\n  tell-jokes")

	stdout.Reset()
	stderr.Reset()
	require.NoError(t, cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), []string{"prompts", "show", "tell-jokes"}))
	require.Equal(t, content+"\n", stdout.String())
}

func TestPromptsCustomOverrideBuiltin(t *testing.T) {
	t.Parallel()
	customDir := t.TempDir()
	content := "custom small commits"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "small-commits"), []byte(content), 0o644))

	k, stdout, stderr, env := getRemuda(t)
	env["REMUDA_PROMPTS_DIR"] = customDir
	require.NoError(t, cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), []string{"prompts", "show", "small-commits"}))
	require.Equal(t, content+"\n", stdout.String())

	stdout.Reset()
	stderr.Reset()
	require.NoError(t, cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), []string{"prompts", "list"}))
	require.Equal(t,
		`Built-in prompts:
  make-pr           When you're done, open a GitHub PR using gh; assume you're already on a feature branc...
  update-docs       Keep documentation aligned with code changes.
  refactor-cohesion Improve cohesion and shared patterns while refactoring.
  minimal-change    Keep edits scoped to what the request strictly needs.
  prototype         Favor quick proofs of concept over production-hardening.

Custom prompts:
  small-commits     custom small commits (overrides built-in)
`, stdout.String())
}

func TestVibeListPromptsTruncatesLongCustomDescription(t *testing.T) {
	t.Parallel()
	customDir := t.TempDir()
	longLine := "This is a very long custom prompt subject that should be truncated in list output so rows stay readable in terminals."
	content := longLine + "\nsecond line ignored in description"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "long-subject"), []byte(content), 0o644))

	k, stdout, stderr, env := getRemuda(t)
	env["REMUDA_PROMPTS_DIR"] = customDir
	require.NoError(t, cli.Run(cli.NewContext(t.Context(), k, cli.WithEnv(env), cli.WithHomeDir(env["HOME"]), cli.Stdout(stdout), cli.Stderr(stderr)), []string{"prompts", "list"}))

	require.Contains(t, stdout.String(), "\nCustom prompts:\n")
	require.Regexp(t, `(?m)^  long-subject\s+This is a very long custom prompt subject that should be truncated in list output so\.\.\.$`, stdout.String())
	require.NotContains(t, stdout.String(), longLine)
}
