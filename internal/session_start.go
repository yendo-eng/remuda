package internal

import (
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func startSessionWithEnv(manager session.SessionManager, sessionName, command string, provider env.Provider, agent string, extraEnvNames []string) error {
	envValues := launchEnvValues(provider)
	if manager.Name() == string(session.SessionManagerTmux) {
		envValues = tmuxSessionEnvValues(provider, agent, extraEnvNames)
	}
	if starter, ok := manager.(session.EnvStarter); ok {
		return starter.StartWithEnv(sessionName, command, envValues)
	}
	return manager.Start(sessionName, command)
}
