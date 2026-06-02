package internal

import "strings"

func remudaAgentEnvPrefix(agentName, model string) string {
	prefix := "REMUDA_AGENT=" + singleQuote(agentName)
	if strings.TrimSpace(model) == "" {
		return prefix
	}
	return prefix + " REMUDA_MODEL=" + singleQuote(model)
}
