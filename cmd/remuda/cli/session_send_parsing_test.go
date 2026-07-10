package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionSendParse_NameAndPickConflict(t *testing.T) {
	t.Parallel()
	err := runCLI(t, "session", "send", "--name", "org/repo/one", "--pick", "hello")
	require.ErrorContains(t, err, "--name and --pick")
}
