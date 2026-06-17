package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util"
)

type agentLaunchCommand struct {
	Workspace string

	SessionName string

	AgentName string
	Model     string
	Command   string

	Detached bool
	Attach   bool

	ReplaceExisting bool

	Container           bool
	ContainerImage      string
	ContainerOpts       []string
	ContainerInheritEnv []string
	Yolo                bool

	EnvOverrides map[string]string
}

type agentLaunchResult struct {
	Workspace      string
	ContainerName  string
	ContainerImage string
}

func (k Remuda) launchAgentSession(cmd agentLaunchCommand) (agentLaunchResult, error) {
	workspace := strings.TrimSpace(cmd.Workspace)
	if workspace == "" {
		return agentLaunchResult{}, errors.New("workspace path is empty")
	}

	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		workspaceAbs = workspace
	}

	sessionName := strings.TrimSpace(cmd.SessionName)
	if sessionName == "" {
		sessionName = session.SessionNameFromWorkspaceName(workspaceAbs)
	}

	containerName := docker.ContainerNameFromSession(sessionName)

	envProvider := k.launchEnvProvider(cmd, sessionName, workspaceAbs)
	launchCmd, containerImage, err := k.composeLaunchCommand(
		VibeCommand{
			Agent:               cmd.AgentName,
			Yolo:                cmd.Yolo,
			Container:           cmd.Container,
			ContainerName:       cmd.ContainerImage,
			ContainerOpts:       cmd.ContainerOpts,
			ContainerInheritEnv: cmd.ContainerInheritEnv,
		},
		workspaceAbs,
		cmd.Command,
		sessionName,
		containerName,
		envProvider,
	)
	if err != nil {
		return agentLaunchResult{}, err
	}

	result := agentLaunchResult{
		Workspace:      workspaceAbs,
		ContainerName:  containerName,
		ContainerImage: containerImage,
	}

	if !cmd.Detached {
		execCmd := util.CmdWithEnvAndLogger(k.logger(), launchEnvValues(envProvider), "bash", "-lc", launchCmd)
		execCmd.Dir = workspaceAbs
		execCmd.Stdin = k.IO.In
		execCmd.Stdout = k.IO.Out
		execCmd.Stderr = k.IO.Err
		return result, execCmd.Run()
	}

	if cmd.ReplaceExisting {
		if _, err := k.Session.Find(sessionName); err == nil {
			logger := k.logger()
			logger.Debug().Str("session", sessionName).Msg("existing session found; killing due to --force")
			if err := k.Session.Kill(sessionName); err != nil {
				return agentLaunchResult{}, errors.Wrapf(err, "killing existing session %q", sessionName)
			}
		} else if !errors.Is(err, session.ErrSessionNotFound) {
			return agentLaunchResult{}, errors.Wrapf(err, "checking for existing session %q", sessionName)
		}
	}

	startCmd := fmt.Sprintf("cd %s && %s", shellSingleQuote(workspaceAbs), launchCmd)
	startCmd = wrapWithCrashRecoverySleep(startCmd)
	if err := startSessionWithEnv(k.Session, sessionName, startCmd, envProvider); err != nil {
		return agentLaunchResult{}, err
	}

	if cmd.Attach {
		return result, k.SessionAttach(sessionName)
	}

	return result, nil
}

func (k Remuda) launchEnvProvider(cmd agentLaunchCommand, sessionName, workspaceAbs string) env.Provider {
	provider := env.NewMutableProvider(k.envProvider())
	for key, value := range cmd.EnvOverrides {
		if strings.TrimSpace(key) == "" {
			continue
		}
		provider.Setenv(key, value)
	}

	if strings.TrimSpace(cmd.AgentName) != "" {
		provider.Setenv("REMUDA_AGENT", cmd.AgentName)
	}
	if strings.TrimSpace(cmd.Model) != "" {
		provider.Setenv("REMUDA_MODEL", cmd.Model)
	}
	provider.Setenv("BD_ACTOR", sessionName)
	if _, ok := provider.LookupEnv("BEADS_DIR"); !ok {
		if beadsDir, ok := sharedBeadsDirForWorkspace(workspaceAbs); ok {
			provider.Setenv("BEADS_DIR", beadsDir)
		}
	}
	if cmd.Container && strings.EqualFold(cmd.AgentName, "claude") && cmd.Yolo {
		provider.Setenv("IS_SANDBOX", "1")
	}

	return provider
}

func launchEnvValues(provider env.Provider) []string {
	envValues := env.Environ(provider)
	if !envHasKey(envValues, "PATH") {
		if path := strings.TrimSpace(os.Getenv("PATH")); path != "" {
			envValues = append(envValues, "PATH="+path)
		}
	}
	return envValues
}

func sharedBeadsDirForWorkspace(workspaceAbs string) (string, bool) {
	beadsDir := filepath.Join(filepath.Dir(workspaceAbs), ".beads_worktree", ".beads")
	info, err := os.Stat(beadsDir)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return beadsDir, true
}
