package internal

import (
	"context"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/logging"
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
	var agentArgs []string
	prompt := assemblePrompt(cmd.BeforePrompt, cmd.Prompt, cmd.AfterPrompt)
	if agentCmd == "" {
		launcher, err := agentlauncher.Resume(agentName, model, cmd.ReasoningLevel, cmd.Yolo)
		if err != nil {
			return err
		}
		agentCmd = launcher.Command(prompt)
		agentArgs = launcher.Arguments("")
	} else {
		agentCmd = agentlauncher.Custom(agentCmd).Command(prompt)
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
		Args:                agentArgs,
		Prompt:              prompt,
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
