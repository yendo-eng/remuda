package cli

import (
	"strings"

	kyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/configfile"
	"gopkg.in/yaml.v3"
)

// profileRef identifies a selected profile and where it came from, so error
// messages can point at the offending config location.
type profileRef struct {
	Name string
	// PerRepoSlug is set when the profile was selected by per_repo.<slug>.profile.
	PerRepoSlug string
}

// selectProfile picks the active profile: explicit --profile flag, then
// REMUDA_PROFILE, then per_repo.<slug>.profile.
func selectProfile(flagValue string, flagIsExplicit bool, env EnvProvider, cfg *configfile.V1, slug string) profileRef {
	if flagIsExplicit {
		if trimmed := strings.TrimSpace(flagValue); trimmed != "" {
			return profileRef{Name: trimmed}
		}
	}
	if trimmed := strings.TrimSpace(envOrDefault(env).Getenv("REMUDA_PROFILE")); trimmed != "" {
		return profileRef{Name: trimmed}
	}
	slug = normalizeRepoSlug(slug)
	if cfg == nil || slug == "" {
		return profileRef{}
	}
	overlay, ok := cfg.PerRepo[slug]
	if !ok || overlay.Profile == nil {
		return profileRef{}
	}
	if trimmed := strings.TrimSpace(*overlay.Profile); trimmed != "" {
		return profileRef{Name: trimmed, PerRepoSlug: slug}
	}
	return profileRef{}
}

// newEffectiveConfig flattens the config file into a key-value view with
// overlays applied, lowest precedence first: base config, then
// per_repo.<slug>, then the selected profile. Later layers win key-by-key;
// lists replace, except per_repo container.opts which append to the base
// (historic behavior).
func newEffectiveConfig(cfg *configfile.V1, slug string, profile profileRef) (*koanf.Koanf, error) {
	k := koanf.New(".")
	if cfg == nil {
		if profile.Name != "" {
			return nil, pkgerrors.Errorf("profile %q requested but no config file found", profile.Name)
		}
		return k, nil
	}

	base := *cfg
	base.PerRepo = nil
	base.Profiles = nil
	if err := loadIntoKoanf(k, base); err != nil {
		return nil, err
	}

	slug = normalizeRepoSlug(slug)
	if slug != "" {
		if overlay, ok := cfg.PerRepo[slug]; ok {
			var appendedOpts []string
			if overlay.Defaults != nil && overlay.Defaults.Container != nil && overlay.Defaults.Container.Opts != nil &&
				len(*overlay.Defaults.Container.Opts) > 0 && k.Exists("defaults.container.opts") {
				appendedOpts = append(append([]string{}, k.Strings("defaults.container.opts")...), (*overlay.Defaults.Container.Opts)...)
			}
			err := loadIntoKoanf(k, configfile.V1{
				Repos:    overlay.Repos,
				Session:  overlay.Session,
				Defaults: overlay.Defaults,
			})
			if err != nil {
				return nil, err
			}
			if appendedOpts != nil {
				if err := k.Set("defaults.container.opts", appendedOpts); err != nil {
					return nil, err
				}
			}
		}
	}

	if profile.Name != "" {
		defaults, ok := cfg.Profiles[profile.Name]
		if !ok {
			if profile.PerRepoSlug != "" {
				return nil, pkgerrors.Errorf(
					"per_repo[%q].profile references unknown profile %q; define it under profiles in config.yaml",
					profile.PerRepoSlug, profile.Name,
				)
			}
			return nil, pkgerrors.Errorf("unknown profile %q; define it under profiles in config.yaml", profile.Name)
		}
		if err := loadIntoKoanf(k, configfile.V1{Defaults: &defaults}); err != nil {
			return nil, err
		}
	}

	return k, nil
}

// loadIntoKoanf merges a config struct into k by round-tripping through YAML.
// The round trip drops nil (unset) fields via omitempty, so only explicitly
// configured keys participate in the merge.
func loadIntoKoanf(k *koanf.Koanf, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return pkgerrors.Wrap(err, "marshal config overlay")
	}
	if err := k.Load(rawbytes.Provider(data), kyaml.Parser()); err != nil {
		return pkgerrors.Wrap(err, "merge config overlay")
	}
	return nil
}

// effectiveStrings reads a []string config value with an "is set" indicator.
func effectiveStrings(k *koanf.Koanf, key string) ([]string, bool) {
	if k == nil || !k.Exists(key) {
		return nil, false
	}
	return k.Strings(key), true
}

// effectiveAgentArgsFromKoanf returns defaults.agent_args.<agent> from the
// effective config with the CLI-provided args appended.
func effectiveAgentArgsFromKoanf(k *koanf.Koanf, agent string, cliArgs []string) []string {
	var resolved []string
	if k != nil {
		agent = strings.ToLower(strings.TrimSpace(agent))
		if args, ok := effectiveStrings(k, "defaults.agent_args."+agent); ok {
			resolved = append(resolved, args...)
		}
	}
	resolved = append(resolved, cliArgs...)
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}
