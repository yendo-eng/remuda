package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/util"
	"github.com/yendo-eng/remuda/internal/util/shell"
)

// WorkspacesEdit opens an on-disk workspace in the user's editor. Unlike
// WorkspacesRemove it accepts active workspaces: editing one is the common case.
func (k Remuda) WorkspacesEdit(workspace, editorCmd string) error {
	cmd := strings.TrimSpace(editorCmd)
	if cmd == "" {
		return pkgerrors.New("editor command is required")
	}

	workspaceAbs, err := filepath.Abs(strings.TrimSpace(workspace))
	if err != nil {
		return pkgerrors.Wrapf(err, "resolve workspace %q", workspace)
	}
	workspaceAbs = filepath.Clean(workspaceAbs)

	if err := validateWorkspacePath(k.Config.ReposBaseDir, workspaceAbs); err != nil {
		return pkgerrors.Wrapf(err, "invalid workspace %q", workspaceAbs)
	}

	info, err := os.Stat(workspaceAbs)
	if err != nil {
		return pkgerrors.Wrapf(err, "stat workspace %q", workspaceAbs)
	}
	if !info.IsDir() {
		return pkgerrors.Errorf("workspace %q is not a directory", workspaceAbs)
	}

	return launchEditor(k.logger(), k.IO, cmd, workspaceAbs, k.envProvider())
}

func launchEditor(logger zerolog.Logger, io IO, editorCmd, workspace string, provider env.Provider) error {
	provider = env.OrDefault(provider)
	shellPath := strings.TrimSpace(provider.Getenv("SHELL"))
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	command := fmt.Sprintf("%s %s", editorCmd, shell.SingleQuote(workspace))
	cmd := util.CmdWithLogger(logger, shellPath, "-lc", command)
	cmd.Stdin = io.In
	cmd.Stdout = io.Out
	cmd.Stderr = io.Err

	return pkgerrors.Wrapf(cmd.Run(), "launch editor %q", editorCmd)
}
