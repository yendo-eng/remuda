package internal

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
)

// isolatedSessionEnvNames is the session.AgentStart ForwardEnv of an agent's
// launch.
func isolatedSessionEnvNames(agent string, extraEnvNames, overrideEnvNames []string) []string {
	names := isolatedContainerEnvNames(agent, extraEnvNames)
	for _, name := range overrideEnvNames {
		name = strings.TrimSpace(name)
		if util.IsValidEnvVarName(name) {
			names = append(names, name)
		}
	}
	return names
}

// isolatedContainerEnvNames returns env var names that should be explicitly exported
// before running containerized commands in isolated sessions.
//
// This includes user-requested --container-inherit-env values plus implicit env
// forwards added by launch composition (currently ANTHROPIC_API_KEY for
// claude/bash) so the cleared session environment doesn't drop those values.
func isolatedContainerEnvNames(agent string, containerInheritEnv []string) []string {
	names := make([]string, 0, len(containerInheritEnv)+1)
	seen := make(map[string]struct{}, len(containerInheritEnv)+1)
	add := func(raw string) {
		name := strings.TrimSpace(raw)
		if name == "" || !util.IsValidEnvVarName(name) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, name := range containerInheritEnv {
		add(name)
	}

	if strings.EqualFold(agent, "claude") || strings.EqualFold(agent, "bash") {
		add("ANTHROPIC_API_KEY")
	}

	return names
}
