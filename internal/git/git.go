package git

import (
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/util"
)

// Git defines an interface for git operations.
type Git interface {
	// Clone clones a git repository to the specified directory.
	Clone(repoURL, dir string) error

	// Pull pulls the latest changes in the specified directory.
	Pull(dir string) error

	// WorktreeAdd adds a new worktree at the specified path and branch.
	WorktreeAdd(dir, branch string, args ...string) error

	// WorktreeRemove removes a worktree at the specified path.
	WorktreeRemove(dir string, args ...string) error

	// Checkout checks out the specified branch in the given directory.
	Checkout(dir string, args ...string) error

	// ShowRef verifies if a ref exists.
	ShowRef(dir, ref string, opts ...string) error

	// RevParse resolves a git revision to a commit hash.
	RevParse(dir, rev string, opts ...string) (string, error)

	// Branch
	Branch(dir string, args ...string) error
}

type shellGit struct {
	logger zerolog.Logger
}

func NewShellGit(logger zerolog.Logger) Git {
	return &shellGit{logger: logger}
}

func (g *shellGit) Clone(repoURL, dir string) error {
	// Use -- to prevent repoURL from being interpreted as a git option.
	return util.RunCmd(g.logger, "git", "clone", "--", repoURL, dir)
}

func (g *shellGit) Pull(dir string) error {
	return util.RunCmd(g.logger, "git", "-C", dir, "pull")
}

func (g *shellGit) WorktreeAdd(baseDir, dir string, args ...string) error {
	return util.RunCmd(g.logger, "git", append([]string{"-C", baseDir, "worktree", "add", dir}, args...)...)
}

func (g *shellGit) WorktreeRemove(dir string, args ...string) error {
	args = append([]string{"-C", dir, "worktree", "remove"}, args...)
	return util.RunCmd(g.logger, "git", args...)
}

func (g *shellGit) Checkout(dir string, args ...string) error {
	return util.RunCmd(g.logger, "git", append([]string{"-C", dir, "checkout"}, args...)...)
}

func (g *shellGit) ShowRef(dir, ref string, opts ...string) error {
	args := []string{"-C", dir, "show-ref", ref}
	args = append(args, opts...)
	return util.RunCmd(g.logger, "git", args...)
}

func (g *shellGit) RevParse(dir, rev string, opts ...string) (string, error) {
	args := []string{"-C", dir, "rev-parse"}
	args = append(args, opts...)
	args = append(args, rev)
	return util.RunCmdOutput(g.logger, "git", args...)
}

func (g *shellGit) Branch(dir string, args ...string) error {
	args = append([]string{"-C", dir, "branch"}, args...)
	return util.RunCmd(g.logger, "git", args...)
}
