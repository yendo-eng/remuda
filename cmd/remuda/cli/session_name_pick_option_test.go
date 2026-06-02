package cli_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
)

func TestSessionNamePickOptionErrorsWithoutNameOrPick(t *testing.T) {
	t.Parallel()

	cmd := cli.SessionAttachCmd{}
	ctx := cli.NewContext(
		context.Background(),
		internal.Remuda{
			IO: internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
		},
	)

	err := cmd.Run(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "--name or --pick is required")
}

func TestSessionNamePickOptionRequiresTTYForPick(t *testing.T) {
	t.Parallel()

	// This test exercises the "no TTY available" branch. When /dev/tty is
	// available, --pick may proceed far enough to require fzf (and could hang if
	// fzf is installed), so skip to keep this deterministic across environments.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		require.NoError(t, tty.Close())
		t.Skip("/dev/tty available; skipping non-interactive --pick assertion")
	}

	// When neither stdout/stdin are TTYs (io.Discard) and /dev/tty isn't
	// available (typical in CI), --pick should error. In real usage,
	// command substitution like `cd $(remuda session path --pick)` works
	// because /dev/tty is available even when stdout is piped.
	ctx := cli.NewContext(
		context.Background(),
		internal.Remuda{IO: internal.IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}},
	)

	_, sessionErr := (cli.SessionNamePickOption{Pick: true}).SessionName(ctx)
	require.Error(t, sessionErr)
	require.ErrorContains(t, sessionErr, "requires an interactive TTY")
}
