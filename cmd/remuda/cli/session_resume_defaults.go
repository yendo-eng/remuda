package cli

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func applyDefaultsToSessionResume(cmd *SessionResumeCmd, kctx *kong.Context, cfg *configfile.V1, env EnvProvider) error {
	if cmd == nil || cfg == nil || cfg.Defaults == nil {
		return nil
	}

	env = envOrDefault(env)
	defaults := cfg.Defaults

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

func resolveSessionResumeReasoningLevel(cfg *configfile.V1, env EnvProvider) string {
	env = envOrDefault(env)
	if val, ok := env.LookupEnv("REMUDA_REASONING_LEVEL"); ok {
		trimmed := strings.TrimSpace(val)
		if trimmed != "" {
			return trimmed
		}
	}
	if cfg == nil || cfg.Defaults == nil || cfg.Defaults.ReasoningLevel == nil {
		return ""
	}
	return strings.TrimSpace(*cfg.Defaults.ReasoningLevel)
}

func resolveSessionResumeAgent(cfg *configfile.V1, env EnvProvider) string {
	env = envOrDefault(env)
	if val, ok := env.LookupEnv("REMUDA_AGENT"); ok {
		trimmed := strings.TrimSpace(val)
		if trimmed != "" && strings.EqualFold(trimmed, "claude") {
			return "claude"
		}
		if trimmed != "" {
			return "codex"
		}
	}
	if cfg == nil || cfg.Defaults == nil || cfg.Defaults.Agent == nil {
		return "codex"
	}
	if strings.EqualFold(strings.TrimSpace(*cfg.Defaults.Agent), "claude") {
		return "claude"
	}
	return "codex"
}
