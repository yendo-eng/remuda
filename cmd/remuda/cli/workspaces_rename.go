package cli

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// WorkspacesRenameCmd renames an inactive workspace path and branch.
type WorkspacesRenameCmd struct {
	Target  string `arg:"" name:"target" help:"Absolute workspace path or org/repo/workspace identifier." predictor:"workspace-dir"`
	NewName string `arg:"" name:"new-name" help:"New workspace name (also used as the default branch name)."`
}

func (c WorkspacesRenameCmd) Validate() error {
	if err := validateWorkspaceTarget(c.Target); err != nil {
		return err
	}

	newName := strings.TrimSpace(c.NewName)
	if newName == "" {
		return errors.New("new-name cannot be blank")
	}
	if strings.Contains(newName, "/") || strings.Contains(newName, string(filepath.Separator)) {
		return errors.Errorf("invalid new-name %q: must be a single workspace name", c.NewName)
	}
	if newName == "." || newName == ".." {
		return errors.Errorf("invalid new-name %q", c.NewName)
	}

	return nil
}

func (c WorkspacesRenameCmd) Run(ctx Context) error {
	resolved, err := resolveWorkspaceTargets([]string{c.Target}, ctx)
	if err != nil {
		return err
	}

	renamed, err := ctx.Remuda.WorkspacesRename(resolved[0], c.NewName)
	if err != nil {
		return errors.Wrap(err, "workspaces rename")
	}

	ctx.Remuda.IO.Outf("renamed %s -> %s\n", renamed.OldPath, renamed.NewPath)
	return nil
}
