package configfile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/enums"
)

func TestParseV1_MissingVersion(t *testing.T) {
	_, err := ParseV1([]byte("defaults:\n  agent: codex\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "version")
}

func TestParseV1_UnsupportedVersion(t *testing.T) {
	_, err := ParseV1([]byte("version: 2\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}

func TestParseV1_InvalidPerRepoKey(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nper_repo:\n  \"not-a-slug\":\n    defaults:\n      agent: codex\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "per_repo")
}

func TestParseV1_ValidProfiles(t *testing.T) {
	yaml := `version: 1
profiles:
  opus:
    agent: codex
  "team/fast_codex":
    model: gpt-5
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	require.Len(t, cfg.Profiles, 2)
	profile, ok := cfg.Profiles["opus"]
	require.True(t, ok)
	require.NotNil(t, profile.Agent)
	require.Equal(t, "codex", *profile.Agent)

	teamProfile, ok := cfg.Profiles["team/fast_codex"]
	require.True(t, ok)
	require.NotNil(t, teamProfile.Model)
	require.Equal(t, "gpt-5", *teamProfile.Model)
}

func TestParseV1_JiraConfigRoundTrip(t *testing.T) {
	yaml := `version: 1
jira:
  endpoint: https://jira.example.atlassian.net
  user: dev@example.com
  api_token: secret-token
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Jira)
	require.NotNil(t, cfg.Jira.Endpoint)
	require.Equal(t, "https://jira.example.atlassian.net", *cfg.Jira.Endpoint)
	require.NotNil(t, cfg.Jira.User)
	require.Equal(t, "dev@example.com", *cfg.Jira.User)
	require.NotNil(t, cfg.Jira.APIToken)
	require.Equal(t, "secret-token", *cfg.Jira.APIToken)
}

func TestParseV1_InvalidJiraEndpoint(t *testing.T) {
	yaml := `version: 1
jira:
  endpoint: jira.example.atlassian.net
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "jira.endpoint")
	require.Contains(t, err.Error(), "scheme and host")
}

func TestParseV1_EmptyJiraTokenIsAllowed(t *testing.T) {
	yaml := `version: 1
jira:
  endpoint: https://jira.example.atlassian.net
  user: dev@example.com
  api_token: ""
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Jira)
	require.NotNil(t, cfg.Jira.APIToken)
	require.Equal(t, "", *cfg.Jira.APIToken)
}

func TestParseV1_InvalidProfileNames(t *testing.T) {
	cases := []string{
		`""`,
		`"bad name"`,
		`"a//b"`,
		`"/a"`,
		`"a/"`,
		`"a@b"`,
	}

	for _, name := range cases {
		yaml := "version: 1\nprofiles:\n  " + name + ":\n    agent: codex\n"
		_, err := ParseV1([]byte(yaml))
		require.Error(t, err, "expected error for profile name %s", name)
		require.Contains(t, err.Error(), "profiles["+name+"]")
		require.Contains(t, err.Error(), "profile name")
	}
}

func TestParseV1_UnknownKeyInProfiles(t *testing.T) {
	yaml := `version: 1
profiles:
  fast:
    agent: codex
    typo_key: value
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "typo_key")
}

func TestParseV1_ProfileDefaultsRoundTrip(t *testing.T) {
	yaml := `version: 1
profiles:
  fast:
    yolo: true
    container:
      inherit_env:
        - FOO
        - BAR
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	profile := cfg.Profiles["fast"]
	require.NotNil(t, profile.Yolo)
	require.True(t, *profile.Yolo)
	require.NotNil(t, profile.Container)
	require.NotNil(t, profile.Container.InheritEnv)
	require.Equal(t, []string{"FOO", "BAR"}, *profile.Container.InheritEnv)
}

func TestParseV1_ProfileContainerBoolShorthand(t *testing.T) {
	yaml := `version: 1
profiles:
  fast:
    container: false
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	profile := cfg.Profiles["fast"]
	require.NotNil(t, profile.Container)
	require.NotNil(t, profile.Container.Enabled)
	require.False(t, *profile.Container.Enabled)
}

func TestParseV1_InvalidAgentInProfile(t *testing.T) {
	yaml := `version: 1
profiles:
  fast:
    agent: bogus
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `profiles["fast"].agent`)
	require.Contains(t, err.Error(), "bogus")
}

func TestParseV1_PerRepoKeyNormalized(t *testing.T) {
	yaml := `version: 1
per_repo:
  "Owner/Repo":
    defaults:
      agent: codex
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	_, ok := cfg.PerRepo["owner/repo"]
	require.True(t, ok)
}

func TestParseV1_PerRepoKeyCaseInsensitiveDuplicates(t *testing.T) {
	yaml := `version: 1
per_repo:
  "Owner/Repo":
    defaults:
      agent: codex
  "owner/repo":
    defaults:
      agent: bash
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "per_repo key")
	require.Contains(t, err.Error(), "duplicates")
}

func TestParseV1_PointerFields(t *testing.T) {
	cfg, err := ParseV1([]byte("version: 1\ndefaults:\n  agent: codex\n"))
	require.NoError(t, err)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "codex", *cfg.Defaults.Agent)
	require.Nil(t, cfg.Defaults.Model)
	require.Nil(t, cfg.Defaults.ReasoningLevel)
	require.Nil(t, cfg.Defaults.SlugifyReasoningLevel)
	require.Nil(t, cfg.Defaults.UsePromptsPosition)
	require.Nil(t, cfg.Defaults.AgentArgs)
	require.Nil(t, cfg.Defaults.Yolo)
	require.Nil(t, cfg.Defaults.Merge)
	require.Nil(t, cfg.Defaults.Container)
}

