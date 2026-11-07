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
	"github.com/yendo-eng/remuda/internal/testutils"
)

type sendStubGit struct{}

func (sendStubGit) Clone(repoURL, dir string) error                          { return nil }
func (sendStubGit) Pull(dir string) error                                    { return nil }
func (sendStubGit) WorktreeAdd(dir, branch string, args ...string) error     { return nil }
func (sendStubGit) WorktreeRemove(dir string, args ...string) error          { return nil }
func (sendStubGit) WorktreeLock(dir, worktree, reason string) error          { return nil }
func (sendStubGit) WorktreeUnlock(dir, worktree string) error                { return nil }
func (sendStubGit) Checkout(dir string, args ...string) error                { return nil }
func (sendStubGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (sendStubGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (sendStubGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = sendStubGit{}

func TestSessionSend_UsesPromptArg(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{}
	mgr.AddSession(session.SessionInfo{Name: "org/repo/feat"})
	k := internal.NewRemuda(internal.Config{})
	k.Git = sendStubGit{}
	k.Multiplexer = mgr
	k.IO = internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "hello",
	}
	require.NoError(t, cmd.Run(ctx))
	require.Equal(t, "org/repo/feat", mgr.LastSendName)
	require.Equal(t, "hello", mgr.LastSendInput)
	require.True(t, mgr.LastSendEnter)
}

func TestSessionSend_UsesPromptFromStdin(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{}
	mgr.AddSession(session.SessionInfo{Name: "org/repo/feat"})
	k := internal.NewRemuda(internal.Config{})
	k.Git = sendStubGit{}
	k.Multiplexer = mgr
	k.IO = internal.IO{In: strings.NewReader("from-stdin\n"), Out: io.Discard, Err: io.Discard}
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "-",
	}
	require.NoError(t, cmd.Run(ctx))
	require.Equal(t, "from-stdin", mgr.LastSendInput)
	require.True(t, mgr.LastSendEnter)
}

func TestSessionSend_NoNewline(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{}
	mgr.AddSession(session.SessionInfo{Name: "org/repo/feat"})
	k := internal.NewRemuda(internal.Config{})
	k.Git = sendStubGit{}
	k.Multiplexer = mgr
	k.IO = internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "no-enter",
		NoNewline:                 true,
	}
	require.NoError(t, cmd.Run(ctx))
	require.False(t, mgr.LastSendEnter)
}

func TestSessionSend_RejectsEmptyPrompt(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{}
	mgr.AddSession(session.SessionInfo{Name: "org/repo/feat"})
	k := internal.NewRemuda(internal.Config{})
	k.Git = sendStubGit{}
	k.Multiplexer = mgr
	k.IO = internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/feat"}},
		Prompt:                    "   ",
	}
	require.Error(t, cmd.Run(ctx))
	require.Empty(t, mgr.LastSendInput)
}

func TestSessionSend_MultipleNames(t *testing.T) {
	t.Parallel()
	mgr := &testutils.MockMultiplexer{}
	mgr.AddSession(session.SessionInfo{Name: "org/repo/one"})
	mgr.AddSession(session.SessionInfo{Name: "org/repo/two"})
	k := internal.NewRemuda(internal.Config{})
	k.Git = sendStubGit{}
	k.Multiplexer = mgr
	k.IO = internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	ctx := cli.NewContext(context.Background(), k)

	cmd := cli.SessionSendCmd{
		SessionSendNamePickOption: cli.SessionSendNamePickOption{Names: []string{"org/repo/one", "org/repo/two"}},
		Prompt:                    "hello",
	}
	require.NoError(t, cmd.Run(ctx))
	require.Equal(t, []testutils.SendCall{
		{Name: "org/repo/one", Payload: "hello", AppendNewline: true},
		{Name: "org/repo/two", Payload: "hello", AppendNewline: true},
	}, mgr.Sends)
}
