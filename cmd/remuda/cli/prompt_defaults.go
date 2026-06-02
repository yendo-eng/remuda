package cli

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func promptNamesFromDefaults(names []string) ([]PromptName, error) {
	if len(names) == 0 {
		return nil, nil
	}
	prompts := make([]PromptName, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		var prompt PromptName
		if err := prompt.UnmarshalText([]byte(trimmed)); err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	if len(prompts) == 0 {
		return nil, nil
	}
	return prompts, nil
}

func mergePromptNames(defaults []PromptName, existing []PromptName) []PromptName {
	if len(defaults) == 0 {
		if len(existing) == 0 {
			return nil
		}
		return append([]PromptName(nil), existing...)
	}
	merged := make([]PromptName, 0, len(defaults)+len(existing))
	seen := make(map[string]struct{}, len(defaults)+len(existing))
	for _, prompt := range defaults {
		key := prompt.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, prompt)
	}
	for _, prompt := range existing {
		key := prompt.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, prompt)
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func applyUsePromptDefaults(opts *ContextEngineeringOptions, kctx *kong.Context, cfg *configfile.V1, env EnvProvider) error {
	if opts == nil || cfg == nil || cfg.Defaults == nil || cfg.Defaults.UsePrompts == nil {
		return nil
	}
	env = envOrDefault(env)
	if envSet(env, "REMUDA_USE_PROMPTS") {
		return nil
	}
	if !flagExplicit(kctx, "use") {
		return nil
	}
	useDefaults, err := promptNamesFromDefaults(*cfg.Defaults.UsePrompts)
	if err != nil {
		return err
	}
	opts.Use = mergePromptNames(useDefaults, opts.Use)
	return nil
}
