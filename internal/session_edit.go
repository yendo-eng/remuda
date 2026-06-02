package internal

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/util"
)

// SessionEdit resolves the workspace for a session and opens it in the user's editor.
func (k Remuda) SessionEdit(sessionName, editorCmd string) error {
	workspace, err := k.SessionWorkspacePath(sessionName)
	if err != nil {
		return err
	}

	cmd := strings.TrimSpace(editorCmd)
	if cmd == "" {
		return errors.New("editor command is required")
	}

	return launchEditor(k.logger(), k.IO, cmd, workspace, k.envProvider())
}

func launchEditor(logger zerolog.Logger, io IO, editorCmd, workspace string, provider env.Provider) error {
	provider = env.OrDefault(provider)
	shell := strings.TrimSpace(provider.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}

	command := fmt.Sprintf("%s %s", editorCmd, singleQuote(workspace))
	cmd := util.CmdWithLogger(logger, shell, "-lc", command)
	cmd.Stdin = io.In
	cmd.Stdout = io.Out
	cmd.Stderr = io.Err

	return errors.Wrapf(cmd.Run(), "launch editor %q", editorCmd)
}

func singleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
