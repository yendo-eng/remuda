package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestInvocationUsesRepo_CommandOnly(t *testing.T) {
	t.Parallel()
	require.True(t, invocationUsesRepo([]string{"vibe", "--name", "wk", "prompt"}))
	require.True(t, invocationUsesRepo([]string{"-v", "--session-manager", "tmux", "vibe", "--name", "wk", "prompt"}))
	require.False(t, invocationUsesRepo([]string{"prompts", "vibe"}), "positional arg matching a command should not enable overlays")
	require.False(t, invocationUsesRepo([]string{"--help", "vibe"}), "help should not enable overlays")
	require.True(t, invocationUsesRepo([]string{"session", "resume", "/tmp/workspace"}))
	require.False(t, invocationUsesRepo([]string{"session", "list"}))
}

func TestInferRepoSlugForInvocation_PrefersExplicitFlagsOverPRURL(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{Version: 1}
	args := []string{
		"vibe-check",
		"--name", "wk",
		"--repo-url", "https://github.com/Explicit/Repo.git",
		"https://github.com/other/repo/pull/123",
	}
	ctx := newTestContextWithEnv(t, EnvMap{})
	require.Equal(t, "explicit/repo", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_ExpandsShorthandRepoURLFlag(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{Version: 1}
	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", "github.com/Acme/Repo",
		"prompt",
	}
	ctx := newTestContextWithEnv(t, EnvMap{})
	require.Equal(t, "acme/repo", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_DoesNotScanPRURLForVibe(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{Version: 1}
	args := []string{
		"vibe",
		"--name", "wk",
		"please review https://github.com/test/repo/pull/123",
	}
	ctx := newTestContextWithEnv(t, EnvMap{})
	require.Equal(t, "", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_UsesWorkspacePathForVibeIn(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	}
	cfg := &configfile.V1{Version: 1}
	workspace := filepath.Join(base, "acme", "widgets", "feature-1")
	args := []string{
		"vibe",
		"--in", workspace,
	}
	ctx := newTestContextWithEnv(t, env)
	require.Equal(t, "acme/widgets", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_UsesWorkspacePathForSessionResume(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	}
	cfg := &configfile.V1{Version: 1}
	workspace := filepath.Join(base, "acme", "remuda", "wk")
	args := []string{
		"session",
		"resume",
		"--container-name", "vibe-dev",
		workspace,
	}
	ctx := newTestContextWithEnv(t, env)
	require.Equal(t, "acme/remuda", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_SessionResumeSkipsProfileFlagValue(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	}
	cfg := &configfile.V1{Version: 1}
	workspace := filepath.Join(base, "acme", "remuda", "wk")
	args := []string{
		"session",
		"resume",
		"--profile", "fast",
		workspace,
	}
	ctx := newTestContextWithEnv(t, env)
	require.Equal(t, "acme/remuda", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_SessionResumePickSkipsDefaults(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	}
	cfg := &configfile.V1{Version: 1}
	args := []string{
		"session",
		"resume",
		"--pick",
	}
	ctx := newTestContextWithEnv(t, env)
	require.Equal(t, "", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_UsesCloneRepoURLArg(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{Version: 1}
	args := []string{
		"clone",
		"--name", "wk",
		"https://github.com/acme/utils.git",
	}
	ctx := newTestContextWithEnv(t, EnvMap{})
	require.Equal(t, "acme/utils", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_UsesConfigBaseDirForVibeIn(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	env := EnvMap{
		"REMUDA_REPOS_BASE_DIR": "",
	}
	cfg := &configfile.V1{
		Version: 1,
		Repos: &configfile.ReposV1{
			BaseDir: &base,
		},
	}
	workspace := filepath.Join(base, "org", "repo", "wk")
	args := []string{
		"vibe",
		"--in", workspace,
	}
	ctx := newTestContextWithEnv(t, env)
	require.Equal(t, "org/repo", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestInferRepoSlugForInvocation_IgnoresWorkspaceOutsideBaseDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	outside := t.TempDir()
	env := EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	}
	cfg := &configfile.V1{Version: 1}
	workspace := filepath.Join(outside, "org", "repo", "wk")
	args := []string{
		"vibe",
		"--in", workspace,
	}
	ctx := newTestContextWithEnv(t, env)
	require.Equal(t, "", inferRepoSlugForInvocation(ctx, cfg, args))
}

func TestMergeOverlayV1IntoConfig_MergesContainerInheritEnv(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{Version: 1}
	envs := []string{"AWS_REGION", "FOO_BAR"}
	overlay := configfile.OverlayV1{
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				InheritEnv: &envs,
			},
		},
	}

	mergeOverlayV1IntoConfig(cfg, overlay, false)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Container)
	require.NotNil(t, cfg.Defaults.Container.InheritEnv)
	require.Equal(t, []string{"AWS_REGION", "FOO_BAR"}, *cfg.Defaults.Container.InheritEnv)
}

func TestMergeOverlayV1IntoConfig_MergesContainerOptsWhenEnabled(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				Opts: &[]string{"--network=host"},
			},
		},
	}
	overlay := configfile.OverlayV1{
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				Opts: &[]string{"--gpus all"},
			},
		},
	}

	mergeOverlayV1IntoConfig(cfg, overlay, true)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Container)
	require.NotNil(t, cfg.Defaults.Container.Opts)
	require.Equal(t, []string{"--network=host", "--gpus all"}, *cfg.Defaults.Container.Opts)
}

func TestMergeOverlayV1IntoConfig_DoesNotMergeContainerOptsWhenDisabled(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				Opts: &[]string{"--network=host"},
			},
		},
	}
	overlay := configfile.OverlayV1{
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				Opts: &[]string{"--gpus all"},
			},
		},
	}

	mergeOverlayV1IntoConfig(cfg, overlay, false)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Container)
	require.NotNil(t, cfg.Defaults.Container.Opts)
	require.Equal(t, []string{"--gpus all"}, *cfg.Defaults.Container.Opts)
}

