package jira_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

func TestBuildJiraContext(t *testing.T) {
	t.Parallel()

	mock := jira.Mock{
		Tickets: map[string]jira.Issue{
			"PROJ-1": {Key: "PROJ-1", Summary: "Details for PROJ-1"},
			"ENG-2":  {Key: "ENG-2", Summary: "Details for ENG-2"},
		},
	}

	ids := []string{"PROJ-1", "ENG-2"}
	issues, err := jira.FetchIssues(mock, ids, jira.AuthConfig{})
	require.NoError(t, err, "FetchIssues failed")
	got, err := jira.BuildContext(issues)
	require.NoError(t, err, "BuildJiraContext failed")

	first, err := jira.FormatIssue(issues[0])
	require.NoError(t, err)
	second, err := jira.FormatIssue(issues[1])
	require.NoError(t, err)
	require.Equal(t,
		"---------- Ticket PROJ-1 ----------\n"+first+"---------- Ticket ENG-2 ----------\n"+second,
		got,
		"jira context mismatch",
	)
}
