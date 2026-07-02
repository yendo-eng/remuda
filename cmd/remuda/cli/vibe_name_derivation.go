package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/llm"
	"github.com/yendo-eng/remuda/internal/logging"
)

const (
	jiraNoSummaryPlaceholder = "(no summary)"
)

var jiraTitleSeedPattern = regexp.MustCompile(`[A-Za-z0-9]`)

func deriveDefaultVibeWorkspaceName(ctx Context, cmd VibeCmd) (string, bool, error) {
	if strings.TrimSpace(cmd.Name) != "" || strings.TrimSpace(cmd.In) != "" {
		return "", false, nil
	}

	if jiraName, ok, err := deriveWorkspaceNameFromJira(ctx, cmd.ContextEngineeringOptions, cmd.SlugifyReasoningLevel); err != nil {
		return "", false, err
	} else if ok {
		return jiraName, true, nil
	}

	seed := strings.TrimSpace(cmd.Prompt)
	if seed == "" {
		seed = "session"
	}

	generated, err := slugifyNameSeed(ctx, seed, cmd.SlugifyReasoningLevel)
	if err != nil {
		return "", false, err
	}
	generated = strings.TrimSpace(generated)
	if generated == "" {
		generated = "session"
	}
	return generated, true, nil
}

func deriveWorkspaceNameFromJira(ctx Context, opts ContextEngineeringOptions, slugifyReasoningLevel string) (string, bool, error) {
	normalizedJira, err := normalizeAndValidateJiraKeys(opts.Jira)
	if err != nil {
		return "", false, err
	}
	if len(normalizedJira) == 0 {
		return "", false, nil
	}
	if ctx.Remuda.Jira == nil {
		return "", true, pkgerrors.New("jira client is not configured")
	}

	firstKey := normalizedJira[0]
	if setter, ok := ctx.Remuda.Jira.(jira.AuthConfigSetter); ok {
		setter.SetAuthConfigOverride(jira.AuthConfig{
			Endpoint: opts.JiraEndpoint,
			User:     opts.JiraUser,
			Token:    opts.JiraToken,
		})
	}

	ticketText, err := ctx.Remuda.Jira.GetTicket(firstKey)
	if err != nil {
		return "", true, pkgerrors.Wrapf(err, "get ticket %s", firstKey)
	}
	title := extractJiraTitleFromTicketText(ticketText, firstKey)
	logger := logging.FromContext(ctx.ctx)
	if !jiraTitleCanProduceSlug(title) {
		logger.Warn().
			Str("jira_key", firstKey).
			Msg("jira ticket title is empty or unsuitable for slug; using jira key as workspace name")
		return firstKey, true, nil
	}

	titleSlug, err := slugifyNameSeed(ctx, title, slugifyReasoningLevel)
	if err != nil {
		return "", true, pkgerrors.Wrap(err, "slugify jira ticket title")
	}
	titleSlug = strings.TrimSpace(titleSlug)
	if titleSlug == "" {
		logger.Warn().
			Str("jira_key", firstKey).
			Msg("jira ticket title slug is empty; using jira key as workspace name")
		return firstKey, true, nil
	}

	return fmt.Sprintf("%s-%s", firstKey, titleSlug), true, nil
}

func slugifyNameSeed(ctx Context, seed string, slugifyReasoningLevel string) (string, error) {
	logger := logging.FromContext(ctx.ctx)
	service := llm.NewFromEnvProvider(
		ctx.Remuda.Env,
		llm.WithSlugifyReasoningLevel(slugifyReasoningLevel),
		llm.WithLogger(logger),
	)

	slugCtx, cancel := context.WithTimeout(ctx.ctx, 6*time.Second)
	defer cancel()

	slug, slugErr := service.Slugify(slugCtx, seed)
	if slugErr != nil {
		if ctx.ctx.Err() != nil || errors.Is(slugErr, context.Canceled) {
			return "", pkgerrors.Wrap(slugErr, "slugify workspace name")
		}
		logger.Debug().Err(slugErr).Msg("llm slugify failed; falling back to local slugify")
		localSlug, err := llm.LocalSlugify(seed)
		if err != nil {
			return "", pkgerrors.Wrap(err, "local slugify workspace name")
		}
		slug = localSlug
	}

	return slug, nil
}

func extractJiraTitleFromTicketText(ticketText string, jiraKey string) string {
	firstLine := strings.TrimSpace(ticketText)
	if firstLine == "" {
		return ""
	}
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = strings.TrimSpace(firstLine[:idx])
	}
	if firstLine == "" {
		return ""
	}

	if key := strings.ToUpper(strings.TrimSpace(jiraKey)); key != "" {
		prefix := key + ":"
		if strings.HasPrefix(strings.ToUpper(firstLine), prefix) {
			title := strings.TrimSpace(firstLine[len(prefix):])
			if strings.EqualFold(title, jiraNoSummaryPlaceholder) {
				return ""
			}
			return title
		}
	}

	if idx := strings.IndexByte(firstLine, ':'); idx >= 0 {
		title := strings.TrimSpace(firstLine[idx+1:])
		if strings.EqualFold(title, jiraNoSummaryPlaceholder) {
			return ""
		}
		return title
	}

	if strings.EqualFold(firstLine, jiraNoSummaryPlaceholder) {
		return ""
	}

	return firstLine
}

func jiraTitleCanProduceSlug(title string) bool {
	return jiraTitleSeedPattern.MatchString(strings.TrimSpace(title))
}
