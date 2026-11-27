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

func TestOpenCodeLauncher_Command_OmitsModel(t *testing.T) {
	l := OpenCode("foo'bar; rm -rf /#variant")
	require.Equal(t, "opencode --prompt 'do stuff'", l.Command("do stuff"))
}

func TestOpenCodeLauncher_EnvironmentEncodesModelAsJSON(t *testing.T) {
	t.Parallel()

	l := OpenCode("foo'bar; rm -rf /#variant")
	require.Equal(t, map[string]string{
		"OPENCODE_CONFIG_CONTENT": `{"model":"foo'bar; rm -rf /#variant"}`,
	}, l.Environment())
}
