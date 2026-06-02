package llm

import (
	"testing"

	"github.com/openai/openai-go/v3/shared"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestNewFromEnvProvider_UsesOpenAIWhenAPIKeyPresent(t *testing.T) {
	provider := env.StaticProvider{
		Values: map[string]string{
			"REMUDA_LLM_OPENAI_MODEL": "gpt-test",
			"OPENAI_API_KEY":          "test-key",
		},
	}

	svc := NewFromEnvProvider(provider)
	openaiSvc, ok := svc.(*openAIService)
	require.True(t, ok)
	require.Equal(t, "gpt-test", openaiSvc.model)
	require.Equal(t, shared.ReasoningEffortLow, openaiSvc.reasoningEffort)
}

func TestNewFromEnvProvider_LocalFallbackWithoutAPIKey(t *testing.T) {
	provider := env.StaticProvider{
		Values: map[string]string{
			"REMUDA_LLM_OPENAI_MODEL": "gpt-test",
		},
	}

	svc := NewFromEnvProvider(provider)
	require.IsType(t, &localService{}, svc)
}

func TestNewFromEnvProvider_UsesSlugifyReasoningLevelOverride(t *testing.T) {
	provider := env.StaticProvider{
		Values: map[string]string{
			"REMUDA_LLM_OPENAI_MODEL": "gpt-test",
			"OPENAI_API_KEY":          "test-key",
		},
	}

	svc := NewFromEnvProvider(provider, WithSlugifyReasoningLevel("high"))
	openaiSvc, ok := svc.(*openAIService)
	require.True(t, ok)
	require.Equal(t, shared.ReasoningEffortHigh, openaiSvc.reasoningEffort)
}
