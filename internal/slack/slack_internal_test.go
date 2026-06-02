package slack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSlackThreadURL_LegacyArchive(t *testing.T) {
	raw := "https://workspace.slack.com/archives/C12345/p1714693146123456"
	info, err := parseSlackThreadURL(raw)
	require.NoError(t, err)
	require.Equal(t, "C12345", info.ChannelID)
	require.Equal(t, "1714693146.123456", info.ThreadTS)
}

func TestParseSlackThreadURL_AppThread(t *testing.T) {
	raw := "https://app.slack.com/client/T00000000/C22222222/thread/C22222222-1714693146.654321"
	info, err := parseSlackThreadURL(raw)
	require.NoError(t, err)
	require.Equal(t, "C22222222", info.ChannelID)
	require.Equal(t, "1714693146.654321", info.ThreadTS)
}

func TestParseSlackThreadURL_QueryParams(t *testing.T) {
	raw := "https://app.slack.com/client/T0/C33333333?thread_ts=1714693146.111234&cid=C33333333"
	info, err := parseSlackThreadURL(raw)
	require.NoError(t, err)
	require.Equal(t, "C33333333", info.ChannelID)
	require.Equal(t, "1714693146.111234", info.ThreadTS)
}

func TestParseSlackThreadURL_Invalid(t *testing.T) {
	_, err := parseSlackThreadURL("https://example.com/foo")
	require.Error(t, err)
}