func TestMergeOverlayV1IntoConfig_EmptyContainerOptsClearsDefaults(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				Opts: &[]string{"--network=host"},
			},
		},
	}
	empty := []string{}
	overlay := configfile.OverlayV1{
		Defaults: &configfile.DefaultsV1{
			Container: &configfile.ContainerV1{
				Opts: &empty,
			},
		},
	}

	mergeOverlayV1IntoConfig(cfg, overlay, true)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Container)
	require.NotNil(t, cfg.Defaults.Container.Opts)
	require.Empty(t, *cfg.Defaults.Container.Opts)
}

func TestMergeOverlayV1IntoConfig_MergesDefaultsExtras(t *testing.T) {
	t.Parallel()
	cfg := &configfile.V1{Version: 1}
	experiments := []string{"exp-a"}
	noUse := []string{"make-pr"}
	yolo := true
	overlay := configfile.OverlayV1{
		Defaults: &configfile.DefaultsV1{
			Experiments: &experiments,
			NoUse:       &noUse,
			Yolo:        &yolo,
		},
	}

	mergeOverlayV1IntoConfig(cfg, overlay, false)

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Experiments)
	require.Equal(t, []string{"exp-a"}, *cfg.Defaults.Experiments)
	require.NotNil(t, cfg.Defaults.NoUse)
	require.Equal(t, []string{"make-pr"}, *cfg.Defaults.NoUse)
	require.NotNil(t, cfg.Defaults.Yolo)
	require.True(t, *cfg.Defaults.Yolo)
}

func TestSelectedProfileForInvocation_FlagOverridesEnv(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROFILE": "env"}
	args := []string{"vibe", "--profile", "fast", "--name", "wk", "prompt"}

	name, ok := selectedProfileForInvocation(args, env)

	require.True(t, ok)
	require.Equal(t, "fast", name)
}

