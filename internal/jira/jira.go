package jira

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Jira interface {
	GetTicket(id string, auth AuthConfig) (Issue, error)
}

// LoggerSetter allows wiring a per-invocation logger into Jira implementations.
type LoggerSetter interface {
	SetLogger(logger zerolog.Logger)
}

func FetchIssues(jira Jira, ids []string, auth AuthConfig) ([]Issue, error) {
	issues := make([]Issue, 0, len(ids))
	for _, id := range ids {
		issue, err := jira.GetTicket(id, auth)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "get ticket %s", id)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// BuildContext returns a formatted block suitable for prepending to an LLM prompt.
func BuildContext(issues []Issue) (string, error) {
	var sb strings.Builder
	for _, issue := range issues {
		text, err := FormatIssue(issue)
		if err != nil {
			return "", pkgerrors.Wrapf(err, "format ticket %s", issue.Key)
		}
		sb.WriteString("---------- Ticket ")
		sb.WriteString(issue.Key)
		sb.WriteString(" ----------\n")
		sb.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}
