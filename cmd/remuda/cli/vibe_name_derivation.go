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

func deriveDefaultVibeWorkspaceName(ctx Context, cmd VibeCmd, issues []jira.Issue) (string, bool, error) {
	if strings.TrimSpace(cmd.Name) != "" || strings.TrimSpace(cmd.In) != "" {
		return "", false, nil
	}

	if jiraName, ok, err := deriveWorkspaceNameFromJira(ctx, cmd.SlugifyReasoningLevel, issues); err != nil {
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

func deriveWorkspaceNameFromJira(ctx Context, slugifyReasoningLevel string, issues []jira.Issue) (string, bool, error) {
	if len(issues) == 0 {
		return "", false, nil
	}
	firstKey := issues[0].Key
	title := strings.TrimSpace(issues[0].Summary)
	if strings.EqualFold(title, jiraNoSummaryPlaceholder) {
		title = ""
	}
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
	service := llm.NewFromEnv(
		ctx.Remuda.Env,
		logger,
		llm.WithSlugifyReasoningLevel(slugifyReasoningLevel),
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

func jiraTitleCanProduceSlug(title string) bool {
	return jiraTitleSeedPattern.MatchString(strings.TrimSpace(title))
}
