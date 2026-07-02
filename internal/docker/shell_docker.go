package docker

import (
	"os"
	"os/exec"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/util"
)

type shellDocker struct {
	logger zerolog.Logger
}

func NewShellDocker() Docker {
	return NewShellDockerWithLogger(logging.DefaultLogger())
}

func NewShellDockerWithLogger(logger zerolog.Logger) Docker {
	return &shellDocker{logger: logger}
}

func (s *shellDocker) SetLogger(logger zerolog.Logger) {
	s.logger = logger
}

func (s shellDocker) CheckRunning() error {
	output, err := util.RunCmdCombinedOutputWithLogger(s.logger, "docker", "ps")
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return pkgerrors.Wrapf(ErrNotRunning, "docker daemon check failed: %s", msg)
		}

		return pkgerrors.Wrapf(ErrNotRunning, "docker daemon check failed: %s", err.Error())
	}
	return nil
}

func (s shellDocker) ContainerRunning(container string) (bool, error) {
	cmd := util.CmdWithLogger(s.logger, "docker", "inspect", "-f", "{{.State.Running}}", container)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if pkgerrors.As(err, &ee) {
			msg := strings.TrimSpace(string(ee.Stderr))
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "no such object") || strings.Contains(lower, "no such container") || strings.Contains(lower, "not found") {
				return false, pkgerrors.Wrapf(ErrContainerNotFound, "docker inspect %q: %s", container, msg)
			}
			return false, pkgerrors.Wrapf(err, "docker inspect %q: %s", container, msg)
		}
		return false, pkgerrors.Wrapf(err, "docker inspect %q", container)
	}

	state := strings.TrimSpace(string(out))
	if state == "" {
		return false, pkgerrors.Errorf("docker inspect %q returned empty state", container)
	}
	return state == "true", nil
}

func (s shellDocker) Exec(container string, command string) error {
	cmd := util.CmdWithLogger(s.logger, "docker", "exec", "-it", container, "bash", "-lc", command)
	cmd.Stdout, cmd.Stdin, cmd.Stderr = os.Stderr, os.Stdin, os.Stderr
	return cmd.Run()
}