func TestParseV1_UsePromptsPosition(t *testing.T) {
	cfg, err := ParseV1([]byte("version: 1\ndefaults:\n  use_prompts_position: after\n"))
	require.NoError(t, err)
	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.UsePromptsPosition)
	require.Equal(t, "after", *cfg.Defaults.UsePromptsPosition)
}

func TestParseV1_InvalidUsePromptsPosition(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\ndefaults:\n  use_prompts_position: beside\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.use_prompts_position")
	require.Contains(t, err.Error(), "beside")
}

func TestParseV1_DefaultsAgentArgsRoundTrip(t *testing.T) {
	yaml := `version: 1
defaults:
  agent_args:
    codex:
      - --foo
      - --bar
`

	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Defaults)
	require.Equal(t, []string{"--foo", "--bar"}, cfg.Defaults.AgentArgs["codex"])
}

func TestParseV1_DefaultsAgentArgsRejectsUnknownAgentKey(t *testing.T) {
	yaml := `version: 1
defaults:
  agent_args:
    fake-agent:
      - --foo
`

	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.agent_args[\"fake-agent\"]")
	require.Contains(t, err.Error(), "invalid value")
}

func TestParseV1_DefaultsAgentArgsRejectsEmptyArg(t *testing.T) {
	yaml := `version: 1
defaults:
  agent_args:
    codex:
      - --foo
      - " "
`

	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.agent_args[\"codex\"][1]")
	require.Contains(t, err.Error(), "agent arg cannot be empty")
}

func TestParseV1_DefaultsMergeGHFlagsRoundTrip(t *testing.T) {
	yaml := `version: 1
defaults:
  merge:
    gh_flags:
      - --squash
      - --delete-branch
`

	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Merge)
	require.NotNil(t, cfg.Defaults.Merge.GHFlags)
	require.Equal(t, []string{"--squash", "--delete-branch"}, *cfg.Defaults.Merge.GHFlags)
}

func TestParseV1_DefaultsMergeGHFlagsRejectsEmptyFlags(t *testing.T) {
	yaml := `version: 1
defaults:
  merge:
    gh_flags:
      - --squash
      - " "
`

	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.merge.gh_flags[1]")
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestParseV1_UnknownTopLevelKey(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nunknown_key: foo\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown_key")
}

func TestParseV1_UnknownNestedKey(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\ndefaults:\n  agent: codex\n  bad_field: oops\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad_field")
}

func TestParseV1_UnknownContainerKey(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\ndefaults:\n  container:\n    bogus: true\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "container.bogus")
}

func TestParseV1_UnknownKeyInPerRepo(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    defaults:
      agent: codex
      typo_key: value
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "typo_key")
}

