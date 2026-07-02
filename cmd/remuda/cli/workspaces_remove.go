package cli

import (
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
)

// WorkspacesRemoveCmd removes one or more explicitly targeted workspaces.
type WorkspacesRemoveCmd struct {
	DryRun  bool     `name:"dry-run" help:"Print what would be removed without deleting anything."`
	Force   bool     `help:"Force-remove linked git worktrees even when git reports desynced or untracked state."`
	Targets []string `arg:"" name:"target" help:"Absolute workspace path or org/repo/workspace identifier." predictor:"workspace-dir"`
}

func (c WorkspacesRemoveCmd) Validate() error {
	for _, target := range c.Targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			return pkgerrors.New("target cannot be blank")
		}

		if strings.HasPrefix(trimmed, "~") {
			if !isSupportedTildePath(trimmed) {
				return pkgerrors.Errorf("invalid target %q: unsupported tilde path", target)
			}
			continue
		}
		if filepath.IsAbs(trimmed) {
			continue
		}
		if !isWorkspaceIdentifier(trimmed) {
			return pkgerrors.Errorf("invalid target %q: expected absolute path or org/repo/workspace identifier", target)
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
		return pkgerrors.Wrap(removeErr, "workspaces remove")
	}

	return nil
}

func resolveWorkspaceTargets(targets []string, ctx Context) ([]string, error) {
	resolved := make([]string, 0, len(targets))
	for _, target := range targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			return nil, pkgerrors.New("target cannot be blank")
		}
		if strings.HasPrefix(trimmed, "~") {
			if !isSupportedTildePath(trimmed) {
				return nil, pkgerrors.Errorf("invalid target %q: unsupported tilde path", target)
			}
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
