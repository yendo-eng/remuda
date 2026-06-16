package internal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util"
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
}

func (k Remuda) SessionResume(ctx context.Context, cmd SessionResumeCommand) error {
	k.SetLogger(logging.FromContext(ctx))

	envProvider := k.envProvider()
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
	containerName := docker.ContainerNameFromSession(sessionName)
	agentName := normalizeSessionResumeAgent(cmd.Agent)

	agentCmd := sessionResumeCommandForAgent(agentName, cmd.Yolo, cmd.ReasoningLevel)
	launchCmd, _, err := k.composeLaunchCommand(
		VibeCommand{
			Agent:               agentName,
			Detached:            cmd.Detached,
			Attach:              cmd.Attach,
			Yolo:                cmd.Yolo,
			Container:           cmd.Container,
			ContainerName:       cmd.ContainerName,
			ContainerOpts:       cmd.ContainerOpts,
			ContainerInheritEnv: cmd.ContainerInheritEnv,
		},
		workspaceAbs,
		agentCmd,
		sessionName,
		containerName,
		envProvider,
	)
	if err != nil {
		return err
	}

	if !cmd.Detached {
		envPrefix := remudaAgentEnvPrefix(agentName, "")
		execCmd := util.CmdWithLogger(k.logger(), "bash", "-lc", envPrefix+" "+launchCmd)
		execCmd.Dir = workspaceAbs
		execCmd.Env = append(env.Environ(envProvider), "BD_ACTOR="+sessionName)
		execCmd.Stdin = k.IO.In
		execCmd.Stdout = k.IO.Out
		execCmd.Stderr = k.IO.Err
		return execCmd.Run()
	}

	envPrefix := remudaAgentEnvPrefix(agentName, "")
	startCmd := fmt.Sprintf("cd %s && %s %s", shellutil.SingleQuote(workspaceAbs), envPrefix, launchCmd)
	startCmd = fmt.Sprintf("export BD_ACTOR=%s; %s", shellutil.SingleQuote(sessionName), startCmd)

	// tmux sessions run inside a long-lived server whose environment can be stale,
	// so explicitly export inherited env vars (plus implicit forwards such as
	// ANTHROPIC_API_KEY for Claude/Bash container runs) to ensure `docker run -e
	// <NAME>` sees the expected value. Avoid doing this for zellij since it types
	// commands into a visible pane.
	tmuxExportEnv := tmuxContainerEnvNames(agentName, cmd.ContainerInheritEnv)
	if cmd.Container && len(tmuxExportEnv) > 0 && k.Session != nil && k.Session.Name() == string(session.SessionManagerTmux) {
		for _, name := range tmuxExportEnv {
			val, ok := envProvider.LookupEnv(name)
			if !ok {
				startCmd = fmt.Sprintf("unset %s; %s", name, startCmd)
				continue
			}
			startCmd = fmt.Sprintf("export %s=%s; %s", name, shellutil.SingleQuote(val), startCmd)
		}
	}

	startCmd = wrapWithCrashRecoverySleep(startCmd)
	if err := startSessionWithEnv(k.Session, sessionName, startCmd, envProvider); err != nil {
		return err
	}

	if cmd.Attach {
		return k.SessionAttach(sessionName)
	}
	return nil
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
		ws, err := s.WorkspacePathFromRoots(k.workspaceRoots()...)
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
