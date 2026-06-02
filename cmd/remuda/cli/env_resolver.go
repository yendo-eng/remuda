package cli

import "github.com/alecthomas/kong"

// EnvResolver provides default flag values from an EnvProvider.
// It mirrors the config resolver's "non-empty" env semantics.
type EnvResolver struct {
	env EnvProvider
}

func NewEnvResolver(env EnvProvider) *EnvResolver {
	return &EnvResolver{env: envOrDefault(env)}
}

func (r *EnvResolver) Validate(_ *kong.Application) error {
	return nil
}

func (r *EnvResolver) Resolve(_ *kong.Context, _ *kong.Path, flag *kong.Flag) (any, error) {
	for _, name := range flag.Envs {
		if val, ok := r.env.LookupEnv(name); ok && val != "" {
			return val, nil
		}
	}
	return nil, nil
}

func isProcessEnvProvider(env EnvProvider) bool {
	_, ok := env.(osEnvProvider)
	return ok
}
