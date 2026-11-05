package internal

import (
	"net/http"
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
	Config      Config
	Git         git.Git
	Multiplexer session.Multiplexer
	Jira        jira.Jira
	Docker      docker.Docker
	GitHub      github.GitHub
	Slack       slack.Slack
	CloneHooks  *CloneHookRegistry
	IO          IO
	Env         env.Provider
	Logger      zerolog.Logger
}

func NewRemuda(cfg Config) Remuda {
	k := Remuda{
		Config:     cfg,
		CloneHooks: NewCloneHookRegistry(),
		IO:         DefaultIO(),
		Env:        env.NewMutableProvider(env.Default()),
		Logger:     logging.DefaultLogger(),
	}
	k.Slack = slack.NewHTTPSlackWithEnv(http.Client{Timeout: 30 * time.Second}, k.Env)
	return k
}

type Config struct {
	// The base directory for cloned repositories. Defaults to "~/.remuda/repos".
	ReposBaseDir string
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
	return Config{ReposBaseDir: base}
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
