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

// attachTestInvocationWithContainerFlags is like attachTestInvocation, but
// also registers VibeContainerOptions so tests can assert that flag-bound
// struct fields (not just the effective config view) are re-resolved by
// overlay re-resolution.
func attachTestInvocationWithContainerFlags(t *testing.T, ctx *Context, cfg *configfile.V1, profiled bool) *VibeContainerOptions {
	t.Helper()
	a := &app{kctx: ctx, cfg: cfg}
	cmd := &cobra.Command{Use: "test"}
	fl := newFlagSet(cmd.Flags())
	if profiled {
		var profile string
		registerProfileFlag(cmd, &profile)
	}
	container := &VibeContainerOptions{}
	container.register(cmd, fl)
	rs, err := beginResolution(fl)
	require.NoError(t, err)
	ctx.inv = &invocation{
		app:      a,
		cmd:      cmd,
		rs:       rs,
		env:      envFromContext(*ctx),
		profiled: profiled,
	}
	return container
}

// attachTestInvocationWithFlags registers VibeContainerOptions and
// ContextEngineeringOptions, parses args (so flags set there are snapshotted
// as explicit by beginResolution), and attaches the resulting invocation to
// ctx. Used to assert that explicitly-set flags survive repeated overlay
// re-resolution passes.
func attachTestInvocationWithFlags(t *testing.T, ctx *Context, cfg *configfile.V1, args []string) (*VibeContainerOptions, *ContextEngineeringOptions) {
	t.Helper()
	a := &app{kctx: ctx, cfg: cfg}
	cmd := &cobra.Command{Use: "test"}
	fl := newFlagSet(cmd.Flags())
	container := &VibeContainerOptions{}
	container.register(cmd, fl)
	contextEng := &ContextEngineeringOptions{}
	contextEng.register(cmd, fl)
	require.NoError(t, cmd.Flags().Parse(args))
	rs, err := beginResolution(fl)
	require.NoError(t, err)
	ctx.inv = &invocation{
		app: a,
		cmd: cmd,
		rs:  rs,
		env: envFromContext(*ctx),
	}
	return container, contextEng
}
