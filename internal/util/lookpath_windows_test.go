//go:build windows

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasPathSeparatorWindows(t *testing.T) {
	require.True(t, hasPathSeparator(`foo\bar`))
	require.True(t, hasPathSeparator("foo/bar"))
	require.False(t, hasPathSeparator("foobar"))
}
