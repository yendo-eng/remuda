package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/github"
)

// These tests touch global repo aliases and ftueSelectRepoFn; keep them serial.
func TestResolveRepoSelectionWithFTUE_ErrorsWhenUnsetWithoutTTY(t *testing.T) {
	installFTUEAliases(t)

	env := EnvMap{}
	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  strings.NewReader(""),
				Out: &bytes.Buffer{},
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
	)

	selection, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository is not configured")
	require.Equal(t, RepoSelection{}, selection)
}

func TestResolveRepoSelectionWithFTUE_UsesPromptOnTTY(t *testing.T) {
	installFTUEAliases(t)

	tty := openDevNullTTY(t)
	orig := ftueSelectRepoFn
	t.Cleanup(func() {
		ftueSelectRepoFn = orig
	})

	calls := 0
	ftueSelectRepoFn = func() (repoChoice, bool, error) {
		calls++
		return repoChoice{Alias: "remuda"}, false, nil
	}

	env := EnvMap{}
	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  tty,
				Out: tty,
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
	)

	selection, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
	}, true)
	require.NoError(t, err)

	want, err := github.RepoOrURL("", "remuda")
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, want, selection.RepoURL)
}

func TestResolveRepoSelectionWithFTUE_PersistsRememberedChoice(t *testing.T) {
	tty := openDevNullTTY(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	env := EnvMap{configOverrideEnvVar: configPath}

	orig := ftueSelectRepoFn
	t.Cleanup(func() {
		ftueSelectRepoFn = orig
	})

	calls := 0
	url := "https://github.com/acme/widgets.git"
	ftueSelectRepoFn = func() (repoChoice, bool, error) {
		calls++
		return repoChoice{URL: url}, true, nil
	}

	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  tty,
				Out: tty,
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
		WithHomeDir(dir),
		WithWorkingDir(dir),
	)

	selection, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
	}, true)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, url, selection.RepoURL)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	parsed, err := configfile.ParseV1(data)
	require.NoError(t, err)
	require.NotNil(t, parsed.Repos)
	require.NotNil(t, parsed.Repos.DefaultRepoURL)
	require.Equal(t, url, *parsed.Repos.DefaultRepoURL)
	require.Nil(t, parsed.Repos.DefaultRepo)
}

func TestResolveRepoSelectionWithFTUE_PersistsRememberedAliasWhenURLProvided(t *testing.T) {
	installFTUEAliases(t)

	tty := openDevNullTTY(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	env := EnvMap{configOverrideEnvVar: configPath}

	orig := ftueSelectRepoFn
	t.Cleanup(func() {
		ftueSelectRepoFn = orig
	})

	calls := 0
	wantURL, err := github.RepoOrURL("", "remuda")
	require.NoError(t, err)
	ftueSelectRepoFn = func() (repoChoice, bool, error) {
		calls++
		return repoChoice{Alias: "remuda", URL: wantURL}, true, nil
	}

	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  tty,
				Out: tty,
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
		WithHomeDir(dir),
		WithWorkingDir(dir),
	)

	selection, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
	}, true)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.Equal(t, wantURL, selection.RepoURL)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	parsed, err := configfile.ParseV1(data)
	require.NoError(t, err)
	require.NotNil(t, parsed.Repos)
	require.NotNil(t, parsed.Repos.DefaultRepo)
	require.Equal(t, "remuda", *parsed.Repos.DefaultRepo)
	require.Nil(t, parsed.Repos.DefaultRepoURL)
}

func TestResolveRepoSelectionWithFTUE_SkipsPromptForExistingWorkspace(t *testing.T) {
	installFTUEAliases(t)

	tty := openDevNullTTY(t)
	orig := ftueSelectRepoFn
	t.Cleanup(func() {
		ftueSelectRepoFn = orig
	})

	calls := 0
	ftueSelectRepoFn = func() (repoChoice, bool, error) {
		calls++
		return repoChoice{Alias: "remuda"}, false, nil
	}

	base := t.TempDir()
	workspace := filepath.Join(base, "acme", "widgets", "feature-1")

	env := EnvMap{}
	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  tty,
				Out: tty,
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
	)

	selection, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback:     true,
		ExistingWorkspace: workspace,
		ReposBaseDir:      base,
	}, true)
	require.NoError(t, err)
	require.Equal(t, 0, calls)
	require.Equal(t, RepoSourceWorkspace, selection.Source)
	require.Equal(t, "acme/widgets", selection.RepoSlug)
}

