package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexLauncher_ReasoningLevelUsesConfigFlag(t *testing.T) {
	t.Parallel()

	for _, level := range []string{"high", "max", "ultra"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()

			l := Codex("gpt-5.6-sol", false, level)
			cmd := l.Command("do stuff")
			require.Contains(t, cmd, "--config model_reasoning_effort='"+level+"'")
		})
	}
}

func TestCodexLauncher_EmptyReasoningLevelOmitsConfigFlag(t *testing.T) {
	l := Codex("gpt-5.2", false, "")
	cmd := l.Command("do stuff")
	require.NotContains(t, cmd, "model_reasoning_effort")
}