func TestSelectedProfileForInvocation_FlagEquals(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROFILE": "env"}
	args := []string{"vibe", "--profile=fast", "--name", "wk", "prompt"}

	name, ok := selectedProfileForInvocation(args, env)

	require.True(t, ok)
	require.Equal(t, "fast", name)
}

func TestSelectedProfileForInvocation_EnvForSupportedCommand(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROFILE": "review"}
	args := []string{"vibe-check", "main"}

	name, ok := selectedProfileForInvocation(args, env)

	require.True(t, ok)
	require.Equal(t, "review", name)
}

func TestSelectedProfileForInvocation_SessionResumeFlag(t *testing.T) {
	t.Parallel()
	args := []string{"session", "resume", "--profile", "fast", "/tmp/workspace"}

	name, ok := selectedProfileForInvocation(args, EnvMap{})

	require.True(t, ok)
	require.Equal(t, "fast", name)
}

func TestSelectedProfileForInvocation_SessionResumeEnv(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROFILE": "ops"}
	args := []string{"session", "resume", "/tmp/workspace"}

	name, ok := selectedProfileForInvocation(args, env)

	require.True(t, ok)
	require.Equal(t, "ops", name)
}

func TestSelectedProfileForInvocation_IgnoresUnsupportedCommand(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROFILE": "fast"}
	args := []string{"clone", "--name", "wk", "--repo-url", "https://github.com/example/example.git"}

	name, ok := selectedProfileForInvocation(args, env)

	require.False(t, ok)
	require.Equal(t, "", name)
}

func TestSelectedProfileForInvocation_RespectsTerminator(t *testing.T) {
	t.Parallel()
	args := []string{"vibe", "--name", "wk", "--", "--profile", "fast"}

	name, ok := selectedProfileForInvocation(args, EnvMap{})

	require.False(t, ok)
	require.Equal(t, "", name)
}

func TestApplyProfileOverlayByName_MergesDefaults(t *testing.T) {
	t.Parallel()
	agent := "codex"
	profileAgent := "opencode"
	model := "gpt-5"
	yolo := true
	cfg := &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Agent: &agent,
		},
		Profiles: map[string]configfile.DefaultsV1{
			"fast": {
				Agent: &profileAgent,
				Model: &model,
				Yolo:  &yolo,
			},
		},
	}

	err := applyProfileOverlayByName(cfg, "fast")

	require.NoError(t, err)
	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "opencode", *cfg.Defaults.Agent)
	require.NotNil(t, cfg.Defaults.Model)
	require.Equal(t, "gpt-5", *cfg.Defaults.Model)
	require.NotNil(t, cfg.Defaults.Yolo)
	require.True(t, *cfg.Defaults.Yolo)
}

