package util_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/util"
)

// CoWCopyDir must produce the same tree whether or not the filesystem under
// the test can clone, so the assertions never depend on cloning succeeding.
func TestCoWCopyDirCopiesTree(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("hello"), 0o644))
	nested := filepath.Join(src, "nested")
	require.NoError(t, os.Mkdir(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "child.txt"), []byte("child"), 0o600))

	dst := filepath.Join(t.TempDir(), "copy")
	_, err := util.CoWCopyDir(src, dst)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dst, "root.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(content))

	child, err := os.ReadFile(filepath.Join(dst, "nested", "child.txt"))
	require.NoError(t, err)
	require.Equal(t, "child", string(child))

	info, err := os.Stat(filepath.Join(dst, "nested", "child.txt"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode()&os.ModePerm)
}

func TestCoWCopyDirOverwritesExistingFiles(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("new"), 0o644))

	dst := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dst, "root.txt"), []byte("stale"), 0o644))

	_, err := util.CoWCopyDir(src, dst)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dst, "root.txt"))
	require.NoError(t, err)
	require.Equal(t, "new", string(content))
}
