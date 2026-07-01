package session_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestTmuxReadBufferTruncatesToLastNonEmptyLines(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell script stub")
	}

	tmp := t.TempDir()
	tmuxPath := filepath.Join(tmp, "tmux")
	script := `#!/bin/sh
cmd="$1"
if [ "$cmd" = "list-sessions" ]; then
  printf '%s\n' 'org/repo/feat 0 1710000000'
  exit 0
fi
	if [ "$cmd" = "capture-pane" ]; then
	  printf 'line1\nline2\nline3\nline4\n\n\n'
	  exit 0
	fi
>&2 printf 'unexpected args: %s\n' "$*"
exit 1
`
	require.NoError(t, os.WriteFile(tmuxPath, []byte(script), 0o755))

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath)

	mgr := session.NewTmuxManager()

	got, err := mgr.ReadBuffer("org/repo/feat", 1)
	require.NoError(t, err)
	require.Equal(t, "line4", got)

	lastTwo, err := mgr.ReadBuffer("org/repo/feat", 2)
	require.NoError(t, err)
	require.Equal(t, "line3\nline4", lastTwo)

	full, err := mgr.ReadBuffer("org/repo/feat", 0)
	require.NoError(t, err)
	require.Equal(t, "line1\nline2\nline3\nline4\n\n\n", full)
}
