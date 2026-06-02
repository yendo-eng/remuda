package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionAttach_EmitsTerminalTitle(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	sessionName := "org/repo/feat"

	sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	require.NoError(t, sessionMgr.Start(sessionName, "echo hi"))

	res := h.RunOK("session", "attach", "--name", sessionName)
	require.Equal(t, "\x1b]2;"+sessionName+"\x07", res.Stderr)
}
