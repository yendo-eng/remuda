package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/v2"
	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/enums"
	expregistry "github.com/yendo-eng/remuda/internal/experiments"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
)

type MultiplexerFactory func(session.SupportedMultiplexer, zerolog.Logger) session.Multiplexer

const (
	defaultCLIName    = "remuda"
	defaultCLIVersion = "unknown"
)

func normalizeCLIName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultCLIName
	}
	base := strings.TrimSpace(filepath.Base(trimmed))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return defaultCLIName
	}
	return base
}

// app wires the cobra command tree to a single CLI invocation.
type app struct {
	cliCtx             *Context
	cliName            string
	version            string
	cfg                *configfile.V1
	multiplexerFactory MultiplexerFactory

	rootFlags                *flagSet
	experiments              ExperimentsOption
	warnedRetiredExperiments map[string]struct{}
	verbose                  bool
	sessionManager           string
}

// prepareOpts controls per-command flag resolution.
type prepareOpts struct {
	fl *flagSet
	// slugFn infers the repo slug from resolved flag/positional values so
	// per_repo overlays can apply. Runs after base env/config resolution.
	slugFn func() string
	// profiled marks commands that honor --profile / REMUDA_PROFILE.
	profiled bool
}

// prepare resolves flags for the current command: snapshot explicit flags,
// apply env+base config, infer the repo slug, apply per_repo/profile
// overlays, then finish invocation-wide setup (logger, session manager,
// repos base dir).
func (a *app) prepare(cmd *cobra.Command, opts prepareOpts) error {
	sets := []*flagSet{a.rootFlags}
	if opts.fl != nil {
		sets = append(sets, opts.fl)
	}
	rs, err := beginResolution(sets...)
	if err != nil {
		return err
	}
	rs.captureExplicitFlags(cmd.Flags())

	env := envFromContext(*a.cliCtx)
	a.cliCtx.inv = &invocation{
		app:      a,
		cmd:      cmd,
		rs:       rs,
		env:      env,
		profiled: opts.profiled,
	}

	base, err := newEffectiveConfig(a.cfg, "", profileRef{})
	if err != nil {
		return err
	}
	if err := rs.apply(env, base); err != nil {
		return err
	}
	a.cliCtx.inv.eff = base

	slug := ""
	if opts.slugFn != nil {
		slug = opts.slugFn()
	}
	if err := a.applyRepoOverlays(slug); err != nil {
		return err
	}
	if err := rs.validateEnums(); err != nil {
		return err
	}

	a.finishSetup()
	return nil
}

func (a *app) validateExperiments(rs *flagResolution) error {
	if strings.TrimSpace(a.experiments.Experiments) == "" {
		return nil
	}
	source := experimentInputSource(rs, a.cliCtx.inv.env)
	retired, err := validateExperiments(a.experiments.Experiments, source)
	if err != nil {
		return err
	}
	for _, name := range retired {
		a.warnRetiredExperiment(name)
	}
	return nil
}

func (a *app) warnRetiredExperiment(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return
	}
	if a.warnedRetiredExperiments == nil {
		a.warnedRetiredExperiments = map[string]struct{}{}
	}
	if _, ok := a.warnedRetiredExperiments[name]; ok {
		return
	}
	a.warnedRetiredExperiments[name] = struct{}{}
	reason, _ := expregistry.RetiredReason(name)
	a.cliCtx.Remuda.IO.Errf("warning: experiment %q %s\n", name, reason)
}

// applyRepoOverlays re-resolves flags with per_repo/profile overlays for the
// given slug. Also invoked after interactive repo selection (FTUE, --pick).
func (a *app) applyRepoOverlays(slug string) error {
	inv := a.cliCtx.inv
	profile := profileRef{}
	if inv.profiled {
		flagValue := ""
		if fl := inv.cmd.Flags().Lookup("profile"); fl != nil {
			flagValue = fl.Value.String()
		}
		profile = selectProfile(flagValue, inv.rs.flagExplicit("profile"), inv.env, a.cfg, slug)
	}

	eff, err := newEffectiveConfig(a.cfg, slug, profile)
	if err != nil {
		return err
	}
	if err := inv.rs.apply(inv.env, eff); err != nil {
		return err
	}
	inv.eff = eff
	inv.slug = normalizeRepoSlug(slug)
	if err := a.validateExperiments(inv.rs); err != nil {
		return err
	}

	if a.cfg != nil && inv.slug != "" {
		if overlay, ok := a.cfg.PerRepo[inv.slug]; ok && overlay.Repos != nil && len(overlay.Repos.Aliases) > 0 {
			github.MergeRepoAliases(overlay.Repos.Aliases)
		}
	}

	a.applyReposBaseDir(eff)
	return nil
}

