package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateReasoningLevel_CodexValid(t *testing.T) {
	for _, level := range CodexReasoningLevels {
		require.NoError(t, ValidateReasoningLevel("codex", "gpt-5", level), "level %q should be valid", level)
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
	require.Empty(t, SupportedReasoningLevels("opencode", "openai/gpt-5"))
}

func TestSuggestedReasoningLevels_Claude(t *testing.T) {
	require.Equal(t, ClaudeEffortLevels, SuggestedReasoningLevels("claude", ""))
}
