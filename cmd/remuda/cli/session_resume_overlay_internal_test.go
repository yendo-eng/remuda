package cli

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestApplyPerRepoOverlaysForPickedSessionResume_ProfileFlagOverridesPerRepoProfile(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	perRepoProfile := "fast"
	perRepoAgent := "bash"
	fastAgent := "opencode"
	reviewAgent := "claude"
	cfg := &configfile.V1{
		Version: 1,
		PerRepo: map[string]configfile.OverlayV1{
			"owner/repo": {
				Profile: &perRepoProfile,
				Defaults: &configfile.DefaultsV1{
					Agent: &perRepoAgent,
				},
			},
		},
		Profiles: map[string]configfile.DefaultsV1{
			"fast": {
				Agent: &fastAgent,
			},
			"review": {
				Agent: &reviewAgent,
			},
		},
	}

	ctx := newTestContextWithEnv(t, EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	})
	ctx.ConfigFile = cfg

	kctx := &kong.Context{Args: []string{"session", "resume", "--pick", "--profile", "review"}}
	require.NoError(t, applyPerRepoOverlaysForPickedSessionResume(ctx, kctx, workspace))

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "claude", *cfg.Defaults.Agent)
}

func TestApplyPerRepoOverlaysForPickedSessionResume_UsesPerRepoProfileWhenProfileUnset(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	workspace := filepath.Join(base, "owner", "repo", "wk")
	perRepoProfile := "fast"
	perRepoAgent := "bash"
	fastAgent := "opencode"
	cfg := &configfile.V1{
		Version: 1,
		PerRepo: map[string]configfile.OverlayV1{
			"owner/repo": {
				Profile: &perRepoProfile,
				Defaults: &configfile.DefaultsV1{
					Agent: &perRepoAgent,
				},
			},
		},
		Profiles: map[string]configfile.DefaultsV1{
			"fast": {
				Agent: &fastAgent,
			},
		},
	}

	ctx := newTestContextWithEnv(t, EnvMap{
		"REMUDA_REPOS_BASE_DIR": base,
	})
	ctx.ConfigFile = cfg

	kctx := &kong.Context{Args: []string{"session", "resume", "--pick"}}
	require.NoError(t, applyPerRepoOverlaysForPickedSessionResume(ctx, kctx, workspace))

	require.NotNil(t, cfg.Defaults)
	require.NotNil(t, cfg.Defaults.Agent)
	require.Equal(t, "opencode", *cfg.Defaults.Agent)
}
