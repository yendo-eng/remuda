package agentlauncher

import (
	"strings"

	"github.com/pkg/errors"
)

// AgentLauncher builds the concrete shell command for the coding agent.
type AgentLauncher interface {
	Name() string

	// Command builds the command string to launch the agent with the given prompt.
	Command(prompt string) string

	// WithRemoteControl enables remote-control launch behavior when supported.
	//
	// The returned bool reports whether remote control was applied by this
	// launcher. Unsupported launchers must return (self, false).
	WithRemoteControl(sessionName string) (AgentLauncher, bool)

	// List of models supported by this agent. If nil, all models are supported.
	SupportedModels() []string
}

type SupportedAgent string

const (
	AgentCodex    SupportedAgent = "codex"
	AgentClaude   SupportedAgent = "claude"
	AgentOpenCode SupportedAgent = "opencode"
	AgentBash     SupportedAgent = "bash"
	AgentDebug    SupportedAgent = "debug"
)

func SupportedAgents() []SupportedAgent {
	return []SupportedAgent{
		AgentCodex,
		AgentClaude,
		AgentOpenCode,
		AgentBash,
		AgentDebug,
	}
}

func IsSupportedAgent(agent string) bool {
	_, _, err := Parse(agent, "", false)
	return err == nil
}

var ErrUnsupportedAgent = errors.New("unsupported agent")

// ModelAgentDefault is a sentinel value meaning "do not pass an explicit model".
// When supplied, Remuda will omit any model flag so the underlying agent CLI
// uses whatever default it chooses.
const ModelAgentDefault = "agent-default"

// DefaultModel returns Remuda's explicit default model for the given agent.
//
// This is intentionally different from the underlying agent CLI's default: Remuda
// prefers to pass an explicit model for determinism, and only relies on the agent
// program's default when ModelAgentDefault is explicitly requested.
func DefaultModel(agent string) string {
	switch SupportedAgent(agent) {
	case AgentCodex:
		return "gpt-5.5"
	case AgentClaude:
		return ""
	case AgentOpenCode:
		return "openai/gpt-5" // TODO: confirm/update OpenCode default model
	default:
		return ""
	}
}

// EffectiveModel normalizes a model selection for deterministic launches.
//
// - Empty model selects the per-agent DefaultModel.
// - ModelAgentDefault is preserved so launchers can omit model flags.
func EffectiveModel(agent, model string) string {
	if model == ModelAgentDefault {
		return model
	}
	if strings.TrimSpace(model) == "" {
		return DefaultModel(agent)
	}
	return model
}

func Parse(
	agent, model string,
	yolo bool,
) (AgentLauncher, string, error) {
	return ParseWithReasoning(agent, model, "", yolo)
}

func ParseWithReasoning(
	agent, model, reasoningLevel string,
	yolo bool,
) (AgentLauncher, string, error) {
	model = EffectiveModel(agent, model)
	switch SupportedAgent(agent) {
	case AgentCodex:
		return Codex(model, yolo, reasoningLevel), model, nil
	case AgentClaude:
		return Claude(model, yolo, reasoningLevel), model, nil
	case AgentOpenCode:
		return OpenCode(model), model, nil
	case AgentBash:
		return Bash(), model, nil
	case AgentDebug:
		return Debug(), model, nil
	default:
		return nil, "", errors.Wrapf(ErrUnsupportedAgent, "agent '%s'", agent)
	}
}
