package internal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminalTitleSequence_StripsControlChars(t *testing.T) {
	t.Parallel()

	seq := terminalTitleSequence("ok\u009c\u009d\u001b\n\t\x7fbad✓")
	require.True(t, strings.HasPrefix(seq, "\x1b]2;"))
	require.True(t, strings.HasSuffix(seq, "\x07"))

	title := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b]2;"), "\x07")
	require.Equal(t, "okbad✓", title)
}

func TestSessionAttach_TerminalTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		terminalTitle string
		wantStderr    string
	}{
		{
			name:          "unset template matches current behavior",
			terminalTitle: "",
			wantStderr:    "\x1b]2;acme/example-repo/feat\x07",
		},
		{
			name:          "custom template",
			terminalTitle: "{repo}/{name}",
			wantStderr:    "\x1b]2;example-repo/feat\x07",
		},
		{
			name:          "off disables title",
			terminalTitle: "off",
			wantStderr:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			k := Remuda{
				Config:  Config{TerminalTitle: tt.terminalTitle},
				Session: &fakeSessionManager{},
				IO:      IO{Err: &stderr},
			}

			require.NoError(t, k.SessionAttach("acme/example-repo/feat"))
			require.Equal(t, tt.wantStderr, stderr.String())
		})
	}
}

