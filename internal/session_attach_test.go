package internal

import (
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

