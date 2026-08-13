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
)

type sendCall struct {
	name          string
	payload       string
	appendNewline bool
}

type recordingMultiplexer struct {
	calls  []sendCall
	errFor map[string]error
}

func (m *recordingMultiplexer) Name() string { return "recording" }
func (m *recordingMultiplexer) Start(sessionName, command string) error {
	return nil
}
func (m *recordingMultiplexer) List() ([]session.SessionInfo, error) { return nil, nil }
func (m *recordingMultiplexer) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m *recordingMultiplexer) Attach(name string) error { return nil }
func (m *recordingMultiplexer) ReadBuffer(name string, lines int) (string, error) {
	return "", nil
}
func (m *recordingMultiplexer) Send(name string, payload string, appendNewline bool) error {
	if m.errFor != nil {
		if err, ok := m.errFor[name]; ok {
			return err
		}
	}
	m.calls = append(m.calls, sendCall{name: name, payload: payload, appendNewline: appendNewline})
	return nil
}
func (m *recordingMultiplexer) Kill(name string) error { return nil }

func TestSendPromptToSessionsSendsToAll(t *testing.T) {
	t.Parallel()
	mgr := &recordingMultiplexer{}
	ctx := NewContext(context.Background(), internal.Remuda{
		Multiplexer: mgr,
		IO:          internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
	})

	err := sendPromptToSessions(ctx, []string{"org/repo/one", "org/repo/two"}, "hello", true)
	require.NoError(t, err)
	require.Equal(t, []sendCall{
		{name: "org/repo/one", payload: "hello", appendNewline: true},
		{name: "org/repo/two", payload: "hello", appendNewline: true},
	}, mgr.calls)
}

func TestSendPromptToSessionsStopsOnError(t *testing.T) {
	t.Parallel()
	mgr := &recordingMultiplexer{errFor: map[string]error{"org/repo/two": errors.New("boom")}}
	ctx := NewContext(context.Background(), internal.Remuda{
		Multiplexer: mgr,
		IO:          internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
	})

	err := sendPromptToSessions(ctx, []string{"org/repo/one", "org/repo/two"}, "hello", true)
	require.Error(t, err)
	require.ErrorContains(t, err, "send to session \"org/repo/two\"")
	require.ErrorContains(t, err, "boom")
	require.Equal(t, []sendCall{
		{name: "org/repo/one", payload: "hello", appendNewline: true},
	}, mgr.calls)
}