func TestParseV1_InvalidAgent(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\ndefaults:\n  agent: invalid_agent\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.agent")
	require.Contains(t, err.Error(), "invalid_agent")
}

func TestParseV1_InvalidSlugifyReasoningLevel(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\ndefaults:\n  slugify_reasoning_level: turbo\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.slugify_reasoning_level")
	require.Contains(t, err.Error(), "turbo")
	require.Contains(t, err.Error(), strings.Join(enums.ValidSlugifyReasoningLevels, ", "))
}

func TestParseV1_InvalidSessionManager(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nsession:\n  manager: screen\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "session.manager")
	require.Contains(t, err.Error(), "screen")
}

func TestParseV1_ValidSessionPruneIgnorePatterns(t *testing.T) {
	yaml := `version: 1
session:
  prune:
    ignore:
      - "org/repo/*"
      - "*/repo/*"
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Session)
	require.NotNil(t, cfg.Session.Prune)
	require.NotNil(t, cfg.Session.Prune.Ignore)
	require.Equal(t, []string{"org/repo/*", "*/repo/*"}, *cfg.Session.Prune.Ignore)
}

func TestParseV1_InvalidSessionPruneIgnorePattern(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nsession:\n  prune:\n    ignore:\n      - \"[\"\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "session.prune.ignore[0]")
	require.Contains(t, err.Error(), "invalid pattern")
}

func TestParseV1_EmptySessionPruneIgnorePattern(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nsession:\n  prune:\n    ignore:\n      - \" \"\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "session.prune.ignore[0]")
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestParseV1_ValidWorkspacesIgnorePatterns(t *testing.T) {
	yaml := `version: 1
workspaces:
  ignore:
    - "org/repo/*"
    - "*/repo/*"
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Workspaces)
	require.NotNil(t, cfg.Workspaces.Ignore)
	require.Equal(t, []string{"org/repo/*", "*/repo/*"}, *cfg.Workspaces.Ignore)
}

func TestParseV1_InvalidWorkspacesIgnorePattern(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nworkspaces:\n  ignore:\n    - \"[\"\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspaces.ignore[0]")
	require.Contains(t, err.Error(), "invalid pattern")
}

func TestParseV1_EmptyWorkspacesIgnorePattern(t *testing.T) {
	_, err := ParseV1([]byte("version: 1\nworkspaces:\n  ignore:\n    - \" \"\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspaces.ignore[0]")
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestParseV1_SessionPruneIgnoreNotAllowedInPerRepo(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    session:
      prune:
        ignore:
          - "owner/repo/*"
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].session.prune`)
	require.Contains(t, err.Error(), "global-only")
}

func TestParseV1_InvalidAgentInPerRepo(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    defaults:
      agent: fake_agent
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].defaults.agent`)
	require.Contains(t, err.Error(), "fake_agent")
}

func TestParseV1_InvalidSessionManagerInPerRepo(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    session:
      manager: byobu
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].session.manager`)
	require.Contains(t, err.Error(), "byobu")
}

func TestParseV1_PerRepoProfileRoundTrip(t *testing.T) {
	yaml := `version: 1
profiles:
  fast:
    agent: codex
per_repo:
  "owner/repo":
    profile: fast
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	overlay, ok := cfg.PerRepo["owner/repo"]
	require.True(t, ok)
	require.NotNil(t, overlay.Profile)
	require.Equal(t, "fast", *overlay.Profile)
}

func TestParseV1_InvalidPerRepoProfileName(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    profile: "bad name"
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].profile`)
	require.Contains(t, err.Error(), "profile name")
}

func TestParseV1_InvalidReposAliasesInPerRepo(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    repos:
      aliases:
        evil: "--upload-pack=evil"
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].repos.aliases["evil"]`)
	require.Contains(t, err.Error(), "cannot start with '-'")
}

