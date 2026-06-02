package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal/github"
)

// These tests mutate the global repo alias registry; keep them serial.
func TestConfig_MergesRepoAliases(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
repos:
  aliases:
    myrepo: https://github.com/acme/myrepo.git
`), 0o644))

	// Load config to trigger alias merge
	opts, err := cli.LoadConfigForKongWithContext(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, opts)

	url, ok := github.ExpandRepoAlias("myrepo")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/myrepo.git", url)

	aliases := github.RepoAliases()
	require.Equal(t, "https://github.com/acme/myrepo.git", aliases["myrepo"])
}

func TestConfig_AliasConfiguredThroughFile(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
repos:
  aliases:
    remuda: https://github.com/custom/remuda-fork.git
`), 0o644))

	_, err := cli.LoadConfigForKongWithContext(ctx)
	require.NoError(t, err)

	url, ok := github.ExpandRepoAlias("remuda")
	require.True(t, ok)
	require.Equal(t, "https://github.com/custom/remuda-fork.git", url)
}

func TestConfig_AliasCaseNormalization(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
repos:
  aliases:
    MyRepo: https://github.com/acme/myrepo.git
`), 0o644))

	_, err := cli.LoadConfigForKongWithContext(ctx)
	require.NoError(t, err)

	// Should match case-insensitively
	url, ok := github.ExpandRepoAlias("myrepo")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/myrepo.git", url)

	url, ok = github.ExpandRepoAlias("MYREPO")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/myrepo.git", url)
}

func TestConfig_AliasValueTrimmed(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
repos:
  aliases:
    spacey: "  https://github.com/acme/spacey.git  "
`), 0o644))

	_, err := cli.LoadConfigForKongWithContext(ctx)
	require.NoError(t, err)

	url, ok := github.ExpandRepoAlias("spacey")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/spacey.git", url, "value should be trimmed")
}

func TestConfig_AliasRejectsDashPrefix(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
repos:
  aliases:
    evil: "--upload-pack=evil"
`), 0o644))

	// Config with dash-prefixed URL should fail validation
	_, err := cli.LoadConfigForKongWithContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot start with '-'")
}

func TestConfig_AliasRejectsEmptyURL(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	home := t.TempDir()
	env := cli.EnvMap{
		"REMUDA_CONFIG":   "",
		"XDG_CONFIG_HOME": "",
	}
	ctx := newTestContext(t, env, cli.WithHomeDir(home), cli.WithWorkingDir(home))

	configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
repos:
  aliases:
    emptyval: ""
`), 0o644))

	// Config with empty URL should fail validation
	_, err := cli.LoadConfigForKongWithContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestMergeRepoAliases_SkipsEmptyKeyOrValue(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	// Direct unit test on MergeRepoAliases (runtime safety net)
	github.MergeRepoAliases(map[string]string{
		"":        "https://github.com/acme/empty-key.git",
		"  ":      "https://github.com/acme/whitespace-key.git",
		"novalue": "",
		"spaces":  "   ",
	})

	_, ok := github.ExpandRepoAlias("")
	require.False(t, ok)

	_, ok = github.ExpandRepoAlias("novalue")
	require.False(t, ok)

	_, ok = github.ExpandRepoAlias("spaces")
	require.False(t, ok)
}

func TestResetRepoAliases(t *testing.T) {
	t.Cleanup(github.ResetRepoAliases)

	// Add a custom alias
	github.MergeRepoAliases(map[string]string{
		"custom": "https://github.com/acme/custom.git",
	})

	url, ok := github.ExpandRepoAlias("custom")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/custom.git", url)

	// Reset should remove custom alias
	github.ResetRepoAliases()

	_, ok = github.ExpandRepoAlias("custom")
	require.False(t, ok, "custom alias should be gone after reset")

	// With no built-ins, reset returns the registry to empty.
	_, ok = github.ExpandRepoAlias("remuda")
	require.False(t, ok)
}
