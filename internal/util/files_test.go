package util_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/util"
)

func TestCopyDirCopiesContents(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("hello"), 0o644))
	nested := filepath.Join(src, "nested")
	require.NoError(t, os.Mkdir(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "child.txt"), []byte("child"), 0o600))

	dst := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, util.CopyDir(src, dst))

	content, err := os.ReadFile(filepath.Join(dst, "root.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(content))

	child, err := os.ReadFile(filepath.Join(dst, "nested", "child.txt"))
	require.NoError(t, err)
	require.Equal(t, "child", string(child))
}

func TestCopyDirCopiesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "root.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.Symlink("root.txt", filepath.Join(src, "root-link")))
	dst := filepath.Join(t.TempDir(), "copy")

	require.NoError(t, util.CopyDir(src, dst))

	info, err := os.Lstat(filepath.Join(dst, "root-link"))
	require.NoError(t, err)
	require.True(t, info.Mode()&os.ModeSymlink != 0)
	target, err := os.Readlink(filepath.Join(dst, "root-link"))
	require.NoError(t, err)
	require.Equal(t, "root.txt", target)
	content, err := os.ReadFile(filepath.Join(dst, "root-link"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(content))
}

func TestCopyDirErrorsWhenSourceNotDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("abc"), 0o644))
	dst := filepath.Join(t.TempDir(), "out")
	require.Error(t, util.CopyDir(file, dst))
}

func TestCopyDirSkipsSockets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows")
	}

	src := t.TempDir()
	socketPath := filepath.Join(src, "bd.sock")
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, listener.Close())
	}()

	dst := filepath.Join(t.TempDir(), "out")
	require.NoError(t, util.CopyDir(src, dst))

	_, err = os.Stat(filepath.Join(dst, "bd.sock"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
