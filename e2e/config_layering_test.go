// Package e2e_test covers config resolution through the full stack: base
// config, per_repo overlay, profile, and flags/env, composing into a single
// docker/agent command. See config_effective.go (newEffectiveConfig) and
// flags.go (flagResolution.apply) for the resolution logic under test.
package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/session"
)

// containerLayeringConfig sets defaults.container.opts at the base layer and
// again at the per_repo layer for yendo-eng/remuda, plus a "review" profile
// that sets its own container.opts.
const containerLayeringConfig = `
version: 1
defaults:
  container:
    image: ghcr.io/acme/vibe-dev:latest
    opts:
      - "--base-opt"
per_repo:
  yendo-eng/remuda:
    defaults:
      container:
        opts:
          - "--per-repo-opt"
profiles:
  review:
    container:
      opts:
        - "--profile-opt"
`

func newContainerLayeringHarness(t *testing.T) (*testutils.Harness, *testutils.MockSessionManager, string) {
	t.Helper()
	runDir := t.TempDir()
	workspace := filepath.Join(runDir, "yendo-eng", "remuda", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(containerLayeringConfig), 0o644))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_CONFIG", configPath)
	h.SetEnv("GH_TOKEN", "test-token")
	h.SetEnv("SSH_AUTH_SOCK", "")

	return h, sessionMgr, workspace
}

// TestConfigLayeringContainerOptsAppendAcrossBaseAndPerRepo is a regression
// test for the bug fixed in b1069b5: per_repo container.opts must append to
// the base opts as distinct docker args, not get stringified into one
// "[a b]" token (which docker rejects as an invalid reference).
func TestConfigLayeringContainerOptsAppendAcrossBaseAndPerRepo(t *testing.T) {
	t.Parallel()
	h, sessionMgr, workspace := newContainerLayeringHarness(t)

	h.RunOK(
		"vibe",
		"--in", workspace,
		"--container",
		"prompt",
	)

	recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)
	require.Contains(t, recorded.CommandRan, "--base-opt")
	require.Contains(t, recorded.CommandRan, "--per-repo-opt")
	require.NotContains(t, recorded.CommandRan, "[--base-opt")
	require.NotContains(t, recorded.CommandRan, "--per-repo-opt]")
	require.Less(t,
		strings.Index(recorded.CommandRan, "--base-opt"),
		strings.Index(recorded.CommandRan, "--per-repo-opt"),
		"base opts should precede appended per_repo opts",
	)
}

// TestConfigLayeringProfileReplacesAppendedContainerOpts shows that a
// selected profile's container.opts REPLACES the base+per_repo combination
// entirely, unlike per_repo which appends onto base.
func TestConfigLayeringProfileReplacesAppendedContainerOpts(t *testing.T) {
	t.Parallel()
	h, sessionMgr, workspace := newContainerLayeringHarness(t)

	h.RunOK(
		"vibe",
		"--in", workspace,
		"--container",
		"--profile", "review",
		"prompt",
	)

	recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)
	require.Contains(t, recorded.CommandRan, "--profile-opt")
	require.NotContains(t, recorded.CommandRan, "--base-opt")
	require.NotContains(t, recorded.CommandRan, "--per-repo-opt")
}

// TestConfigLayeringUseFlagEnvVsProfileMerge covers the --use mergeConfigSlice
// path (flags.go flagResolution.applyOne): when --use is explicit and
// REMUDA_USE_PROMPTS is also set, the env value wins outright and the
// profile's use_prompts never merges in. When the env var is absent, the
// profile's use_prompts is merged in ahead of the explicit --use value.
func TestConfigLayeringUseFlagEnvVsProfileMerge(t *testing.T) {
	t.Parallel()

	const config = `
version: 1
profiles:
  ship:
    use_prompts:
      - "make-pr"
`

	newHarness := func(t *testing.T) (*testutils.Harness, string) {
		t.Helper()
		remoteURL := testutils.InitTestRemote(t)
		runDir := t.TempDir()
		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(config), 0o644))

		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
			testutils.WithDocker(&docker.Mock{Running: true}),
		)
		h.SetEnv("REMUDA_CONFIG", configPath)
		return h, remoteURL
	}

	t.Run("env present means no merge with profile", func(t *testing.T) {
		t.Parallel()
		h, remoteURL := newHarness(t)
		h.SetEnv("REMUDA_USE_PROMPTS", "make-pr")

		res := h.RunOK(
			"vibe",
			"--name", "wk",
			"--repo-url", remoteURL,
			"--no-tmux",
			"--no-container",
			"--profile", "ship",
			"--agent-cmd", "echo ",
			"--use", "small-commits",
			"implement caching",
		)

		outStr := res.Stdout
		require.Contains(t, outStr, "Please work in small, verifiable steps. Use git to manage your changes.")
		require.NotContains(t, outStr, "gh pr create")
	})

	t.Run("env absent merges profile use_prompts ahead of explicit --use", func(t *testing.T) {
		t.Parallel()
		h, remoteURL := newHarness(t)

		res := h.RunOK(
			"vibe",
			"--name", "wk",
			"--repo-url", remoteURL,
			"--no-tmux",
			"--no-container",
			"--profile", "ship",
			"--agent-cmd", "echo ",
			"--use", "small-commits",
			"implement caching",
		)

		outStr := res.Stdout
		require.Contains(t, outStr, "Please work in small, verifiable steps. Use git to manage your changes.")
		require.Contains(t, outStr, "gh pr create")
		require.Less(t,
			strings.Index(outStr, "gh pr create"),
			strings.Index(outStr, "Please work in small, verifiable steps."),
			"profile's make-pr should be merged in ahead of the explicit --use value",
		)
	})
}

