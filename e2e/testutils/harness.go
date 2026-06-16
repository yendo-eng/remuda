package testutils

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/slack"
)

type Harness struct {
	t *testing.T

	RootDir      string
	HomeDir      string
	ReposBaseDir string
	TmpBaseDir   string
	Env          cli.EnvMap
	WorkingDir   string

	Stdin io.Reader

	Stdout *bytes.Buffer
	Stderr *bytes.Buffer

	Git     git.Git
	Session session.SessionManager
	Jira    jira.Jira
	Docker  docker.Docker
	GitHub  github.GitHub
	Slack   slack.Slack

	CloneHooks *internal.CloneHookRegistry

	RemudaConfig internal.Config
	Remuda       internal.Remuda
}

type HarnessOption func(*Harness)

func WithRemudaConfig(cfg internal.Config) HarnessOption {
	return func(h *Harness) {
		if cfg.TmpBaseDir == "" {
			cfg.TmpBaseDir = h.TmpBaseDir
		}
		h.RemudaConfig = cfg
		if cfg.ReposBaseDir != "" {
			h.ReposBaseDir = cfg.ReposBaseDir
		}
		if cfg.TmpBaseDir != "" {
			h.TmpBaseDir = cfg.TmpBaseDir
		}
	}
}

func WithRemudaConfigFromEnv() HarnessOption {
	return func(h *Harness) {
		provider := env.StaticProvider{
			Values:  map[string]string(h.Env),
			HomeDir: h.HomeDir,
		}
		WithRemudaConfig(internal.ConfigFromEnvWithProvider(provider))(h)
	}
}

func WithGitHub(gh github.GitHub) HarnessOption {
	return func(h *Harness) {
		if mock, ok := gh.(*MockGitHub); ok && mock.Env == nil {
			mock.Env = map[string]string(h.Env)
		}
		h.GitHub = gh
	}
}

func WithSessionManager(sm session.SessionManager) HarnessOption {
	return func(h *Harness) {
		h.Session = sm
	}
}

func WithDocker(d docker.Docker) HarnessOption {
	return func(h *Harness) {
		h.Docker = d
	}
}

func WithJira(j jira.Jira) HarnessOption {
	return func(h *Harness) {
		h.Jira = j
	}
}

func WithSlack(s slack.Slack) HarnessOption {
	return func(h *Harness) {
		h.Slack = s
	}
}

func WithCloneHooks(r *internal.CloneHookRegistry) HarnessOption {
	return func(h *Harness) {
		h.CloneHooks = r
	}
}

func WithStdin(r io.Reader) HarnessOption {
	return func(h *Harness) {
		h.Stdin = r
	}
}

func NewHarness(t *testing.T, opts ...HarnessOption) *Harness {
	return newHarness(t, parseEnv(os.Environ()), opts...)
}

