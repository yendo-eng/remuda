package jira

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPJiraGetTicketFormatsIssueAndComments(t *testing.T) {
	t.Parallel()

	fake := &fakeJiraClient{
		issue: Issue{
			Summary:   "Fix pipeline behavior",
			Status:    &Status{Name: "In Progress"},
			IssueType: &IssueType{Name: "Bug"},
			Priority:  &Priority{Name: "P1"},
			Assignee:  &User{DisplayName: "Jane Doe"},
			Reporter:  &User{DisplayName: "John Roe"},
			Created:   time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
			Updated:   time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC),
			Description: json.RawMessage(`{
				"type":"doc",
				"content":[
					{"type":"paragraph","content":[{"type":"text","text":"Description text."}]}
				]
			}`),
		},
		comments: []Comment{
			{
				ID:      "2",
				Author:  &User{DisplayName: "John Roe"},
				Created: time.Date(2026, time.March, 3, 10, 0, 0, 0, time.UTC),
				Body:    json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Second comment."}]}]}`),
			},
			{
				ID:      "1",
				Author:  &User{DisplayName: "Jane Doe"},
				Created: time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC),
				Body:    json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First comment."}]}]}`),
			},
		},
	}

	jira := httpJiraWithClient(fake)
	got, err := jira.GetTicket("PROJ-77", AuthConfig{})
	require.NoError(t, err)

	assert.Equal(t, []string{"PROJ-77"}, fake.issueKeys)
	assert.Equal(t, []string{"PROJ-77"}, fake.commentKeys)
	assert.Equal(t, "PROJ-77", got.Key)
	assert.Equal(t, "Fix pipeline behavior", got.Summary)
	assert.Len(t, got.Comments, 2)
	formatted, err := FormatIssue(got)
	require.NoError(t, err)
	assert.Contains(t, formatted, "PROJ-77: Fix pipeline behavior")
	assert.Contains(t, formatted, "Description text.")
	assert.Contains(t, formatted, "- 2026-03-02 09:00 UTC Jane Doe:")
	assert.Contains(t, formatted, "- 2026-03-03 10:00 UTC John Roe:")
}

func TestHTTPJiraGetTicketWrapsErrorsWithIssueKey(t *testing.T) {
	t.Parallel()

	t.Run("issue error", func(t *testing.T) {
		t.Parallel()

		jira := httpJiraWithClient(&fakeJiraClient{issueErr: errors.New("boom")})
		_, err := jira.GetTicket("PROJ-10", AuthConfig{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-10")
		assert.ErrorContains(t, err, "get issue")
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("comments error", func(t *testing.T) {
		t.Parallel()

		jira := httpJiraWithClient(&fakeJiraClient{
			issue:       Issue{Key: "PROJ-11", Summary: "Test"},
			commentsErr: errors.New("comments broken"),
		})
		_, err := jira.GetTicket("PROJ-11", AuthConfig{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-11")
		assert.ErrorContains(t, err, "get comments")
		assert.ErrorContains(t, err, "comments broken")
	})

	t.Run("formatter error", func(t *testing.T) {
		t.Parallel()

		jira := httpJiraWithClient(&fakeJiraClient{
			issue:    Issue{Key: "PROJ-12", Summary: "Test"},
			comments: []Comment{{ID: "1", Body: json.RawMessage("{")}},
		})
		issue, err := jira.GetTicket("PROJ-12", AuthConfig{})
		require.NoError(t, err)
		_, err = BuildContext([]Issue{issue})
		require.Error(t, err)
		assert.ErrorContains(t, err, "format ticket PROJ-12")
		assert.ErrorContains(t, err, "render comment 1 body")
	})
}

func TestHTTPJiraGetTicketWrapsClientInitializationErrors(t *testing.T) {
	t.Parallel()

	t.Run("auth config", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{
			newClient: func(cfg AuthConfig) (Client, error) { return NewHTTPClient(cfg) },
		}

		_, err := jira.GetTicket("PROJ-13", AuthConfig{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-13")
		assert.ErrorContains(t, err, "create jira client")
		assert.ErrorContains(t, err, "missing Jira configuration fields: endpoint, user, token")
	})

	t.Run("client creation", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{
			newClient: func(AuthConfig) (Client, error) { return nil, errors.New("client failed") },
		}

		_, err := jira.GetTicket("PROJ-14", AuthConfig{Endpoint: "https://example.atlassian.net", User: "user", Token: "token"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-14")
		assert.ErrorContains(t, err, "create jira client")
		assert.ErrorContains(t, err, "client failed")
	})
}

func TestHTTPJiraBuildsClientFromEachAuthConfig(t *testing.T) {
	t.Parallel()

	clients := []*fakeJiraClient{
		{issue: Issue{Key: "PROJ-17", Summary: "First auth config"}},
		{issue: Issue{Key: "PROJ-18", Summary: "Second auth config"}},
	}
	var configs []AuthConfig
	jira := &httpJira{
		newClient: func(cfg AuthConfig) (Client, error) {
			configs = append(configs, cfg)
			return clients[len(configs)-1], nil
		},
	}

	first, err := jira.GetTicket("PROJ-17", AuthConfig{Endpoint: "https://first.example.atlassian.net", User: "first", Token: "first-token"})
	require.NoError(t, err)
	second, err := jira.GetTicket("PROJ-18", AuthConfig{Endpoint: "https://second.example.atlassian.net", User: "second", Token: "second-token"})
	require.NoError(t, err)

	require.Equal(t, []AuthConfig{
		{Endpoint: "https://first.example.atlassian.net", User: "first", Token: "first-token"},
		{Endpoint: "https://second.example.atlassian.net", User: "second", Token: "second-token"},
	}, configs)
	require.Equal(t, "First auth config", first.Summary)
	require.Equal(t, "Second auth config", second.Summary)
}

type fakeJiraClient struct {
	issue       Issue
	issueErr    error
	comments    []Comment
	commentsErr error

	issueKeys   []string
	commentKeys []string
}

func httpJiraWithClient(client Client) *httpJira {
	return &httpJira{newClient: func(AuthConfig) (Client, error) { return client, nil }}
}

func (f *fakeJiraClient) GetIssue(_ context.Context, key string) (Issue, error) {
	f.issueKeys = append(f.issueKeys, key)
	if f.issueErr != nil {
		return Issue{}, f.issueErr
	}
	return f.issue, nil
}

func (f *fakeJiraClient) GetComments(_ context.Context, key string) ([]Comment, error) {
	f.commentKeys = append(f.commentKeys, key)
	if f.commentsErr != nil {
		return nil, f.commentsErr
	}
	return f.comments, nil
}
