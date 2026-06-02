package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseZellijListOutputWithNow(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	input := encodeZellijSessionName("org/repo/attached") + " [Created 1m ago] (ATTACHED)\n" +
		encodeZellijSessionName("org/repo/detached") + " [Created 2m ago]\n" +
		encodeZellijSessionName("org/repo/exited") + " [Created 3m ago] (EXITED - attach to resurrect)\n" +
		"other-session [Created 4m ago]\n"

	got := parseZellijListOutputWithNow(input, now)
	require.Len(t, got, 3)
	require.Equal(t, "org/repo/attached", got[0].Name)
	require.True(t, got[0].Attached)
	require.WithinDuration(t, now.Add(-1*time.Minute), got[0].CreatedAt, time.Second)
	require.Equal(t, "org/repo/detached", got[1].Name)
	require.False(t, got[1].Attached)
	require.WithinDuration(t, now.Add(-2*time.Minute), got[1].CreatedAt, time.Second)
	require.Equal(t, "other-session", got[2].Name)
	require.False(t, got[2].Attached)
	require.WithinDuration(t, now.Add(-4*time.Minute), got[2].CreatedAt, time.Second)
}

func TestParseZellijListOutputWithNow_MissingOrBadCreated(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	input := "session-one [Started 1m ago]\n" +
		"session-two [Created banana ago]\n"

	got := parseZellijListOutputWithNow(input, now)
	require.Len(t, got, 2)
	require.Equal(t, "session-one", got[0].Name)
	require.True(t, got[0].CreatedAt.IsZero())
	require.Equal(t, "session-two", got[1].Name)
	require.True(t, got[1].CreatedAt.IsZero())
}

func TestEncodeDecodeZellijSessionName(t *testing.T) {
	original := "org/repo/feature"
	encoded := encodeZellijSessionName(original)
	require.NotEqual(t, original, encoded)
	decoded, ok := decodeZellijSessionName(encoded)
	require.True(t, ok)
	require.Equal(t, original, decoded)

	_, ok = decodeZellijSessionName("not-ours")
	require.False(t, ok)
}
