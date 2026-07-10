package cli

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// flagSet decorates a pflag.FlagSet with declarative bindings that connect
// flags to environment variables and config-file keys. All precedence
// resolution (flag > env > config > built-in default) happens in one place:
// flagResolution.apply.
type flagSet struct {
	fs        *pflag.FlagSet
	bindings  map[string]*flagBinding
	negations map[string]string // "no-yolo" -> "yolo"
}

type flagBinding struct {
	envs []string // checked in order; first non-empty value wins
	key  string   // config key in the effective config, e.g. "defaults.agent"
	enum []string // when set, the resolved value must be one of these

	// mergeConfigSlice preserves the historic --use semantics: when the flag
	// is set explicitly AND no env var is set, config values are prepended
	// (deduplicated) rather than ignored.
	mergeConfigSlice bool
}

type bindOpt func(*flagBinding)

func bindEnvs(envs ...string) bindOpt {
	return func(b *flagBinding) { b.envs = envs }
}

func bindKey(key string) bindOpt {
	return func(b *flagBinding) { b.key = key }
}

func bindEnum(values ...string) bindOpt {
	return func(b *flagBinding) { b.enum = values }
}

func bindMergeConfigSlice() bindOpt {
	return func(b *flagBinding) { b.mergeConfigSlice = true }
}

func newFlagSet(fs *pflag.FlagSet) *flagSet {
	return &flagSet{
		fs:        fs,
		bindings:  map[string]*flagBinding{},
		negations: map[string]string{},
	}
}

// bind attaches env/config/enum metadata to an already-registered flag.
func (f *flagSet) bind(name string, opts ...bindOpt) {
	if f.fs.Lookup(name) == nil {
		panic(fmt.Sprintf("bind: unknown flag %q", name))
	}
	b := &flagBinding{}
	for _, opt := range opts {
		opt(b)
	}
	f.bindings[name] = b
}

// negatable registers a hidden --no-<name> flag that inverts the boolean
// flag <name>.
func (f *flagSet) negatable(name string) {
	base := f.fs.Lookup(name)
	if base == nil {
		panic(fmt.Sprintf("negatable: unknown flag %q", name))
	}
	neg := "no-" + name
	f.fs.Bool(neg, false, "Disable --"+name+".")
	f.fs.Lookup(neg).Hidden = true
	f.negations[neg] = name
}

// flagResolution tracks which flags the user set on the command line and
// applies env/config values to the rest. It is created once per invocation,
// after parsing, and may apply more than once as overlay context (repo slug,
// profile) is discovered.
type flagResolution struct {
	sets     []*flagSet
	explicit map[string]bool
	// explicitSlices holds the user-provided values of mergeConfigSlice
	// flags, so repeated resolution passes merge against the original flag
	// values rather than compounding earlier merges.
	explicitSlices map[string][]string
}

// beginResolution snapshots explicitly-set flags and reconciles hidden
// --no-<flag> negations into their target flags.
func beginResolution(sets ...*flagSet) (*flagResolution, error) {
	r := &flagResolution{sets: sets, explicit: map[string]bool{}, explicitSlices: map[string][]string{}}
	for _, set := range sets {
		set.fs.Visit(func(fl *pflag.Flag) {
			r.explicit[fl.Name] = true
		})
		for neg, target := range set.negations {
			if !r.explicit[neg] {
				continue
			}
			negVal, err := set.fs.GetBool(neg)
			if err != nil {
				return nil, pkgerrors.Wrapf(err, "read --%s", neg)
			}
			if err := set.fs.Lookup(target).Value.Set(fmt.Sprint(!negVal)); err != nil {
				return nil, pkgerrors.Wrapf(err, "apply --%s", neg)
			}
			r.explicit[target] = true
		}
		for name, b := range set.bindings {
			if !b.mergeConfigSlice || !r.explicit[name] {
				continue
			}
			sv, ok := set.fs.Lookup(name).Value.(pflag.SliceValue)
			if !ok {
				return nil, pkgerrors.Errorf("flag --%s: mergeConfigSlice requires a slice flag", name)
			}
			r.explicitSlices[name] = append([]string(nil), sv.GetSlice()...)
		}
	}
	return r, nil
}

func (r *flagResolution) flagExplicit(name string) bool {
	return r != nil && r.explicit[name]
}

// apply resolves every bound flag that was not set explicitly:
// env (first non-empty) wins over config, config wins over the built-in
// default. Safe to call repeatedly with progressively richer configs.
func (r *flagResolution) apply(env EnvProvider, cfg *koanf.Koanf) error {
	env = envOrDefault(env)
	for _, set := range r.sets {
		for name, b := range set.bindings {
			fl := set.fs.Lookup(name)
			if err := r.applyOne(fl, b, env, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *flagResolution) applyOne(fl *pflag.Flag, b *flagBinding, env EnvProvider, cfg *koanf.Koanf) error {
	envValue, envIsSet := "", false
	for _, key := range b.envs {
		if val, ok := env.LookupEnv(key); ok && val != "" {
			envValue, envIsSet = val, true
			break
		}
	}

	if r.explicit[fl.Name] {
		if !b.mergeConfigSlice || envIsSet || cfg == nil || b.key == "" || !cfg.Exists(b.key) {
			return nil
		}
		merged := mergeUnique(cfg.Strings(b.key), r.explicitSlices[fl.Name])
		return fl.Value.(pflag.SliceValue).Replace(merged)
	}

	if envIsSet {
		return setFlagFromString(fl, envValue)
	}

	if cfg != nil && b.key != "" && cfg.Exists(b.key) {
		return setFlagFromConfig(fl, cfg.Get(b.key))
	}

	return nil
}

// validateEnums checks resolved values against their allowed enum lists.
func (r *flagResolution) validateEnums() error {
	for _, set := range r.sets {
		for name, b := range set.bindings {
			if len(b.enum) == 0 {
				continue
			}
			value := set.fs.Lookup(name).Value.String()
			found := false
			for _, allowed := range b.enum {
				if value == allowed {
					found = true
					break
				}
			}
			if !found {
				return pkgerrors.Errorf("--%s must be one of %s but got %q", name, strings.Join(b.enum, ", "), value)
			}
		}
	}
	return nil
}

// setFlagFromString applies an env-provided string to a flag. Slice flags
// split the value on commas.
func setFlagFromString(fl *pflag.Flag, value string) error {
	if sv, ok := fl.Value.(pflag.SliceValue); ok {
		return sv.Replace(splitCSV(value))
	}
	return fl.Value.Set(value)
}

// setFlagFromConfig applies a config value to a flag. Lists arrive as []any
// from YAML parsing or []string from overlay merging (koanf.Set).
func setFlagFromConfig(fl *pflag.Flag, value any) error {
	var items []string
	switch list := value.(type) {
	case []any:
		items = make([]string, 0, len(list))
		for _, item := range list {
			items = append(items, fmt.Sprint(item))
		}
	case []string:
		items = list
	default:
		return fl.Value.Set(fmt.Sprint(value))
	}

	if sv, ok := fl.Value.(pflag.SliceValue); ok {
		return sv.Replace(items)
	}
	// Scalar flag fed from a config list (e.g. --experiments).
	return fl.Value.Set(strings.Join(items, ","))
}

func mergeUnique(first, second []string) []string {
	merged := make([]string, 0, len(first)+len(second))
	seen := make(map[string]struct{}, len(first)+len(second))
	for _, list := range [][]string{first, second} {
		for _, item := range list {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			merged = append(merged, item)
		}
	}
	return merged
}
