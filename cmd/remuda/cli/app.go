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
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
)

type SessionManagerFactory func(session.SupportedSessionManager, zerolog.Logger) session.SessionManager

const defaultCLIName = "remuda"
const defaultCLIVersion = "unknown"

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
	kctx           *Context
	cliName        string
	version        string
	cfg            *configfile.V1
	sessionFactory SessionManagerFactory

	rootFlags      *flagSet
	experiments    ExperimentsOption
	verbose        bool
	sessionManager string
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

	env := envFromContext(*a.kctx)
	a.kctx.inv = &invocation{
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
	a.kctx.inv.eff = base

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
	if rs == nil {
		return nil
	}
	for _, set := range rs.sets {
		fl := set.fs.Lookup("experiments")
		if fl == nil {
			continue
		}
		retired, err := validateExperiments(fl.Value.String(), rs.source("experiments"))
		if err != nil {
			return err
		}
		for _, name := range retired {
			a.warnRetiredExperiment(name)
		}
	}
	return nil
}

func (a *app) warnRetiredExperiment(name string) {
	a.kctx.Remuda.IO.Errf("warning: experiment %q %s\n", name, retiredExperimentsRegistry()[name])
}

// applyRepoOverlays re-resolves flags with per_repo/profile overlays for the
// given slug. Also invoked after interactive repo selection (FTUE, --pick).
func (a *app) applyRepoOverlays(slug string) error {
	inv := a.kctx.inv
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
	if !inv.rs.flagExplicit("experiments") && strings.TrimSpace(inv.env.Getenv("REMUDA_EXPERIMENTS")) == "" && eff.Exists("defaults.experiments") {
		inv.rs.resolved["experiments"] = experimentConfigSource(a.cfg, inv.slug, profile)
	}
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
	kctx := a.kctx
	env := envFromContext(*kctx)
	if base := env.Getenv("REMUDA_REPOS_BASE_DIR"); base != "" {
		kctx.Remuda.Config.ReposBaseDir = base
		return
	}
	// Only apply config defaults when the caller hasn't already chosen a base dir.
	// This preserves behavior for tests and other non-main entrypoints that
	// construct internal.Remuda with an explicit Config.
	if kctx.Remuda.Config.ReposBaseDir != internal.ConfigFromEnvWithProvider(kctx.Remuda.Env).ReposBaseDir {
		return
	}

	baseDir := strings.TrimSpace(eff.String("repos.base_dir"))
	if baseDir == "" {
		return
	}

	// Best-effort: Expand "~" and "~/" to HOME when present.
	if strings.HasPrefix(baseDir, "~") {
		home, homeErr := homeDirFromContext(*kctx)
		if expanded, err := expandHomePath(baseDir, home, homeErr); err == nil && expanded != "" {
			baseDir = expanded
		}
	}

	kctx.Remuda.Config.ReposBaseDir = baseDir
}

// finishSetup applies the resolved --verbose and --session-manager values.
func (a *app) finishSetup() {
	kctx := a.kctx

	logLevel := zerolog.InfoLevel
	if a.verbose {
		logLevel = zerolog.DebugLevel
	}
	logger := logging.NewConsoleLogger(kctx.Remuda.IO.Err, logLevel)
	kctx.Remuda.SetLogger(logger)
	kctx.ctx = logging.WithLogger(kctx.ctx, logger)

	// Wire the selected session manager after resolution so --session-manager
	// and config-file defaults take effect for this invocation. Preserve
	// injected session managers (eg. e2e mocks) unless we're using the
	// built-in managers.
	if kctx.Remuda.Session == nil ||
		kctx.Remuda.Session.Name() == string(session.SessionManagerTmux) ||
		kctx.Remuda.Session.Name() == string(session.SessionManagerZellij) {
		kctx.Remuda.Session = a.sessionFactory(session.SupportedSessionManager(a.sessionManager), logger)
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
	pf.StringVar(&a.sessionManager, "session-manager", string(session.SessionManagerTmux), "Session manager to use.")
	a.rootFlags = newFlagSet(pf)
	a.experiments.registerPersistent(root, a.rootFlags)
	a.rootFlags.bind("session-manager",
		bindEnvs("REMUDA_SESSION_MANAGER"),
		bindKey("session.manager"),
		bindEnum(enums.ValidSessionManagers...),
	)

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

func applyCloneHooksFromConfig(kctx *Context, cfg *configfile.V1) {
	if kctx == nil || kctx.Remuda.CloneHooks == nil {
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

	kctx.Remuda.CloneHooks.SetConfigHooks(hooksByRepo)
}

func Run(kctx Context, args []string) error {
	return RunWithName(kctx, defaultCLIName, args)
}

func RunWithName(kctx Context, cliName string, args []string) error {
	cliName = normalizeCLIName(cliName)

	env := envFromContext(kctx)
	sessionFactory := kctx.SessionManagerFactory
	if sessionFactory == nil {
		sessionFactory = session.NewSessionManagerWithLogger
	}
	logger := logging.NewConsoleLogger(kctx.Remuda.IO.Err, zerolog.InfoLevel)
	kctx.Remuda.SetLogger(logger)
	kctx.ctx = logging.WithLogger(kctx.ctx, logger)

	// Completion functions may need a session manager before command flags
	// resolve, so wire one from the environment up front.
	if kctx.Remuda.Session == nil {
		managerName := session.SessionManagerTmux
		if sessionMgr := env.Getenv("REMUDA_SESSION_MANAGER"); sessionMgr != "" {
			managerName = session.SupportedSessionManager(sessionMgr)
		}
		kctx.Remuda.Session = sessionFactory(managerName, logger)
	}

	cfg, discovery, err := loadConfigV1(kctx)
	if err != nil {
		strictRequested := strings.TrimSpace(env.Getenv(configOverrideEnvVar)) != ""
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

	kctx.ConfigFile = cfg
	applyCloneHooksFromConfig(&kctx, cfg)

	// Keep alias catalog in sync with the parsed config for the remainder of startup.
	if cfg != nil && cfg.Repos != nil && len(cfg.Repos.Aliases) > 0 {
		github.MergeRepoAliases(cfg.Repos.Aliases)
	}

	version := strings.TrimSpace(kctx.Version)
	if version == "" {
		version = defaultCLIVersion
	}

	a := &app{
		kctx:           &kctx,
		cliName:        cliName,
		version:        version,
		cfg:            cfg,
		sessionFactory: sessionFactory,
	}
	root := a.buildRoot()
	root.SetArgs(args)
	root.SetIn(kctx.Remuda.IO.In)
	root.SetOut(kctx.Remuda.IO.Out)
	root.SetErr(kctx.Remuda.IO.Err)

	// Completion callbacks run outside prepare(); give them access to the
	// CLI context through the command context.
	execCtx := context.WithValue(kctx.ctx, completionContextKey{}, &kctx)

	return root.ExecuteContext(execCtx)
}
