package jira_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

const (
	testJiraUser  = "agent@example.com"
	testJiraToken = "super-secret-token"
)

func TestHTTPClientGetIssueUsesExpectedEndpointAndQuery(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jira/rest/api/3/issue/PROJ-123", r.URL.Path)
		assert.Equal(t, "summary,status,assignee,reporter,priority,created,updated,description,issuetype", r.URL.Query().Get("fields"))
		assert.Equal(t, "renderedFields", r.URL.Query().Get("expand"))

		user, token, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, testJiraUser, user)
		assert.Equal(t, testJiraToken, token)

		writeJSON(t, w, issuePayload{
			Key: "PROJ-123",
			Fields: issueFieldsPayload{
				Summary:     "Fix race condition",
				Status:      &namePayload{Name: "In Progress"},
				Assignee:    nil,
				Reporter:    nil,
				Priority:    nil,
				IssueType:   &namePayload{Name: "Bug"},
				Created:     "2026-02-01T10:00:00.000+0000",
				Updated:     "2026-02-02T11:00:00.000+0000",
				Description: json.RawMessage(`{"type":"doc","content":[]}`),
			},
			RenderedFields: issueRenderedFieldsPayload{Description: "<p>Fallback</p>"},
		})
	}))
	t.Cleanup(server.Close)

	client := newHTTPClientForTest(t, server.URL+"/jira")
	issue, err := client.GetIssue(context.Background(), "PROJ-123")
	require.NoError(t, err)

	assert.Equal(t, "PROJ-123", issue.Key)
	assert.Equal(t, "Fix race condition", issue.Summary)
	require.NotNil(t, issue.Status)
	assert.Equal(t, "In Progress", issue.Status.Name)
	assert.Nil(t, issue.Assignee)
	assert.Nil(t, issue.Reporter)
	assert.Nil(t, issue.Priority)
	require.NotNil(t, issue.IssueType)
	assert.Equal(t, "Bug", issue.IssueType.Name)
	assert.Equal(t, "<p>Fallback</p>", issue.RenderedDescription)
	assert.False(t, issue.Created.IsZero())
	assert.False(t, issue.Updated.IsZero())
}

func TestHTTPClientGetCommentsPaginatesAndSortsByCreatedAscending(t *testing.T) {
	t.Parallel()

	requestedOffsets := make([]int, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/jira/rest/api/3/issue/PROJ-1/comment", r.URL.Path)
		assert.Equal(t, "50", r.URL.Query().Get("maxResults"))

		offset, err := strconv.Atoi(r.URL.Query().Get("startAt"))
		require.NoError(t, err)
		requestedOffsets = append(requestedOffsets, offset)

		switch offset {
		case 0:
			writeJSON(t, w, commentsPagePayload{
				StartAt:    0,
				MaxResults: 2,
				Total:      3,
				Comments: []commentPayload{
					{ID: "3", Created: "2026-02-03T10:00:00.000+0000", Updated: "2026-02-03T10:10:00.000+0000"},
					{ID: "1", Created: "2026-02-01T10:00:00.000+0000", Updated: "2026-02-01T10:10:00.000+0000"},
				},
			})
		case 2:
			writeJSON(t, w, commentsPagePayload{
				StartAt:    2,
				MaxResults: 2,
				Total:      3,
				Comments: []commentPayload{
					{ID: "2", Created: "2026-02-02T10:00:00.000+0000", Updated: "2026-02-02T10:10:00.000+0000"},
				},
			})
		default:
			t.Fatalf("unexpected startAt offset: %d", offset)
		}
	}))
	t.Cleanup(server.Close)

	client := newHTTPClientForTest(t, server.URL+"/jira")
	comments, err := client.GetComments(context.Background(), "PROJ-1")
	require.NoError(t, err)
	require.Len(t, comments, 3)

	assert.Equal(t, []int{0, 2}, requestedOffsets)
	assert.Equal(t, "1", comments[0].ID)
	assert.Equal(t, "2", comments[1].ID)
	assert.Equal(t, "3", comments[2].ID)
	assert.True(t, comments[0].Created.Before(comments[1].Created))
	assert.True(t, comments[1].Created.Before(comments[2].Created))
}

