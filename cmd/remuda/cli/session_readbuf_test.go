package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionReadbufAllMutuallyExclusiveWithName(t *testing.T) {
	t.Parallel()

	parser, _ := testutils.RemudaParser(t)

	_, err := parser.Parse([]string{"session", "readbuf", "--all", "--name", "org/repo/feat1"})
	require.Error(t, err)
	require.ErrorContains(t, err, "can't be used together")
}
