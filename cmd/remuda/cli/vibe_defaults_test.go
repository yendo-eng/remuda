package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestApplyPerRepoDefaultsToVibe_AppliesDefaults(t *testing.T) {
	t.Parallel()
	strptr := func(s string) *string { return &s }
	boolptr := func(b bool) *bool { return &b }

	usePrompts := []string{"small-commits"}
	noUsePrompts := []string{"make-pr"}
	experiments := []string{"exp-a", "exp-b"}
	containerOpts := []string{"--net=host"}
	containerEnv := []string{"AWS_REGION"}

	cfg := &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Agent:                 strptr("opencode"),
			Model:                 strptr("gpt-4"),
			ReasoningLevel:        strptr("high"),
			SlugifyReasoningLevel: strptr("medium"),
			AgentCmd:              strptr("custom-agent"),
			SkipVersionCheck:      boolptr(true),
			UsePrompts:            &usePrompts,
			NoUse:                 &noUsePrompts,
			Experiments:           &experiments,
			Yolo:                  boolptr(true),
			Container: &configfile.ContainerV1{
				Enabled:    boolptr(true),
				Image:      strptr("custom-image"),
				Opts:       &containerOpts,
				InheritEnv: &containerEnv,
			},
		},
	}

	cmd := VibeCmd{
		AgentSessionOptions: AgentSessionOptions{
			Agent:            "codex",
			Model:            "gpt-3.5",
			AgentCmd:         "old-agent",
			SkipVersionCheck: false,
		},
		VibeContainerOptions: VibeContainerOptions{
			Container:           false,
			ContainerName:       "vibe-dev",
			ContainerOpt:        []string{"--old-opt"},
			ContainerInheritEnv: []string{"OLD_ENV"},
		},
	}

	require.NoError(t, applyPerRepoDefaultsToVibe(&cmd, nil, cfg, EnvMap{}))
	require.Equal(t, "opencode", cmd.Agent)
	require.Equal(t, "gpt-4", cmd.Model)
	require.Equal(t, "high", cmd.ReasoningLevel)
	require.Equal(t, "medium", cmd.SlugifyReasoningLevel)
	require.Equal(t, "custom-agent", cmd.AgentCmd)
	require.True(t, cmd.SkipVersionCheck)
	require.Equal(t, "exp-a,exp-b", cmd.Experiments)
	require.True(t, cmd.Yolo)
	require.True(t, cmd.Container)
	require.Equal(t, "custom-image", cmd.ContainerName)
	require.Equal(t, []string{"--net=host"}, cmd.ContainerOpt)
	require.Equal(t, []string{"AWS_REGION"}, cmd.ContainerInheritEnv)
	require.Len(t, cmd.Use, 1)
	require.Equal(t, "small-commits", cmd.Use[0].String())
	require.Len(t, cmd.NoUse, 1)
	require.Equal(t, "make-pr", cmd.NoUse[0].String())
}
