package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, content, 0o644))
}

func writeConfigFile(t *testing.T, h *testutils.Harness, content string) string {
	t.Helper()
	xdgHome := strings.TrimSpace(h.Getenv("XDG_CONFIG_HOME"))
	if xdgHome != "" {
		configPath := filepath.Join(xdgHome, "remuda", "config.yaml")
		writeFile(t, configPath, []byte(content))
		return configPath
	}
	home := h.HomeDir
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		require.NoError(t, err)
		require.NotEmpty(t, home)
	}
	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	writeFile(t, configPath, []byte(content))
	return configPath
}

// =============================================================================
// Config Precedence Tests (using use_prompts as observable field)
// =============================================================================

// Note: Testing config precedence for 'agent' field through e2e is complex because
// --agent-cmd overrides agent resolution. Instead, we test precedence using use_prompts
// which is directly observable in the output, and rely on config_resolver_test.go for
// unit-level precedence testing of all fields.

func TestConfigPrecedence_ConfigProvidesDefaultPrompts(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	// use_prompts from config should be applied
	writeConfigFile(t, h, `
version: 1
defaults:
  use_prompts:
    - small-commits
`)
	remoteURL := testutils.InitTestRemote(t)

	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify prompt from config was applied (small-commits content)
	outStr := res.Stdout
	require.Contains(t, outStr, "Please work in small, verifiable steps", "use_prompts from config should be applied")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfigPrecedence_EnvOverridesConfigPrompts(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	// Create config with use_prompts, then override via env with a different prompt
	writeConfigFile(t, h, `
version: 1
defaults:
  use_prompts:
    - small-commits
`)
	// Set env var to a different prompt to override config
	// "make-pr" has distinct content from "small-commits"
	h.SetEnv("REMUDA_USE_PROMPTS", "make-pr")

	remoteURL := testutils.InitTestRemote(t)

	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify make-pr prompt was used (from env), not small-commits (from config)
	outStr := res.Stdout
	require.Contains(t, outStr, "open a GitHub Pull Request", "env use_prompts should override config")
	require.NotContains(t, outStr, "Please work in small, verifiable steps", "config use_prompts should be overridden")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfigPrecedence_ProfileFlagOverridesDefaults(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	writeConfigFile(t, h, `
version: 1
defaults:
  use_prompts:
    - small-commits
profiles:
  fast:
    use_prompts:
      - make-pr
`)
	remoteURL := testutils.InitTestRemote(t)

	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--profile", "fast",
		"implement caching",
	)

	outStr := res.Stdout
	require.Contains(t, outStr, "open a GitHub Pull Request", "profile should override defaults")
	require.NotContains(t, outStr, "Please work in small, verifiable steps", "defaults should be overridden by profile")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfigPrecedence_ProfileEnvOverridesDefaults(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	writeConfigFile(t, h, `
version: 1
defaults:
  use_prompts:
    - small-commits
profiles:
  fast:
    use_prompts:
      - make-pr
`)
	h.SetEnv("REMUDA_PROFILE", "fast")
	remoteURL := testutils.InitTestRemote(t)

	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	outStr := res.Stdout
	require.Contains(t, outStr, "open a GitHub Pull Request", "profile env should override defaults")
	require.NotContains(t, outStr, "Please work in small, verifiable steps", "defaults should be overridden by profile")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfigPrecedence_UseFlagAddsProfileDefaults(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	writeConfigFile(t, h, `
version: 1
profiles:
  fast:
    use_prompts:
      - small-commits
`)
	customPromptDir := filepath.Join(h.HomeDir, "prompts")
	writeFile(t, filepath.Join(customPromptDir, "custom-flag-prompt"), []byte("CUSTOM_FLAG_PROMPT"))
	h.SetEnv("REMUDA_PROMPTS_DIR", customPromptDir)

	remoteURL := testutils.InitTestRemote(t)

	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--profile", "fast",
		"--use", "custom-flag-prompt",
		"implement caching",
	)

	outStr := res.Stdout
	require.Contains(t, outStr, "Please work in small, verifiable steps", "profile defaults should be applied with --use")
	require.Contains(t, outStr, "CUSTOM_FLAG_PROMPT", "--use should add to profile defaults")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfigReposBaseDir_ConfigUsedWhenEnvUnset(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t, testutils.WithRemudaConfigFromEnv())

	baseDir := t.TempDir()
	writeConfigFile(t, h, fmt.Sprintf(`
version: 1
repos:
  base_dir: %q
`, baseDir))

	remoteURL := testutils.InitTestRemote(t)
	res := h.RunOK(
		"clone",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-clone-hooks",
	)

	clonedPath := strings.TrimSpace(res.Stdout)
	require.True(t, strings.HasPrefix(clonedPath, filepath.Clean(baseDir)+string(os.PathSeparator)))
}

func TestConfigPrecedence_ProfileOverridesPerRepo(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	writeConfigFile(t, h, `
version: 1
per_repo:
  acme/rocket:
    defaults:
      use_prompts:
        - small-commits
profiles:
  fast:
    use_prompts:
      - make-pr
`)
	workspace := filepath.Join(h.ReposBaseDir, "acme", "rocket", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	resNoProfile := h.RunOK(
		"vibe",
		"--in", workspace,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	outNoProfile := resNoProfile.Stdout
	require.Contains(t, outNoProfile, "Please work in small, verifiable steps", "per_repo defaults should apply without profile")

	resProfile := h.RunOK(
		"vibe",
		"--in", workspace,
		"--profile", "fast",
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	outProfile := resProfile.Stdout
	require.Contains(t, outProfile, "open a GitHub Pull Request", "profile should override per_repo defaults")
	require.NotContains(t, outProfile, "Please work in small, verifiable steps", "per_repo defaults should be overridden by profile")
}

func TestConfigPrecedence_ExplicitProfileOverridesPerRepoSelectedProfile(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	writeConfigFile(t, h, `
version: 1
profiles:
  fast:
    use_prompts:
      - make-pr
  review:
    use_prompts:
      - small-commits
per_repo:
  acme/rocket:
    profile: fast
`)
	workspace := filepath.Join(h.ReposBaseDir, "acme", "rocket", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	resPerRepoProfile := h.RunOK(
		"vibe",
		"--in", workspace,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	outPerRepoProfile := resPerRepoProfile.Stdout
	require.Contains(t, outPerRepoProfile, "open a GitHub Pull Request", "per_repo profile should apply when no explicit profile is passed")
	require.NotContains(t, outPerRepoProfile, "Please work in small, verifiable steps", "per_repo-selected profile should win when explicit profile is absent")

	resExplicitProfile := h.RunOK(
		"vibe",
		"--in", workspace,
		"--profile", "review",
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	outExplicitProfile := resExplicitProfile.Stdout
	require.Contains(t, outExplicitProfile, "Please work in small, verifiable steps", "explicit --profile should override per_repo-selected profile")
	require.NotContains(t, outExplicitProfile, "open a GitHub Pull Request", "explicit --profile should replace per_repo-selected profile")
}

func TestConfigPrecedence_FlagOverridesEnvAndConfigPrompts(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	// Create a custom prompt file to use with --use flag
	writeConfigFile(t, h, `
version: 1
defaults:
  use_prompts:
    - small-commits
`)

	// Create custom prompt dir and file
	customPromptDir := filepath.Join(h.HomeDir, "prompts")
	writeFile(t, filepath.Join(customPromptDir, "custom-flag-prompt"), []byte("This is a custom flag prompt for testing"))
	h.SetEnv("REMUDA_PROMPTS_DIR", customPromptDir)

	// Set env var too
	h.SetEnv("REMUDA_USE_PROMPTS", "small-commits")

	remoteURL := testutils.InitTestRemote(t)

	// Use --use flag to override both env and config
	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"--use", "custom-flag-prompt",
		"implement caching",
	)

	// Verify flag prompt was used, not config or env
	outStr := res.Stdout
	require.Contains(t, outStr, "This is a custom flag prompt for testing", "flag should override env and config")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfigPrecedence_BuiltInDefaultsWhenNoConfig(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	// No config file - should use built-in defaults
	remoteURL := testutils.InitTestRemote(t)
	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify command succeeds with defaults and no extra prompts
	outStr := res.Stdout
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
	// Should not have any built-in prompts unless explicitly requested
	require.NotContains(t, outStr, "Please work in small, verifiable steps", "no prompts should be applied by default")
}

// =============================================================================
// Config Discovery Tests
// =============================================================================

func TestConfigDiscovery_RemudaConfigEnvOverride(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("XDG_CONFIG_HOME", "")

	// Create a custom prompts dir for the custom config
	customPromptsDir := filepath.Join(h.HomeDir, "custom-prompts")
	writeFile(t, filepath.Join(customPromptsDir, "custom-config-prompt"), []byte("CUSTOM_CONFIG_MARKER_TEXT"))
	h.SetEnv("REMUDA_PROMPTS_DIR", customPromptsDir)

	// Create a config at a custom location
	customConfigPath := filepath.Join(h.HomeDir, "custom", "my-config.yaml")
	writeFile(t, customConfigPath, []byte(`
version: 1
defaults:
  use_prompts:
    - custom-config-prompt
`))

	// Also create a config at the default XDG location with different prompt (should be ignored)
	xdgPromptsDir := filepath.Join(h.HomeDir, "xdg-prompts")
	writeFile(t, filepath.Join(xdgPromptsDir, "xdg-prompt"), []byte("XDG_PROMPT_MARKER_TEXT"))

	xdgConfig := filepath.Join(h.HomeDir, ".config", "remuda", "config.yaml")
	writeFile(t, xdgConfig, []byte(`
version: 1
defaults:
  use_prompts:
    - xdg-prompt
`))

	// Set REMUDA_CONFIG to custom path
	h.SetEnv("REMUDA_CONFIG", customConfigPath)

	remoteURL := testutils.InitTestRemote(t)
	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify the custom config was used (custom prompt, not xdg)
	outStr := res.Stdout
	require.Contains(t, outStr, "CUSTOM_CONFIG_MARKER_TEXT", "REMUDA_CONFIG should override XDG path")
	require.NotContains(t, outStr, "XDG_PROMPT_MARKER_TEXT", "XDG config should be ignored when REMUDA_CONFIG is set")
}

func TestConfigDiscovery_XDGConfigHome(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("REMUDA_CONFIG", "")

	// Create prompts directory
	promptsDir := filepath.Join(h.HomeDir, "prompts")
	writeFile(t, filepath.Join(promptsDir, "xdg-test-prompt"), []byte("XDG_CONFIG_HOME_MARKER"))
	writeFile(t, filepath.Join(promptsDir, "legacy-test-prompt"), []byte("LEGACY_CONFIG_MARKER"))
	h.SetEnv("REMUDA_PROMPTS_DIR", promptsDir)

	// Create a custom XDG_CONFIG_HOME location
	xdgHome := filepath.Join(h.HomeDir, "custom-xdg")
	h.SetEnv("XDG_CONFIG_HOME", xdgHome)

	// Create config at XDG location
	xdgConfig := filepath.Join(xdgHome, "remuda", "config.yaml")
	writeFile(t, xdgConfig, []byte(`
version: 1
defaults:
  use_prompts:
    - xdg-test-prompt
`))

	// Also create a legacy config (should be ignored)
	legacyConfig := filepath.Join(h.HomeDir, ".remuda", "config.yaml")
	writeFile(t, legacyConfig, []byte(`
version: 1
defaults:
  use_prompts:
    - legacy-test-prompt
`))

	remoteURL := testutils.InitTestRemote(t)
	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify XDG config was used
	outStr := res.Stdout
	require.Contains(t, outStr, "XDG_CONFIG_HOME_MARKER", "XDG_CONFIG_HOME should be preferred over legacy")
	require.NotContains(t, outStr, "LEGACY_CONFIG_MARKER", "legacy config should be ignored")
}

func TestConfigDiscovery_LegacyPathFallback(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("REMUDA_CONFIG", "")
	h.SetEnv("XDG_CONFIG_HOME", "")

	// Create prompts directory
	promptsDir := filepath.Join(h.HomeDir, "prompts")
	writeFile(t, filepath.Join(promptsDir, "legacy-prompt"), []byte("LEGACY_PATH_FALLBACK_MARKER"))
	h.SetEnv("REMUDA_PROMPTS_DIR", promptsDir)

	// Only create legacy config (no XDG config)
	legacyConfig := filepath.Join(h.HomeDir, ".remuda", "config.yaml")
	writeFile(t, legacyConfig, []byte(`
version: 1
defaults:
  use_prompts:
    - legacy-prompt
`))

	remoteURL := testutils.InitTestRemote(t)
	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify legacy config was used
	outStr := res.Stdout
	require.Contains(t, outStr, "LEGACY_PATH_FALLBACK_MARKER", "legacy path should be used as fallback")
}

func TestConfigDiscovery_MissingConfigIsNotError(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("REMUDA_CONFIG", "")
	h.SetEnv("XDG_CONFIG_HOME", "")

	// No config files at all

	remoteURL := testutils.InitTestRemote(t)

	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Should succeed even without any config file
	require.Contains(t, res.Stdout, "implement caching")
}

func TestConfigDiscovery_InvalidConfigErrors(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("REMUDA_CONFIG", "")
	h.SetEnv("XDG_CONFIG_HOME", "")

	// Create an invalid config file
	configPath := filepath.Join(h.HomeDir, ".config", "remuda", "config.yaml")
	writeFile(t, configPath, []byte(`
version: 999
`))

	remoteURL := testutils.InitTestRemote(t)

	// Should error on invalid config
	res := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "unsupported config version")
}

func TestConfigDiscovery_StrictModeFailsOnMissingFile(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("XDG_CONFIG_HOME", "")

	// Set REMUDA_CONFIG to a non-existent file
	nonExistent := filepath.Join(h.HomeDir, "does-not-exist.yaml")
	h.SetEnv("REMUDA_CONFIG", nonExistent)

	remoteURL := testutils.InitTestRemote(t)

	// Should error when REMUDA_CONFIG points to missing file
	res := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "does not exist")
}

// =============================================================================
// Multiple Config Fields Tests
// =============================================================================

func TestConfig_MultipleFieldsApplied(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	writeConfigFile(t, h, `
version: 1
defaults:
  use_prompts:
    - small-commits
`)
	remoteURL := testutils.InitTestRemote(t)

	// Create a custom prompt to verify model is also set
	promptsDir := filepath.Join(h.HomeDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	h.SetEnv("REMUDA_PROMPTS_DIR", promptsDir)

	// Use echo as agent-cmd to capture the prompt output
	res := h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux", // Run in foreground to capture output
		"--no-container",
		"--agent-cmd", "echo ",
		"implement caching",
	)

	// Verify the prompt includes small-commits content (from use_prompts)
	outStr := res.Stdout
	require.Contains(t, outStr, "Please work in small, verifiable steps", "use_prompts should be applied from config")
	require.Contains(t, outStr, "implement caching", "user prompt should be included")
}

func TestConfig_ContainerSettingsDisabled(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	// Test that container.enabled=false in config is respected
	writeConfigFile(t, h, `
version: 1
defaults:
  container:
    enabled: false
`)
	remoteURL := testutils.InitTestRemote(t)

	// Should succeed because container is disabled in config
	res := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--agent-cmd", "true",
		"prompt",
	)
	require.NoError(t, res.Err, res.String())
}

func TestConfig_ContainerFlagOverridesConfig(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	// Test that --no-container flag overrides container.enabled=true in config
	writeConfigFile(t, h, `
version: 1
defaults:
  container:
    enabled: true
`)
	remoteURL := testutils.InitTestRemote(t)

	// Should succeed because --no-container overrides config
	res := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container", // Should override config
		"--agent-cmd", "true",
		"prompt",
	)
	require.NoError(t, res.Err, res.String())
}

func TestConfig_ValidationRejectsUnknownKeys(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("XDG_CONFIG_HOME", "")

	// Create a config with unknown keys
	configPath := filepath.Join(h.HomeDir, ".config", "remuda", "config.yaml")
	writeFile(t, configPath, []byte(`
version: 1
defaults:
  agent: codex
  unknown_field: "should fail"
`))

	remoteURL := testutils.InitTestRemote(t)

	// Should error on unknown keys
	res := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "unknown_field")
}

func TestConfig_ValidationRejectsInvalidEnumValues(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	h.SetEnv("XDG_CONFIG_HOME", "")

	// Create a config with invalid enum value
	configPath := filepath.Join(h.HomeDir, ".config", "remuda", "config.yaml")
	writeFile(t, configPath, []byte(`
version: 1
defaults:
  agent: invalid_agent_name
`))

	remoteURL := testutils.InitTestRemote(t)

	// Should error on invalid agent name
	res := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "invalid value")
	require.Contains(t, res.Err.Error(), "invalid_agent_name")
}
