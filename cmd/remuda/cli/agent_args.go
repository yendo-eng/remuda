package cli

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/configfile"
)

func effectiveAgentArgs(cfg *configfile.V1, agent string, cliArgs []string) []string {
	resolved := agentArgsFromDefaults(cfg, agent)
	resolved = append(resolved, cliArgs...)
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func agentArgsFromDefaults(cfg *configfile.V1, agent string) []string {
	if cfg == nil || cfg.Defaults == nil || cfg.Defaults.AgentArgs == nil {
		return nil
	}

	agent = strings.ToLower(strings.TrimSpace(agent))
	args, ok := cfg.Defaults.AgentArgs[agent]
	if !ok || len(args) == 0 {
		return nil
	}

	return append([]string(nil), args...)
}
