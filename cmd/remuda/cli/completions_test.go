package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/github"
)

// runComplete drives cobra's __complete protocol through the real CLI stack
// and returns the completion candidates.
func runComplete(t *testing.T, env cli.EnvMap, home string, args ...string) []string {
	t.Helper()
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)

	var stdout, stderr bytes.Buffer
	ctx := cli.NewContext(
		context.Background(),
		internal.Remuda{},
		cli.WithEnv(env),
		cli.WithHomeDir(home),
		cli.WithWorkingDir(home),
		cli.Stdout(&stdout),
		cli.Stderr(&stderr),
	)
	err := cli.Run(ctx, append([]string{"__complete"}, args...))
	require.NoError(t, err, stderr.String())

	var candidates []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		// Strip completion descriptions ("candidate\tdescription").
		if idx := strings.IndexByte(line, '\t'); idx != -1 {
			line = line[:idx]
		}
		candidates = append(candidates, line)
	}
	return candidates
}

func writeCompletionConfig(t *testing.T, home, content string) {
	t.Helper()
	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))
}

func TestCompleteModel_UsesConfigDefaultAgent(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, "version: 1\ndefaults:\n  agent: opencode\n")

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--model", "")

	expected, _, err := agentlauncher.Parse("opencode", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestCompleteModel_EnvOverridesConfig(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, "version: 1\ndefaults:\n  agent: opencode\n")

	got := runComplete(t, cli.EnvMap{"REMUDA_AGENT": "claude"}, home, "vibe", "--model", "")

	expected, _, err := agentlauncher.Parse("claude", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestCompleteModel_FlagOverridesEnvAndConfig(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, "version: 1\ndefaults:\n  agent: opencode\n")

	got := runComplete(t, cli.EnvMap{"REMUDA_AGENT": "opencode"}, home,
		"vibe", "--agent", "claude", "--model", "")

	expected, _, err := agentlauncher.Parse("claude", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestCompleteAgent_ListsValidAgents(t *testing.T) {
	home := t.TempDir()

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--agent", "")
	require.Equal(t, []string{"codex", "opencode", "claude", "bash"}, got)
}

func TestCompleteReasoningLevel_UsesConfigDefaults(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, "version: 1\ndefaults:\n  agent: codex\n")

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--reasoning-level", "")
	require.Equal(t, agentlauncher.SuggestedReasoningLevels("codex", agentlauncher.EffectiveModel("codex", "")), got)
}

func TestCompleteNoUse_UsesConfigDefaultsAndUseFlags(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, "version: 1\ndefaults:\n  use_prompts: [small-commits]\n")

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--use", "make-pr", "--no-use", "")
	require.ElementsMatch(t, []string{"small-commits", "make-pr"}, got)
}

func TestCompleteNoUse_ExplicitUseReplacesEnvDefaults(t *testing.T) {
	home := t.TempDir()

	got := runComplete(t, cli.EnvMap{"REMUDA_USE_PROMPTS": "small-commits"}, home,
		"vibe", "--use", "make-pr", "--no-use", "")
	require.ElementsMatch(t, []string{"make-pr"}, got)
}

func TestCompleteNoUse_NoDefaultsReturnsEmpty(t *testing.T) {
	home := t.TempDir()

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--no-use", "")
	require.Empty(t, got)
}

func TestCompleteProfileNames_FromConfig(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, `
version: 1
profiles:
  fast:
    model: gpt-5
  review:
    agent: claude
`)

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--profile", "")
	require.Equal(t, []string{"fast", "review"}, got)
}

func TestCompleteRepoAliases_FromConfig(t *testing.T) {
	home := t.TempDir()
	writeCompletionConfig(t, home, `
version: 1
repos:
  aliases:
    utils: https://github.com/acme/utils.git
    widgets: https://github.com/acme/widgets.git
`)

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--repo", "")
	require.Equal(t, []string{"utils", "widgets"}, got)
}

func TestCompletePromptNames_ReturnsAllPrompts(t *testing.T) {
	home := t.TempDir()

	got := runComplete(t, cli.EnvMap{}, home, "vibe", "--use", "")
	require.Contains(t, got, "small-commits")
	require.Contains(t, got, "make-pr")
}

func TestCompletionsCmd_GeneratesBashScript(t *testing.T) {
	home := t.TempDir()

	var stdout, stderr bytes.Buffer
	ctx := cli.NewContext(
		context.Background(),
		internal.Remuda{},
		cli.WithEnv(cli.EnvMap{}),
		cli.WithHomeDir(home),
		cli.WithWorkingDir(home),
		cli.Stdout(&stdout),
		cli.Stderr(&stderr),
	)
	require.NoError(t, cli.Run(ctx, []string{"completions", "bash"}))
	require.Contains(t, stdout.String(), "remuda")
	require.Contains(t, stdout.String(), "__remuda")
}
