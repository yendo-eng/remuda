package internal

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

type SessionResumeCommand struct {
	Workspace string
	// Agent selects agent-specific resume command behavior. Defaults to codex.
	Agent string

	Detached bool
	Attach   bool
	Yolo     bool
	// ReasoningLevel overrides Codex reasoning effort when set.
	ReasoningLevel string

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
		return errors.New("workspace path is required")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return errors.Wrap(err, "failed to expand workspace path")
	}

	if err := validateWorkspacePath(k.Config.ReposBaseDir, workspaceAbs); err != nil {
		return errors.Wrapf(err, "invalid workspace %q", workspaceAbs)
	}
	if err := k.ensureWorkspaceInactive(workspaceAbs); err != nil {
		return err
	}

	sessionName := session.SessionNameFromWorkspaceName(workspaceAbs)
	agentName := normalizeSessionResumeAgent(cmd.Agent)

	agentCmd := sessionResumeCommandForAgent(agentName, cmd.Yolo, cmd.ReasoningLevel)
	_, err = k.launchAgentSession(agentLaunchCommand{
		Workspace:           workspaceAbs,
		SessionName:         sessionName,
		AgentName:           agentName,
		Command:             agentCmd,
		Detached:            cmd.Detached,
		Attach:              cmd.Attach,
		Container:           cmd.Container,
		ContainerImage:      cmd.ContainerName,
		ContainerOpts:       cmd.ContainerOpts,
		ContainerInheritEnv: cmd.ContainerInheritEnv,
		Yolo:                cmd.Yolo,
		EnvOverrides:        cmd.EnvOverrides,
	})
	return err
}

func (k Remuda) SessionResumeCodex(ctx context.Context, cmd SessionResumeCommand) error {
	if strings.TrimSpace(cmd.Agent) == "" {
		cmd.Agent = "codex"
	}
	return k.SessionResume(ctx, cmd)
}

func sessionResumeCommandForAgent(agent string, yolo bool, reasoningLevel string) string {
	switch normalizeSessionResumeAgent(agent) {
	case "claude":
		return claudeResumeCommand(yolo, reasoningLevel)
	default:
		return codexResumeCommand(yolo, reasoningLevel)
	}
}

func normalizeSessionResumeAgent(agent string) string {
	if strings.EqualFold(strings.TrimSpace(agent), "claude") {
		return "claude"
	}
	return "codex"
}

func codexResumeCommand(yolo bool, reasoningLevel string) string {
	command := "codex resume --last"
	if yolo {
		command += " --dangerously-bypass-approvals-and-sandbox --config shell_environment_policy.ignore_default_excludes=\"true\""
	}
	reasoningLevel = strings.TrimSpace(reasoningLevel)
	if reasoningLevel != "" {
		command += " --config model_reasoning_effort="
		command += shellutil.SingleQuote(reasoningLevel)
	}
	return command
}

func claudeResumeCommand(yolo bool, reasoningLevel string) string {
	command := "claude --continue"
	if yolo {
		command += " --dangerously-skip-permissions"
	}
	reasoningLevel = strings.TrimSpace(reasoningLevel)
	if reasoningLevel != "" {
		command += " --effort " + shellutil.SingleQuote(reasoningLevel)
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
			return errors.Errorf("workspace %q is active (session %q); refuse to resume", targetAbs, s.Name)
		}
	}
	return nil
}
