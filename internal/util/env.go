package util

import "github.com/yendo-eng/remuda/internal/env"

func EnvOr(provider env.Provider, key, defaultValue string) string {
	provider = env.OrDefault(provider)
	if value := provider.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
