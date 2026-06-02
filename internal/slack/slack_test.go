package slack_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/slack"
)

func TestBuildSlackThreadContext(t *testing.T) {
	urls := []string{"https://workspace.slack.com/archives/C1/p1234567890123456", "https://workspace.slack.com/archives/C2/p1234567890123456"}

	slackMock := testutils.MockSlack{
		Threads: map[string]string{
			urls[0]: "content for " + urls[0],
			urls[1]: "content for " + urls[1],
		},
	}

	got, err := slack.BuildSlackThreadContext(slackMock, urls)
	require.NoError(t, err)

	expected := "---------- Slack Thread https://workspace.slack.com/archives/C1/p1234567890123456 ----------\ncontent for https://workspace.slack.com/archives/C1/p1234567890123456\n---------- Slack Thread https://workspace.slack.com/archives/C2/p1234567890123456 ----------\ncontent for https://workspace.slack.com/archives/C2/p1234567890123456\n"
	require.Equal(t, expected, got)
}