// applyReposBaseDir honors precedence env > config > built-in default for
// the repos base directory.
func (a *app) applyReposBaseDir(eff *koanf.Koanf) {
	cliCtx := a.cliCtx
	env := envFromContext(*cliCtx)
	if base := env.Get("REMUDA_REPOS_BASE_DIR"); base != "" {
		cliCtx.Remuda.Config.ReposBaseDir = base
		return
	}
	// Only apply config defaults when the caller hasn't already chosen a base dir.
	// This preserves behavior for tests and other non-main entrypoints that
	// construct internal.Remuda with an explicit Config.
	if cliCtx.Remuda.Config.ReposBaseDir != internal.ConfigFromEnvWithProvider(cliCtx.Remuda.Env).ReposBaseDir {
		return
	}

	baseDir := strings.TrimSpace(eff.String("repos.base_dir"))
	if baseDir == "" {
		return
	}

	// Best-effort: Expand "~" and "~/" to HOME when present.
	if strings.HasPrefix(baseDir, "~") {
		home, homeErr := homeDirFromContext(*cliCtx)
		if expanded, err := expandHomePath(baseDir, home, homeErr); err == nil && expanded != "" {
			baseDir = expanded
		}
	}

	cliCtx.Remuda.Config.ReposBaseDir = baseDir
}

// finishSetup applies the resolved --verbose and --session-manager values.
func (a *app) finishSetup() {
	cliCtx := a.cliCtx

	logLevel := zerolog.InfoLevel
	if a.verbose {
		logLevel = zerolog.DebugLevel
	}
	logger := logging.NewConsoleLogger(cliCtx.Remuda.IO.Err, logLevel)
	cliCtx.Remuda.SetLogger(logger)
	cliCtx.ctx = logging.WithLogger(cliCtx.ctx, logger)

	// Wire the selected session manager after resolution so --session-manager
	// and config-file defaults take effect for this invocation. Preserve
	// injected session managers (eg. e2e mocks) unless we're using the
	// built-in managers.
	if cliCtx.Remuda.Multiplexer == nil ||
		cliCtx.Remuda.Multiplexer.Name() == string(session.MultiplexerTmux) ||
		cliCtx.Remuda.Multiplexer.Name() == string(session.MultiplexerZellij) ||
		cliCtx.Remuda.Multiplexer.Name() == string(session.MultiplexerHerdr) {
		cliCtx.Remuda.Multiplexer = a.multiplexerFactory(session.SupportedMultiplexer(a.sessionManager), logger)
	}
}

func (a *app) buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           a.cliName,
		Short:         "Clone repositories and launch AI coding sessions.",
		Version:       a.version,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.PrintErrln(cmd.UsageString())
			return pkgerrors.New("expected a command")
		},
		CompletionOptions: cobra.CompletionOptions{
			// The user-facing entrypoint is the `completions` command below.
			DisableDefaultCmd: true,
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErrln(cmd.UsageString())
		return err
	})

	pf := root.PersistentFlags()
	pf.BoolVarP(&a.verbose, "verbose", "v", false, "Enable verbose logging.")
	// The user-facing flag, environment variable, and config key deliberately retain "session manager" vocabulary.
	pf.StringVar(&a.sessionManager, "session-manager", string(session.MultiplexerTmux), "Session manager to use.")
	a.rootFlags = newFlagSet(pf)
	a.experiments.registerPersistent(root, a.rootFlags)
	a.rootFlags.bind("session-manager",
		bindEnvs("REMUDA_SESSION_MANAGER"),
		bindKey("session.manager"),
		bindEnum(enums.ValidMultiplexers...),
	)
	registerStaticCompletion(root, "session-manager", enums.ValidMultiplexers)

	root.AddCommand(
		a.cloneCmd(),
		a.vibeCmd(),
		a.vibeCheckCmd(),
		a.workspacesCmd(),
		a.repoCmd(),
		a.configCmd(),
		a.promptsCmd(),
		a.sessionCmd(),
		a.llmCmd(),
		a.completionsCmd(root),
	)
	return root
}

