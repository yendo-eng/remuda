package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

// For `remuda vibe`, --name is optional: it is derived later at runtime.
func TestVibe_AllowsMissingName(t *testing.T) {
	t.Parallel()
	parser, _ := testutils.RemudaParser(t)

	// Only parse to validate flags. We don't execute Run() here.
	_, err := parser.Parse([]string{
		"vibe",
		"--agent-cmd", "true",
		"hello world",
	})

	require.NoError(t, err, "expected --name to be optional for vibe")
}
