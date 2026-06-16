package internal

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/util"
)

type PrunedWorkspace struct {
	Path  string
	Bytes int64
}

func (k Remuda) Workspaces() ([]string, error) {
	return k.WorkspacesWithIgnore(nil)
}

func (k Remuda) WorkspacesWithIgnore(ignore []string) ([]string, error) {
	return k.WorkspacesWithOptions(ignore, false)
}

// WorkspacesWithOptions lists all candidate workspaces. When includeTmp is true
// it also scans the OS-temp root used by --tmp sessions; otherwise temp
// workspaces are hidden.
func (k Remuda) WorkspacesWithOptions(ignore []string, includeTmp bool) ([]string, error) {
	candidates := k.listCandidateWorkspaces(includeTmp)
	return filterInactiveWorkspaces(k.enumerationRoots(includeTmp), candidates, map[string]struct{}{}, ignore)
}

func (k Remuda) ActiveWorkspaces() ([]string, error) {
	return k.activeWorkspaces(nil, false)
}

func (k Remuda) ActiveWorkspacesWithIgnore(ignore []string) ([]string, error) {
	return k.activeWorkspaces(ignore, false)
}

func (k Remuda) ActiveWorkspacesWithOptions(ignore []string, includeTmp bool) ([]string, error) {
	return k.activeWorkspaces(ignore, includeTmp)
}

func (k Remuda) InactiveWorkspaces() ([]string, error) {
	return k.inactiveWorkspaces(nil, false)
}

func (k Remuda) InactiveWorkspacesWithIgnore(ignore []string) ([]string, error) {
	return k.inactiveWorkspaces(ignore, false)
}

func (k Remuda) InactiveWorkspacesWithOptions(ignore []string, includeTmp bool) ([]string, error) {
	return k.inactiveWorkspaces(ignore, includeTmp)
}

// enumerationRoots returns the roots scanned when enumerating workspaces. The
// temp root is only included when explicitly requested via --include-tmp.
func (k Remuda) enumerationRoots(includeTmp bool) []string {
	if includeTmp {
		return k.workspaceRoots()
	}
	return []string{k.Config.ReposBaseDir}
}

// listCandidateWorkspaces enumerates candidate workspaces (<root>/<org>/<repo>/<folder>,
// excluding .repo_cache) under the persistent repos base dir and, when includeTmp
// is set, the OS-temp root as well.
func (k Remuda) listCandidateWorkspaces(includeTmp bool) []string {
	candidates := listWorkspaceDirs(k.Config.ReposBaseDir)
	if includeTmp && strings.TrimSpace(k.Config.TmpBaseDir) != "" {
		candidates = append(candidates, listWorkspaceDirs(k.Config.TmpBaseDir)...)
	}
	return candidates
}

func (k Remuda) activeWorkspaces(ignore []string, includeTmp bool) ([]string, error) {
	active, err := k.activeWorkspaceSet()
	if err != nil {
		return nil, err
	}

	candidates := k.listCandidateWorkspaces(includeTmp)
	return filterActiveWorkspaces(k.enumerationRoots(includeTmp), candidates, active, ignore)
}

func (k Remuda) inactiveWorkspaces(ignore []string, includeTmp bool) ([]string, error) {
	active, err := k.activeWorkspaceSet()
	if err != nil {
		return nil, err
	}

	candidates := k.listCandidateWorkspaces(includeTmp)
	return filterInactiveWorkspaces(k.enumerationRoots(includeTmp), candidates, active, ignore)
}

