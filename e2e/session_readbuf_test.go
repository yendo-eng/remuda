package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestSessionReadbufZeroLinesReadsEntireBuffer(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{ReadBuf: "log contents"}

	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	res := h.RunOK("session", "readbuf", "--name", "org/repo/feat", "-n", "0")
	require.Equal(t, "org/repo/feat", sess.LastReadName)
	require.Equal(t, 0, sess.LastReadLines)
	require.Equal(t, "log contents", res.Stdout)
}

func TestSessionReadbufRejectsNegativeLines(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	res := h.Run("session", "readbuf", "--name", "org/repo/feat", "--lines=-5")
	require.Error(t, res.Err)
	require.ErrorContains(t, res.Err, "greater than or equal to 0")
}

func TestSessionReadbufAllOutputsAllSessions(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	sess.AddSessionWithBuffer("org/repo/feat1", "line1\nline2")
	sess.AddSessionWithBuffer("org/repo/feat2", "lineA\nlineB\nlineC")
	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	expected := `org/repo/feat1:1: line1
org/repo/feat1:2: line2
org/repo/feat2:1: lineA
org/repo/feat2:2: lineB
org/repo/feat2:3: lineC
`
	res := h.RunOK("session", "readbuf", "--all")
	require.Equal(t, expected, res.Stdout)
}

func TestSessionReadbufAllWithNoSessions(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	res := h.RunOK("session", "readbuf", "--all")
	require.Empty(t, res.Stdout)
}

func TestSessionReadbufAllRejectsNegativeLines(t *testing.T) {
	t.Parallel()
	sess := &testutils.MockSessionManager{}
	sess.AddSessionWithBuffer("org/repo/feat1", "line1")
	h := testutils.NewHarness(t, testutils.WithSessionManager(sess))

	res := h.Run("session", "readbuf", "--all", "--lines=-1")
	require.Error(t, res.Err)
	require.ErrorContains(t, res.Err, "greater than or equal to 0")
}
