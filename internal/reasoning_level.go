package internal

import (
	"strings"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
)

func resolveReasoningLevel(logger zerolog.Logger, agentName, model, agentCmd, level string) string {
	level = strings.TrimSpace(level)
	if level == "" {
		return ""
	}
	if strings.TrimSpace(agentCmd) != "" {
		return ""
	}

	switch agentlauncher.SupportedAgent(agentName) {
	case agentlauncher.AgentCodex:
		if err := agentlauncher.ValidateReasoningLevel(agentName, model, level); err != nil {
			logger.Warn().
				Str("agent", agentName).
				Str("model", model).
				Str("reasoning_level", level).
				Msg("reasoning level may not be supported by codex model")
		}
		return level
	case agentlauncher.AgentClaude:
		return level
	case agentlauncher.AgentOpenCode:
		logger.Warn().
			Str("agent", agentName).
			Str("model", model).
			Str("reasoning_level", level).
			Msg("reasoning-level ignored: opencode does not support reasoning variants")
		return ""
	case agentlauncher.AgentBash:
		return ""
	default:
		return ""
	}
}
