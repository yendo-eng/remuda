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

	jira := &httpJira{client: fake}
	got, err := jira.GetTicket("PROJ-77")
	require.NoError(t, err)

	assert.Equal(t, []string{"PROJ-77"}, fake.issueKeys)
	assert.Equal(t, []string{"PROJ-77"}, fake.commentKeys)
	assert.Contains(t, got, "PROJ-77: Fix pipeline behavior")
	assert.Contains(t, got, "Description text.")
	assert.Contains(t, got, "- 2026-03-02 09:00 UTC Jane Doe:")
	assert.Contains(t, got, "- 2026-03-03 10:00 UTC John Roe:")
}

func TestHTTPJiraGetTicketWrapsErrorsWithIssueKey(t *testing.T) {
	t.Parallel()

	t.Run("issue error", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{client: &fakeJiraClient{issueErr: errors.New("boom")}}
		_, err := jira.GetTicket("PROJ-10")
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-10")
		assert.ErrorContains(t, err, "get issue")
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("comments error", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{client: &fakeJiraClient{
			issue:       Issue{Key: "PROJ-11", Summary: "Test"},
			commentsErr: errors.New("comments broken"),
		}}
		_, err := jira.GetTicket("PROJ-11")
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-11")
		assert.ErrorContains(t, err, "get comments")
		assert.ErrorContains(t, err, "comments broken")
	})

	t.Run("formatter error", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{client: &fakeJiraClient{
			issue:    Issue{Key: "PROJ-12", Summary: "Test"},
			comments: []Comment{{ID: "1", Body: json.RawMessage("{")}},
		}}
		_, err := jira.GetTicket("PROJ-12")
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-12")
		assert.ErrorContains(t, err, "format issue")
		assert.ErrorContains(t, err, "render comment 1 body")
	})
}

func TestHTTPJiraGetTicketWrapsClientInitializationErrors(t *testing.T) {
	t.Parallel()

	t.Run("auth config", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{
			loadAuthConfig: func() (AuthConfig, error) { return AuthConfig{}, errors.New("missing config") },
		}

		_, err := jira.GetTicket("PROJ-13")
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-13")
		assert.ErrorContains(t, err, "load auth config")
		assert.ErrorContains(t, err, "missing config")
	})

	t.Run("client creation", func(t *testing.T) {
		t.Parallel()

		jira := &httpJira{
			loadAuthConfig: func() (AuthConfig, error) {
				return AuthConfig{Endpoint: "https://example.atlassian.net", User: "user", Token: "token"}, nil
			},
			newClient: func(AuthConfig) (Client, error) { return nil, errors.New("client failed") },
		}

		_, err := jira.GetTicket("PROJ-14")
		require.Error(t, err)
		assert.ErrorContains(t, err, "jira ticket PROJ-14")
		assert.ErrorContains(t, err, "create jira client")
		assert.ErrorContains(t, err, "client failed")
	})
}

func TestHTTPJiraAuthConfigOverride(t *testing.T) {
	t.Parallel()

	t.Run("uses complete override without loading config", func(t *testing.T) {
		t.Parallel()

		fake := &fakeJiraClient{
			issue: Issue{Key: "PROJ-15", Summary: "Test"},
		}
		loadCalls := 0
		var gotCfg AuthConfig
		jira := &httpJira{
			loadAuthConfig: func() (AuthConfig, error) {
				loadCalls++
				return AuthConfig{}, errors.New("should not load")
			},
			newClient: func(cfg AuthConfig) (Client, error) {
				gotCfg = cfg
				return fake, nil
			},
		}
		jira.SetAuthConfigOverride(AuthConfig{
			Endpoint: "https://override.example.atlassian.net/",
			User:     "override@example.com",
			Token:    "override-token",
		})

		_, err := jira.GetTicket("PROJ-15")
		require.NoError(t, err)
		require.Equal(t, 0, loadCalls)
		require.Equal(t, "https://override.example.atlassian.net", gotCfg.Endpoint)
		require.Equal(t, "override@example.com", gotCfg.User)
		require.Equal(t, "override-token", gotCfg.Token)
	})

	t.Run("merges partial override with loaded config", func(t *testing.T) {
		t.Parallel()

		fake := &fakeJiraClient{
			issue: Issue{Key: "PROJ-16", Summary: "Test"},
		}
		var gotCfg AuthConfig
		jira := &httpJira{
			loadAuthConfig: func() (AuthConfig, error) {
				return AuthConfig{
					Endpoint: "https://base.example.atlassian.net",
					User:     "base-user@example.com",
					Token:    "base-token",
				}, nil
			},
			newClient: func(cfg AuthConfig) (Client, error) {
				gotCfg = cfg
				return fake, nil
			},
		}
		jira.SetAuthConfigOverride(AuthConfig{
			Endpoint: "https://override.example.atlassian.net",
			User:     "override-user@example.com",
		})

		_, err := jira.GetTicket("PROJ-16")
		require.NoError(t, err)
		require.Equal(t, "https://override.example.atlassian.net", gotCfg.Endpoint)
		require.Equal(t, "override-user@example.com", gotCfg.User)
		require.Equal(t, "base-token", gotCfg.Token)
	})
}

type fakeJiraClient struct {
	issue       Issue
	issueErr    error
	comments    []Comment
	commentsErr error

	issueKeys   []string
	commentKeys []string
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
