package cli_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/session"
)

type sendStubGit struct{}

func (sendStubGit) Clone(repoURL, dir string) error                          { return nil }
func (sendStubGit) Pull(dir string) error                                    { return nil }
func (sendStubGit) WorktreeAdd(dir, branch string, args ...string) error     { return nil }
func (sendStubGit) WorktreeRemove(dir string, args ...string) error          { return nil }
func (sendStubGit) WorktreeMove(dir, src, dst string) error                  { return nil }
func (sendStubGit) Checkout(dir string, args ...string) error                { return nil }
func (sendStubGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (sendStubGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (sendStubGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = sendStubGit{}

type captureSendManager struct {
	name          string
	payload       string
	appendNewline bool
	calls         []sendCall
}

type sendCall struct {
	name          string
	payload       string
	appendNewline bool
}

func (c *captureSendManager) Name() string                            { return "capture" }
func (c *captureSendManager) Start(sessionName, command string) error { return nil }
func (c *captureSendManager) List() ([]session.SessionInfo, error)    { return nil, nil }
func (c *captureSendManager) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (c *captureSendManager) Attach(name string) error { return nil }
func (c *captureSendManager) ReadBuffer(name string, lines int) (string, error) {
	return "", nil
}
func (c *captureSendManager) Send(name string, payload string, appendNewline bool) error {
	c.name = name
	c.payload = payload
	c.appendNewline = appendNewline
	c.calls = append(c.calls, sendCall{name: name, payload: payload, appendNewline: appendNewline})
	return nil
}
func (c *captureSendManager) Kill(name string) error { return nil }

func TestSessionSend_UsesPromptArg(t *testing.T) {
	t.Parallel()
	mgr := &captureSendManager{}
	k := internal.NewRemuda(
		internal.Config{},
		sendStubGit{},
		mgr,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}),
	)
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "hello",
	}
	require.NoError(t, cmd.Run(ctx))
	require.Equal(t, "org/repo/feat", mgr.name)
	require.Equal(t, "hello", mgr.payload)
	require.True(t, mgr.appendNewline)
}

func TestSessionSend_UsesPromptFromStdin(t *testing.T) {
	t.Parallel()
	mgr := &captureSendManager{}
	k := internal.NewRemuda(
		internal.Config{},
		sendStubGit{},
		mgr,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: strings.NewReader("from-stdin\n"), Out: io.Discard, Err: io.Discard}),
	)
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "-",
	}
	require.NoError(t, cmd.Run(ctx))
	require.Equal(t, "from-stdin", mgr.payload)
	require.True(t, mgr.appendNewline)
}

func TestSessionSend_NoNewline(t *testing.T) {
	t.Parallel()
	mgr := &captureSendManager{}
	k := internal.NewRemuda(
		internal.Config{},
		sendStubGit{},
		mgr,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}),
	)
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "no-enter",
		NoNewline:                 true,
	}
	require.NoError(t, cmd.Run(ctx))
	require.False(t, mgr.appendNewline)
}

func TestSessionSend_RejectsEmptyPrompt(t *testing.T) {
	t.Parallel()
	mgr := &captureSendManager{}
	k := internal.NewRemuda(
		internal.Config{},
		sendStubGit{},
		mgr,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}),
	)
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "   ",
	}
	require.Error(t, cmd.Run(ctx))
	require.Empty(t, mgr.payload)
}

func TestSessionSend_MultipleNames(t *testing.T) {
	t.Parallel()
	mgr := &captureSendManager{}
	k := internal.NewRemuda(
		internal.Config{},
		sendStubGit{},
		mgr,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}),
	)
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/one", "org/repo/two"}},
		Prompt:                    "hello",
	}
	require.NoError(t, cmd.Run(ctx))
	require.Equal(t, []sendCall{
		{name: "org/repo/one", payload: "hello", appendNewline: true},
		{name: "org/repo/two", payload: "hello", appendNewline: true},
	}, mgr.calls)
}
