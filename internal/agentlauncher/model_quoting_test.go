package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexLauncher_Model_IsShellQuoted(t *testing.T) {
	l := Codex("foo'bar; rm -rf /", false, "")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--model 'foo'\\''bar; rm -rf /'")
}

func TestOpenCodeLauncher_Model_IsShellQuoted(t *testing.T) {
	l := OpenCode("foo'bar; rm -rf /")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--model 'foo'\\''bar; rm -rf /'")
}
