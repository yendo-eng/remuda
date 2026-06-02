package internal

import "github.com/yendo-eng/remuda/internal/env"

func (k Remuda) envProvider() env.Provider {
	if k.Env == nil {
		return env.NewMutableProvider(nil)
	}
	return env.NewMutableProvider(k.Env)
}