func TestParseV1_PerRepoCloneHooksRoundTrip(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    clone_hooks:
      - name: bootstrap
        argv: ["./scripts/bootstrap", "--fast"]
      - argv: ["bash", "-lc", "echo hi"]
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	overlay, ok := cfg.PerRepo["owner/repo"]
	require.True(t, ok)
	require.Len(t, overlay.CloneHooks, 2)
	require.Equal(t, "bootstrap", overlay.CloneHooks[0].Name)
	require.Equal(t, []string{"./scripts/bootstrap", "--fast"}, overlay.CloneHooks[0].Argv)
	require.Equal(t, []string{"bash", "-lc", "echo hi"}, overlay.CloneHooks[1].Argv)
}

func TestParseV1_PerRepoCloneHooksRejectsEmptyArgv(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    clone_hooks:
      - name: invalid
        argv: []
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].clone_hooks[0].argv`)
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestParseV1_PerRepoCloneHooksRejectsBlankExecutable(t *testing.T) {
	yaml := `version: 1
per_repo:
  "owner/repo":
    clone_hooks:
      - name: invalid
        argv: ["   ", "--fast"]
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].clone_hooks[0].argv[0]`)
	require.Contains(t, err.Error(), "executable cannot be empty")
}

func TestParseV1_ValidAgents(t *testing.T) {
	for _, agent := range enums.ValidAgents {
		yaml := "version: 1\ndefaults:\n  agent: " + agent + "\n"
		cfg, err := ParseV1([]byte(yaml))
		require.NoError(t, err, "agent %q should be valid", agent)
		require.NotNil(t, cfg.Defaults.Agent)
		require.Equal(t, agent, *cfg.Defaults.Agent)
	}
}

func TestParseV1_ClaudeAcceptedInDefaultsProfilesAndPerRepo(t *testing.T) {
	yaml := `version: 1
defaults:
  agent: claude
profiles:
  fast:
    agent: claude
per_repo:
  "owner/repo":
    defaults:
      agent: claude
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "claude", *cfg.Defaults.Agent)

	profile := cfg.Profiles["fast"]
	require.NotNil(t, profile.Agent)
	require.Equal(t, "claude", *profile.Agent)

	overlay, ok := cfg.PerRepo["owner/repo"]
	require.True(t, ok)
	require.NotNil(t, overlay.Defaults)
	require.NotNil(t, overlay.Defaults.Agent)
	require.Equal(t, "claude", *overlay.Defaults.Agent)
}

func TestParseV1_ValidSessionManagers(t *testing.T) {
	for _, manager := range enums.ValidSessionManagers {
		yaml := "version: 1\nsession:\n  manager: " + manager + "\n"
		cfg, err := ParseV1([]byte(yaml))
		require.NoError(t, err, "session manager %q should be valid", manager)
		require.NotNil(t, cfg.Session.Manager)
		require.Equal(t, manager, *cfg.Session.Manager)
	}
}

func TestParseV1_InvalidAliasURL_DashPrefix(t *testing.T) {
	yaml := `version: 1
repos:
  aliases:
    evil: "--upload-pack=evil"
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `repos.aliases["evil"]`)
	require.Contains(t, err.Error(), "cannot start with '-'")
}

func TestParseV1_InvalidAliasURL_Empty(t *testing.T) {
	yaml := `version: 1
repos:
  aliases:
    emptyval: ""
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), `repos.aliases["emptyval"]`)
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestParseV1_ValidAliases(t *testing.T) {
	yaml := `version: 1
repos:
  aliases:
    myrepo: https://github.com/acme/myrepo.git
    sshrepo: git@github.com:acme/sshrepo.git
`
	cfg, err := ParseV1([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Repos)
	require.Len(t, cfg.Repos.Aliases, 2)
	require.Equal(t, "https://github.com/acme/myrepo.git", cfg.Repos.Aliases["myrepo"])
	require.Equal(t, "git@github.com:acme/sshrepo.git", cfg.Repos.Aliases["sshrepo"])
}

func TestParseV1_InvalidExperimentName_Empty(t *testing.T) {
	yaml := `version: 1
defaults:
  experiments:
    - ""
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.experiments[0]")
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestParseV1_InvalidContainerInheritEnvName(t *testing.T) {
	yaml := `version: 1
defaults:
  container:
    inherit_env:
      - "BAD=NOPE"
`
	_, err := ParseV1([]byte(yaml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "defaults.container.inherit_env[0]")
	require.Contains(t, err.Error(), "invalid env var name")
}
