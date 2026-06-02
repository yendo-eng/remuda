package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportedAgents_ContainsClaude(t *testing.T) {
	require.Contains(t, SupportedAgents(), AgentClaude)
}

func TestIsSupportedAgent_Claude(t *testing.T) {
	require.True(t, IsSupportedAgent(string(AgentClaude)))
}

func TestParse_ClaudeLauncherNameIsClaude(t *testing.T) {
	launcher, _, err := Parse(string(AgentClaude), "", false)
	require.NoError(t, err)
	require.Equal(t, "claude", launcher.Name())
}
