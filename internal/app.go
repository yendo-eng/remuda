package internal

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/slack"
)

type Remuda struct {
	Config     Config
	Git        git.Git
	Session    session.SessionManager
	Jira       jira.Jira
	Docker     docker.Docker
	GitHub     github.GitHub
	Slack      slack.Slack
	CloneHooks *CloneHookRegistry
	IO         IO
	Env        env.Provider
	Logger     *zerolog.Logger
}

func WithCloneHooks(r *CloneHookRegistry) func(*Remuda) {
	return func(k *Remuda) {
		k.CloneHooks = r
	}
}

func WithIO(io IO) func(*Remuda) {
	return func(k *Remuda) {
		k.IO = io
	}
}

func WithSlack(slack slack.Slack) func(*Remuda) {
	return func(k *Remuda) {
		k.Slack = slack
	}
}

func WithEnvProvider(provider env.Provider) func(*Remuda) {
	return func(k *Remuda) {
		k.Env = env.NewMutableProvider(provider)
	}
}

func WithLogger(logger zerolog.Logger) func(*Remuda) {
	return func(k *Remuda) {
		k.SetLogger(logger)
	}
}

func NewRemuda(
	cfg Config,
	git git.Git,
	sessionManager session.SessionManager,
	jira jira.Jira,
	docker docker.Docker,
	gitHub github.GitHub,
	opts ...func(*Remuda),
) Remuda {
	k := Remuda{
		Config:     cfg,
		Git:        git,
		Session:    sessionManager,
		Jira:       jira,
		Docker:     docker,
		GitHub:     gitHub,
		CloneHooks: NewCloneHookRegistry(),
		IO:         DefaultIO(),
		Env:        env.Default(),
	}

	for _, opt := range opts {
		opt(&k)
	}

	k.Env = env.NewMutableProvider(k.Env)
	if k.Slack == nil {
		k.Slack = slack.NewHTTPSlackWithEnv(http.Client{Timeout: 30 * time.Second}, k.Env)
	}

	return k
}

func (k Remuda) logger() zerolog.Logger {
	if k.Logger != nil {
		return *k.Logger
	}
	return logging.DefaultLogger()
}

func (k *Remuda) SetLogger(logger zerolog.Logger) {
	k.Logger = &logger
	if setter, ok := k.Git.(git.LoggerSetter); ok {
		setter.SetLogger(logger)
	}
	if setter, ok := k.Session.(session.LoggerSetter); ok {
		setter.SetLogger(logger)
	}
	if setter, ok := k.Docker.(docker.LoggerSetter); ok {
		setter.SetLogger(logger)
	}
	if setter, ok := k.Jira.(jira.LoggerSetter); ok {
		setter.SetLogger(logger)
	}
	if setter, ok := k.GitHub.(github.LoggerSetter); ok {
		setter.SetLogger(logger)
	}
}

type Config struct {
	// The base directory for cloned repositories. Defaults to "~/.remuda/repos".
	ReposBaseDir string

	// The namespaced root under the OS temp dir where --tmp session worktrees are
	// placed (e.g. "<os-temp>/remuda"). Worktrees live at <TmpBaseDir>/<org>/<repo>/<name>
	// while the persistent .repo_cache stays under ReposBaseDir. Overridable via
	// REMUDA_TMP_DIR.
	TmpBaseDir string
}

func ConfigFromEnv() Config {
	return ConfigFromEnvWithProvider(env.Default())
}

func ConfigFromEnvWithProvider(provider env.Provider) Config {
	provider = env.OrDefault(provider)
	base := provider.Getenv("REMUDA_REPOS_BASE_DIR")
	if base == "" {
		base = defaultReposBaseDir(provider)
	}
	tmp := provider.Getenv("REMUDA_TMP_DIR")
	if tmp == "" {
		tmp = defaultTmpBaseDir()
	}
	return Config{ReposBaseDir: base, TmpBaseDir: tmp}
}

// defaultTmpBaseDir returns the namespaced root under the OS temp dir used for
// --tmp session worktrees. Namespacing under "remuda" keeps temp workspaces
// enumerable on demand and avoids colliding with unrelated temp files.
func defaultTmpBaseDir() string {
	return filepath.Join(os.TempDir(), "remuda")
}

func defaultReposBaseDir(provider env.Provider) string {
	provider = env.OrDefault(provider)
	home, err := provider.UserHomeDir()
	if err != nil || home == "" {
		logger := logging.DefaultLogger()
		logger.Warn().Err(err).Str("default", "./repos").Msg("unable to determine home directory; defaulting repos base dir to current working directory")
		if wd, wdErr := provider.WorkingDir(); wdErr == nil && strings.TrimSpace(wd) != "" {
			return filepath.Join(wd, "repos")
		}
		return "./repos"
	}
	return filepath.Join(home, ".remuda", "repos")
}
