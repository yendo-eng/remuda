package llm

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/openai/openai-go/v3/shared"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
)

// Service defines a very small surface for simple LLM-backed utilities.
type Service interface {
	// Slugify returns a short, URL/branch-safe kebab-case slug derived from input.
	Slugify(ctx context.Context, prompt string) (string, error)
}

// Options configures a Service implementation.
type Options struct {
	// Model selects provider model (when applicable).
	Model string
	// APIKey for the provider (when applicable).
	APIKey string
	// HTTPClient for outbound calls. Defaults to a 30s-timeout client.
	HTTPClient *http.Client
	// SlugifyReasoningLevel configures OpenAI reasoning effort for slugify.
	SlugifyReasoningLevel string
	// Logger for debug messages.
	Logger *zerolog.Logger
}

// Option configures LLM service options.
type Option func(*Options)

// WithSlugifyReasoningLevel overrides the reasoning level for slugify requests.
func WithSlugifyReasoningLevel(level string) Option {
	return func(o *Options) {
		o.SlugifyReasoningLevel = level
	}
}

// WithLogger configures the logger used by the LLM service.
func WithLogger(logger zerolog.Logger) Option {
	return func(o *Options) {
		o.Logger = &logger
	}
}

// NewFromEnv constructs a Service based on environment variables.
//
// Env vars:
//   - REMUDA_LLM_OPENAI_MODEL (default: "gpt-5-nano")
//   - OPENAI_API_KEY or REMUDA_OPENAI_API_KEY
func NewFromEnv() Service {
	return NewFromEnvProvider(env.Default())
}

// NewFromEnvProvider constructs a Service based on environment variables supplied by provider.
func NewFromEnvProvider(provider env.Provider, opts ...Option) Service {
	provider = env.OrDefault(provider)
	model := strings.TrimSpace(provider.Getenv("REMUDA_LLM_OPENAI_MODEL"))
	if model == "" {
		model = "gpt-5-nano"
	}

	apiKey := strings.TrimSpace(provider.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(provider.Getenv("REMUDA_OPENAI_API_KEY"))
	}

	options := Options{
		Model:      model,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(&options)
	}

	logger := logging.DefaultLogger()
	if options.Logger != nil {
		logger = *options.Logger
	}
	providerName := "local"
	if options.APIKey != "" {
		providerName = "openai"
	}
	logger.Debug().
		Str("provider", providerName).
		Str("model", options.Model).
		Bool("has_api_key", options.APIKey != "").
		Msg("llm service configuration")

	if options.APIKey != "" {
		return &openAIService{
			apiKey:          options.APIKey,
			model:           options.Model,
			httpClient:      options.HTTPClient,
			reasoningEffort: resolveSlugifyReasoningEffort(logger, options.SlugifyReasoningLevel),
			logger:          logger,
		}
	}

	return &localService{}
}

func resolveSlugifyReasoningEffort(logger zerolog.Logger, level string) shared.ReasoningEffort {
	normalized := strings.ToLower(strings.TrimSpace(level))
	if normalized == "" {
		return shared.ReasoningEffortLow
	}
	if !slices.Contains(enums.ValidSlugifyReasoningLevels, normalized) {
		logger.Debug().Str("reasoning_level", level).Msg("invalid slugify reasoning level; using low")
		return shared.ReasoningEffortLow
	}
	return shared.ReasoningEffort(normalized)
}
