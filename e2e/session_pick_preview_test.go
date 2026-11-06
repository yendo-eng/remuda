package e2e_test

import (
	"bytes"
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

// fzfPreviewStub is a stand-in for fzf. As in fzf, the --preview command is run
// for the first row with {} replaced by the single-quoted row, and its output
// is logged next to the stub. Then that row is selected.
const fzfPreviewStub = `#!/bin/sh
read -r row
while [ $# -gt 0 ]; do
	if [ "$1" = --preview ]; then preview=$2; fi
	shift
done
FZF_PREVIEW_LINES=10 sh -c "$(printf '%s' "$preview" | sed "s|{}|'$row'|g")" >"${0%/*}/preview.log" 2>&1
printf '%s\n' "$row"
`

func TestSessionPickPreviewsTmuxFirstPane(t *testing.T) {
	t.Parallel()

	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not available")
	}

	baseEnv := testutils.ProcessEnvMap()
	bin := filepath.Join(t.TempDir(), "remuda")
	buildRemudaBinary(t, repoRootDir(t), bin, baseEnv, false)

	// The tmux socket, and herdr-core's under XDG_STATE_HOME, have to fit
	// macOS's 103-byte cap, which t.TempDir and the suite's HOME can exceed.
	socketDir, err := os.MkdirTemp("", "sock")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	env := map[string]string{"TMUX_TMPDIR": socketDir, "XDG_STATE_HOME": socketDir}
	tmux := func(ctx context.Context, args ...string) (string, error) {
		//nolint:gosec // G204: realTmux is the tmux on PATH, and args are test-owned.
		cmd := exec.CommandContext(ctx, realTmux, args...)
		if err := testutils.ApplyE2EEnvIsolationToCmd(cmd, baseEnv, env); err != nil {
			return "", err
		}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := tmux(t.Context(), "-f", os.DevNull, "new-session", "-d", "-s", "org/repo/agent", "echo agent pane; exec cat")
	require.NoError(t, err, out)
	t.Cleanup(func() { _, _ = tmux(context.Background(), "kill-server") })
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		out, err := tmux(t.Context(), "capture-pane", "-p", "-t", "=org/repo/agent:")
		assert.NoError(c, err, out)
		assert.Contains(c, out, "agent pane")
	}, 10*time.Second, 50*time.Millisecond)

	// As when a shell is opened next to the agent, a second window is made
	// current.
	out, err = tmux(t.Context(), "new-window", "-t", "=org/repo/agent:", "exec cat")
	require.NoError(t, err, out)

	for _, sessionManager := range []string{"tmux", "zellij"} {
		t.Run(sessionManager+" session manager", func(t *testing.T) {
			t.Parallel()

			stubDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(stubDir, "fzf"), []byte(fzfPreviewStub), 0o755))
			// With --pick, stdin and stdout have to be character devices, as a
			// terminal is, so /dev/null is used.
			devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
			require.NoError(t, err)
			t.Cleanup(func() { _ = devNull.Close() })

			//nolint:gosec // G204: bin is the remuda built by this test, and args are test-owned.
			cmd := exec.CommandContext(t.Context(), bin, "--session-manager", sessionManager, "session", "path", "--pick")
			overrides := map[string]string{"PATH": stubDir + string(os.PathListSeparator) + baseEnv["PATH"]}
			maps.Copy(overrides, env)
			require.NoError(t, testutils.ApplyE2EEnvIsolationToCmd(cmd, baseEnv, overrides))
			var stderr bytes.Buffer
			cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, &stderr
			require.NoError(t, cmd.Run(), stderr.String())

			preview, err := os.ReadFile(filepath.Join(stubDir, "preview.log"))
			require.NoError(t, err)
			require.Contains(t, string(preview), "agent pane")
		})
	}
}
