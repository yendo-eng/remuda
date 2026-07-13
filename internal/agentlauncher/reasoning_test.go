package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateReasoningLevel_CodexValid(t *testing.T) {
	t.Parallel()

	for _, level := range []string{"none", "minimal", "low", "medium", "high", "xhigh"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, ValidateReasoningLevel("codex", "gpt-5.5", level))
		})
	}
}

func TestSupportedReasoningLevels_CodexModelAware(t *testing.T) {
	t.Parallel()

	standard := []string{"none", "minimal", "low", "medium", "high", "xhigh"}
	tests := []struct {
		name  string
		model string
		want  []string
	}{
		{name: "gpt 5.6 series", model: "gpt-5.6-sol", want: CodexReasoningLevels},
		{name: "older codex model", model: "gpt-5.5", want: standard},
		{name: "legacy codex model", model: "gpt-5-codex", want: standard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, SupportedReasoningLevels("codex", tt.model))
		})
	}
}

func TestValidateReasoningLevel_CodexSupportsGPT56Efforts(t *testing.T) {
	t.Parallel()

	for _, level := range []string{"max", "ultra"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, ValidateReasoningLevel("codex", "gpt-5.6-sol", level))
		})
	}
}

func TestValidateReasoningLevel_CodexInvalid(t *testing.T) {
	err := ValidateReasoningLevel("codex", "gpt-5", "super")
	require.Error(t, err)
	require.Contains(t, err.Error(), "codex")
	require.Contains(t, err.Error(), "gpt-5")
	require.Contains(t, err.Error(), "valid values")
}

func TestSupportedReasoningLevels_UnsupportedAgent(t *testing.T) {
	t.Parallel()

	require.Empty(t, SupportedReasoningLevels("opencode", "openai/gpt-5"))
}

func TestSuggestedReasoningLevels_Claude(t *testing.T) {
	t.Parallel()

	require.Equal(t, ClaudeEffortLevels, SuggestedReasoningLevels("claude", ""))
}
