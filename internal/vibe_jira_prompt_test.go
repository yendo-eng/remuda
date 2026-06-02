package internal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

func TestVibeBuildsPromptWithJiraCodeBlocks(t *testing.T) {
	t.Parallel()

	issue := jira.Issue{
		Key:       "RBL-3027",
		Summary:   "Preserve code blocks from Jira",
		Status:    &jira.Status{Name: "In Progress"},
		IssueType: &jira.IssueType{Name: "Bug"},
		Created:   time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC),
		Updated:   time.Date(2026, time.May, 2, 9, 30, 0, 0, time.UTC),
		Description: json.RawMessage(`{
			"type":"doc",
			"content":[
				{
					"type":"paragraph",
					"content":[{"type":"text","text":"Migration:"}]
				},
				{
					"type":"extension",
					"attrs":{
						"extensionType":"com.atlassian.ecosystem",
						"extensionKey":"com.atlassian.confluence.macro.core"
					}
				},
				{
					"type":"paragraph",
					"content":[{"type":"text","text":"Likely queries we’ll want to support"}]
				}
			]
		}`),
		RenderedDescription: `<p>Migration:</p><pre><code>CREATE TABLE payment_methods (
  id UUID PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'active'
);</code></pre><p>Likely queries we’ll want to support</p>`,
	}

	ticketText, err := jira.FormatIssue(issue, nil)
	require.NoError(t, err)

	jiraContext, err := jira.BuildContext(jira.Mock{
		Tickets: map[string]string{
			"RBL-3027": ticketText,
		},
	}, []string{"RBL-3027"})
	require.NoError(t, err)

	sm := &captureSessionManager{}
	k := Remuda{
		Session: sm,
		IO:      DefaultIO(),
	}

	err = k.Vibe(context.Background(), VibeCommand{
		AgentCmd:          "echo",
		Prompt:            "main prompt",
		BeforePrompt:      []string{jiraContext},
		Detached:          true,
		ExistingWorkspace: t.TempDir(),
	})
	require.NoError(t, err)

	require.Contains(t, sm.startCmd, "RBL-3027: Preserve code blocks from Jira")
	require.Contains(t, sm.startCmd, "Description:")
	require.Contains(t, sm.startCmd, "```")
	require.Contains(t, sm.startCmd, "CREATE TABLE payment_methods (")
	require.Contains(t, sm.startCmd, "status TEXT NOT NULL DEFAULT")
	require.Contains(t, sm.startCmd, "active")
	require.Contains(t, sm.startCmd, "main prompt")
}
