package cli

import (
	"context"
	"io"

	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/github"
)

type Context struct {
	ctx                context.Context
	Remuda             internal.Remuda
	ConfigFile         *configfile.V1
	Version            string
	Env                EnvProvider
	MultiplexerFactory MultiplexerFactory
	dirs               contextDirs
	inv                *invocation
}

// invocation carries per-command parse state: which flags were set
// explicitly, the effective (overlay-merged) config view, and the hooks
// needed to re-resolve overlays after interactive repo selection.
type invocation struct {
	app      *app
	cmd      *cobra.Command
	rs       *flagResolution
	eff      *koanf.Koanf
	env      EnvProvider
	slug     string
	profiled bool
}

// FlagExplicit reports whether the user set the flag on the command line
// (as opposed to env/config resolution).
func (c Context) FlagExplicit(name string) bool {
	if c.inv == nil {
		return false
	}
	return c.inv.rs.flagExplicit(name)
}

func (c Context) ExperimentEnabled(name string) bool {
	if c.inv == nil || c.inv.app == nil {
		return false
	}
	return c.inv.app.experiments.ExperimentEnabled(name)
}

// EffectiveConfig is the overlay-merged config view for this invocation.
// Never nil after parsing; empty when no config file was found.
func (c Context) EffectiveConfig() *koanf.Koanf {
	if c.inv == nil || c.inv.eff == nil {
		return koanf.New(".")
	}
	return c.inv.eff
}

// ApplyRepoOverlays re-resolves flags with the per_repo/profile overlays for
// a repo slug discovered mid-run (interactive selection, --pick).
func (c Context) ApplyRepoOverlays(slug string) error {
	if c.inv == nil {
		return nil
	}
	return c.inv.app.applyRepoOverlays(slug)
}

func NewContext(
	ctx context.Context,
	rem internal.Remuda,
	opts ...func(*Context),
) Context {
	cliCtx := Context{
		ctx:    ctx,
		Remuda: rem,
	}

	for _, opt := range opts {
		opt(&cliCtx)
	}

	cliCtx.Env = envOrDefault(cliCtx.Env)
	cliCtx.Remuda.Env = env.NewMutableProvider(contextEnvProvider{
		base: cliCtx.Env,
		dirs: cliCtx.dirs,
	})
	if cliCtx.Remuda.GitHub == nil {
		cliCtx.Remuda.GitHub = github.NewGhCLIWithEnv(cliCtx.Remuda.Env)
	} else if setter, ok := cliCtx.Remuda.GitHub.(github.EnvProviderSetter); ok {
		cliCtx.Remuda.GitHub = setter.WithEnv(cliCtx.Remuda.Env)
	}

	return cliCtx
}

func Stdout(w io.Writer) func(*Context) {
	return func(ctx *Context) {
		ctx.Remuda.IO.Out = w
	}
}

func Stderr(w io.Writer) func(*Context) {
	return func(ctx *Context) {
		ctx.Remuda.IO.Err = w
	}
}

// WithEnv injects an EnvProvider for this CLI invocation.
func WithEnv(env EnvProvider) func(*Context) {
	return func(ctx *Context) {
		ctx.Env = env
	}
}

// WithMultiplexerFactory overrides how multiplexers are constructed.
func WithMultiplexerFactory(factory MultiplexerFactory) func(*Context) {
	return func(ctx *Context) {
		ctx.MultiplexerFactory = factory
	}
}

// WithVersion injects the version string used by --version output.
func WithVersion(version string) func(*Context) {
	return func(ctx *Context) {
		ctx.Version = version
	}
}

// WithWorkingDir overrides the working directory for this CLI invocation.
func WithWorkingDir(dir string) func(*Context) {
	return func(ctx *Context) {
		ctx.dirs.working = fixedPathFor(dir, env.ErrWorkingDirUnavailable)
	}
}

// WithHomeDir overrides the home directory for this CLI invocation.
func WithHomeDir(dir string) func(*Context) {
	return func(ctx *Context) {
		ctx.dirs.home = fixedPathFor(dir, errHomeDirUnavailable)
	}
}
