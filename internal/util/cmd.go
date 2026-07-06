package util

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
)

func CmdWithLogger(logger zerolog.Logger, name string, args ...string) *exec.Cmd {
	logger.Debug().Str("cmd", name+" "+strings.Join(args, " ")).Msg("command")
	//nolint:gosec,noctx // G204: intentionally executes caller-provided commands; legacy helper has no context parameter.
	return exec.Command(name, args...)
}

func Cmd(name string, args ...string) *exec.Cmd {
	return CmdWithLogger(logging.DefaultLogger(), name, args...)
}

func CmdWithEnvAndLogger(logger zerolog.Logger, env []string, name string, args ...string) *exec.Cmd {
	logger.Debug().Str("cmd", name+" "+strings.Join(args, " ")).Msg("command")
	//nolint:gosec,noctx // G204: intentionally executes caller-provided commands; legacy helper has no context parameter.
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
		if path, err := lookupPathWithEnv(name, env); err == nil {
			cmd.Path = path
			if len(cmd.Args) > 0 {
				cmd.Args[0] = name
			}
			cmd.Err = nil
		} else if cmd.Err == nil {
			cmd.Err = err
		}
	}
	return cmd
}

func CmdWithEnv(env []string, name string, args ...string) *exec.Cmd {
	return CmdWithEnvAndLogger(logging.DefaultLogger(), env, name, args...)
}

func RunCmdWithLogger(logger zerolog.Logger, name string, args ...string) error {
	return runCmd(logger, CmdWithLogger(logger, name, args...), name, args)
}

func RunCmd(name string, args ...string) error {
	return RunCmdWithLogger(logging.DefaultLogger(), name, args...)
}

func RunCmdWithEnvAndLogger(logger zerolog.Logger, env []string, name string, args ...string) error {
	return runCmd(logger, CmdWithEnvAndLogger(logger, env, name, args...), name, args)
}

// runCmd executes cmd, always capturing its combined output so a failure's
// stderr/stdout can be attached to the returned error instead of a bare
// *exec.ExitError. If verbose/debug logging is on, output is additionally
// streamed to stderr for live diagnosis.
func runCmd(logger zerolog.Logger, cmd *exec.Cmd, name string, args []string) error {
	var buf bytes.Buffer
	if logger.GetLevel() <= zerolog.DebugLevel {
		cmd.Stdout = io.MultiWriter(os.Stderr, &buf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}

	if err := cmd.Run(); err != nil {
		return pkgerrors.Wrapf(err, "%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(buf.String()))
	}
	return nil
}

func RunCmdWithEnv(env []string, name string, args ...string) error {
	return RunCmdWithEnvAndLogger(logging.DefaultLogger(), env, name, args...)
}

func RunCmdOutputWithLogger(logger zerolog.Logger, name string, args ...string) (string, error) {
	cmd := CmdWithLogger(logger, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func RunCmdOutput(name string, args ...string) (string, error) {
	return RunCmdOutputWithLogger(logging.DefaultLogger(), name, args...)
}

func RunCmdCombinedOutputWithLogger(logger zerolog.Logger, name string, args ...string) (string, error) {
	cmd := CmdWithLogger(logger, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func RunCmdCombinedOutput(name string, args ...string) (string, error) {
	return RunCmdCombinedOutputWithLogger(logging.DefaultLogger(), name, args...)
}
