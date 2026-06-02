package util_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/util"
)

func TestSSHRewriteSnippet_HasKeyPieces(t *testing.T) {
	s := util.SSHRewriteSnippet()
	// Minimal assertions to avoid coupling to exact whitespace
	require.Contains(t, s, "git remote get-url origin")
	require.Contains(t, s, "https://github.com/")
	require.Contains(t, s, "git remote set-url origin \"git@github.com:")
}
