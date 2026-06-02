package testutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMockSessionManager_FindSession_ReturnsPointerToBackingSession(t *testing.T) {
	var m MockSessionManager
	require.NoError(t, m.Start("test-session", "initial-cmd"))

	sess := m.FindSession("test-session")
	require.NotNil(t, sess)

	wantCreatedAt := time.Unix(123, 0)
	sess.CreatedAt = wantCreatedAt
	sess.CommandRan = "mutated-cmd"

	info, err := m.Find("test-session")
	require.NoError(t, err)
	require.Equal(t, wantCreatedAt, info.CreatedAt)

	sess2 := m.FindSession("test-session")
	require.NotNil(t, sess2)
	require.Equal(t, "mutated-cmd", sess2.CommandRan)
}

