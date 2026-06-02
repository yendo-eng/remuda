package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAbsPathFromContextExpandsTildeWithWorkingDir(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	workingDir := t.TempDir()
	ctx := Context{
		HomeDir:       home,
		homeDirSet:    true,
		WorkingDir:    workingDir,
		workingDirSet: true,
	}

	tildePath := filepath.Join("~", "repos", "acme", "widgets", "wk")
	got := absPathFromContext(tildePath, ctx)

	want := filepath.Join(home, "repos", "acme", "widgets", "wk")
	require.Equal(t, want, got)
}
