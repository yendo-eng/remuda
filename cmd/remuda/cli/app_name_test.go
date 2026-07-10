package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeCLIName(t *testing.T) {
	t.Parallel()

	require.Equal(t, defaultCLIName, normalizeCLIName(""))
	require.Equal(t, defaultCLIName, normalizeCLIName("   "))
	require.Equal(t, "remuda", normalizeCLIName("/usr/local/bin/remuda"))
	require.Equal(t, "remuda", normalizeCLIName("./remuda"))
}
