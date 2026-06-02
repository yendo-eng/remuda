package git

import (
	"fmt"
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
)

func ValidateBranchName(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}

	out, err := util.RunCmdCombinedOutput("git", "check-ref-format", "--branch", branch)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg != "" {
			return fmt.Errorf("invalid branch name %q: %w (%s)", branch, err, msg)
		}
		return fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	return nil
}
