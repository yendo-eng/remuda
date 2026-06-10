package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/logging"
)

func TestBuildOpenCodeStateMountOpts_LinuxPath(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".local", "share", "opencode")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	opts := buildOpenCodeStateMountOpts(logging.NewDisabledLogger(), "linux", tmp)
	require.Equal(t, []string{fmt.Sprintf("-v %q:%q:rw", stateDir, "/root/.local/share/opencode")}, opts)
}

func TestBuildOpenCodeStateMountOpts_MacPath(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "Library", "Application Support", "opencode")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	opts := buildOpenCodeStateMountOpts(logging.NewDisabledLogger(), "darwin", tmp)
	require.Equal(t, []string{fmt.Sprintf("-v %q:%q:rw", stateDir, "/root/.local/share/opencode")}, opts)
}

func TestBuildOpenCodeStateMountOpts_CreatesPreferredDir(t *testing.T) {
	t.Run("linux", func(t *testing.T) {
		tmp := t.TempDir()
		stateDir := filepath.Join(tmp, ".local", "share", "opencode")

		opts := buildOpenCodeStateMountOpts(logging.NewDisabledLogger(), "linux", tmp)
		require.Equal(t, []string{fmt.Sprintf("-v %q:%q:rw", stateDir, "/root/.local/share/opencode")}, opts)
		require.DirExists(t, stateDir)
	})

	t.Run("darwin", func(t *testing.T) {
		tmp := t.TempDir()
		stateDir := filepath.Join(tmp, "Library", "Application Support", "opencode")

		opts := buildOpenCodeStateMountOpts(logging.NewDisabledLogger(), "darwin", tmp)
		require.Equal(t, []string{fmt.Sprintf("-v %q:%q:rw", stateDir, "/root/.local/share/opencode")}, opts)
		require.DirExists(t, stateDir)
	})
}
