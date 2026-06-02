package session_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestTmuxAttach_ResolvesDotsToUnderscores(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "tmux-args.txt")

	name := "tmux"
	if runtime.GOOS == "windows" {
		name = "tmux.bat"
	}
	tmuxPath := filepath.Join(tmp, name)

	script := strings.Join([]string{
		"#!/bin/sh",
		"set -e",
		`cmd="$1"`,
		`if [ "$cmd" = "list-sessions" ]; then`,
		`  echo "acme/remuda/session-123_6-code-review 0"`,
		"  exit 0",
		"fi",
		`if [ "$cmd" = "attach" ]; then`,
		`  echo "$@" > "$TMUX_ARGS_FILE"`,
		"  exit 0",
		"fi",
		"exit 0",
	}, "\n") + "\n"

	require.NoError(t, os.WriteFile(tmuxPath, []byte(script), 0o755))

	old := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+old)
	t.Setenv("TMUX_ARGS_FILE", argsFile)

	mgr := session.NewTmuxManager()
	require.NoError(t, mgr.Attach("acme/remuda/session-123.6-code-review"))

	got, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	require.Equal(t, "attach -t acme/remuda/session-123_6-code-review:\n", string(got))
}