func (k Remuda) activeWorkspaceSet() (map[string]struct{}, error) {
	// Build a set of active workspace paths (absolute) from sessions.
	sessions, err := k.Session.List()
	if err != nil {
		return nil, err
	}

	active := map[string]struct{}{}
	for _, s := range sessions {
		if !s.IsRemudaSession() {
			continue
		}

		if ws, err := s.WorkspacePathFromRoots(k.workspaceRoots()...); err == nil {
			abs, _ := filepath.Abs(ws)
			active[abs] = struct{}{}
		}
	}

	return active, nil
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

func filterInactiveWorkspaces(
	roots []string,
	candidates []string,
	active map[string]struct{},
	ignore []string,
) ([]string, error) {
	return filterWorkspacesByActivity(roots, candidates, active, ignore, false)
}

func filterActiveWorkspaces(
	roots []string,
	candidates []string,
	active map[string]struct{},
	ignore []string,
) ([]string, error) {
	return filterWorkspacesByActivity(roots, candidates, active, ignore, true)
}

func filterWorkspacesByActivity(
	roots []string,
	candidates []string,
	active map[string]struct{},
	ignore []string,
	activeOnly bool,
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
		if activeOnly && !isActive {
			continue
		}
		if !activeOnly && isActive {
			continue
		}
		if len(ignore) > 0 {
			rel, err := workspaceRelPath(roots, ws)
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

// workspaceRelPath returns the org/repo/folder ignore-match key for a workspace,
// computed against whichever configured root contains it.
func workspaceRelPath(roots []string, workspace string) (string, error) {
	if err := validateWorkspacePathRoots(roots, workspace); err != nil {
		return "", err
	}
	for _, base := range roots {
		if !pathWithin(base, workspace) {
			continue
		}
		org, repo, folder := util.SplitWorkspacePath(base, workspace)
		if org != "" && repo != "" && folder != "" {
			return path.Join(org, repo, folder), nil
		}
	}
	return "", errors.New("workspace must be at depth 3 under a workspace root (org/repo/folder)")
}

func validateIgnorePatterns(patterns []string) error {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			return errors.New("ignore pattern is empty")
		}
		if _, err := path.Match(pattern, "org/repo/workspace"); err != nil {
			return errors.Wrapf(err, "invalid ignore pattern %q", pattern)
		}
	}
	return nil
}

func matchIgnorePatterns(patterns []string, rel string) (bool, error) {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			return false, errors.New("ignore pattern is empty")
		}
		matched, err := path.Match(pattern, rel)
		if err != nil {
			return false, errors.Wrapf(err, "invalid ignore pattern %q", pattern)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// validateWorkspace checks the workspace against every configured root (the
// persistent repos base dir and, when set, the OS-temp root for --tmp sessions),
// accepting it when it is a valid depth-3 workspace under any of them.
func (k Remuda) validateWorkspace(workspace string) error {
	return validateWorkspacePathRoots(k.workspaceRoots(), workspace)
}

// ValidateWorkspace is the exported form of validateWorkspace for CLI callers.
func (k Remuda) ValidateWorkspace(workspace string) error {
	return k.validateWorkspace(workspace)
}

func validateWorkspacePathRoots(roots []string, workspace string) error {
	if len(roots) == 0 {
		return errors.New("no workspace roots provided")
	}
	var firstErr error
	for _, base := range roots {
		if strings.TrimSpace(base) == "" {
			continue
		}
		err := validateWorkspacePath(base, workspace)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		return errors.New("repos base dir is empty")
	}
	return firstErr
}

func validateWorkspacePath(base, workspace string) error {
	if base == "" {
		return errors.New("repos base dir is empty")
	}

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return errors.Wrap(err, "abs repos base dir")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return errors.Wrap(err, "abs workspace")
	}

	rel, err := filepath.Rel(baseAbs, workspaceAbs)
	if err != nil {
		return errors.Wrap(err, "rel workspace")
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return errors.New("workspace must be within repos base dir")
	}

	segments := []string{}
	for _, s := range strings.Split(rel, "/") {
		if s != "" && s != "." {
			segments = append(segments, s)
		}
	}
	if len(segments) != 3 {
		return errors.New("workspace must be at depth 3 under repos base dir (org/repo/folder)")
	}

	for _, s := range segments {
		if s == ".." {
			return errors.New("workspace must be within repos base dir")
		}
	}

	excluded := map[string]struct{}{
		".repo_cache": {},
	}
	if _, ok := excluded[segments[2]]; ok {
		return errors.New("workspace folder is not a prunable workspace")
	}

	return nil
}

func ValidateWorkspacePath(base, workspace string) error {
	return validateWorkspacePath(base, workspace)
}
