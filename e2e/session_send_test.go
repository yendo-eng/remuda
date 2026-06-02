package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionSendUsesPromptArg(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	sess.AddSessionWithBuffer("org/repo/feat", "")
	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	h.RunOK("session", "send", "--name", "org/repo/feat", "hello")
	require.Equal(t, "org/repo/feat", sess.LastSendName)
	require.Equal(t, "hello", sess.LastSendInput)
	require.True(t, sess.LastSendEnter)
}

func TestSessionSendReadsPromptFromStdin(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	sess.AddSessionWithBuffer("org/repo/feat", "")
	h := testutils.NewHarness(t,
		testutils.WithSessionManager(sess),
		testutils.WithStdin(strings.NewReader("from-stdin\n")),
	)

	h.RunOK("session", "send", "--name", "org/repo/feat", "-")
	require.Equal(t, "from-stdin", sess.LastSendInput)
	require.True(t, sess.LastSendEnter)
}

func TestSessionSendNoNewline(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	sess.AddSessionWithBuffer("org/repo/feat", "")
	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	h.RunOK("session", "send", "--name", "org/repo/feat", "--no-newline", "noop")
	require.False(t, sess.LastSendEnter)
}