func TestHTTPClientMapsCommonHTTPStatusErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		status         int
		retryAfter     string
		expectContains []string
	}{
		{
			name:           "401 unauthorized",
			status:         http.StatusUnauthorized,
			expectContains: []string{"authentication failed", "REMUDA_JIRA_API_TOKEN"},
		},
		{
			name:           "403 forbidden",
			status:         http.StatusForbidden,
			expectContains: []string{"authentication failed", "REMUDA_JIRA_API_TOKEN"},
		},
		{
			name:           "404 not found",
			status:         http.StatusNotFound,
			expectContains: []string{"not found or inaccessible", "PROJ-404"},
		},
		{
			name:           "429 rate limited",
			status:         http.StatusTooManyRequests,
			retryAfter:     "12",
			expectContains: []string{"rate limited", "retry after 12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"errorMessages":["request failed"]}`))
			}))
			t.Cleanup(server.Close)

			client := newHTTPClientForTest(t, server.URL)
			_, err := client.GetIssue(context.Background(), "PROJ-404")
			require.Error(t, err)
			for _, expected := range tt.expectContains {
				assert.ErrorContains(t, err, expected)
			}
			assert.NotContains(t, err.Error(), testJiraToken)
		})
	}
}

func TestHTTPClientRequestTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writeJSON(t, w, issuePayload{
			Key: "PROJ-1",
			Fields: issueFieldsPayload{
				Summary:   "timeout test",
				Created:   "2026-02-01T10:00:00.000+0000",
				Updated:   "2026-02-01T10:00:00.000+0000",
				IssueType: &namePayload{Name: "Task"},
			},
		})
	}))
	t.Cleanup(server.Close)

	client := newHTTPClientForTest(t, server.URL, jira.WithRequestTimeout(20*time.Millisecond))
	_, err := client.GetIssue(context.Background(), "PROJ-1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "timed out")
	assert.NotContains(t, err.Error(), testJiraToken)
}

func newHTTPClientForTest(t *testing.T, endpoint string, opts ...jira.HTTPClientOption) jira.Client {
	t.Helper()

	cfg := jira.AuthConfig{
		Endpoint: endpoint,
		User:     testJiraUser,
		Token:    testJiraToken,
	}
	client, err := jira.NewHTTPClient(cfg, opts...)
	require.NoError(t, err)
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

type issuePayload struct {
	Key            string                     `json:"key"`
	Fields         issueFieldsPayload         `json:"fields"`
	RenderedFields issueRenderedFieldsPayload `json:"renderedFields,omitempty"`
}

type issueFieldsPayload struct {
	Summary     string          `json:"summary"`
	Status      *namePayload    `json:"status,omitempty"`
	Assignee    *userPayload    `json:"assignee,omitempty"`
	Reporter    *userPayload    `json:"reporter,omitempty"`
	Priority    *namePayload    `json:"priority,omitempty"`
	IssueType   *namePayload    `json:"issuetype,omitempty"`
	Created     string          `json:"created"`
	Updated     string          `json:"updated"`
	Description json.RawMessage `json:"description,omitempty"`
}

type issueRenderedFieldsPayload struct {
	Description string `json:"description,omitempty"`
}

type commentsPagePayload struct {
	StartAt    int              `json:"startAt"`
	MaxResults int              `json:"maxResults"`
	Total      int              `json:"total"`
	Comments   []commentPayload `json:"comments"`
}

type commentPayload struct {
	ID      string          `json:"id"`
	Author  *userPayload    `json:"author,omitempty"`
	Created string          `json:"created"`
	Updated string          `json:"updated"`
	Body    json.RawMessage `json:"body,omitempty"`
}

type namePayload struct {
	Name string `json:"name"`
}

type userPayload struct {
	AccountID   string `json:"accountId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}
