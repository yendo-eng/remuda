package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionShellNameOrPickRequired(t *testing.T) {
	t.Parallel()
	err := runCLI(t, "session", "shell")
	require.ErrorContains(t, err, "at least one of the flags in the group [name pick] is required")
}
