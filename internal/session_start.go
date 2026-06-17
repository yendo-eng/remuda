package internal

import (
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func startSessionWithEnv(manager session.SessionManager, sessionName, command string, provider env.Provider) error {
	envValues := launchEnvValues(provider)
	if starter, ok := manager.(session.EnvStarter); ok {
		return starter.StartWithEnv(sessionName, command, envValues)
	}
	return manager.Start(sessionName, command)
}