func TestProfileOverridesDefaultsForYoloAndContainer(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  yolo: true
  container:
    enabled: true
profiles:
  fast:
    yolo: false
    container:
      enabled: false
`), 0o644))

	env := EnvMap{
		"REMUDA_CONFIG": configPath,
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(home), WithWorkingDir(home))

	cfg, _, err := loadConfigV1(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	args := []string{"vibe", "--profile", "fast", "--name", "wk", "prompt"}
	require.NoError(t, applyProfileOverlay(ctx, cfg, args))

	var cli CLI
	parserOpts := []kong.Option{kong.Name("remuda"), kong.Bind(&ctx)}
	parserOpts = append(parserOpts, kongOptionsFromConfig(cfg, envFromContext(ctx))...)
	parser, err := kong.New(&cli, parserOpts...)
	require.NoError(t, err)

	_, err = parser.Parse(args)
	require.NoError(t, err)
	require.False(t, cli.Vibe.Yolo)
	require.False(t, cli.Vibe.Container)
}

func TestProfileOverridesPerRepoNoUse(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  no_use:
    - small-commits
per_repo:
  "owner/repo":
    defaults:
      no_use:
        - make-pr
profiles:
  fast:
    no_use:
      - update-docs
`), 0o644))

	env := EnvMap{
		"REMUDA_CONFIG": configPath,
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(home), WithWorkingDir(home))

	cfg, _, err := loadConfigV1(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	args := []string{"vibe", "--profile", "fast", "--repo-url", "https://github.com/owner/repo", "--name", "wk", "prompt"}
	require.NoError(t, applyPerRepoOverlay(ctx, cfg, args))
	require.NoError(t, applyProfileOverlay(ctx, cfg, args))

	var cli CLI
	parserOpts := []kong.Option{kong.Name("remuda"), kong.Bind(&ctx)}
	parserOpts = append(parserOpts, kongOptionsFromConfig(cfg, envFromContext(ctx))...)
	parser, err := kong.New(&cli, parserOpts...)
	require.NoError(t, err)

	_, err = parser.Parse(args)
	require.NoError(t, err)
	require.Len(t, cli.Vibe.NoUse, 1)
	require.Equal(t, "update-docs", cli.Vibe.NoUse[0].String())
}

func TestApplyProfileOverlay_UsesPerRepoProfileWhenNoFlagOrEnv(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
per_repo:
  owner/repo:
    profile: fast
profiles:
  fast:
    agent: opencode
`), 0o644))

	env := EnvMap{
		"REMUDA_CONFIG": configPath,
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(home), WithWorkingDir(home))

	cfg, _, err := loadConfigV1(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	args := []string{"vibe", "--repo-url", "https://github.com/owner/repo", "--name", "wk", "prompt"}
	require.NoError(t, applyProfileOverlay(ctx, cfg, args))

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "opencode", *cfg.Defaults.Agent)
}

func TestApplyProfileOverlay_ProfileFlagOverridesPerRepoProfile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
per_repo:
  owner/repo:
    profile: fast
profiles:
  fast:
    agent: opencode
  review:
    agent: claude
`), 0o644))

	env := EnvMap{
		"REMUDA_CONFIG": configPath,
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(home), WithWorkingDir(home))

	cfg, _, err := loadConfigV1(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	args := []string{"vibe", "--repo-url", "https://github.com/owner/repo", "--profile", "review", "--name", "wk", "prompt"}
	require.NoError(t, applyProfileOverlay(ctx, cfg, args))

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "claude", *cfg.Defaults.Agent)
}

func TestApplyProfileOverlay_ProfileEnvOverridesPerRepoProfile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
per_repo:
  owner/repo:
    profile: fast
profiles:
  fast:
    agent: opencode
  review:
    agent: claude
`), 0o644))

	env := EnvMap{
		"REMUDA_CONFIG":  configPath,
		"REMUDA_PROFILE": "review",
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(home), WithWorkingDir(home))

	cfg, _, err := loadConfigV1(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	args := []string{"vibe", "--repo-url", "https://github.com/owner/repo", "--name", "wk", "prompt"}
	require.NoError(t, applyProfileOverlay(ctx, cfg, args))

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "claude", *cfg.Defaults.Agent)
}

func TestApplyProfileOverlay_UnknownPerRepoProfileIncludesSlugAndName(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
per_repo:
  owner/repo:
    profile: missing
`), 0o644))

	env := EnvMap{
		"REMUDA_CONFIG": configPath,
	}
	ctx := newTestContextWithEnv(t, env, WithHomeDir(home), WithWorkingDir(home))

	cfg, _, err := loadConfigV1(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	args := []string{"vibe", "--repo-url", "https://github.com/owner/repo", "--name", "wk", "prompt"}
	err = applyProfileOverlay(ctx, cfg, args)
	require.Error(t, err)
	require.Contains(t, err.Error(), `per_repo["owner/repo"].profile`)
	require.Contains(t, err.Error(), `"missing"`)
}
