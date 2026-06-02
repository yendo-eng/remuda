package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexLauncher_Command_EmptyPromptOmitsPositionalArg(t *testing.T) {
	l := Codex("gpt-5.1-codex", false, "")
	cmd := l.Command("")
	require.NotContains(t, cmd, " ''")
	require.NotContains(t, cmd, "''")
	require.NotContains(t, cmd, " -- '")
}

func TestOpenCodeLauncher_Command_EmptyPromptOmitsPromptFlag(t *testing.T) {
	l := OpenCode("gpt-4o")
	cmd := l.Command("")
	require.Equal(t, "opencode --model 'gpt-4o'", cmd)
}

func TestCustomLauncher_Command_EmptyPromptOmitsPositionalArg(t *testing.T) {
	l := Custom("true")
	cmd := l.Command("")
	require.Equal(t, "true", cmd)
}
