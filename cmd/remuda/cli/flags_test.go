package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
	"gopkg.in/yaml.v3"
)

func parseTestConfig(t *testing.T, content string) *configfile.V1 {
	t.Helper()
	var cfg configfile.V1
	require.NoError(t, yaml.Unmarshal([]byte(content), &cfg))
	return &cfg
}

func newResolutionFixture(t *testing.T, args ...string) (*flagSet, *flagResolution, *struct {
	Agent        string
	Model        string
	Use          []string
	UsePosition  string
	Experiments  string
	Yolo         bool
	ContainerOpt []string
}) {
	t.Helper()
	vals := &struct {
		Agent        string
		Model        string
		Use          []string
		UsePosition  string
		Experiments  string
		Yolo         bool
		ContainerOpt []string
	}{}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.StringVar(&vals.Agent, "agent", "codex", "")
	fs.StringVar(&vals.Model, "model", "", "")
	fs.StringSliceVar(&vals.Use, "use", nil, "")
	fs.StringVar(&vals.UsePosition, "use-position", "before", "")
	fs.StringVar(&vals.Experiments, "experiments", "", "")
	fs.BoolVar(&vals.Yolo, "yolo", false, "")
	fs.StringSliceVar(&vals.ContainerOpt, "container-opt", nil, "")

	fl := newFlagSet(fs)
	fl.bind("agent", bindEnvs("REMUDA_AGENT"), bindKey("defaults.agent"), bindEnum("codex", "opencode", "claude", "bash"))
	fl.bind("model", bindEnvs("REMUDA_MODEL"), bindKey("defaults.model"))
	fl.bind("use", bindEnvs("REMUDA_USE_PROMPTS"), bindKey("defaults.use_prompts"), bindMergeConfigSlice())
	fl.bind("use-position", bindEnvs("REMUDA_USE_PROMPTS_POSITION"), bindKey("defaults.use_prompts_position"), bindEnum("before", "after"))
	fl.bind("experiments", bindEnvs("REMUDA_EXPERIMENTS"), bindKey("defaults.experiments"))
	fl.bind("yolo", bindEnvs("REMUDA_YOLO"), bindKey("defaults.yolo"))
	fl.bind("container-opt", bindEnvs("REMUDA_CONTAINER_OPTS"), bindKey("defaults.container.opts"))
	fl.negatable("yolo")

	require.NoError(t, fs.Parse(args))
	rs, err := beginResolution(fl)
	require.NoError(t, err)
	return fl, rs, vals
}

func TestFlagResolution_Precedence(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  agent: opencode
  model: config-model
`)

	tests := []struct {
		name      string
		args      []string
		env       EnvMap
		wantAgent string
		wantModel string
	}{
		{name: "config over default", wantAgent: "opencode", wantModel: "config-model"},
		{name: "env over config", env: EnvMap{"REMUDA_AGENT": "claude"}, wantAgent: "claude", wantModel: "config-model"},
		{name: "flag over env and config", args: []string{"--agent", "bash"}, env: EnvMap{"REMUDA_AGENT": "claude"}, wantAgent: "bash", wantModel: "config-model"},
		{name: "empty env ignored", env: EnvMap{"REMUDA_AGENT": ""}, wantAgent: "opencode", wantModel: "config-model"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, rs, vals := newResolutionFixture(t, tc.args...)
			eff, err := newEffectiveConfig(cfg, "", profileRef{})
			require.NoError(t, err)
			require.NoError(t, rs.apply(envOrDefault(tc.env), eff))
			require.NoError(t, rs.validateEnums())
			require.Equal(t, tc.wantAgent, vals.Agent)
			require.Equal(t, tc.wantModel, vals.Model)
		})
	}
}

func TestFlagResolution_UsePromptPositionPrecedence(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  use_prompts_position: after
`)

	tests := []struct {
		name string
		args []string
		env  EnvMap
		want string
	}{
		{name: "config", want: "after"},
		{name: "environment", env: EnvMap{"REMUDA_USE_PROMPTS_POSITION": "before"}, want: "before"},
		{name: "flag", args: []string{"--use-position", "after"}, env: EnvMap{"REMUDA_USE_PROMPTS_POSITION": "before"}, want: "after"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, rs, vals := newResolutionFixture(t, test.args...)
			eff, err := newEffectiveConfig(cfg, "", profileRef{})
			require.NoError(t, err)
			require.NoError(t, rs.apply(test.env, eff))
			require.NoError(t, rs.validateEnums())
			require.Equal(t, test.want, vals.UsePosition)
		})
	}
}

