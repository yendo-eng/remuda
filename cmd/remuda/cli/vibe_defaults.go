package cli

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func applyPerRepoDefaultsToVibe(cmd *VibeCmd, kctx *kong.Context, cfg *configfile.V1, env EnvProvider) error {
	if cmd == nil || cfg == nil || cfg.Defaults == nil {
		return nil
	}

	env = envOrDefault(env)
	defaults := cfg.Defaults

	if defaults.Agent != nil && !flagExplicit(kctx, "agent") && !envSet(env, "REMUDA_AGENT") {
		cmd.Agent = *defaults.Agent
	}
	if defaults.Model != nil && !flagExplicit(kctx, "model") && !envSet(env, "REMUDA_MODEL") {
		cmd.Model = *defaults.Model
	}
	if defaults.ReasoningLevel != nil && !flagExplicit(kctx, "reasoning-level") && !envSet(env, "REMUDA_REASONING_LEVEL") {
		cmd.ReasoningLevel = *defaults.ReasoningLevel
	}
	if defaults.SlugifyReasoningLevel != nil && !flagExplicit(kctx, "slugify-reasoning-level") && !envSet(env, "REMUDA_SLUGIFY_REASONING_LEVEL") {
		cmd.SlugifyReasoningLevel = *defaults.SlugifyReasoningLevel
	}
	if defaults.AgentCmd != nil && !flagExplicit(kctx, "agent-cmd") {
		cmd.AgentCmd = *defaults.AgentCmd
	}
	if defaults.UsePrompts != nil && !envSet(env, "REMUDA_USE_PROMPTS") {
		useDefaults, err := promptNamesFromDefaults(*defaults.UsePrompts)
		if err != nil {
			return err
		}
		if flagExplicit(kctx, "use") {
			cmd.Use = mergePromptNames(useDefaults, cmd.Use)
		} else {
			cmd.Use = useDefaults
		}
	}
	if defaults.NoUse != nil && !flagExplicit(kctx, "no-use") {
		noUse := make([]PromptName, 0, len(*defaults.NoUse))
		for _, name := range *defaults.NoUse {
			var prompt PromptName
			if err := prompt.UnmarshalText([]byte(strings.TrimSpace(name))); err != nil {
				return err
			}
			noUse = append(noUse, prompt)
		}
		cmd.NoUse = noUse
	}
	if defaults.Experiments != nil && !flagExplicit(kctx, "experiments") && !envSet(env, "REMUDA_EXPERIMENTS") {
		cmd.Experiments = strings.Join(*defaults.Experiments, ",")
	}
	if defaults.Yolo != nil && !flagExplicit(kctx, "yolo") && !envSet(env, "REMUDA_YOLO") {
		cmd.Yolo = *defaults.Yolo
	}

	if defaults.Container == nil {
		return nil
	}

	if defaults.Container.Enabled != nil && !flagExplicit(kctx, "container") && !envSet(env, "REMUDA_CONTAINER") {
		cmd.Container = *defaults.Container.Enabled
	}
	if defaults.Container.Image != nil && !flagExplicit(kctx, "container-name") {
		cmd.ContainerName = *defaults.Container.Image
	}
	if defaults.Container.Opts != nil && !flagExplicit(kctx, "container-opt") && !envSet(env, "REMUDA_CONTAINER_OPTS") {
		cmd.ContainerOpt = append([]string(nil), (*defaults.Container.Opts)...)
	}
	if defaults.Container.InheritEnv != nil && !flagExplicit(kctx, "container-inherit-env") && !envSet(env, "REMUDA_CONTAINER_INHERIT_ENVS") {
		cmd.ContainerInheritEnv = append([]string(nil), (*defaults.Container.InheritEnv)...)
	}

	return nil
}
