package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientRequestTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	const token = "super-secret-token"
	client, err := NewHTTPClient(AuthConfig{Endpoint: server.URL, User: "agent@example.com", Token: token})
	require.NoError(t, err)
	c, ok := client.(*httpClient)
	require.True(t, ok)
	c.requestTimeout = 20 * time.Millisecond

	_, err = c.GetIssue(context.Background(), "PROJ-1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "timed out")
	assert.NotContains(t, err.Error(), token)
}
