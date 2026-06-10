package internal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util"
)

func (k Remuda) SessionKill(
	name string,
	cleanup bool,
	closePRComment *string,
	mergePR bool,
	mergeFlags []string,
	closeBD bool,
) error {
	return k.killOne(name, cleanup, closePRComment, mergePR, mergeFlags, closeBD)
}

func (k Remuda) killOne(
	name string,
	cleanup bool,
	closePRComment *string,
	mergePR bool,
	mergeFlags []string,
	closeBD bool,
) error {
	var workspacePath string
	needsWorkspace := closePRComment != nil || mergePR || closeBD
	if needsWorkspace {
		var err error
		workspacePath, err = k.workspacePathForSession(name)
		if err != nil {
			return err
		}
	}

	if mergePR {
		if len(mergeFlags) == 0 {
			mergeFlags = []string{"--rebase"}
		}
		res, err := k.GitHub.MergePullRequest(workspacePath, mergeFlags)
		if err != nil {
			return err
		}
		if res == nil {
			return errors.Errorf("no pull request associated with session %q; cannot merge", name)
		}
		if res.Merged {
			k.IO.Outf("Merged PR #%d for session %q (%s) with flags: %s\n", res.Number, name, res.URL, strings.Join(mergeFlags, " "))
		} else {
			return errors.Errorf("failed to merge PR #%d for session %q", res.Number, name)
		}
		closePRComment = nil

		// Attempt to close beads issue if applicable.
		k.closeBDIssue(workspacePath)
	} else if closeBD {
		k.closeBDIssue(workspacePath)
	}

	if err := k.Session.Kill(name); err != nil {
		return err
	}

	if closePRComment != nil {
		res, err := k.GitHub.ClosePullRequest(workspacePath, *closePRComment)
		if err != nil {
			return err
		}
		switch {
		case res == nil:
			k.IO.Outf("No PR associated with session %q\n", name)
		case res.Closed:
			k.IO.Outf("Closed PR #%d for session %q (%s)\n", res.Number, name, res.URL)
		default:
			k.IO.Outf("PR #%d already %s for session %q (%s)\n", res.Number, strings.ToLower(res.State), name, res.URL)
		}
	}

	if cleanup {
		if err := k.cleanupWorkspaceForSession(name); err != nil {
			return err
		}
	}

	return nil
}

func (k Remuda) workspacePathForSession(sessionName string) (string, error) {
	workspace, err := session.SessionInfo{Name: sessionName}.WorkspacePath(k.Config.ReposBaseDir)
	if err != nil {
		return "", errors.Wrap(err, "get workspace path from session name")
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return "", errors.Wrap(err, "resolve workspace path")
	}
	return abs, nil
}

// cleanupWorkspaceForSession removes the workspace directory and its git worktree.
func (k Remuda) cleanupWorkspaceForSession(sessionName string) error {
	logger := k.logger()
	ws, err := session.SessionInfo{Name: sessionName}.WorkspacePath(k.Config.ReposBaseDir)
	if err != nil {
		return errors.Wrap(err, "get workspace path from session name")
	}

	// Compute cache dir to remove worktree from.
	// Layout: <base>/<org>/<repo>/<folder>; cache at <base>/<org>/<repo>/.repo_cache
	parts := strings.Split(sessionName, "/")
	baseDir := filepath.Join(k.Config.ReposBaseDir, parts[0], parts[1])
	cacheDir := filepath.Join(baseDir, ".repo_cache")
	if err := k.Git.WorktreeRemove(cacheDir, ws); err != nil {
		logger.Warn().Err(err).Msgf("removing git worktree %q from cache %q", ws, cacheDir)
	}
	return os.RemoveAll(ws)
}

// // deriveWorkspacePathFromSessionName maps org/repo/folder → base/org/repo/folder.
// func deriveWorkspacePathFromSessionName(base, name string) (string, bool) {
// 	parts := strings.Split(strings.TrimSpace(name), "/")
// 	if len(parts) != 3 {
// 		return "", false
// 	}
// 	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
// 		return "", false
// 	}
// 	return filepath.Join(base, parts[0], parts[1], parts[2]), true
// }

func (k Remuda) closeBDIssue(workspacePath string) bool {
	logger := k.logger()
	// determine the git branch at the workspace path
	branchName, err := util.RunCmdOutputWithLogger(logger, "git", "-C", workspacePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		logger.Warn().Err(err).Msg("unable to determine git branch for beads issue closure")
		return false
	}

	// trim whitespace/newline from branch name since cmd output may contain it
	branchName = strings.TrimSpace(branchName)

	cmdEnv := env.Environ(k.envProvider())
	err = util.RunCmdWithEnvAndLogger(logger, cmdEnv, "br", "close", branchName)
	if err != nil {
		logger.Warn().Err(err).Str("issue_id", branchName).Msg("unable to close beads issue")
		return false
	}

	k.IO.Outf("Closed beads issue %q\n", branchName)
	return true
}
