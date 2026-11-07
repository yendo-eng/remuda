package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionAttach_EmitsTerminalTitle(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	sessionName := "org/repo/feat"

	sessionMgr, ok := h.Multiplexer.(*testutils.MockMultiplexer)
	require.True(t, ok)
	require.NoError(t, sessionMgr.Start(session.AgentStart{SessionName: sessionName, Command: "echo hi"}))

	res := h.RunOK("session", "attach", "--name", sessionName)
	require.Equal(t, "\x1b]2;"+sessionName+"\x07", res.Stderr)
}
