package git

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/util"
)

func ValidateBranchName(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return pkgerrors.Errorf("branch name is required")
	}

	out, err := util.RunCmdCombinedOutput("git", "check-ref-format", "--branch", branch)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg != "" {
			return pkgerrors.Wrapf(err, "invalid branch name %q (%s)", branch, msg)
		}
		return pkgerrors.Wrapf(err, "invalid branch name %q", branch)
	}
	return nil
}
