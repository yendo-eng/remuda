package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBashLauncher_CommandAndVersion(t *testing.T) {
	b := Bash()
	require.Equal(t, "bash -l", b.Command("ignored"))
}

func TestBashLauncher_Command_AppendsExtraArgs(t *testing.T) {
	b := Bash()
	require.Equal(t, "bash -l '--noprofile'", b.Command("ignored", "--noprofile"))
}

func TestParse_Bash_PropagatesModel(t *testing.T) {
	_, model, err := Parse(string(AgentBash), "whatever", false)
	require.NoError(t, err)
	require.Equal(t, "whatever", model)
}
