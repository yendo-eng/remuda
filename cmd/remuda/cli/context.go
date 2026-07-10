package cli

import (
	"context"
	"io"
	"os"

	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/github"
)

type Context struct {
	ctx                   context.Context
	Remuda                internal.Remuda
	ConfigFile            *configfile.V1
	Version               string
	Env                   EnvProvider
	SessionManagerFactory SessionManagerFactory
	WorkingDir            string
	HomeDir               string
	inv                   *invocation
	homeDirErr            error
	workingDirErr         error
	homeDirSet            bool
	workingDirSet         bool
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
	k internal.Remuda,
	opts ...func(*Context),
) Context {
	kctx := Context{
		ctx:    ctx,
		Remuda: k,
	}

	for _, opt := range opts {
		opt(&kctx)
	}

	if kctx.Env == nil {
		kctx.Env = defaultEnvProvider()
	}
	if !kctx.homeDirSet {
		home, err := defaultHomeDir()
		kctx.HomeDir = home
		kctx.homeDirErr = err
	}
	if !kctx.workingDirSet {
		wd, err := defaultWorkingDir()
		kctx.WorkingDir = wd
		kctx.workingDirErr = err
	}

	kctx.Remuda.Env = internalEnvProvider{
		env:        kctx.Env,
		homeDir:    kctx.HomeDir,
		homeErr:    kctx.homeDirErr,
		workingDir: kctx.WorkingDir,
		workingErr: kctx.workingDirErr,
	}
	kctx.Remuda.Env = env.NewMutableProvider(kctx.Remuda.Env)
	if kctx.Remuda.GitHub == nil {
		kctx.Remuda.GitHub = github.NewGhCLIWithEnv(kctx.Remuda.Env)
	} else if setter, ok := kctx.Remuda.GitHub.(github.EnvProviderSetter); ok {
		kctx.Remuda.GitHub = setter.WithEnv(kctx.Remuda.Env)
	}

	return kctx
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

// WithSessionManagerFactory overrides how session managers are constructed.
func WithSessionManagerFactory(factory SessionManagerFactory) func(*Context) {
	return func(ctx *Context) {
		ctx.SessionManagerFactory = factory
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
		ctx.WorkingDir = dir
		ctx.workingDirSet = true
		if dir == "" {
			ctx.workingDirErr = env.ErrWorkingDirUnavailable
		} else {
			ctx.workingDirErr = nil
		}
	}
}

// WithHomeDir overrides the home directory for this CLI invocation.
func WithHomeDir(dir string) func(*Context) {
	return func(ctx *Context) {
		ctx.HomeDir = dir
		ctx.homeDirSet = true
		if dir == "" {
			ctx.homeDirErr = errHomeDirUnavailable
		} else {
			ctx.homeDirErr = nil
		}
	}
}

type internalEnvProvider struct {
	env        EnvProvider
	homeDir    string
	homeErr    error
	workingDir string
	workingErr error
}

func (p internalEnvProvider) Getenv(key string) string {
	return p.env.Getenv(key)
}

func (p internalEnvProvider) LookupEnv(key string) (string, bool) {
	return p.env.LookupEnv(key)
}

func (p internalEnvProvider) UserHomeDir() (string, error) {
	if p.homeErr != nil {
		return "", p.homeErr
	}
	if p.homeDir == "" {
		return "", env.ErrHomeDirUnavailable
	}
	return p.homeDir, nil
}

func (p internalEnvProvider) WorkingDir() (string, error) {
	if p.workingErr != nil {
		return "", p.workingErr
	}
	if p.workingDir == "" {
		return "", env.ErrWorkingDirUnavailable
	}
	return p.workingDir, nil
}

func (p internalEnvProvider) Environ() []string {
	if environer, ok := p.env.(interface{ Environ() []string }); ok {
		return environer.Environ()
	}
	return os.Environ()
}
