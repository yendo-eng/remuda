package internal

import (
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func startSessionWithEnv(manager session.Multiplexer, sessionName, workspace, command string, commandArgv []string, container bool, agent string, agentArgs []string, prompt string, provider env.Provider, extraEnvNames, overrideEnvNames []string) error {
	envValues := launchEnvValues(provider)
	if manager.Name() == string(session.MultiplexerTmux) {
		envValues = tmuxSessionEnvValues(provider, agent, extraEnvNames, overrideEnvNames)
	}
	if starter, ok := manager.(session.AgentStarter); ok {
		return starter.StartAgent(session.AgentStart{
			SessionName: sessionName,
			Workspace:   workspace,
			Command:     command,
			CommandArgv: commandArgv,
			Container:   container,
			Agent:       agent,
			Args:        agentArgs,
			Prompt:      prompt,
			Env:         envValues,
		})
	}
	if starter, ok := manager.(session.EnvStarter); ok {
		return starter.StartWithEnv(sessionName, command, envValues)
	}
	return manager.Start(sessionName, command)
}
