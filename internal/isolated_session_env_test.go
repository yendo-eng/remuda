package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsolatedContainerEnvNames_IncludesImplicitAnthropicForClaude(t *testing.T) {
	got := isolatedContainerEnvNames("claude", nil)
	require.Equal(t, []string{"ANTHROPIC_API_KEY"}, got)
}

func TestIsolatedContainerEnvNames_MergesAndDedupesRequestedNames(t *testing.T) {
	got := isolatedContainerEnvNames("bash", []string{
		" AWS_REGION ",
		"ANTHROPIC_API_KEY",
		"",
		"BAD=NOPE",
		"AWS_REGION",
	})
	require.Equal(t, []string{"AWS_REGION", "ANTHROPIC_API_KEY"}, got)
}

func TestIsolatedContainerEnvNames_OmitsImplicitAnthropicForCodex(t *testing.T) {
	got := isolatedContainerEnvNames("codex", []string{"GITHUB_TOKEN"})
	require.Equal(t, []string{"GITHUB_TOKEN"}, got)
}
