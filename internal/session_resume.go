package internal

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/session"
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
	// BeforePrompt is prepended to Prompt in order.
	BeforePrompt []string
	// AfterPrompt is appended to Prompt in order.
	AfterPrompt []string

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
	if err := validateMultiplexerLaunch(k.Multiplexer, cmd.AgentCmd); err != nil {
		return err
	}

	workspace := strings.TrimSpace(cmd.Workspace)
	if workspace == "" {
		return pkgerrors.New("workspace path is required")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to expand workspace path")
	}

	if err := ValidateWorkspacePath(k.Config.ReposBaseDir, workspaceAbs); err != nil {
		return pkgerrors.Wrapf(err, "invalid workspace %q", workspaceAbs)
	}
	if err := k.ensureWorkspaceInactive(workspaceAbs); err != nil {
		return err
	}

	sessionName := session.SessionNameFromWorkspaceName(workspaceAbs)
	agentName := normalizeSessionResumeAgent(cmd.Agent)
	model := strings.TrimSpace(cmd.Model)

	agentCmd := strings.TrimSpace(cmd.AgentCmd)
	var agent agentlauncher.AgentLauncher
	var agentArgs []string
	prompt := assemblePrompt(cmd.BeforePrompt, cmd.Prompt, cmd.AfterPrompt)
	if agentCmd == "" {
		extraArgs := sessionAgentArgs(k.Multiplexer, cmd.Detached, agentName)
		agent, err = agentlauncher.Resume(agentName, model, cmd.ReasoningLevel, cmd.Yolo)
		if err != nil {
			return err
		}
		agentCmd = agent.Command(prompt, extraArgs...)
		agentArgs = agent.Arguments("", extraArgs...)
	} else {
		custom := agentlauncher.Custom(agentCmd)
		agentCmd = custom.Command(prompt)
		// Only the resume command is replaced by --agent-cmd, so a built-in
		// agent's container env and mounts still apply.
		agent, _, err = agentlauncher.Parse(agentName, model, cmd.Yolo)
		switch {
		case errors.Is(err, agentlauncher.ErrUnsupportedAgent):
			agent = custom
		case err != nil:
			return err
		}
	}

	_, err = k.launchAgentSession(agentLaunchCommand{
		Workspace:           workspaceAbs,
		SessionName:         sessionName,
		Agent:               agent,
		Model:               model,
		Command:             agentCmd,
		Args:                agentArgs,
		Prompt:              prompt,
		Detached:            cmd.Detached,
		Attach:              cmd.Attach,
		Container:           cmd.Container,
		ContainerImage:      cmd.ContainerName,
		ContainerOpts:       cmd.ContainerOpts,
		ContainerInheritEnv: cmd.ContainerInheritEnv,
		EnvOverrides:        cmd.EnvOverrides,
	})
	return err
}

func normalizeSessionResumeAgent(agent string) string {
	trimmed := strings.TrimSpace(strings.ToLower(agent))
	if trimmed == "" {
		return "codex"
	}
	return trimmed
}

func (k Remuda) ensureWorkspaceInactive(workspaceAbs string) error {
	active, err := k.activeWorkspaceSessions()
	if err != nil {
		return err
	}

	if sessionName, ok := active[workspaceAbs]; ok {
		return pkgerrors.Errorf("workspace %q is active (session %q); refuse to resume", workspaceAbs, sessionName)
	}
	return nil
}
