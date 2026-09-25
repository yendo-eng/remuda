package internal

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/util"
)

type RemovedWorkspace struct {
	Path  string
	Bytes int64
}

type WorkspaceActivity uint8

const (
	WorkspaceActivityAll WorkspaceActivity = iota
	WorkspaceActivityActive
	WorkspaceActivityInactive
)

func (k Remuda) Workspaces(activity WorkspaceActivity, ignore []string) ([]string, error) {
	if activity > WorkspaceActivityInactive {
		panic(fmt.Sprintf("unknown workspace activity %d", activity))
	}

	candidates := listWorkspaceDirs(k.Config.ReposBaseDir)
	active := map[string]string{}
	if activity != WorkspaceActivityAll {
		var err error
		active, err = k.activeWorkspaceSessions()
		if err != nil {
			return nil, err
		}
	}

	return filterWorkspaces(k.Config.ReposBaseDir, candidates, active, ignore, activity)
}

func isLinkedGitWorktree(workspace string) bool {
	info, err := os.Stat(filepath.Join(workspace, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// listWorkspaceDirs finds directories at depth 3: base/org/repo/<folder> excluding
// non-workspace directories like .repo_cache.
func listWorkspaceDirs(base string) []string {
	var out []string
	excluded := map[string]struct{}{
		".repo_cache": {},
	}
	if baseAbs, err := filepath.Abs(base); err == nil {
		base = baseAbs
	}
	orgs, err := os.ReadDir(base)
	if err != nil {
		return out
	}
	for _, o := range orgs {
		if !o.IsDir() {
			continue
		}
		repos, err := os.ReadDir(filepath.Join(base, o.Name()))
		if err != nil {
			continue
		}
		for _, r := range repos {
			if !r.IsDir() {
				continue
			}
			root := filepath.Join(base, o.Name(), r.Name())
			entries, err := os.ReadDir(root)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if _, ok := excluded[e.Name()]; ok {
					continue
				}
				out = append(out, filepath.Join(root, e.Name()))
			}
		}
	}
	return out
}

func filterWorkspaces(
	base string,
	candidates []string,
	active map[string]string,
	ignore []string,
	activity WorkspaceActivity,
) ([]string, error) {
	if len(ignore) > 0 {
		if err := validateIgnorePatterns(ignore); err != nil {
			return nil, err
		}
	}
	var filtered []string
	for _, ws := range candidates {
		abs, _ := filepath.Abs(ws)
		_, isActive := active[abs]
		if activity == WorkspaceActivityActive && !isActive {
			continue
		}
		if activity == WorkspaceActivityInactive && isActive {
			continue
		}
		if len(ignore) > 0 {
			rel, err := workspaceRelPath(base, ws)
			if err != nil {
				return nil, err
			}
			matched, err := matchIgnorePatterns(ignore, rel)
			if err != nil {
				return nil, err
			}
			if matched {
				continue
			}
		}
		filtered = append(filtered, ws)
	}
	return filtered, nil
}

func workspaceRelPath(base, workspace string) (string, error) {
	if err := ValidateWorkspacePath(base, workspace); err != nil {
		return "", err
	}
	org, repo, folder := util.SplitWorkspacePath(base, workspace)
	if org == "" || repo == "" || folder == "" {
		return "", pkgerrors.New("workspace must be at depth 3 under repos base dir (org/repo/folder)")
	}
	return path.Join(org, repo, folder), nil
}

func validateIgnorePatterns(patterns []string) error {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			return pkgerrors.New("ignore pattern is empty")
		}
		if _, err := path.Match(pattern, "org/repo/workspace"); err != nil {
			return pkgerrors.Wrapf(err, "invalid ignore pattern %q", pattern)
		}
	}
	return nil
}

func matchIgnorePatterns(patterns []string, rel string) (bool, error) {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			return false, pkgerrors.New("ignore pattern is empty")
		}
		matched, err := path.Match(pattern, rel)
		if err != nil {
			return false, pkgerrors.Wrapf(err, "invalid ignore pattern %q", pattern)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func ValidateWorkspacePath(base, workspace string) error {
	if base == "" {
		return pkgerrors.New("repos base dir is empty")
	}

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return pkgerrors.Wrap(err, "abs repos base dir")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return pkgerrors.Wrap(err, "abs workspace")
	}

	rel, err := filepath.Rel(baseAbs, workspaceAbs)
	if err != nil {
		return pkgerrors.Wrap(err, "rel workspace")
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return pkgerrors.New("workspace must be within repos base dir")
	}

	segments := []string{}
	for _, s := range strings.Split(rel, "/") {
		if s != "" && s != "." {
			segments = append(segments, s)
		}
	}
	if len(segments) != 3 {
		return pkgerrors.New("workspace must be at depth 3 under repos base dir (org/repo/folder)")
	}

	for _, s := range segments {
		if s == ".." {
			return pkgerrors.New("workspace must be within repos base dir")
		}
	}

	excluded := map[string]struct{}{
		".repo_cache": {},
	}
	if _, ok := excluded[segments[2]]; ok {
		return pkgerrors.New("workspace folder is not a removable workspace")
	}

	return nil
}
