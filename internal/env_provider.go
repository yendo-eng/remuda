package internal

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/env"
)

func (k Remuda) envProvider() env.Provider {
	if k.Env == nil {
		return env.NewMutableProvider(nil)
	}
	return env.NewMutableProvider(k.Env)
}

func envHasKey(envValues []string, key string) bool {
	prefix := key + "="
	for _, kv := range envValues {
		if strings.HasPrefix(kv, prefix) {
			return true
		}
	}
	return false
}
