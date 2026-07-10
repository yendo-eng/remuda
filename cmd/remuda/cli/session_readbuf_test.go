package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionReadbufAllMutuallyExclusiveWithName(t *testing.T) {
	t.Parallel()
	err := runCLI(t, "session", "readbuf", "--all", "--name", "org/repo/feat1")
	require.ErrorContains(t, err, "none of the others can be")
}
