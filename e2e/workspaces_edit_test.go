package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestWorkspacesEdit(t *testing.T) {
	t.Parallel()

	// editorScript points the editor env at a stub echoing the path it was handed.
	editorScript := func(t *testing.T, h *testutils.Harness) {
		t.Helper()
		scriptPath := filepath.Join(t.TempDir(), "editor.sh")
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho \"EDIT:$1\"\n"), 0o755))
		h.SetEnv("REMUDA_EDITOR", scriptPath)
		h.SetEnv("VISUAL", "")
		h.SetEnv("EDITOR", "")
		h.SetEnv("SHELL", "/bin/sh")
	}

	t.Run("launches configured editor for an identifier", func(t *testing.T) {
		h := testutils.NewHarness(t)

		workspace := filepath.Join(h.RemudaConfig.ReposBaseDir, "org", "repo", "feat alpha")
		require.NoError(t, os.MkdirAll(workspace, 0o755))
		editorScript(t, h)

		res := h.RunOK("workspaces", "edit", "org/repo/feat alpha")
		require.Equal(t, "EDIT:"+workspace+"\n", res.Stdout)
	})

	t.Run("launches configured editor for an absolute path", func(t *testing.T) {
		h := testutils.NewHarness(t)

		workspace := filepath.Join(h.RemudaConfig.ReposBaseDir, "org", "repo", "feat")
		require.NoError(t, os.MkdirAll(workspace, 0o755))
		editorScript(t, h)

		res := h.RunOK("workspaces", "edit", workspace)
		require.Equal(t, "EDIT:"+workspace+"\n", res.Stdout)
	})

	t.Run("edits an active workspace", func(t *testing.T) {
		h := testutils.NewHarness(t)

		sessionName := "org/repo/active"
		workspace := filepath.Join(h.RemudaConfig.ReposBaseDir, "org", "repo", "active")
		require.NoError(t, os.MkdirAll(workspace, 0o755))
		sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
		require.True(t, ok)
		require.NoError(t, sessionMgr.Start(sessionName, "echo"))
		editorScript(t, h)

		res := h.RunOK("workspaces", "edit", sessionName)
		require.Equal(t, "EDIT:"+workspace+"\n", res.Stdout)
	})

	t.Run("errors when the workspace is missing", func(t *testing.T) {
		h := testutils.NewHarness(t)
		editorScript(t, h)

		res := h.Run("workspaces", "edit", "org/repo/nope")
		require.ErrorContains(t, res.Err, "stat workspace")
	})

	t.Run("errors when editor unset", func(t *testing.T) {
		h := testutils.NewHarness(t)

		workspace := filepath.Join(h.RemudaConfig.ReposBaseDir, "org", "repo", "feat")
		require.NoError(t, os.MkdirAll(workspace, 0o755))

		h.SetEnv("REMUDA_EDITOR", "")
		h.SetEnv("VISUAL", "")
		h.SetEnv("EDITOR", "")

		res := h.Run("workspaces", "edit", "org/repo/feat")
		require.ErrorContains(t, res.Err, "no editor configured")
	})
}
