package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func newTestContextWithEnv(t *testing.T, env EnvProvider, opts ...func(*Context)) Context {
	t.Helper()
	options := make([]func(*Context), 0, len(opts)+1)
	if env != nil {
		options = append(options, WithEnv(env))
	}
	options = append(options, opts...)
	return NewContext(context.Background(), internal.Remuda{}, options...)
}

// attachTestInvocation gives ctx a minimal parsed-command invocation so
// overlay re-resolution (ApplyRepoOverlays) works in unit tests.
func attachTestInvocation(t *testing.T, ctx *Context, cfg *configfile.V1, profiled bool) {
	t.Helper()
	a := &app{kctx: ctx, cfg: cfg}
	cmd := &cobra.Command{Use: "test"}
	fl := newFlagSet(cmd.Flags())
	if profiled {
		var profile string
		registerProfileFlag(cmd, &profile)
	}
	rs, err := beginResolution(fl)
	require.NoError(t, err)
	ctx.inv = &invocation{
		app:      a,
		cmd:      cmd,
		rs:       rs,
		env:      envFromContext(*ctx),
		profiled: profiled,
	}
}