// TestConfigLayeringSessionResumeAgentCoercion covers resolveSessionResumeAgent:
// a per_repo defaults.agent flowing through the effective config silently
// coerces to codex when --agent is omitted (only claude is resumed as-is),
// while an explicit --agent value that isn't resume-supported still errors,
// even though it came through the same layered config.
func TestConfigLayeringSessionResumeAgentCoercion(t *testing.T) {
	t.Parallel()

	const config = `
version: 1
per_repo:
  yendo-eng/remuda:
    defaults:
      agent: opencode
`

	newHarness := func(t *testing.T) (*testutils.Harness, *testutils.MockSessionManager, string) {
		t.Helper()
		runDir := t.TempDir()
		workspace := filepath.Join(runDir, "yendo-eng", "remuda", "wk")
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(config), 0o644))

		sessionMgr := &testutils.MockSessionManager{}
		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
			testutils.WithSessionManager(sessionMgr),
			testutils.WithDocker(&docker.Mock{Running: false}),
		)
		h.SetEnv("REMUDA_CONFIG", configPath)
		h.SetEnv("REMUDA_CONTAINER", "false")
		return h, sessionMgr, workspace
	}

	t.Run("no --agent silently coerces configured opencode to codex", func(t *testing.T) {
		t.Parallel()
		h, sessionMgr, workspace := newHarness(t)

		h.RunOK("session", "resume", workspace)

		recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
		require.NotNil(t, recorded)
		require.Contains(t, recorded.CommandRan, "codex resume --last")
	})

	t.Run("explicit --agent opencode errors despite configured agent", func(t *testing.T) {
		t.Parallel()
		h, _, workspace := newHarness(t)

		res := h.Run("session", "resume", "--agent", "opencode", workspace)
		require.ErrorContains(t, res.Err, `session resume unsupported for agent "opencode"`)
	})
}

// TestConfigLayeringOpenAIAPIKeyExplicitness covers the FlagExplicit("openai-api-key")
// check in vibe.go: the key travels into the session start environment (never
// the command string) when supplied via flag or via OPENAI_API_KEY env, and
// vibe.go injects nothing (the host's key, if any, passes through untouched)
// when neither is set.
func TestConfigLayeringOpenAIAPIKeyExplicitness(t *testing.T) {
	t.Parallel()

	newHarness := func(t *testing.T) (*testutils.Harness, *testutils.MockSessionManager, string) {
		t.Helper()
		workspaceRoot := t.TempDir()
		workspace := filepath.Join(workspaceRoot, "org", "repo", "wk")
		require.NoError(t, os.MkdirAll(workspace, 0o755))

		sessionMgr := &testutils.MockSessionManager{}
		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
			testutils.WithSessionManager(sessionMgr),
			testutils.WithDocker(&docker.Mock{Running: true}),
		)
		h.SetEnv("REMUDA_MODEL", "")
		h.SetEnv("OPENAI_API_KEY", "")
		h.SetEnv("REMUDA_OPENAI_API_KEY", "")
		return h, sessionMgr, workspace
	}

	t.Run("explicit flag lands in start env", func(t *testing.T) {
		t.Parallel()
		h, sessionMgr, workspace := newHarness(t)

		h.RunOK("vibe", "--in", workspace, "--no-container", "--agent-cmd", "true", "--openai-api-key", "flag-secret", "prompt")

		recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
		require.NotNil(t, recorded)
		require.NotContains(t, recorded.CommandRan, "flag-secret")
		value, ok := sessionEnvValue(recorded.StartEnv, "OPENAI_API_KEY")
		require.True(t, ok)
		require.Equal(t, "flag-secret", value)
	})

	t.Run("env var lands in start env", func(t *testing.T) {
		t.Parallel()
		h, sessionMgr, workspace := newHarness(t)
		h.SetEnv("OPENAI_API_KEY", "env-secret")

		h.RunOK("vibe", "--in", workspace, "--no-container", "--agent-cmd", "true", "prompt")

		recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
		require.NotNil(t, recorded)
		require.NotContains(t, recorded.CommandRan, "env-secret")
		value, ok := sessionEnvValue(recorded.StartEnv, "OPENAI_API_KEY")
		require.True(t, ok)
		require.Equal(t, "env-secret", value)
	})

	t.Run("empty when neither flag nor env is set", func(t *testing.T) {
		t.Parallel()
		h, sessionMgr, workspace := newHarness(t)

		h.RunOK("vibe", "--in", workspace, "--no-container", "--agent-cmd", "true", "prompt")

		recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
		require.NotNil(t, recorded)
		// EnvOverrides is nil (unset) in this case, so whatever the host's
		// OPENAI_API_KEY happened to be (cleared above) passes through
		// untouched rather than vibe.go injecting a value.
		value, _ := sessionEnvValue(recorded.StartEnv, "OPENAI_API_KEY")
		require.Empty(t, value)
	})
}
