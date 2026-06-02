package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexLauncher_ReasoningLevelUsesConfigFlag(t *testing.T) {
	l := Codex("gpt-5.2", false, "high")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--config model_reasoning_effort='high'")
}

func TestCodexLauncher_EmptyReasoningLevelOmitsConfigFlag(t *testing.T) {
	l := Codex("gpt-5.2", false, "")
	cmd := l.Command("do stuff")
	require.NotContains(t, cmd, "model_reasoning_effort")
}
