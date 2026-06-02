package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionEdit(t *testing.T) {
	t.Parallel()
	t.Run("launches configured editor", func(t *testing.T) {
		h := testutils.NewHarness(t)

		baseDir := h.RemudaConfig.ReposBaseDir
		sessionName := "org/repo/feat alpha"
		workspace := filepath.Join(baseDir, "org", "repo", "feat alpha")
		require.NoError(t, os.MkdirAll(workspace, 0o755))

		sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
		require.True(t, ok)
		require.NoError(t, sessionMgr.Start(sessionName, "echo"))

		scriptDir := t.TempDir()
		scriptPath := filepath.Join(scriptDir, "editor.sh")
		script := []byte("#!/bin/sh\necho \"EDIT:$1\"\n")
		require.NoError(t, os.WriteFile(scriptPath, script, 0o755))

		h.SetEnv("REMUDA_EDITOR", scriptPath)
		h.SetEnv("VISUAL", "")
		h.SetEnv("EDITOR", "")
		h.SetEnv("SHELL", "/bin/sh")

		res := h.RunOK("session", "edit", "--name", sessionName)
		require.Equal(t, "EDIT:"+workspace+"\n", res.Stdout)
	})

	t.Run("errors when editor unset", func(t *testing.T) {
		h := testutils.NewHarness(t)

		baseDir := h.RemudaConfig.ReposBaseDir
		workspace := filepath.Join(baseDir, "org", "repo", "feat")
		require.NoError(t, os.MkdirAll(workspace, 0o755))
		sessionName := "org/repo/feat"

		sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
		require.True(t, ok)
		require.NoError(t, sessionMgr.Start(sessionName, "echo"))

		h.SetEnv("REMUDA_EDITOR", "")
		h.SetEnv("VISUAL", "")
		h.SetEnv("EDITOR", "")

		res := h.Run("session", "edit", "--name", sessionName)
		require.ErrorContains(t, res.Err, "no editor configured")
	})
}
