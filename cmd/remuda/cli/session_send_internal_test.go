package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/testutils"
)

func TestSendPromptToSessionsSendsToAll(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{}
	mgr.AddSession(sessionInfo("org/repo/one"))
	mgr.AddSession(sessionInfo("org/repo/two"))
	ctx := NewContext(context.Background(), internal.Remuda{
		Multiplexer: mgr,
		IO:          internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
	})

	err := sendPromptToSessions(ctx, []string{"org/repo/one", "org/repo/two"}, "hello", true)
	require.NoError(t, err)
	require.Equal(t, []testutils.SendCall{
		{Name: "org/repo/one", Payload: "hello", AppendNewline: true},
		{Name: "org/repo/two", Payload: "hello", AppendNewline: true},
	}, mgr.Sends)
}

func TestSendPromptToSessionsStopsOnError(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{SendErrs: map[string]error{"org/repo/two": errors.New("boom")}}
	mgr.AddSession(sessionInfo("org/repo/one"))
	mgr.AddSession(sessionInfo("org/repo/two"))
	ctx := NewContext(context.Background(), internal.Remuda{
		Multiplexer: mgr,
		IO:          internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
	})

	err := sendPromptToSessions(ctx, []string{"org/repo/one", "org/repo/two"}, "hello", true)
	require.Error(t, err)
	require.ErrorContains(t, err, "send to session \"org/repo/two\"")
	require.ErrorContains(t, err, "boom")
	require.Equal(t, []testutils.SendCall{
		{Name: "org/repo/one", Payload: "hello", AppendNewline: true},
	}, mgr.Sends)
}

func sessionInfo(name string) session.SessionInfo { return session.SessionInfo{Name: name} }
