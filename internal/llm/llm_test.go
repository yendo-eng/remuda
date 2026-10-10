package llm

import (
	"testing"

	"github.com/openai/openai-go/v3/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestNewFromEnv_UsesOpenAIWhenAPIKeyPresent(t *testing.T) {
	provider := env.StaticProvider{
		Values: map[string]string{
			"REMUDA_LLM_OPENAI_MODEL": "gpt-test",
			"OPENAI_API_KEY":          "test-key",
		},
	}

	svc := NewFromEnv(provider, zerolog.Nop())
	openaiSvc, ok := svc.(*openAIService)
	require.True(t, ok)
	require.Equal(t, "gpt-test", openaiSvc.model)
	require.Equal(t, shared.ReasoningEffortLow, openaiSvc.reasoningEffort)
}

func TestNewFromEnv_LocalFallbackWithoutAPIKey(t *testing.T) {
	provider := env.StaticProvider{
		Values: map[string]string{
			"REMUDA_LLM_OPENAI_MODEL": "gpt-test",
		},
	}

	svc := NewFromEnv(provider, zerolog.Nop())
	require.IsType(t, &localService{}, svc)
}

func TestNewFromEnv_UsesSlugifyReasoningLevelOverride(t *testing.T) {
	provider := env.StaticProvider{
		Values: map[string]string{
			"REMUDA_LLM_OPENAI_MODEL": "gpt-test",
			"OPENAI_API_KEY":          "test-key",
		},
	}

	svc := NewFromEnv(provider, zerolog.Nop(), WithSlugifyReasoningLevel("high"))
	openaiSvc, ok := svc.(*openAIService)
	require.True(t, ok)
	require.Equal(t, shared.ReasoningEffortHigh, openaiSvc.reasoningEffort)
}
