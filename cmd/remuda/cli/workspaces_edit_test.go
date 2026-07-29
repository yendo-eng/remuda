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

func TestWorkspacesEditArgs(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		args    []string
		wantErr string
	}{
		"requires a target":     {args: []string{}, wantErr: "accepts 1 arg(s), received 0"},
		"rejects extra targets": {args: []string{"org/repo/one", "org/repo/two"}, wantErr: "accepts 1 arg(s), received 2"},
		"rejects relative path": {args: []string{"repo/feat"}, wantErr: "expected absolute path or org/repo/workspace identifier"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := runCLI(t, append([]string{"workspaces", "edit"}, tc.args...)...)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
