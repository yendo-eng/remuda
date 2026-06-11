package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestConfigResolver_Precedence(t *testing.T) {
	t.Parallel()
	t.Run("config experiments set default experiments flag", func(t *testing.T) {
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
  experiments:
    - my-experiment
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, opts)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		// Experiments should be sourced from config when env vars are unset.
		_, err = parser.Parse([]string{"vibe", "--agent-cmd", "true", "prompt"})
		require.NoError(t, err)
		require.True(t, c.Vibe.ExperimentEnabled("my-experiment"))
	})

	t.Run("config provides defaults when no env or flags", func(t *testing.T) {
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
  agent: bash
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, opts)

		// Parse with config resolver and verify agent is set from config
		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "prompt"})
		require.NoError(t, err)
		require.Equal(t, "bash", c.Vibe.Agent)
	})

	t.Run("env overrides config", func(t *testing.T) {
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
  agent: bash
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, opts)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "prompt"})
		require.NoError(t, err)
		require.Equal(t, "opencode", c.Vibe.Agent, "env should override config")
	})

	t.Run("flag overrides env and config", func(t *testing.T) {
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
  agent: bash
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		// Flag should override both env and config
		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "--agent", "codex", "prompt"})
		require.NoError(t, err)
		require.Equal(t, "codex", c.Vibe.Agent, "flag should override env and config")
	})

	t.Run("jira auth follows flag over env over config precedence", func(t *testing.T) {
		home := t.TempDir()
		env := cli.EnvMap{
			"REMUDA_CONFIG":         "",
			"XDG_CONFIG_HOME":       "",
			"REMUDA_JIRA_ENDPOINT":  "https://env-jira.example.atlassian.net",
			"REMUDA_JIRA_USER":      "env-user@example.com",
			"REMUDA_JIRA_API_TOKEN": "env-token",
		}

		configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
jira:
  endpoint: https://config-jira.example.atlassian.net
  user: config-user@example.com
  api_token: config-token
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, opts)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--jira-endpoint", "https://flag-jira.example.atlassian.net", "--jira-user", "flag-user@example.com", "--jira-token", "flag-token", "prompt"})
		require.NoError(t, err)
		require.Equal(t, "https://flag-jira.example.atlassian.net", c.Vibe.JiraEndpoint)
		require.Equal(t, "flag-user@example.com", c.Vibe.JiraUser)
		require.Equal(t, "flag-token", c.Vibe.JiraToken)

		var cEnv cli.CLI
		parserEnv, err := kong.New(&cEnv, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parserEnv.Parse([]string{"vibe", "--name", "test", "prompt"})
		require.NoError(t, err)
		require.Equal(t, "https://env-jira.example.atlassian.net", cEnv.Vibe.JiraEndpoint)
		require.Equal(t, "env-user@example.com", cEnv.Vibe.JiraUser)
		require.Equal(t, "env-token", cEnv.Vibe.JiraToken)

		ctxConfigOnly := newTestContext(t, cli.EnvMap{"REMUDA_CONFIG": "", "XDG_CONFIG_HOME": ""}, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		optsConfigOnly, err := cli.LoadConfigForKongWithContext(ctxConfigOnly)
		require.NoError(t, err)
		require.NotEmpty(t, optsConfigOnly)

		var cConfigOnly cli.CLI
		parserConfigOnly, err := kong.New(&cConfigOnly, append(optsConfigOnly, kong.Name("remuda"), kong.Bind(&ctxConfigOnly))...)
		require.NoError(t, err)

		_, err = parserConfigOnly.Parse([]string{"vibe", "--name", "test", "prompt"})
		require.NoError(t, err)
		require.Equal(t, "https://config-jira.example.atlassian.net", cConfigOnly.Vibe.JiraEndpoint)
		require.Equal(t, "config-user@example.com", cConfigOnly.Vibe.JiraUser)
		require.Equal(t, "config-token", cConfigOnly.Vibe.JiraToken)
	})

	t.Run("no config returns nil options", func(t *testing.T) {
		home := t.TempDir()
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "",
			"XDG_CONFIG_HOME": "",
		}
		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)
		require.Nil(t, opts)
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		home := t.TempDir()
		env := cli.EnvMap{
			"REMUDA_CONFIG":   "",
			"XDG_CONFIG_HOME": "",
		}

		configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
		require.NoError(t, os.WriteFile(configPath, []byte(`
version: 999
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		_, err := cli.LoadConfigForKongWithContext(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported config version")
	})
}

func TestConfigResolver_SliceFields(t *testing.T) {
	t.Parallel()
	t.Run("use_prompts slice is decoded correctly", func(t *testing.T) {
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
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "prompt"})
		require.NoError(t, err)
		require.Len(t, c.Vibe.Use, 1)
		require.Equal(t, "small-commits", c.Vibe.Use[0].String())
	})

	t.Run("no_use slice is decoded correctly", func(t *testing.T) {
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
  no_use:
    - make-pr
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "prompt"})
		require.NoError(t, err)
		require.Len(t, c.Vibe.NoUse, 1)
		require.Equal(t, "make-pr", c.Vibe.NoUse[0].String())
	})

	t.Run("no_use invalid prompt fails parse", func(t *testing.T) {
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
  no_use:
    - does-not-exist
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "prompt"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown prompt: does-not-exist")
	})

	t.Run("container_opts slice is decoded correctly", func(t *testing.T) {
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
  container:
    opts:
      - --network=host
      - --privileged
    inherit_env:
      - AWS_REGION
      - FOO_BAR
`), 0o644))

		ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
		opts, err := cli.LoadConfigForKongWithContext(ctx)
		require.NoError(t, err)

		var c cli.CLI
		parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"vibe", "--name", "test", "--repo-url", "https://github.com/test/repo", "prompt"})
		require.NoError(t, err)
		require.Equal(t, []string{"--network=host", "--privileged"}, c.Vibe.ContainerOpt)
		require.Equal(t, []string{"AWS_REGION", "FOO_BAR"}, c.Vibe.ContainerInheritEnv)
	})
}

