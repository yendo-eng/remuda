package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
)

func TestAbsPathFromContextExpandsTildeWithWorkingDir(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	workingDir := t.TempDir()
	ctx := NewContext(
		t.Context(),
		internal.Remuda{},
		WithHomeDir(home),
		WithWorkingDir(workingDir),
	)

	tildePath := filepath.Join("~", "repos", "acme", "widgets", "wk")
	got := absPathFromContext(tildePath, ctx)

	want := filepath.Join(home, "repos", "acme", "widgets", "wk")
	require.Equal(t, want, got)
}
