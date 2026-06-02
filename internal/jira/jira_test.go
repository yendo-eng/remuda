package jira_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

// Verify BuildJiraContext concatenates ticket texts correctly.
func TestBuildJiraContext(t *testing.T) {
	mock := jira.Mock{
		Tickets: map[string]string{
			"PROJ-1": "Details for PROJ-1\nLine2",
			"ENG-2":  "Details for ENG-2\nLine2",
		},
	}

	ids := []string{"PROJ-1", "ENG-2"}
	got, err := jira.BuildContext(mock, ids)
	require.NoError(t, err, "BuildJiraContext failed")

	expected := "---------- Ticket PROJ-1 ----------\nDetails for PROJ-1\nLine2\n---------- Ticket ENG-2 ----------\nDetails for ENG-2\nLine2\n"
	require.Equal(t, expected, got, "jira context mismatch")
}
