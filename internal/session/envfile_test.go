package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTmuxPaneEnvFile(t *testing.T) {
	t.Parallel()

	// These are the exact strings tmux sessions have always been started
	// with; change them only to change tmux behavior on purpose.
	env := []string{
		"OPENAI_API_KEY=it's secret",
		"TMUX=/tmp/tmux-501/default,1,0",
		"TMUX_PANE=%1",
		"TMUX_TMPDIR=/tmp/x",
		"BAD-NAME=x",
		"NOEQUALS",
		" =blank",
		"PATH=/usr/bin:/bin",
	}
	require.Equal(t,
		"export OPENAI_API_KEY='it'\\''s secret'\n"+
			"export TMUX_TMPDIR='/tmp/x'\n"+
			"export PATH='/usr/bin:/bin'\n",
		tmuxPaneEnv.envFileContents(env),
	)
	require.Equal(t,
		`exec env -i TERM="$TERM" TMUX="$TMUX" TMUX_PANE="$TMUX_PANE" USER="$USER" LOGNAME="$LOGNAME" bash -lc 'trap '\''rm -f -- '\''\'\'''\''/tmp/remuda-tmux-env-123'\''\'\'''\'''\'' EXIT; . '\''/tmp/remuda-tmux-env-123'\''; rm -f -- '\''/tmp/remuda-tmux-env-123'\''; cd '\''/w'\'' && codex; sleep 3600'`,
		tmuxPaneEnv.commandWithEnvFile("/tmp/remuda-tmux-env-123", "cd '/w' && codex; sleep 3600"),
	)
}
