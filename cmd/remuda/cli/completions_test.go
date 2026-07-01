package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/github"
)

var builtinPromptNames = []string{
	"small-commits",
	"make-pr",
	"update-docs",
	"refactor-cohesion",
	"minimal-change",
	"prototype",
}

func TestPredictModel_UsesConfigDefaultAgent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: opencode
`), 0o644))

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictModel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--model"}})

	expected, _, err := agentlauncher.Parse("opencode", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestPredictModel_EnvOverridesConfig(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
		"REMUDA_AGENT":    "codex",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: opencode
`), 0o644))

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictModel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--model"}})

	expected, _, err := agentlauncher.Parse("codex", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestPredictModel_FlagOverridesEnvAndConfig(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
		"REMUDA_AGENT":    "codex",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: codex
`), 0o644))

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictModel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--agent=opencode", "--model"}})

	expected, _, err := agentlauncher.Parse("opencode", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestPredictModel_ClaudeUsesSupportedModels(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictModel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--agent=claude", "--model"}})

	expected, _, err := agentlauncher.Parse("claude", "", false)
	require.NoError(t, err)
	require.Equal(t, expected.SupportedModels(), got)
}

func TestPredictReasoningLevel_UsesConfigDefaults(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: codex
  model: gpt-5
`), 0o644))

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictReasoningLevel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--reasoning-level"}})

	expected := agentlauncher.SupportedReasoningLevels("codex", "gpt-5")
	require.Equal(t, expected, got)
}

func TestPredictReasoningLevel_FlagOverridesConfigAgent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: codex
  model: gpt-5
`), 0o644))

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictReasoningLevel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--agent=opencode", "--reasoning-level"}})

	require.Empty(t, got)
}

func TestPredictReasoningLevel_ClaudeSuggestsEffortLevels(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
		"REMUDA_AGENT":    "opencode",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent: opencode
`), 0o644))

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictReasoningLevel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--agent=claude", "--reasoning-level"}})

	require.Equal(t, agentlauncher.ClaudeEffortLevels, got)
}

func TestPredictReasoningLevel_ClaudePrefersDynamicEffortChoices(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	helpText := `
Usage: claude [options]
  --model <model> Model for the current session. Provide an alias for the latest model (e.g. 'sonnet' or 'opus') or a model's full name (e.g. 'claude-sonnet-4-6').
  --effort <level> Effort level for the current session (low, medium, high)
`
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
		"PATH":            stubClaudeOnPath(t, helpText),
	}

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	predict := cli.PredictReasoningLevel(ctx, parser)
	got := predict(complete.Args{All: []string{"vibe", "--agent=claude", "--reasoning-level"}})

	require.Equal(t, []string{"low", "medium", "high"}, got)
}

func TestPredictRepoAliases_EmptyByDefault(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)

	got := cli.PredictRepoAliases(internal.Remuda{}).Predict(complete.Args{})
	require.Empty(t, got)
}

func TestPredictRepoAliases_ReturnsConfiguredAliases(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"zz":    "https://github.com/acme/zz.git",
		"alpha": "https://github.com/acme/alpha.git",
	})

	got := cli.PredictRepoAliases(internal.Remuda{}).Predict(complete.Args{})
	require.Equal(t, []string{"alpha", "zz"}, got)
}

func TestPredictPromptNames_ReturnsAllPrompts(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_USE_PROMPTS": "make-pr",
		"REMUDA_CONFIG":      "",
		"XDG_CONFIG_HOME":    "",
	}

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictPromptNames(ctx).Predict(complete.Args{All: []string{"vibe", "--use"}})
	require.ElementsMatch(t, builtinPromptNames, got)
}

func TestPredictNoUsePromptNames_UsesConfigDefaultsAndUseFlags(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  use_prompts:
    - small-commits
    - make-pr
`), 0o644))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictNoUsePromptNames(ctx).Predict(complete.Args{
		All: []string{"vibe", "--use", "minimal-change", "--no-use"},
	})
	require.ElementsMatch(t, []string{"small-commits", "make-pr", "minimal-change"}, got)
}

func TestPredictNoUsePromptNames_ExplicitUseReplacesEnvDefaults(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_USE_PROMPTS": "prototype",
		"REMUDA_CONFIG":      "",
		"XDG_CONFIG_HOME":    "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  use_prompts:
    - make-pr
`), 0o644))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictNoUsePromptNames(ctx).Predict(complete.Args{
		All: []string{"vibe", "--use", "make-pr", "--no-use"},
	})
	require.ElementsMatch(t, []string{"make-pr"}, got)
}

func TestPredictNoUsePromptNames_ExcludesAlreadyTypedNoUse(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_USE_PROMPTS": "make-pr,small-commits",
		"REMUDA_CONFIG":      "",
		"XDG_CONFIG_HOME":    "",
	}

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictNoUsePromptNames(ctx).Predict(complete.Args{
		All: []string{"vibe", "--no-use", "make-pr", "--no-use"},
	})
	require.ElementsMatch(t, []string{"small-commits"}, got)
}

func TestPredictNoUsePromptNames_NoDefaultsAndNoUseFlagsReturnsEmpty(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictNoUsePromptNames(ctx).Predict(complete.Args{
		All: []string{"vibe", "--no-use"},
	})
	require.Empty(t, got)
}

func TestRemudaPredictors_WorkspaceDir_PredictsDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "workspace"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644))

	ctx := newTestContext(t, nil)

	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	predictors := cli.RemudaPredictors(ctx, parser)
	predictor := predictors["workspace-dir"]
	require.NotNil(t, predictor)

	got := predictor.Predict(complete.Args{Last: root})
	require.Contains(t, got, filepath.Join(root, "workspace")+"/")
	require.NotContains(t, got, "file.txt")
}

func TestPredictWorkspaceDir_ExpandsTilde(t *testing.T) {
	t.Parallel()
	home := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(home, "workspace"), 0o755))

	got := cli.PredictWorkspaceDir(home).Predict(complete.Args{Last: "~/wor"})
	require.Contains(t, got, "~/workspace/")
}

func TestPredictProfileNames_UsesPreloadedConfig(t *testing.T) {
	t.Parallel()
	cfg, err := configfile.ParseV1([]byte(`
version: 1
profiles:
  fast:
    agent: codex
  team/slow:
    agent: opencode
`))
	require.NoError(t, err)

	ctx := newTestContext(t, nil, func(ctx *cli.Context) {
		ctx.ConfigFile = cfg
	})

	got := cli.PredictProfileNames(ctx).Predict(complete.Args{})
	require.Equal(t, []string{"fast", "team/slow"}, got)
}

func TestPredictProfileNames_LoadsConfigFromDisk(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
profiles:
  fast:
    agent: codex
  team/slow:
    agent: opencode
`), 0o644))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictProfileNames(ctx).Predict(complete.Args{})
	require.Equal(t, []string{"fast", "team/slow"}, got)
}

func TestPredictProfileNames_NoConfigReturnsEmpty(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	got := cli.PredictProfileNames(ctx).Predict(complete.Args{})
	require.Empty(t, got)
}

func stubClaudeOnPath(t *testing.T, helpText string) string {
	t.Helper()

	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, "claude")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--help\" ]; then\n" +
		"  /bin/cat <<'EOF'\n" +
		strings.TrimSpace(helpText) + "\n" +
		"EOF\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"--version\" ]; then\n" +
		"  echo \"2.1.66 (Claude Code)\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"unsupported args: $*\" >&2\n" +
		"exit 1\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))
	return binDir
}
