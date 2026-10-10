package internal

import (
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util"
)

type SessionKillCommand struct {
	Name           string
	Cleanup        bool
	ClosePRComment *string
	MergePR        bool
	MergeFlags     []string
	CloseBD        bool
}

func (k Remuda) SessionKill(cmd SessionKillCommand) error {
	var workspacePath string
	needsWorkspace := cmd.ClosePRComment != nil || cmd.MergePR || cmd.CloseBD
	if needsWorkspace {
		var err error
		workspacePath, err = k.workspacePathForSession(cmd.Name)
		if err != nil {
			return err
		}
	}

	if cmd.MergePR {
		if len(cmd.MergeFlags) == 0 {
			cmd.MergeFlags = []string{"--rebase"}
		}
		res, err := k.GitHub.MergePullRequest(workspacePath, cmd.MergeFlags)
		if err != nil {
			return err
		}
		if res == nil {
			return pkgerrors.Errorf("no pull request associated with session %q; cannot merge", cmd.Name)
		}
		if res.Merged {
			k.IO.Outf("Merged PR #%d for session %q (%s) with flags: %s\n", res.Number, cmd.Name, res.URL, strings.Join(cmd.MergeFlags, " "))
		} else {
			return pkgerrors.Errorf("failed to merge PR #%d for session %q", res.Number, cmd.Name)
		}
		cmd.ClosePRComment = nil

		// Attempt to close beads issue if applicable.
		k.closeBDIssue(workspacePath)
	} else if cmd.CloseBD {
		k.closeBDIssue(workspacePath)
	}

	if err := k.Multiplexer.Kill(cmd.Name); err != nil {
		return err
	}

	if cmd.ClosePRComment != nil {
		res, err := k.GitHub.ClosePullRequest(workspacePath, *cmd.ClosePRComment)
		if err != nil {
			return err
		}
		switch {
		case res == nil:
			k.IO.Outf("No PR associated with session %q\n", cmd.Name)
		case res.Closed:
			k.IO.Outf("Closed PR #%d for session %q (%s)\n", res.Number, cmd.Name, res.URL)
		default:
			k.IO.Outf("PR #%d already %s for session %q (%s)\n", res.Number, strings.ToLower(res.State), cmd.Name, res.URL)
		}
	}

	if cmd.Cleanup {
		if err := k.cleanupWorkspaceForSession(cmd.Name); err != nil {
			return err
		}
	}

	return nil
}

func (k Remuda) workspacePathForSession(sessionName string) (string, error) {
	workspace, err := session.SessionInfo{Name: sessionName}.WorkspacePath(k.Config.ReposBaseDir)
	if err != nil {
		return "", pkgerrors.Wrap(err, "get workspace path from session name")
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return "", pkgerrors.Wrap(err, "resolve workspace path")
	}
	return abs, nil
}

// cleanupWorkspaceForSession removes the workspace directory and its git worktree.
func (k Remuda) cleanupWorkspaceForSession(sessionName string) error {
	ws, err := k.workspacePathForSession(sessionName)
	if err != nil {
		return err
	}
	return k.RemoveWorkspace(ws, false, true)
}

func (k Remuda) closeBDIssue(workspacePath string) bool {
	logger := k.Logger
	// determine the git branch at the workspace path
	branchName, err := util.RunCmdOutput(logger, "git", "-C", workspacePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		logger.Warn().Err(err).Msg("unable to determine git branch for beads issue closure")
		return false
	}

	// trim whitespace/newline from branch name since cmd output may contain it
	branchName = strings.TrimSpace(branchName)

	cmdEnv := env.Environ(k.envProvider())
	err = util.RunCmdWithEnv(logger, cmdEnv, "br", "close", branchName)
	if err != nil {
		logger.Warn().Err(err).Str("issue_id", branchName).Msg("unable to close beads issue")
		return false
	}

	k.IO.Outf("Closed beads issue %q\n", branchName)
	return true
}
