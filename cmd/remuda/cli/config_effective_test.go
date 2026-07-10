package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEffectiveConfig_PerRepoOverridesBase(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  agent: codex
  model: base-model
per_repo:
  acme/utils:
    defaults:
      model: repo-model
`)

	eff, err := newEffectiveConfig(cfg, "acme/utils", profileRef{})
	require.NoError(t, err)
	require.Equal(t, "codex", eff.String("defaults.agent"))
	require.Equal(t, "repo-model", eff.String("defaults.model"))
}

func TestNewEffectiveConfig_PerRepoContainerOptsAppendToBase(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  container:
    opts: ["--base"]
per_repo:
  acme/utils:
    defaults:
      container:
        opts: ["--repo"]
`)

	eff, err := newEffectiveConfig(cfg, "acme/utils", profileRef{})
	require.NoError(t, err)
	require.Equal(t, []string{"--base", "--repo"}, eff.Strings("defaults.container.opts"))
}

func TestNewEffectiveConfig_PerRepoContainerOptsReplaceWhenBaseUnset(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
per_repo:
  acme/utils:
    defaults:
      container:
        opts: ["--repo"]
`)

	eff, err := newEffectiveConfig(cfg, "acme/utils", profileRef{})
	require.NoError(t, err)
	require.Equal(t, []string{"--repo"}, eff.Strings("defaults.container.opts"))
}

func TestNewEffectiveConfig_ProfileOverridesPerRepo(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  agent: codex
profiles:
  fast:
    agent: claude
    no_use: []
per_repo:
  acme/utils:
    defaults:
      agent: opencode
      no_use: [make-pr]
`)

	eff, err := newEffectiveConfig(cfg, "acme/utils", profileRef{Name: "fast"})
	require.NoError(t, err)
	require.Equal(t, "claude", eff.String("defaults.agent"))
	require.Empty(t, eff.Strings("defaults.no_use"))
}

func TestNewEffectiveConfig_PerRepoAgentArgsReplacePerAgent(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  agent_args:
    codex: ["--global"]
    claude: ["--keep"]
per_repo:
  acme/utils:
    defaults:
      agent_args:
        codex: ["--repo"]
`)

	eff, err := newEffectiveConfig(cfg, "acme/utils", profileRef{})
	require.NoError(t, err)
	require.Equal(t, []string{"--repo"}, effectiveAgentArgsFromKoanf(eff, "codex", nil))
	require.Equal(t, []string{"--keep"}, effectiveAgentArgsFromKoanf(eff, "claude", nil))
	require.Equal(t, []string{"--repo", "--cli"}, effectiveAgentArgsFromKoanf(eff, "codex", []string{"--cli"}))
}

func TestNewEffectiveConfig_UnknownProfileErrors(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, "version: 1\n")

	_, err := newEffectiveConfig(cfg, "", profileRef{Name: "missing"})
	require.ErrorContains(t, err, `unknown profile "missing"`)

	_, err = newEffectiveConfig(cfg, "", profileRef{Name: "missing", PerRepoSlug: "acme/utils"})
	require.ErrorContains(t, err, `per_repo["acme/utils"].profile references unknown profile "missing"`)
}

func TestNewEffectiveConfig_ProfileWithoutConfigErrors(t *testing.T) {
	t.Parallel()
	_, err := newEffectiveConfig(nil, "", profileRef{Name: "fast"})
	require.ErrorContains(t, err, `profile "fast" requested but no config file found`)
}

func TestSelectProfile_Precedence(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
profiles:
  fast: {}
  review: {}
per_repo:
  acme/utils:
    profile: review
`)

	// Explicit flag wins.
	got := selectProfile("fast", true, EnvMap{"REMUDA_PROFILE": "review"}, cfg, "acme/utils")
	require.Equal(t, profileRef{Name: "fast"}, got)

	// Env beats per_repo.
	got = selectProfile("", false, EnvMap{"REMUDA_PROFILE": "fast"}, cfg, "acme/utils")
	require.Equal(t, profileRef{Name: "fast"}, got)

	// per_repo profile when nothing else is set.
	got = selectProfile("", false, EnvMap{}, cfg, "acme/utils")
	require.Equal(t, profileRef{Name: "review", PerRepoSlug: "acme/utils"}, got)

	// Nothing selected.
	got = selectProfile("", false, EnvMap{}, cfg, "other/repo")
	require.Equal(t, profileRef{}, got)
}
