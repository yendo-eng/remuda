package llm

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// Opt-in integration test. Skipped unless REMUDA_LLM_OPENAI_IT=1 and OPENAI_API_KEY is set.
func TestOpenAIIntegration_Slugify(t *testing.T) {
	if os.Getenv("REMUDA_LLM_OPENAI_IT") != "1" {
		t.Skip("integration disabled: set REMUDA_LLM_OPENAI_IT=1 to run")
	}
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("REMUDA_OPENAI_API_KEY") == "" {
		t.Skip("missing OPENAI_API_KEY/REMUDA_OPENAI_API_KEY; skipping integration test")
	}

	svc := NewFromEnv()

	// Expect OpenAI call to succeed; errors should not be silently swallowed.
	slug, err := svc.Slugify(context.Background(), "Fix: Allow --repo utils in vibe start")
	require.NoError(t, err)
	require.NotEmpty(t, slug)

	// Validate slug shape: kebab-case with only [a-z0-9-], no leading/trailing hyphen, single hyphens.
	re := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	require.Regexp(t, re, slug)
	require.LessOrEqual(t, len(slug), 64)
}
