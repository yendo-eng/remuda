package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionKillCmdValidateRejectsEmptyMergeFlags(t *testing.T) {
	t.Parallel()

	cmd := SessionKillCmd{
		SessionKillNamePickOption: SessionKillNamePickOption{Name: "acme-org/example-repo/test-session"},
		MergeFlag:                 []string{"--squash", " "},
	}

	err := cmd.Validate()
	require.ErrorContains(t, err, "--merge-flag[1] cannot be empty")
}
