package jira

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	unknownValue       = "Unknown"
	noneValue          = "None"
	unassignedValue    = "Unassigned"
	noDescriptionValue = "(none)"
	noCommentsValue    = "(none)"
	noCommentBodyValue = "(no comment body)"
	noSummaryValue     = "(no summary)"
	jiraTimeLayout     = "2006-01-02 15:04 UTC"
)

// FormatIssue renders Jira issue and comments into prompt-ready text.
func FormatIssue(issue Issue, comments []Comment) (string, error) {
	key := strings.TrimSpace(issue.Key)
	if key == "" {
		return "", errors.New("jira issue key cannot be empty")
	}

	summary := strings.TrimSpace(issue.Summary)
	if summary == "" {
		summary = noSummaryValue
	}

	description := RenderADFToTextWithHTMLFallback(issue.Description, issue.RenderedDescription)
	orderedComments := sortCommentsByCreated(comments)
	commentBodies := make([]string, len(orderedComments))
	for i, comment := range orderedComments {
		body, err := RenderADFToText(comment.Body)
		if err != nil {
			id := strings.TrimSpace(comment.ID)
			if id == "" {
				id = fmt.Sprintf("%d", i+1)
			}
			return "", fmt.Errorf("render comment %s body: %w", id, err)
		}
		commentBodies[i] = body
	}

	var sb strings.Builder
	sb.WriteString(key)
	sb.WriteString(": ")
	sb.WriteString(summary)
	sb.WriteString("\n")

	sb.WriteString("Status: ")
	sb.WriteString(orStatusName(issue.Status, unknownValue))
	sb.WriteString(" | Type: ")
	sb.WriteString(orIssueTypeName(issue.IssueType, unknownValue))
	sb.WriteString(" | Priority: ")
	sb.WriteString(orPriorityName(issue.Priority, noneValue))
	sb.WriteString("\n")

	sb.WriteString("Assignee: ")
	sb.WriteString(orUserDisplayName(issue.Assignee, unassignedValue))
	sb.WriteString(" | Reporter: ")
	sb.WriteString(orUserDisplayName(issue.Reporter, unknownValue))
	sb.WriteString("\n")

	sb.WriteString("Created: ")
	sb.WriteString(formatJiraTime(issue.Created))
	sb.WriteString(" | Updated: ")
	sb.WriteString(formatJiraTime(issue.Updated))
	sb.WriteString("\n\n")

	sb.WriteString("Description:\n")
	if description == "" {
		sb.WriteString(noDescriptionValue)
	} else {
		sb.WriteString(description)
	}
	sb.WriteString("\n\n")

	sb.WriteString("Comments:\n")
	if len(orderedComments) == 0 {
		sb.WriteString(noCommentsValue)
		sb.WriteString("\n")
		return sb.String(), nil
	}

	for i, comment := range orderedComments {
		sb.WriteString("- ")
		sb.WriteString(formatJiraTime(comment.Created))
		sb.WriteString(" ")
		sb.WriteString(orUserDisplayName(comment.Author, unknownValue))
		sb.WriteString(":\n")

		body := commentBodies[i]
		if body == "" {
			body = noCommentBodyValue
		}
		for _, line := range strings.Split(body, "\n") {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func sortCommentsByCreated(comments []Comment) []Comment {
	ordered := make([]Comment, len(comments))
	copy(ordered, comments)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Created.Equal(ordered[j].Created) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Created.Before(ordered[j].Created)
	})
	return ordered
}

func formatJiraTime(value time.Time) string {
	if value.IsZero() {
		return unknownValue
	}
	return value.UTC().Format(jiraTimeLayout)
}

func orStatusName(value *Status, fallback string) string {
	if value == nil {
		return fallback
	}
	name := strings.TrimSpace(value.Name)
	if name == "" {
		return fallback
	}
	return name
}

func orIssueTypeName(value *IssueType, fallback string) string {
	if value == nil {
		return fallback
	}
	name := strings.TrimSpace(value.Name)
	if name == "" {
		return fallback
	}
	return name
}

func orPriorityName(value *Priority, fallback string) string {
	if value == nil {
		return fallback
	}
	name := strings.TrimSpace(value.Name)
	if name == "" {
		return fallback
	}
	return name
}

func orUserDisplayName(value *User, fallback string) string {
	if value == nil {
		return fallback
	}
	name := strings.TrimSpace(value.DisplayName)
	if name == "" {
		return fallback
	}
	return name
}
