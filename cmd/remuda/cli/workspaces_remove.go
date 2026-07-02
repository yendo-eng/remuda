package cli

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// WorkspacesRemoveCmd removes one or more explicitly targeted workspaces.
type WorkspacesRemoveCmd struct {
	DryRun  bool     `name:"dry-run" help:"Print what would be removed without deleting anything."`
	Force   bool     `help:"Force-remove linked git worktrees even when git reports desynced or untracked state."`
	Targets []string `arg:"" name:"target" help:"Absolute workspace path or org/repo/workspace identifier." predictor:"workspace-dir"`
}

func (c WorkspacesRemoveCmd) Validate() error {
	for _, target := range c.Targets {
		if err := validateWorkspaceTarget(target); err != nil {
			return err
		}
	}
	return nil
}

func (c WorkspacesRemoveCmd) Run(ctx Context) error {
	resolved, err := resolveWorkspaceTargets(c.Targets, ctx)
	if err != nil {
		return err
	}

	removed, removeErr := ctx.Remuda.WorkspacesRemove(resolved, c.DryRun, c.Force)
	for _, ws := range removed {
		if c.DryRun {
			ctx.Remuda.IO.Outf("would remove %s\n", ws.Path)
		} else {
			ctx.Remuda.IO.Outf("removed %s\n", ws.Path)
		}
	}
	if removeErr != nil {
		return errors.Wrap(removeErr, "workspaces remove")
	}

	return nil
}

func resolveWorkspaceTargets(targets []string, ctx Context) ([]string, error) {
	resolved := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := validateWorkspaceTarget(target); err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(target)
		if strings.HasPrefix(trimmed, "~") {
			resolved = append(resolved, absPathFromContext(trimmed, ctx))
			continue
		}
		if isWorkspaceIdentifier(trimmed) {
			trimmed = filepath.Join(
				ctx.Remuda.Config.ReposBaseDir,
				filepath.FromSlash(trimmed),
			)
		}
		resolved = append(resolved, absPathFromContext(trimmed, ctx))
	}
	return resolved, nil
}

func validateWorkspaceTarget(target string) error {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return errors.New("target cannot be blank")
	}

	if strings.HasPrefix(trimmed, "~") {
		if !isSupportedTildePath(trimmed) {
			return errors.Errorf("invalid target %q: unsupported tilde path", target)
		}
		return nil
	}
	if filepath.IsAbs(trimmed) {
		return nil
	}
	if !isWorkspaceIdentifier(trimmed) {
		return errors.Errorf("invalid target %q: expected absolute path or org/repo/workspace identifier", target)
	}
	return nil
}

func isSupportedTildePath(value string) bool {
	return value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~"+string(filepath.Separator))
}

func isWorkspaceIdentifier(value string) bool {
	cleaned := strings.TrimSpace(filepath.ToSlash(value))
	if cleaned == "" || strings.HasPrefix(cleaned, "/") {
		return false
	}
	parts := strings.Split(cleaned, "/")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, "~") {
			return false
		}
	}
	return true
}
