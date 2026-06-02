package jira_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

func TestFormatIssueTypicalTicket(t *testing.T) {
	t.Parallel()

	issue := jira.Issue{
		Key:       "PROJ-123",
		Summary:   "Fix payment retry backoff",
		Status:    &jira.Status{Name: "In Progress"},
		IssueType: &jira.IssueType{Name: "Bug"},
		Priority:  &jira.Priority{Name: "P1"},
		Assignee:  &jira.User{DisplayName: "Jane Doe"},
		Reporter:  &jira.User{DisplayName: "John Roe"},
		Created:   time.Date(2025, time.January, 12, 9, 32, 0, 0, time.UTC),
		Updated:   time.Date(2025, time.January, 30, 14, 10, 0, 0, time.UTC),
		Description: json.RawMessage(`{
			"type":"doc",
			"content":[
				{"type":"paragraph","content":[{"type":"text","text":"Retry backoff is too aggressive for provider X."}]}
			]
		}`),
	}

	comments := []jira.Comment{
		{
			ID:      "2",
			Author:  &jira.User{DisplayName: "John Roe"},
			Created: time.Date(2025, time.January, 20, 16, 44, 0, 0, time.UTC),
			Body:    json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Verified issue in staging."}]}]}`),
		},
		{
			ID:      "1",
			Author:  &jira.User{DisplayName: "Jane Doe"},
			Created: time.Date(2025, time.January, 15, 10, 1, 0, 0, time.UTC),
			Body:    json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Working on a patch."}]}]}`),
		},
	}

	got, err := jira.FormatIssue(issue, comments)
	require.NoError(t, err)

	expected := "PROJ-123: Fix payment retry backoff\n" +
		"Status: In Progress | Type: Bug | Priority: P1\n" +
		"Assignee: Jane Doe | Reporter: John Roe\n" +
		"Created: 2025-01-12 09:32 UTC | Updated: 2025-01-30 14:10 UTC\n\n" +
		"Description:\n" +
		"Retry backoff is too aggressive for provider X.\n\n" +
		"Comments:\n" +
		"- 2025-01-15 10:01 UTC Jane Doe:\n" +
		"  Working on a patch.\n" +
		"- 2025-01-20 16:44 UTC John Roe:\n" +
		"  Verified issue in staging.\n"

	assert.Equal(t, expected, got)
	assert.True(t, len(got) > 0)
	assert.Equal(t, byte('\n'), got[len(got)-1])
}

func TestFormatIssueMissingOptionalFields(t *testing.T) {
	t.Parallel()

	issue := jira.Issue{
		Key:       "PROJ-7",
		Summary:   "Handle nil fields",
		Status:    &jira.Status{Name: "To Do"},
		IssueType: &jira.IssueType{Name: "Task"},
		Priority:  nil,
		Assignee:  nil,
		Reporter:  nil,
		Created:   time.Time{},
		Updated:   time.Time{},
	}

	got, err := jira.FormatIssue(issue, []jira.Comment{{
		ID:      "1",
		Author:  nil,
		Created: time.Date(2025, time.March, 1, 1, 2, 0, 0, time.FixedZone("PST", -8*60*60)),
		Body:    json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":""}]}]}`),
	}})
	require.NoError(t, err)

	assert.Contains(t, got, "Status: To Do | Type: Task | Priority: None")
	assert.Contains(t, got, "Assignee: Unassigned | Reporter: Unknown")
	assert.Contains(t, got, "Created: Unknown | Updated: Unknown")
	assert.Contains(t, got, "- 2025-03-01 09:02 UTC Unknown:")
	assert.Contains(t, got, "  (no comment body)")
	assert.NotContains(t, got, "<nil>")
}

func TestFormatIssueNoComments(t *testing.T) {
	t.Parallel()

	issue := jira.Issue{
		Key:       "PROJ-8",
		Summary:   "No comments case",
		Status:    &jira.Status{Name: "Done"},
		IssueType: &jira.IssueType{Name: "Story"},
		Priority:  &jira.Priority{Name: "P3"},
		Assignee:  &jira.User{DisplayName: "Sam"},
		Reporter:  &jira.User{DisplayName: "Lee"},
		Created:   time.Date(2025, time.May, 10, 8, 0, 0, 0, time.UTC),
		Updated:   time.Date(2025, time.May, 10, 9, 0, 0, 0, time.UTC),
	}

	got, err := jira.FormatIssue(issue, nil)
	require.NoError(t, err)

	assert.Contains(t, got, "Comments:\n(none)\n")
	assert.Equal(t, byte('\n'), got[len(got)-1])
}
