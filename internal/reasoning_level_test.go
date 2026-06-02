package internal

import (
	"bytes"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
)

func TestResolveReasoningLevel_ClaudePreservesValue(t *testing.T) {
	got, err := resolveReasoningLevel(
		zerolog.New(io.Discard),
		string(agentlauncher.AgentClaude),
		"",
		"",
		" high ",
	)
	require.NoError(t, err)
	require.Equal(t, "high", got)
}

func TestResolveReasoningLevel_ClaudeAllowsArbitraryEffortValue(t *testing.T) {
	got, err := resolveReasoningLevel(
		zerolog.New(io.Discard),
		string(agentlauncher.AgentClaude),
		"",
		"",
		" turbo-plus ",
	)
	require.NoError(t, err)
	require.Equal(t, "turbo-plus", got)
}

func TestResolveReasoningLevel_WhitespaceValueIsIgnored(t *testing.T) {
	got, err := resolveReasoningLevel(
		zerolog.New(io.Discard),
		string(agentlauncher.AgentClaude),
		"",
		"",
		" \t\n ",
	)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestResolveReasoningLevel_AgentCmdBypassesBuiltInReasoning(t *testing.T) {
	got, err := resolveReasoningLevel(
		zerolog.New(io.Discard),
		string(agentlauncher.AgentClaude),
		"",
		"custom-agent --run",
		"high",
	)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestResolveReasoningLevel_OpenCodeWarnsAndIgnores(t *testing.T) {
	var logs bytes.Buffer
	got, err := resolveReasoningLevel(
		zerolog.New(&logs),
		string(agentlauncher.AgentOpenCode),
		"openai/gpt-5",
		"",
		"high",
	)
	require.NoError(t, err)
	require.Equal(t, "", got)
	require.Contains(t, logs.String(), "reasoning-level ignored")
}

func TestResolveReasoningLevel_CodexValidationStillApplies(t *testing.T) {
	_, err := resolveReasoningLevel(
		zerolog.New(io.Discard),
		string(agentlauncher.AgentCodex),
		"gpt-5",
		"",
		"turbo",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reasoning-level")
}
