package util

import (
	"os"
	"os/exec"
	"strings"

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
	cmd := CmdWithLogger(logger, name, args...)
	// Default to quiet execution: capture output so it doesn't pollute the
	// user's terminal. If verbose/debug logging is on, stream output to stderr
	// for easier diagnosis.
	if logger.GetLevel() <= zerolog.DebugLevel {
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		return cmd.Run()
	}

	_, err := cmd.CombinedOutput()
	return err
}

func RunCmd(name string, args ...string) error {
	return RunCmdWithLogger(logging.DefaultLogger(), name, args...)
}

func RunCmdWithEnvAndLogger(logger zerolog.Logger, env []string, name string, args ...string) error {
	cmd := CmdWithEnvAndLogger(logger, env, name, args...)
	// Default to quiet execution: capture output so it doesn't pollute the
	// user's terminal. If verbose/debug logging is on, stream output to stderr
	// for easier diagnosis.
	if logger.GetLevel() <= zerolog.DebugLevel {
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		return cmd.Run()
	}

	_, err := cmd.CombinedOutput()
	return err
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