func TestFlagResolution_SliceAndScalarConversions(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  use_prompts: [small-commits, make-pr]
  experiments: [exp-a, exp-b]
  container:
    opts: ["--net=host"]
`)

	_, rs, vals := newResolutionFixture(t)
	eff, err := newEffectiveConfig(cfg, "", profileRef{})
	require.NoError(t, err)
	require.NoError(t, rs.apply(EnvMap{}, eff))
	require.Equal(t, []string{"small-commits", "make-pr"}, vals.Use)
	require.Equal(t, "exp-a,exp-b", vals.Experiments)
	require.Equal(t, []string{"--net=host"}, vals.ContainerOpt)
}

// Base and per_repo container opts concatenate in the effective config,
// which stores the merged list as []string rather than YAML's []any; the
// flag must receive the elements, not a stringified slice.
func TestFlagResolution_AppliesConcatenatedContainerOpts(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  container:
    opts: ["-v /base:/base"]
per_repo:
  acme/utils:
    defaults:
      container:
        opts: ["-v /repo:/repo", "--cpus=4"]
`)

	_, rs, vals := newResolutionFixture(t)
	eff, err := newEffectiveConfig(cfg, "acme/utils", profileRef{})
	require.NoError(t, err)
	require.NoError(t, rs.apply(EnvMap{}, eff))
	require.Equal(t, []string{"-v /base:/base", "-v /repo:/repo", "--cpus=4"}, vals.ContainerOpt)
}

func TestFlagResolution_EnvSliceSplitsOnComma(t *testing.T) {
	t.Parallel()
	_, rs, vals := newResolutionFixture(t)
	require.NoError(t, rs.apply(EnvMap{"REMUDA_USE_PROMPTS": "a,b"}, nil))
	require.Equal(t, []string{"a", "b"}, vals.Use)
}

func TestFlagResolution_MergeConfigSliceAddsConfigDefaults(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  use_prompts: [small-commits]
`)

	_, rs, vals := newResolutionFixture(t, "--use", "custom")
	eff, err := newEffectiveConfig(cfg, "", profileRef{})
	require.NoError(t, err)
	require.NoError(t, rs.apply(EnvMap{}, eff))
	require.Equal(t, []string{"small-commits", "custom"}, vals.Use)
}

func TestFlagResolution_MergeConfigSliceSkippedWhenEnvSet(t *testing.T) {
	t.Parallel()
	cfg := parseTestConfig(t, `
version: 1
defaults:
  use_prompts: [small-commits]
`)

	_, rs, vals := newResolutionFixture(t, "--use", "custom")
	eff, err := newEffectiveConfig(cfg, "", profileRef{})
	require.NoError(t, err)
	require.NoError(t, rs.apply(EnvMap{"REMUDA_USE_PROMPTS": "from-env"}, eff))
	require.Equal(t, []string{"custom"}, vals.Use)
}

// A second resolution pass with an overlay-updated config must merge against
// the user's original flag values, not compound the first pass's merge.
func TestFlagResolution_MergeConfigSliceReappliesCleanly(t *testing.T) {
	t.Parallel()
	base := parseTestConfig(t, `
version: 1
defaults:
  use_prompts: [base-prompt]
per_repo:
  acme/utils:
    defaults:
      use_prompts: [repo-prompt]
`)

	_, rs, vals := newResolutionFixture(t, "--use", "custom")

	eff, err := newEffectiveConfig(base, "", profileRef{})
	require.NoError(t, err)
	require.NoError(t, rs.apply(EnvMap{}, eff))
	require.Equal(t, []string{"base-prompt", "custom"}, vals.Use)

	eff, err = newEffectiveConfig(base, "acme/utils", profileRef{})
	require.NoError(t, err)
	require.NoError(t, rs.apply(EnvMap{}, eff))
	require.Equal(t, []string{"repo-prompt", "custom"}, vals.Use)
}

func TestFlagResolution_NegationOverridesEnv(t *testing.T) {
	t.Parallel()
	_, rs, vals := newResolutionFixture(t, "--no-yolo")
	require.NoError(t, rs.apply(EnvMap{"REMUDA_YOLO": "true"}, nil))
	require.False(t, vals.Yolo)
	require.True(t, rs.flagExplicit("yolo"))
}

func TestFlagResolution_EnumRejectsInvalidResolvedValue(t *testing.T) {
	t.Parallel()
	_, rs, _ := newResolutionFixture(t)
	require.NoError(t, rs.apply(EnvMap{"REMUDA_AGENT": "not-an-agent"}, nil))
	err := rs.validateEnums()
	require.ErrorContains(t, err, "--agent must be one of")
	require.ErrorContains(t, err, "not-an-agent")
}

func TestFlagResolution_TracksResolvedSource(t *testing.T) {
	t.Parallel()

	t.Run("flag", func(t *testing.T) {
		t.Parallel()
		_, rs, _ := newResolutionFixture(t, "--experiments", "not-real")
		require.NoError(t, rs.apply(EnvMap{"REMUDA_EXPERIMENTS": "ignored"}, nil))
		require.Equal(t, "--experiments", rs.source("experiments"))
	})

	t.Run("environment", func(t *testing.T) {
		t.Parallel()
		_, rs, _ := newResolutionFixture(t)
		require.NoError(t, rs.apply(EnvMap{"REMUDA_EXPERIMENTS": "not-real"}, nil))
		require.Equal(t, "REMUDA_EXPERIMENTS", rs.source("experiments"))
	})

	t.Run("config", func(t *testing.T) {
		t.Parallel()
		cfg := parseTestConfig(t, `
version: 1
defaults:
  experiments: [not-real]
`)
		_, rs, _ := newResolutionFixture(t)
		eff, err := newEffectiveConfig(cfg, "", profileRef{})
		require.NoError(t, err)
		require.NoError(t, rs.apply(EnvMap{}, eff))
		require.Equal(t, "defaults.experiments", rs.source("experiments"))
	})
}
