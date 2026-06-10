package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/willabides/kongplete"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
)

// CLI aggregates the available command structs defined in the commands package.
type CLI struct {
	Clone       CloneCmd                     `cmd:"" help:"Clone a repository into a local workspace."`
	Vibe        VibeCmd                      `cmd:"" help:"Clone and launch an AI coding session."`
	VibeCheck   VibeCheckCmd                 `cmd:"" help:"Review a pull request with AI assistance."`
	Workspaces  WorkspacesCmd                `cmd:"" help:"Inspect Remuda-managed workspaces on disk."`
	Repo        RepoCmd                      `cmd:"" help:"Inspect configured repository aliases."`
	Config      ConfigCmd                    `cmd:"" help:"Manage configuration."`
	Prompts     PromptsCmd                   `cmd:"" help:"Manage and view saved prompts."`
	Session     SessionCmd                   `cmd:"" help:"Manage running sessions (tmux or zellij)."`
	LLM         LLMRootCmd                   `cmd:"" help:"LLM utilities (experimental)."`
	Completions kongplete.InstallCompletions `cmd:"" help:"Install shell completions for remuda."`

	Version        kong.VersionFlag                `name:"version" help:"Print version and exit."`
	Verbose        bool                            `short:"v" help:"Enable verbose logging."`
	SessionManager session.SupportedSessionManager `help:"Session manager to use." env:"REMUDA_SESSION_MANAGER" default:"tmux"`
}

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

func invokedCLIName(kctx *kong.Context) string {
	if kctx == nil || kctx.Kong == nil || kctx.Model == nil {
		return defaultCLIName
	}
	return normalizeCLIName(kctx.Model.Name)
}

func applyReposBaseDirFromConfig(kctx *Context, cfg *configfile.V1) {
	if cfg == nil || cfg.Repos == nil || cfg.Repos.BaseDir == nil {
		return
	}
	// Honor precedence: flags (n/a) > env > config > defaults.
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

	baseDir := strings.TrimSpace(*cfg.Repos.BaseDir)
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

	var cli CLI
	env := envFromContext(kctx)
	sessionFactory := kctx.SessionManagerFactory
	if sessionFactory == nil {
		sessionFactory = session.NewSessionManagerWithLogger
	}
	logger := logging.NewConsoleLogger(kctx.Remuda.IO.Err, zerolog.InfoLevel)
	kctx.Remuda.SetLogger(logger)
	kctx.ctx = logging.WithLogger(kctx.ctx, logger)

	// Since we need to initialize the session manager early for predictors,
	// we have to do some setup before kong.Parse.
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

	if err := applyPerRepoOverlay(kctx, cfg, args); err != nil {
		return err
	}
	if err := applyProfileOverlay(kctx, cfg, args); err != nil {
		return err
	}

	// If env vars are not set, source repos.base_dir from config file to honor
	// PRD precedence: flags (n/a) > env > config > defaults.
	applyReposBaseDirFromConfig(&kctx, cfg)

	// Build parser first so kongplete can hook completion handling.
	version := strings.TrimSpace(kctx.Version)
	if version == "" {
		version = defaultCLIVersion
	}

	parserOpts := []kong.Option{kong.UsageOnError(), kong.Name(cliName), kong.Vars{"version": version}}
	parserOpts = append(parserOpts, kongOptionsFromConfig(cfg, env)...)
	parserOpts = append(parserOpts, kong.Bind(&kctx))
	parser := kong.Must(&cli, parserOpts...)

	// Enable shell completion handling. This exits early when completing.
	RunCompletions(parser, kctx)

	// Parse CLI args normally after completion handling.
	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	kctx.KongCtx = ctx

	// Initialize per-invocation logger
	logLevel := zerolog.InfoLevel
	if cli.Verbose {
		logLevel = zerolog.DebugLevel
	}
	logger = logging.NewConsoleLogger(kctx.Remuda.IO.Err, logLevel)
	kctx.Remuda.SetLogger(logger)
	kctx.ctx = logging.WithLogger(kctx.ctx, logger)

	// Wire the selected session manager after parsing so --session-manager and
	// config-file defaults actually take effect for this invocation.
	// Preserve injected session managers (eg. e2e mocks) unless we're using the
	// built-in managers.
	if kctx.Remuda.Session == nil ||
		kctx.Remuda.Session.Name() == string(session.SessionManagerTmux) ||
		kctx.Remuda.Session.Name() == string(session.SessionManagerZellij) {
		kctx.Remuda.Session = sessionFactory(cli.SessionManager, logger)
	}

	return ctx.Run(kctx)
}

func RunCompletions(parser *kong.Kong, kctx Context) {
	disabled := logging.NewDisabledLogger()
	kctx.Remuda.SetLogger(disabled)
	kctx.ctx = logging.WithLogger(kctx.ctx, disabled)
	kongplete.Complete(parser, kongplete.WithPredictors(RemudaPredictors(kctx, parser)))
}
