package internal

import (
	"context"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

type SessionResumeCommand struct {
	Workspace string
	// Agent selects agent-specific resume command behavior. Defaults to codex.
	Agent string
	// Model overrides the resume model when supported.
	Model string
	// AgentCmd overrides the built-in resume command entirely.
	AgentCmd string
	// Prompt is injected into the resumed conversation when provided.
	Prompt string

	Detached bool
	Attach   bool
	Yolo     bool
	// ReasoningLevel overrides Codex reasoning effort when set.
	ReasoningLevel string
	// OpenAIAPIKey overrides OPENAI_API_KEY for this launch.
	OpenAIAPIKey string

	Container           bool
	ContainerName       string
	ContainerOpts       []string
	ContainerInheritEnv []string

	// Environment values to apply to the launched agent without embedding them
	// in the shell command string.
	EnvOverrides map[string]string
}

func (k Remuda) SessionResume(ctx context.Context, cmd SessionResumeCommand) error {
	k.SetLogger(logging.FromContext(ctx))

	workspace := strings.TrimSpace(cmd.Workspace)
	if workspace == "" {
		return pkgerrors.New("workspace path is required")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to expand workspace path")
	}

	if err := validateWorkspacePath(k.Config.ReposBaseDir, workspaceAbs); err != nil {
		return pkgerrors.Wrapf(err, "invalid workspace %q", workspaceAbs)
	}
	if err := k.ensureWorkspaceInactive(workspaceAbs); err != nil {
		return err
	}

	sessionName := session.SessionNameFromWorkspaceName(workspaceAbs)
	agentName := normalizeSessionResumeAgent(cmd.Agent)
	model := strings.TrimSpace(cmd.Model)

	agentCmd := strings.TrimSpace(cmd.AgentCmd)
	if agentCmd == "" {
		var err error
		agentCmd, err = sessionResumeCommandForAgent(agentName, model, cmd.Yolo, cmd.ReasoningLevel, cmd.Prompt)
		if err != nil {
			return err
		}
	} else {
		agentCmd = agentlauncher.Custom(agentCmd).Command(cmd.Prompt)
	}

	envOverrides := make(map[string]string, len(cmd.EnvOverrides)+1)
	for key, value := range cmd.EnvOverrides {
		envOverrides[key] = value
	}
	if openAIAPIKey := strings.TrimSpace(cmd.OpenAIAPIKey); openAIAPIKey != "" {
		envOverrides["OPENAI_API_KEY"] = openAIAPIKey
	}
	if len(envOverrides) == 0 {
		envOverrides = nil
	}

	_, err = k.launchAgentSession(agentLaunchCommand{
		Workspace:           workspaceAbs,
		SessionName:         sessionName,
		AgentName:           agentName,
		Model:               model,
		Command:             agentCmd,
		Detached:            cmd.Detached,
		Attach:              cmd.Attach,
		Container:           cmd.Container,
		ContainerImage:      cmd.ContainerName,
		ContainerOpts:       cmd.ContainerOpts,
		ContainerInheritEnv: cmd.ContainerInheritEnv,
		Yolo:                cmd.Yolo,
		EnvOverrides:        envOverrides,
	})
	return err
}

func sessionResumeCommandForAgent(agent, model string, yolo bool, reasoningLevel, prompt string) (string, error) {
	switch normalizeSessionResumeAgent(agent) {
	case "claude":
		return claudeResumeCommand(model, yolo, reasoningLevel, prompt), nil
	case "codex":
		return codexResumeCommand(model, yolo, reasoningLevel, prompt), nil
	case "opencode", "bash":
		return "", pkgerrors.Errorf("session resume unsupported for agent %q", normalizeSessionResumeAgent(agent))
	default:
		return "", pkgerrors.Errorf("session resume unsupported for agent %q", normalizeSessionResumeAgent(agent))
	}
}

func normalizeSessionResumeAgent(agent string) string {
	trimmed := strings.TrimSpace(strings.ToLower(agent))
	switch trimmed {
	case "":
		return "codex"
	case "codex", "claude", "opencode", "bash":
		return trimmed
	default:
		return trimmed
	}
}

func codexResumeCommand(model string, yolo bool, reasoningLevel, prompt string) string {
	command := "codex resume --last"
	model = strings.TrimSpace(model)
	if model != "" && model != agentlauncher.ModelAgentDefault {
		command += " --model " + shellutil.SingleQuote(model)
	}
	if yolo {
		command += " --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust --config shell_environment_policy.ignore_default_excludes=\"true\""
	}
	reasoningLevel = strings.TrimSpace(reasoningLevel)
	if reasoningLevel != "" {
		command += " --config model_reasoning_effort="
		command += shellutil.SingleQuote(reasoningLevel)
	}
	prompt = strings.TrimSpace(prompt)
	if prompt != "" {
		command += " -- '"
		command += shellutil.EscapeSingleQuotes(prompt)
		command += "'"
	}
	return command
}

func claudeResumeCommand(model string, yolo bool, reasoningLevel, prompt string) string {
	command := "claude --continue"
	model = strings.TrimSpace(model)
	if model != "" && model != agentlauncher.ModelAgentDefault {
		command += " --model " + shellutil.SingleQuote(model)
	}
	if yolo {
		command += " --dangerously-skip-permissions"
	}
	reasoningLevel = strings.TrimSpace(reasoningLevel)
	if reasoningLevel != "" {
		command += " --effort " + shellutil.SingleQuote(reasoningLevel)
	}
	prompt = strings.TrimSpace(prompt)
	if prompt != "" {
		command += " '"
		command += shellutil.EscapeSingleQuotes(prompt)
		command += "'"
	}
	return command
}

func (k Remuda) ensureWorkspaceInactive(workspaceAbs string) error {
	sessions, err := k.Session.List()
	if err != nil {
		return err
	}

	targetAbs, err := filepath.Abs(workspaceAbs)
	if err != nil {
		targetAbs = workspaceAbs
	}

	for _, s := range sessions {
		if !s.IsRemudaSession() {
			continue
		}
		ws, err := s.WorkspacePath(k.Config.ReposBaseDir)
		if err != nil {
			continue
		}
		wsAbs, err := filepath.Abs(ws)
		if err != nil {
			wsAbs = ws
		}
		if wsAbs == targetAbs {
			return pkgerrors.Errorf("workspace %q is active (session %q); refuse to resume", targetAbs, s.Name)
		}
	}
	return nil
}
