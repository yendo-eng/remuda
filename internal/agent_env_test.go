package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemudaAgentEnvPrefix_DefaultValues(t *testing.T) {
	prefix := remudaAgentEnvPrefix("codex", "openai/gpt-5")
	require.Equal(t, "REMUDA_AGENT='codex' REMUDA_MODEL='openai/gpt-5'", prefix)
}

func TestRemudaAgentEnvPrefix_QuotesValues(t *testing.T) {
	prefix := remudaAgentEnvPrefix("agent'withquote", "")
	require.Equal(t, "REMUDA_AGENT='agent'\\''withquote'", prefix)
}
