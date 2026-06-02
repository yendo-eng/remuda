package cli_test

import (
	"context"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
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

func newParserWithEnv(t *testing.T, env cli.EnvProvider, opts ...func(*cli.Context)) (*kong.Kong, *cli.CLI, cli.Context) {
	t.Helper()
	ctx := newTestContext(t, env, opts...)
	var c cli.CLI
	kongOpts := []kong.Option{kong.Name("remuda"), kong.Bind(&ctx)}
	if env != nil {
		kongOpts = append(kongOpts, kong.Resolvers(cli.NewEnvResolver(env)))
	}
	parser, err := kong.New(&c, kongOpts...)
	require.NoError(t, err)
	return parser, &c, ctx
}
