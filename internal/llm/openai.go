package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"github.com/rs/zerolog"
)

// openAIService now uses the official OpenAI Go SDK.
type openAIService struct {
	apiKey          string
	model           string
	httpClient      *http.Client
	reasoningEffort shared.ReasoningEffort
	logger          zerolog.Logger
}

// We call the Responses API with a plain string input and aggregate OutputText.
func (s *openAIService) Slugify(ctx context.Context, prompt string) (string, error) {
	user := buildSlugPrompt(prompt)

	// Build client with API key and optional custom HTTP client.
	opts := []option.RequestOption{option.WithAPIKey(s.apiKey)}
	if s.httpClient != nil {
		opts = append(opts, option.WithHTTPClient(s.httpClient))
	}
	client := openai.NewClient(opts...)

	s.logger.Debug().Str("provider", "openai").Str("model", s.model).Int("input_chars", len(user)).Msg("llm openai request")

	params := responses.ResponseNewParams{
		Model:           shared.ResponsesModel(s.model),
		Input:           responses.ResponseNewParamsInputUnion{OfString: openai.String(user)},
		MaxOutputTokens: openai.Int(64),
		Reasoning:       shared.ReasoningParam{Effort: s.reasoningEffort},
		Text:            responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{OfText: &shared.ResponseFormatTextParam{}}},
	}

	resp, err := client.Responses.New(ctx, params)
	if err != nil {
		// Preserve debuggability and return error (match previous behavior);
		// caller can decide fallback or surface the failure.
		s.logger.Debug().Err(err).Msg("llm openai request failed")
		return "", err
	}

	text := strings.TrimSpace(resp.OutputText())
	if text == "" {
		itemTypes := map[string]int{}
		contentTypes := map[string]int{}
		messageStatuses := map[string]int{}
		for _, item := range resp.Output {
			if item.Type != "" {
				itemTypes[item.Type]++
			}
			if item.Status != "" {
				messageStatuses[item.Status]++
			}
			for _, content := range item.Content {
				if content.Type != "" {
					contentTypes[content.Type]++
				}
			}
		}
		logEvent := s.logger.Debug().
			Str("response_id", resp.ID).
			Int("output_items", len(resp.Output)).
			Interface("output_item_types", itemTypes).
			Interface("output_content_types", contentTypes).
			Interface("output_message_statuses", messageStatuses)
		if resp.Error.Code != "" || resp.Error.Message != "" {
			logEvent = logEvent.
				Str("error_code", string(resp.Error.Code)).
				Str("error_message", resp.Error.Message)
		}
		if resp.IncompleteDetails.Reason != "" {
			logEvent = logEvent.Str("incomplete_reason", resp.IncompleteDetails.Reason)
		}
		logEvent.Msg("llm openai empty text; using local fallback")
		// Fall back to user's original prompt to produce a meaningful slug.
		return (&localService{}).Slugify(ctx, prompt)
	} else {
		preview := text
		if len(preview) > 100 {
			preview = preview[:100] + "…"
		}
		s.logger.Debug().Str("text_preview", preview).Msg("llm openai response")
	}
	// Normalize via local rules regardless for safety and determinism.
	return (&localService{}).Slugify(ctx, text)
}

// buildSlugPrompt adapts a high-performing title prompt for slug generation.
func buildSlugPrompt(userMsg string) string {
	const tpl = `You are a slug generator. You output ONLY a kebab-case slug. Nothing else.
<task>
Convert the user's message into a short, branch-safe slug for version control.
Follow all rules in <rules>
Use the <examples> so you know what a good slug looks like.
Your output must be:
- A single line
- <=50 characters
- lowercase kebab-case
- Only [a-z0-9-]
- No leading/trailing hyphen
- No consecutive hyphens
- No explanations
</task>
<rules>
- Focus on the main topic or question the user needs to retrieve
- Prefer -ing verbs for actions when natural (Debugging, Implementing, Analyzing)
- Keep exact: technical terms, numbers, filenames, HTTP codes; replace punctuation with "-"
- Remove filler words: the, this, my, a, an
- Never assume tech stack
- Never use tools unless explicitly in input
- Never answer questions; output only the slug
- If >50 chars, drop least-important trailing words to fit whole tokens
- Transliterate non-ASCII to ASCII; drop emojis
- If input is very short/conversational, output a meaningful short slug (e.g., greeting, quick-check-in)
</rules>
<examples>
"debug 500 errors in production" → debugging-production-500-errors
"refactor user service" → refactoring-user-service
"why is app.js failing" → analyzing-app-js-failure
"implement rate limiting" → implementing-rate-limiting
"how do I connect postgres to my API" → connecting-postgres-to-api
"best practices for React hooks" → react-hooks-best-practices
"Fix: Allow --repo utils in vibe start" → allowing-repo-utils-in-vibe-start
"hello" → greeting
</examples>
<input>
%s
</input>`
	return fmt.Sprintf(tpl, userMsg)
}
