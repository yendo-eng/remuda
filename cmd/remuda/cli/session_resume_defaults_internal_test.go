package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestApplyDefaultsToSessionResume_UsesProfileDefaults(t *testing.T) {
	profileImage := "profile-image"
	profileOpts := []string{"--network=host"}
	profileInherit := []string{"FOO"}
	yolo := true
	enabled := true

	cfg := &configfile.V1{
		Profiles: map[string]configfile.DefaultsV1{
			"review": {
				Yolo: &yolo,
				Container: &configfile.ContainerV1{
					Enabled:    &enabled,
					Image:      &profileImage,
					Opts:       &profileOpts,
					InheritEnv: &profileInherit,
				},
			},
		},
	}

	require.NoError(t, applyProfileOverlayByName(cfg, "review"))

	cmd := SessionResumeCmd{
		VibeContainerOptions: VibeContainerOptions{ContainerName: "vibe-dev"},
	}

	require.NoError(t, applyDefaultsToSessionResume(&cmd, nil, cfg, EnvMap{}))
	require.True(t, cmd.Yolo)
	require.True(t, cmd.Container)
	require.Equal(t, profileImage, cmd.ContainerName)
	require.Equal(t, profileOpts, cmd.ContainerOpt)
	require.Equal(t, profileInherit, cmd.ContainerInheritEnv)
}

func TestResolveSessionResumeReasoningLevel_UsesProfileDefault(t *testing.T) {
	level := "high"
	cfg := &configfile.V1{
		Defaults: &configfile.DefaultsV1{
			ReasoningLevel: &level,
		},
	}

	require.Equal(t, level, resolveSessionResumeReasoningLevel(cfg, EnvMap{}))
}

func TestResolveSessionResumeReasoningLevel_EnvOverridesProfile(t *testing.T) {
	level := "high"
	cfg := &configfile.V1{
		Defaults: &configfile.DefaultsV1{
			ReasoningLevel: &level,
		},
	}
	env := EnvMap{"REMUDA_REASONING_LEVEL": "low"}

	require.Equal(t, "low", resolveSessionResumeReasoningLevel(cfg, env))
}

func TestResolveSessionResumeAgent_UsesProfileDefault(t *testing.T) {
	agent := "claude"
	cfg := &configfile.V1{
		Defaults: &configfile.DefaultsV1{
			Agent: &agent,
		},
	}

	require.Equal(t, "claude", resolveSessionResumeAgent(cfg, EnvMap{}))
}

func TestResolveSessionResumeAgent_EnvOverridesProfile(t *testing.T) {
	agent := "claude"
	cfg := &configfile.V1{
		Defaults: &configfile.DefaultsV1{
			Agent: &agent,
		},
	}
	env := EnvMap{"REMUDA_AGENT": "codex"}

	require.Equal(t, "codex", resolveSessionResumeAgent(cfg, env))
}

func TestResolveSessionResumeAgent_BlankEnvUsesProfileDefault(t *testing.T) {
	agent := "claude"
	cfg := &configfile.V1{
		Defaults: &configfile.DefaultsV1{
			Agent: &agent,
		},
	}
	env := EnvMap{"REMUDA_AGENT": "   "}

	require.Equal(t, "claude", resolveSessionResumeAgent(cfg, env))
}
