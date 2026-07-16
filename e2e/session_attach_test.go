package e2e_test

import (
	"os"
	"path/filepath"
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

func TestSessionAttach_TerminalTitleFromConfig(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	sessionName := "org/repo/feat"

	sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	require.NoError(t, sessionMgr.Start(sessionName, "echo hi"))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("version: 1\nsession:\n  terminal_title: \"{repo}/{name}\"\n"), 0o644))
	h.SetEnv("REMUDA_CONFIG", configPath)

	res := h.RunOK("session", "attach", "--name", sessionName)
	require.Equal(t, "\x1b]2;repo/feat\x07", res.Stderr)
}

func TestSessionAttach_TerminalTitleOffFromEnv(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	sessionName := "org/repo/feat"

	sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	require.NoError(t, sessionMgr.Start(sessionName, "echo hi"))

	h.SetEnv("REMUDA_TERMINAL_TITLE", "off")

	res := h.RunOK("session", "attach", "--name", sessionName)
	require.Empty(t, res.Stderr)
}

func TestSessionAttach_TerminalTitleUnknownPlaceholderRejected(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	sessionName := "org/repo/feat"

	sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	require.NoError(t, sessionMgr.Start(sessionName, "echo hi"))

	h.SetEnv("REMUDA_TERMINAL_TITLE", "{branch}")

	res := h.Run("session", "attach", "--name", sessionName)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "session.terminal_title")
	require.Contains(t, res.Err.Error(), "{branch}")
}
