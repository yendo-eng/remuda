package agentlauncher

import (
	"fmt"
	"strings"
)

// CodexReasoningLevels is the canonical list of reasoning levels supported by Codex.
var CodexReasoningLevels = []string{
	"none",
	"minimal",
	"low",
	"medium",
	"high",
	"xhigh",
}

// ClaudeEffortLevels is the canonical list of effort levels supported by Claude.
var ClaudeEffortLevels = []string{
	"low",
	"medium",
	"high",
}

// SupportedReasoningLevels returns the supported reasoning levels for the agent/model.
func SupportedReasoningLevels(agent, model string) []string {
	switch SupportedAgent(agent) {
	case AgentCodex:
		return append([]string(nil), CodexReasoningLevels...)
	default:
		return nil
	}
}

// SuggestedReasoningLevels returns shell-completion suggestions for reasoning levels.
//
// Claude maps Remuda reasoning-level values directly to its --effort flag.
func SuggestedReasoningLevels(agent, model string) []string {
	switch SupportedAgent(agent) {
	case AgentClaude:
		return append([]string(nil), ClaudeEffortLevels...)
	default:
		return SupportedReasoningLevels(agent, model)
	}
}

// ValidateReasoningLevel returns an error if the reasoning level is invalid for the agent/model.
func ValidateReasoningLevel(agent, model, level string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		return nil
	}

	allowed := SupportedReasoningLevels(agent, model)
	if len(allowed) == 0 {
		return fmt.Errorf("reasoning-level %q is not supported for agent %q (model %q)", level, agent, model)
	}

	for _, candidate := range allowed {
		if candidate == level {
			return nil
		}
	}

	return fmt.Errorf("reasoning-level %q is invalid for agent %q (model %q). valid values: %s",
		level, agent, model, strings.Join(allowed, ", "))
}
