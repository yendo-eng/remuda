package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTmpContainerMountError(t *testing.T) {
	home := filepath.FromSlash("/Users/dev")
	osTmp := filepath.FromSlash("/var/folders/xy/remuda")
	homeTmp := filepath.Join(home, ".remuda", "tmp")

	t.Run("darwin temp worktree under OS temp dir fails", func(t *testing.T) {
		ws := filepath.Join(osTmp, "org", "repo", "wk")
		err := tmpContainerMountError(ws, osTmp, home, "darwin")
		require.Error(t, err)
		require.Contains(t, err.Error(), "REMUDA_TMP_DIR")
		require.Contains(t, err.Error(), "File Sharing")
	})

	t.Run("darwin temp worktree under home is allowed", func(t *testing.T) {
		ws := filepath.Join(homeTmp, "org", "repo", "wk")
		require.NoError(t, tmpContainerMountError(ws, homeTmp, home, "darwin"))
	})

	t.Run("non-darwin always allowed", func(t *testing.T) {
		ws := filepath.Join(osTmp, "org", "repo", "wk")
		require.NoError(t, tmpContainerMountError(ws, osTmp, home, "linux"))
	})

	t.Run("non-tmp workspace is not affected", func(t *testing.T) {
		repos := filepath.FromSlash("/Users/dev/.remuda/repos")
		ws := filepath.Join(repos, "org", "repo", "wk")
		require.NoError(t, tmpContainerMountError(ws, osTmp, home, "darwin"))
	})

	t.Run("no temp base dir is a no-op", func(t *testing.T) {
		ws := filepath.Join(osTmp, "org", "repo", "wk")
		require.NoError(t, tmpContainerMountError(ws, "", home, "darwin"))
	})
}
