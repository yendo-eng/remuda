package cli

import (
	"fmt"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/configfile"
)

const (
	experimentUsePromptsContextWrapper = "use-prompts-context-wrapper"
)

type experiment struct {
	Name        string
	Description string
}

func experimentsRegistry() []experiment {
	return []experiment{
		{
			Name:        experimentUsePromptsContextWrapper,
			Description: "wrap saved prompt context before injecting it into the agent prompt",
		},
	}
}

func retiredExperimentsRegistry() map[string]string {
	return map[string]string{
		"auto-workspace-name": "was mainlined and is now a no-op; remove it",
	}
}

func experimentCompletionValues() []string {
	experiments := experimentsRegistry()
	out := make([]string, 0, len(experiments))
	for _, exp := range experiments {
		out = append(out, exp.Name+"\t"+exp.Description)
	}
	return out
}

func validateExperiments(raw string, source string) ([]string, error) {
	names := splitFlexibleList(raw)
	if len(names) == 0 {
		return nil, nil
	}

	valid := make(map[string]struct{}, len(experimentsRegistry()))
	validNames := make([]string, 0, len(experimentsRegistry()))
	for _, exp := range experimentsRegistry() {
		name := strings.ToLower(strings.TrimSpace(exp.Name))
		valid[name] = struct{}{}
		validNames = append(validNames, exp.Name)
	}

	retired := retiredExperimentsRegistry()
	var retiredNames []string
	seenRetired := map[string]struct{}{}
	for _, name := range names {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if _, ok := valid[normalized]; ok {
			continue
		}
		if _, ok := retired[normalized]; ok {
			if _, seen := seenRetired[normalized]; !seen {
				retiredNames = append(retiredNames, normalized)
				seenRetired[normalized] = struct{}{}
			}
			continue
		}
		return retiredNames, pkgerrors.Errorf("%s: unknown experiment %q (valid: %s)", source, name, strings.Join(validNames, ", "))
	}
	return retiredNames, nil
}

func validateConfigExperiments(cfg *configfile.V1, warnRetired func(string)) error {
	if cfg == nil {
		return nil
	}
	if cfg.Defaults != nil {
		if err := validateConfigExperimentList("defaults.experiments", cfg.Defaults.Experiments, warnRetired); err != nil {
			return err
		}
	}
	for name, defaults := range cfg.Profiles {
		if err := validateConfigExperimentList(fmt.Sprintf("profiles[%q].experiments", name), defaults.Experiments, warnRetired); err != nil {
			return err
		}
	}
	for slug, overlay := range cfg.PerRepo {
		if overlay.Defaults == nil {
			continue
		}
		if err := validateConfigExperimentList(fmt.Sprintf("per_repo[%q].defaults.experiments", slug), overlay.Defaults.Experiments, warnRetired); err != nil {
			return err
		}
	}
	return nil
}

func validateConfigExperimentList(source string, experiments *[]string, warnRetired func(string)) error {
	if experiments == nil {
		return nil
	}
	retired, err := validateExperiments(strings.Join(*experiments, ","), source)
	if err != nil {
		return err
	}
	for _, name := range retired {
		if warnRetired != nil {
			warnRetired(name)
		}
	}
	return nil
}

func experimentConfigSource(cfg *configfile.V1, slug string, profile profileRef) string {
	source := "defaults.experiments"
	if cfg == nil {
		return source
	}
	if cfg.Defaults != nil && cfg.Defaults.Experiments != nil {
		source = "defaults.experiments"
	}
	if slug != "" {
		if overlay, ok := cfg.PerRepo[slug]; ok && overlay.Defaults != nil && overlay.Defaults.Experiments != nil {
			source = fmt.Sprintf("per_repo[%q].defaults.experiments", slug)
		}
	}
	if profile.Name != "" {
		if defaults, ok := cfg.Profiles[profile.Name]; ok && defaults.Experiments != nil {
			source = fmt.Sprintf("profiles[%q].experiments", profile.Name)
		}
	}
	return source
}