func TestConfigResolver_AllFields(t *testing.T) {
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
repos:
  default_repo: test-repo
session:
  manager: zellij
defaults:
  agent: opencode
  model: gpt-5
  reasoning_level: high
  slugify_reasoning_level: medium
  yolo: true
  experiments:
    - my-experiment
  use_prompts:
    - small-commits
  container:
    enabled: true
    image: custom-image
    opts:
      - --network=host
    inherit_env:
      - AWS_REGION
`), 0o644))

	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))
	opts, err := cli.LoadConfigForKongWithContext(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, opts)

	var c cli.CLI
	parser, err := kong.New(&c, append(opts, kong.Name("remuda"), kong.Bind(&ctx))...)
	require.NoError(t, err)

	_, err = parser.Parse([]string{"vibe", "--name", "test", "prompt"})
	require.NoError(t, err)

	// Verify all fields were set from config
	require.Equal(t, "zellij", string(c.SessionManager))
	require.Equal(t, "opencode", c.Vibe.Agent)
	require.Equal(t, "gpt-5", c.Vibe.Model)
	require.Equal(t, "high", c.Vibe.ReasoningLevel)
	require.Equal(t, "medium", c.Vibe.SlugifyReasoningLevel)
	require.True(t, c.Vibe.Yolo)
	require.True(t, c.Vibe.ExperimentEnabled("my-experiment"))
	require.Len(t, c.Vibe.Use, 1)
	require.Equal(t, "small-commits", c.Vibe.Use[0].String())
	require.True(t, c.Vibe.Container)
	require.Equal(t, "custom-image", c.Vibe.ContainerName)
	require.Equal(t, []string{"--network=host"}, c.Vibe.ContainerOpt)
	require.Equal(t, []string{"AWS_REGION"}, c.Vibe.ContainerInheritEnv)
	require.NotNil(t, c.Vibe.Repo)
	require.Equal(t, "test-repo", *c.Vibe.Repo)
}

func TestConfigResolver_NilConfig(t *testing.T) {
	t.Parallel()
	// Test that NewConfigResolver with nil doesn't panic
	resolver := cli.NewConfigResolver(nil, nil)
	require.NotNil(t, resolver)

	// Validate should succeed
	require.NoError(t, resolver.Validate(nil))
}

func TestConfigResolver_EmptyConfig(t *testing.T) {
	t.Parallel()
	// Test with a config that has no values set
	cfg := &configfile.V1{Version: 1}
	resolver := cli.NewConfigResolver(cfg, nil)

	// Should not panic and return nil for all lookups
	val, err := resolver.Resolve(nil, nil, &kong.Flag{Value: &kong.Value{Name: "agent"}})
	require.NoError(t, err)
	require.Nil(t, val)
}
