package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
)

// runCLI runs the CLI in-process with captured output and a temp home.
func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	var out, errOut bytes.Buffer
	k := internal.NewRemuda(internal.Config{}, nil, nil, nil, nil, nil)
	kctx := cli.NewContext(context.Background(), k,
		cli.Stdout(&out),
		cli.Stderr(&errOut),
		cli.WithEnv(cli.EnvMap{}),
		cli.WithHomeDir(t.TempDir()),
	)
	return cli.Run(kctx, args)
}

func TestSessionEditNameOrPickRequired(t *testing.T) {
	t.Parallel()
	err := runCLI(t, "session", "edit")
	require.ErrorContains(t, err, "at least one of the flags in the group [name pick] is required")
}
