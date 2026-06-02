package internal

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
)

// tmuxContainerEnvNames returns env var names that should be explicitly exported
// before running containerized commands in detached tmux sessions.
//
// This includes user-requested --container-inherit-env values plus implicit env
// forwards added by launch composition (currently ANTHROPIC_API_KEY for
// claude/bash) so stale tmux server environments don't drop those values.
func tmuxContainerEnvNames(agent string, containerInheritEnv []string) []string {
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
