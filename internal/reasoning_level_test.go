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
	t.Parallel()

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

func TestResolveReasoningLevel_CodexWarnsAndPassesThroughUnsupportedLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		level string
	}{
		{name: "high tier on older model", model: "gpt-5.5", level: "max"},
		{name: "unknown level", model: "gpt-5.6-sol", level: "turbo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			got, err := resolveReasoningLevel(
				zerolog.New(&logs),
				string(agentlauncher.AgentCodex),
				tt.model,
				"",
				tt.level,
			)
			require.NoError(t, err)
			require.Equal(t, tt.level, got)
			require.Contains(t, logs.String(), "reasoning level may not be supported by codex model")
			require.Contains(t, logs.String(), tt.model)
			require.Contains(t, logs.String(), tt.level)
		})
	}
}

func TestResolveReasoningLevel_CodexSupportsGPT56HighTierWithoutWarning(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	got, err := resolveReasoningLevel(
		zerolog.New(&logs),
		string(agentlauncher.AgentCodex),
		"gpt-5.6-luna",
		"",
		"max",
	)
	require.NoError(t, err)
	require.Equal(t, "max", got)
	require.Empty(t, logs.String())
}