func newHarness(t *testing.T, baseEnv map[string]string, opts ...HarnessOption) *Harness {
	t.Helper()

	rootDir := t.TempDir()
	homeDir := filepath.Join(rootDir, "home")
	reposBaseDir := filepath.Join(rootDir, "repos")
	tmpBaseDir := filepath.Join(rootDir, "tmp-repos")

	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	require.NoError(t, os.MkdirAll(reposBaseDir, 0o755))

	contract := DefaultE2EEnvIsolationContract(homeDir)
	if baseEnv == nil {
		baseEnv = map[string]string{}
	}
	sanitized := parseEnv(contract.SanitizeProcessEnv(formatEnv(baseEnv), nil))
	require.NoError(t, contract.EnsureFilesystem())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	defaultSession := &MockSessionManager{}
	defaultGitHub := &MockGitHub{}
	defaultSlack := &MockSlack{}
	defaultDocker := &docker.Mock{Running: false}

	h := &Harness{
		t:            t,
		RootDir:      rootDir,
		HomeDir:      homeDir,
		ReposBaseDir: reposBaseDir,
		TmpBaseDir:   tmpBaseDir,
		Env:          cli.EnvMap(sanitized),
		Stdin:        strings.NewReader(""),
		Stdout:       stdout,
		Stderr:       stderr,

		Git:     git.NewShellGit(),
		Session: defaultSession,
		Jira:    jira.Mock{},
		Docker:  defaultDocker,
		GitHub:  defaultGitHub,
		Slack:   defaultSlack,

		RemudaConfig: internal.Config{ReposBaseDir: reposBaseDir, TmpBaseDir: tmpBaseDir},
	}

	for _, opt := range opts {
		opt(h)
	}

	if h.RemudaConfig.TmpBaseDir == "" {
		h.RemudaConfig.TmpBaseDir = h.TmpBaseDir
	}

	require.NoError(t, os.MkdirAll(h.RemudaConfig.ReposBaseDir, 0o755))

	logger := logging.NewConsoleLogger(io.Discard, zerolog.InfoLevel)
	remudaOpts := []func(*internal.Remuda){
		internal.WithLogger(logger),
		internal.WithSlack(h.Slack),
		internal.WithIO(internal.IO{
			In:  h.Stdin,
			Out: stdout,
			Err: stderr,
		}),
		internal.WithEnvProvider(env.StaticProvider{
			Values:  map[string]string(h.Env),
			HomeDir: h.HomeDir,
		}),
	}
	if h.CloneHooks != nil {
		remudaOpts = append(remudaOpts, internal.WithCloneHooks(h.CloneHooks))
	}

	h.Remuda = internal.NewRemuda(h.RemudaConfig, h.Git, h.Session, h.Jira, h.Docker, h.GitHub, remudaOpts...)

	return h
}

type CLIRunResult struct {
	Args   []string
	Stdout string
	Stderr string
	Err    error
}

func (r CLIRunResult) String() string {
	var b strings.Builder
	if len(r.Args) > 0 {
		b.WriteString("args: ")
		b.WriteString(strings.Join(r.Args, " "))
		b.WriteString("\n")
	}
	if r.Err != nil {
		b.WriteString("err: ")
		b.WriteString(r.Err.Error())
		b.WriteString("\n")
	}
	if r.Stdout != "" {
		b.WriteString("stdout:\n")
		b.WriteString(r.Stdout)
		if !strings.HasSuffix(r.Stdout, "\n") {
			b.WriteString("\n")
		}
	}
	if r.Stderr != "" {
		b.WriteString("stderr:\n")
		b.WriteString(r.Stderr)
		if !strings.HasSuffix(r.Stderr, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (h *Harness) Run(args ...string) CLIRunResult {
	h.t.Helper()

	h.Stdout.Reset()
	h.Stderr.Reset()

	runArgs := append([]string(nil), args...)
	ctxOpts := []func(*cli.Context){
		cli.WithEnv(h.Env),
		cli.WithHomeDir(h.HomeDir),
	}
	if h.WorkingDir != "" {
		ctxOpts = append(ctxOpts, cli.WithWorkingDir(h.WorkingDir))
	}
	err := cli.Run(cli.NewContext(h.t.Context(), h.Remuda, ctxOpts...), runArgs)

	return CLIRunResult{
		Args:   runArgs,
		Stdout: h.Stdout.String(),
		Stderr: h.Stderr.String(),
		Err:    err,
	}
}

func (h *Harness) RunOK(args ...string) CLIRunResult {
	h.t.Helper()
	res := h.Run(args...)
	require.NoError(h.t, res.Err, res.String())
	return res
}

func (h *Harness) SetEnv(key, value string) {
	h.t.Helper()
	h.Env[key] = value
	if provider, ok := h.Remuda.Env.(env.StaticProvider); ok {
		if provider.Values == nil {
			provider.Values = map[string]string{}
			h.Remuda.Env = provider
		}
		provider.Values[key] = value
	}
}

func (h *Harness) Getenv(key string) string {
	h.t.Helper()
	return h.Env.Getenv(key)
}

func (h *Harness) SetWorkingDir(dir string) {
	h.t.Helper()
	h.WorkingDir = dir
}