func TestResolveRepoSelectionWithFTUE_AppliesPerRepoOverlay(t *testing.T) {
	installFTUEAliases(t)

	tty := openDevNullTTY(t)
	orig := ftueSelectRepoFn
	t.Cleanup(func() {
		ftueSelectRepoFn = orig
	})
	ftueSelectRepoFn = func() (repoChoice, bool, error) {
		return repoChoice{Alias: "remuda"}, false, nil
	}

	agent := "opencode"
	profileAgent := "claude"
	profileName := "review"
	cfg := &configfile.V1{
		Version: 1,
		Profiles: map[string]configfile.DefaultsV1{
			profileName: {
				Agent: &profileAgent,
			},
		},
		PerRepo: map[string]configfile.OverlayV1{
			"acme/remuda": {
				Profile: &profileName,
				Defaults: &configfile.DefaultsV1{
					Agent: &agent,
				},
				Repos: &configfile.ReposV1{
					Aliases: map[string]string{
						"custom": "https://github.com/acme/utils.git",
					},
				},
			},
		},
	}

	env := EnvMap{}
	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  tty,
				Out: tty,
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
	)
	ctx.ConfigFile = cfg

	selection, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
	}, true)
	require.NoError(t, err)
	require.Equal(t, RepoSourceExplicit, selection.Source)
	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "claude", *cfg.Defaults.Agent)
	url, ok := github.ExpandRepoAlias("custom")
	require.True(t, ok)
	require.Equal(t, "https://github.com/acme/utils.git", url)
}

func TestResolveRepoSelectionWithFTUE_UnknownPerRepoProfileReturnsError(t *testing.T) {
	installFTUEAliases(t)

	tty := openDevNullTTY(t)
	orig := ftueSelectRepoFn
	t.Cleanup(func() {
		ftueSelectRepoFn = orig
	})
	ftueSelectRepoFn = func() (repoChoice, bool, error) {
		return repoChoice{Alias: "remuda"}, false, nil
	}

	profileName := "missing"
	cfg := &configfile.V1{
		Version: 1,
		PerRepo: map[string]configfile.OverlayV1{
			"acme/remuda": {
				Profile: &profileName,
			},
		},
	}

	env := EnvMap{}
	ctx := NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{
				In:  tty,
				Out: tty,
				Err: &bytes.Buffer{},
			},
		},
		WithEnv(env),
	)
	ctx.ConfigFile = cfg

	_, err := resolveRepoSelectionWithFTUE(ctx, nil, CloneRepoOption{}, RepoResolutionOptions{
		AllowFallback: true,
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["acme/remuda"].profile`)
	require.Contains(t, err.Error(), `"missing"`)
}

func TestPersistDefaultRepoSelectionAlias(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	env := EnvMap{configOverrideEnvVar: configPath}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(dir), WithWorkingDir(dir))
	_, err := persistDefaultRepoSelection(ctx, "remuda", "")
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	parsed, err := configfile.ParseV1(data)
	require.NoError(t, err)
	require.NotNil(t, parsed.Repos)
	require.NotNil(t, parsed.Repos.DefaultRepo)
	require.Equal(t, "remuda", *parsed.Repos.DefaultRepo)
	require.Nil(t, parsed.Repos.DefaultRepoURL)
}

func TestPersistDefaultRepoSelectionURL(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	env := EnvMap{configOverrideEnvVar: configPath}

	url := "https://github.com/acme/widgets.git"
	ctx := newTestContextWithEnv(t, env, WithHomeDir(dir), WithWorkingDir(dir))
	_, err := persistDefaultRepoSelection(ctx, "", url)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	parsed, err := configfile.ParseV1(data)
	require.NoError(t, err)
	require.NotNil(t, parsed.Repos)
	require.NotNil(t, parsed.Repos.DefaultRepoURL)
	require.Equal(t, url, *parsed.Repos.DefaultRepoURL)
	require.Nil(t, parsed.Repos.DefaultRepo)
}

func openDevNullTTY(t *testing.T) *os.File {
	t.Helper()
	tty, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tty.Close()
	})
	return tty
}

func installFTUEAliases(t *testing.T) {
	t.Helper()
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)
	github.MergeRepoAliases(map[string]string{
		"widgets": "https://github.com/acme/default.git",
		"remuda":  "https://github.com/acme/remuda.git",
	})
}
