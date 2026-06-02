package internal

import (
	"strings"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
)

func resolveReasoningLevel(logger zerolog.Logger, agentName, model, agentCmd, level string) (string, error) {
	level = strings.TrimSpace(level)
	if level == "" {
		return "", nil
	}
	if strings.TrimSpace(agentCmd) != "" {
		return "", nil
	}

	switch agentlauncher.SupportedAgent(agentName) {
	case agentlauncher.AgentCodex:
		if err := agentlauncher.ValidateReasoningLevel(agentName, model, level); err != nil {
			return "", err
		}
		return level, nil
	case agentlauncher.AgentClaude:
		return level, nil
	case agentlauncher.AgentOpenCode:
		logger.Warn().
			Str("agent", agentName).
			Str("model", model).
			Str("reasoning_level", level).
			Msg("reasoning-level ignored: opencode does not support reasoning variants")
		return "", nil
	case agentlauncher.AgentBash:
		return "", nil
	default:
		return "", nil
	}
}
