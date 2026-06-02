package cli

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCLIName(t *testing.T) {
	t.Parallel()

	require.Equal(t, defaultCLIName, normalizeCLIName(""))
	require.Equal(t, defaultCLIName, normalizeCLIName("   "))
	require.Equal(t, "remuda", normalizeCLIName("/usr/local/bin/remuda"))
	require.Equal(t, "remuda", normalizeCLIName("./remuda"))
}

func TestInvokedCLIName(t *testing.T) {
	t.Parallel()

	t.Run("defaults when context is nil", func(t *testing.T) {
		require.Equal(t, defaultCLIName, invokedCLIName(nil))
	})

	t.Run("defaults when context model is missing", func(t *testing.T) {
		require.Equal(t, defaultCLIName, invokedCLIName(&kong.Context{}))
	})

	t.Run("uses kong application name", func(t *testing.T) {
		kctx := &kong.Context{
			Kong: &kong.Kong{
				Model: &kong.Application{
					Node: &kong.Node{Name: "remuda"},
				},
			},
		}
		require.Equal(t, "remuda", invokedCLIName(kctx))
	})
}
