package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTmuxContainerEnvNames_IncludesImplicitAnthropicForClaude(t *testing.T) {
	got := tmuxContainerEnvNames("claude", nil)
	require.Equal(t, []string{"ANTHROPIC_API_KEY"}, got)
}

func TestTmuxContainerEnvNames_MergesAndDedupesRequestedNames(t *testing.T) {
	got := tmuxContainerEnvNames("bash", []string{
		" AWS_REGION ",
		"ANTHROPIC_API_KEY",
		"",
		"BAD=NOPE",
		"AWS_REGION",
	})
	require.Equal(t, []string{"AWS_REGION", "ANTHROPIC_API_KEY"}, got)
}

func TestTmuxContainerEnvNames_OmitsImplicitAnthropicForCodex(t *testing.T) {
	got := tmuxContainerEnvNames("codex", []string{"GITHUB_TOKEN"})
	require.Equal(t, []string{"GITHUB_TOKEN"}, got)
}
