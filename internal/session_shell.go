package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
)

func (k Remuda) SessionShell(sessionName string) error {
	name := strings.TrimSpace(sessionName)
	if name == "" {
		return fmt.Errorf("session name is required")
	}

	workspace, err := k.SessionWorkspacePath(name)
	if err != nil {
		return err
	}
	if st, err := os.Stat(workspace); err != nil || !st.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return fmt.Errorf("workspace path %q is not accessible: %w", workspace, err)
	}

	containerName := docker.ContainerNameFromSession(name)
	if strings.TrimSpace(containerName) == "" {
		return fmt.Errorf("unable to derive container name from session %q", name)
	}

	if k.Docker == nil {
		k.IO.Errln("docker integration unavailable; opening host shell in", workspace)
		return runHostShell(name, workspace, k.IO, k.envProvider())
	}

	if err := k.Docker.CheckRunning(); err != nil {
		return fmt.Errorf("docker unavailable: %w (use --host to open a host shell in the workspace)", err)
	}

	running, err := k.Docker.ContainerRunning(containerName)
	if err != nil {
		if errors.Is(err, docker.ErrContainerNotFound) {
			k.IO.Errf("container %q unavailable (%v); opening host shell in %s\n", containerName, err, workspace)
			return runHostShell(name, workspace, k.IO, k.envProvider())
		}
		return fmt.Errorf("checking container %q state: %w", containerName, err)
	}
	if !running {
		return fmt.Errorf("container %q is not running (use --host to open a host shell in the workspace)", containerName)
	}

	if err := k.Docker.Exec(containerName, "cd /workspace && exec ${SHELL:-/bin/bash}"); err != nil {
		return fmt.Errorf("docker exec %q failed: %w", containerName, err)
	}

	return nil
}

func (k Remuda) SessionHostShell(sessionName string) error {
	name := strings.TrimSpace(sessionName)
	if name == "" {
		return fmt.Errorf("session name is required")
	}

	workspace, err := k.SessionWorkspacePath(name)
	if err != nil {
		return err
	}
	if st, err := os.Stat(workspace); err != nil || !st.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return fmt.Errorf("workspace path %q is not accessible: %w", workspace, err)
	}

	return runHostShell(name, workspace, k.IO, k.envProvider())
}

func runHostShell(sessionName, workspace string, io IO, provider env.Provider) error {
	provider = env.OrDefault(provider)
	shellEnv := strings.TrimSpace(provider.Getenv("SHELL"))
	if shellEnv == "" {
		shellEnv = "/bin/bash"
	}

	parts := strings.Fields(shellEnv)
	shellPath := parts[0]
	shellArgs := parts[1:]

	//nolint:gosec // G204: opening the user's configured shell is intentional behavior.
	cmd := exec.CommandContext(context.Background(), shellPath, shellArgs...)
	cmd.Dir = workspace
	baseEnv := os.Environ()
	if environer, ok := provider.(interface{ Environ() []string }); ok {
		baseEnv = environer.Environ()
	}
	cmd.Env = append(baseEnv, "BD_ACTOR="+sessionName)

	if io.In != nil {
		cmd.Stdin = io.In
	} else {
		cmd.Stdin = os.Stdin
	}
	if io.Out != nil {
		cmd.Stdout = io.Out
	} else {
		cmd.Stdout = os.Stdout
	}
	if io.Err != nil {
		cmd.Stderr = io.Err
	} else {
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}
