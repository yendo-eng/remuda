package cli_test

import (
	"context"
	"testing"

	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
)

func newTestContext(t *testing.T, env cli.EnvProvider, opts ...func(*cli.Context)) cli.Context {
	t.Helper()
	options := make([]func(*cli.Context), 0, len(opts)+1)
	if env != nil {
		options = append(options, cli.WithEnv(env))
	}
	options = append(options, opts...)
	return cli.NewContext(context.Background(), internal.Remuda{}, options...)
}
