package util

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func Cmd(logger zerolog.Logger, name string, args ...string) *exec.Cmd {
	logger.Debug().Str("cmd", name+" "+strings.Join(args, " ")).Msg("command")
	//nolint:gosec,noctx // G204: intentionally executes caller-provided commands; legacy helper has no context parameter.
	return exec.Command(name, args...)
}

func CmdWithEnv(logger zerolog.Logger, env []string, name string, args ...string) *exec.Cmd {
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

func RunCmd(logger zerolog.Logger, name string, args ...string) error {
	return runCmd(logger, Cmd(logger, name, args...), name)
}

func RunCmdWithEnv(logger zerolog.Logger, env []string, name string, args ...string) error {
	return runCmd(logger, CmdWithEnv(logger, env, name, args...), name)
}

// runCmd executes cmd, always capturing its stdout/stderr so a failure's
// diagnostic text can be attached to the returned error instead of a bare
// *exec.ExitError. If verbose/debug logging is on, output is additionally
// streamed to stderr for live diagnosis. Only the command name is included in
// the error, never argv: args can carry secrets (tmux -e KEY=value env pairs,
// credential-bearing clone URLs), and the process's own stderr already names
// what failed.
func runCmd(logger zerolog.Logger, cmd *exec.Cmd, name string) error {
	var stdout, stderr bytes.Buffer
	if logger.GetLevel() <= zerolog.DebugLevel {
		cmd.Stdout = io.MultiWriter(os.Stderr, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return pkgerrors.Wrap(err, name)
		}
		return pkgerrors.Wrapf(err, "%s: %s", name, msg)
	}
	return nil
}

func RunCmdOutput(logger zerolog.Logger, name string, args ...string) (string, error) {
	cmd := Cmd(logger, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func RunCmdCombinedOutput(logger zerolog.Logger, name string, args ...string) (string, error) {
	cmd := Cmd(logger, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
