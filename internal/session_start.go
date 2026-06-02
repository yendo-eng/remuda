package internal

import (
	"os"
	"strings"

	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func startSessionWithEnv(manager session.SessionManager, sessionName, command string, provider env.Provider) error {
	envValues := env.Environ(provider)
	if !envHasKey(envValues, "PATH") {
		if path := strings.TrimSpace(os.Getenv("PATH")); path != "" {
			envValues = append(envValues, "PATH="+path)
		}
	}
	if starter, ok := manager.(session.EnvStarter); ok {
		return starter.StartWithEnv(sessionName, command, envValues)
	}
	return manager.Start(sessionName, command)
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