func applyCloneHooksFromConfig(cliCtx *Context, cfg *configfile.V1) {
	if cliCtx == nil || cliCtx.Remuda.CloneHooks == nil {
		return
	}

	hooksByRepo := map[string][]internal.CloneHook{}
	if cfg != nil {
		for slug, overlay := range cfg.PerRepo {
			if len(overlay.CloneHooks) == 0 {
				continue
			}
			normalized := normalizeRepoSlug(slug)
			org, repo, ok := strings.Cut(normalized, "/")
			if !ok || strings.TrimSpace(org) == "" || strings.TrimSpace(repo) == "" {
				continue
			}

			repoHooks := make([]internal.CloneHook, 0, len(overlay.CloneHooks))
			for i, hook := range overlay.CloneHooks {
				name := strings.TrimSpace(hook.Name)
				if name == "" {
					name = fmt.Sprintf("config-hook-%d", i+1)
				}
				repoHooks = append(repoHooks, internal.NewConfigCloneHook(name, hook.Argv))
			}
			hooksByRepo[normalized] = repoHooks
		}
	}

	cliCtx.Remuda.CloneHooks.SetConfigHooks(hooksByRepo)
}

func Run(cliCtx Context, args []string) error {
	return RunWithName(cliCtx, defaultCLIName, args)
}

func RunWithName(cliCtx Context, cliName string, args []string) error {
	cliName = normalizeCLIName(cliName)

	env := envFromContext(cliCtx)
	multiplexerFactory := cliCtx.MultiplexerFactory
	if multiplexerFactory == nil {
		multiplexerFactory = session.NewMultiplexerWithLogger
	}
	logger := logging.NewConsoleLogger(cliCtx.Remuda.IO.Err, zerolog.InfoLevel)
	cliCtx.Remuda.SetLogger(logger)
	cliCtx.ctx = logging.WithLogger(cliCtx.ctx, logger)

	// Completion functions may need a session manager before command flags
	// resolve, so wire one from the environment up front.
	if cliCtx.Remuda.Multiplexer == nil {
		managerName := session.MultiplexerTmux
		if sessionMgr := env.Get("REMUDA_SESSION_MANAGER"); sessionMgr != "" {
			managerName = session.SupportedMultiplexer(sessionMgr)
		}
		cliCtx.Remuda.Multiplexer = multiplexerFactory(managerName, logger)
	}

	cfg, discovery, err := loadConfigV1(cliCtx)
	if err != nil {
		strictRequested := strings.TrimSpace(env.Get(configOverrideEnvVar)) != ""
		strict := strictRequested || discovery.Strict

		fields := logger.Warn().Err(err).Bool("strict", strict)
		if strict {
			fields = logger.Error().Err(err).Bool("strict", true)
		}
		if discovery.Path != "" {
			fields = fields.Str("path", discovery.Path)
		}
		if discovery.Source != "" {
			fields = fields.Str("source", string(discovery.Source))
		} else if strictRequested {
			fields = fields.Str("source", string(ConfigFileSourceEnv))
		} else {
			fields = fields.Str("source", "search")
		}
		fields.Msg("failed to load config file during early bootstrap")

		// Config errors are fatal when a config file is discovered.
		return err
	}

	cliCtx.ConfigFile = cfg
	applyCloneHooksFromConfig(&cliCtx, cfg)

	// Keep alias catalog in sync with the parsed config for the remainder of startup.
	if cfg != nil && cfg.Repos != nil && len(cfg.Repos.Aliases) > 0 {
		github.MergeRepoAliases(cfg.Repos.Aliases)
	}

	version := strings.TrimSpace(cliCtx.Version)
	if version == "" {
		version = defaultCLIVersion
	}

	a := &app{
		cliCtx:             &cliCtx,
		cliName:            cliName,
		version:            version,
		cfg:                cfg,
		multiplexerFactory: multiplexerFactory,
	}
	root := a.buildRoot()
	root.SetArgs(args)
	root.SetIn(cliCtx.Remuda.IO.In)
	root.SetOut(cliCtx.Remuda.IO.Out)
	root.SetErr(cliCtx.Remuda.IO.Err)

	// Completion callbacks run outside prepare(); give them access to the
	// CLI context through the command context.
	execCtx := context.WithValue(cliCtx.ctx, completionContextKey{}, &cliCtx)

	return root.ExecuteContext(execCtx)
}
