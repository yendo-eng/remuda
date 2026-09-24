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
	maxOnly := append(append([]string(nil), standard...), "max")
	tests := []struct {
		name  string
		model string
		want  []string
	}{
		{name: "gpt 5.6 sol", model: "gpt-5.6-sol", want: CodexReasoningLevels},
		{name: "gpt 5.6 terra", model: "gpt-5.6-terra", want: CodexReasoningLevels},
		{name: "gpt 5.6 luna", model: "gpt-5.6-luna", want: maxOnly},
		{name: "gpt 6 astra", model: "gpt-6-astra", want: CodexReasoningLevels},
		{name: "gpt 6 sol", model: "gpt-6-sol", want: CodexReasoningLevels},
		{name: "gpt 6 luna", model: "gpt-6-luna", want: maxOnly},
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

func TestValidateReasoningLevel_CodexSupportsModelEfforts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model string
		level string
	}{
		{model: "gpt-5.6-sol", level: "max"},
		{model: "gpt-5.6-sol", level: "ultra"},
		{model: "gpt-5.6-luna", level: "max"},
		{model: "gpt-6-astra", level: "max"},
		{model: "gpt-6-astra", level: "ultra"},
		{model: "gpt-6-sol", level: "ultra"},
		{model: "gpt-6-luna", level: "max"},
	}
	for _, tt := range tests {
		t.Run(tt.model+"/"+tt.level, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, ValidateReasoningLevel("codex", tt.model, tt.level))
		})
	}
}

func TestValidateReasoningLevel_CodexRejectsUltraForLunaModels(t *testing.T) {
	t.Parallel()

	for _, model := range []string{"gpt-5.6-luna", "gpt-6-luna"} {
		t.Run(model, func(t *testing.T) {
			t.Parallel()

			require.Error(t, ValidateReasoningLevel("codex", model, "ultra"))
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
