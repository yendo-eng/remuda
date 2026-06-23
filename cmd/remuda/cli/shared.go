package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/slack"
	"github.com/yendo-eng/remuda/internal/util"
)

// NameWizardOption groups CLI switches that require either --name or --wizard.
type NameWizardOption struct {
	Name   string `name:"name" help:"Workspace name; reused as branch (and session) name." xor:"name_or_wizard" required:""`
	Wizard bool   `name:"wizard" help:"Launch interactive wizard for this command (requires a TTY)." xor:"name_or_wizard"`
}

// CloneRepoOption groups the CLI switches for selecting which repo to clone.
type CloneRepoOption struct {
	Repo             *string `kong:"env=REMUDA_DEFAULT_REPO,help='Shorthand repository alias to clone; expands to a full URL. Alias values come from config (repos.aliases) or environment-resolved defaults. If omitted and no defaults are set, interactive TTY runs may prompt to choose a default repo (skipped for --wizard/--in or non-interactive).',predictor='repo-alias'"`
	RepoURL          *string `env:"REMUDA_DEFAULT_REPO_URL" help:"Direct git repository URL to clone. Overrides alias; if neither is set, interactive TTY runs may prompt to choose a default repo (skipped for --wizard/--in or non-interactive)."`
	Force            bool    `help:"Replace existing workspace if it exists."`
	SkipCacheRefresh bool    `help:"Skip refreshing the repo cache before cloning. May be out of date with the upstream."`
}

func (o *CloneRepoOption) AfterApply(*Context) error {
	if o == nil || o.RepoURL == nil {
		return nil
	}
	o.RepoURL = optionalString(github.ExpandRepoURL(*o.RepoURL))
	return nil
}

type FullCloneOption struct {
	FullClone bool `name:"full-clone" negatable:"" help:"Clone the entire repository instead of creating a linked worktree (slower, higher disk usage)."`
}

// CloneHooksOption exposes the shared toggle for skipping built-in clone hooks.
type CloneHooksOption struct {
	NoCloneHooks bool `name:"no-clone-hooks" help:"Skip running all post-clone hooks (built-in and config-defined)."`
}

type PromptName string

func (b PromptName) String() string {
	return string(b)
}

func (b *PromptName) UnmarshalText(text []byte) error {
	*b = PromptName(text)

	return nil
}

// ContextEngineeringOptions captures the common flags to help add context to agent
// sessions.
type ContextEngineeringOptions struct {
	Jira         []string     `name:"jira" help:"JIRA ticket ID to prepend as context. Repeatable. For vibe, this also drives default name derivation when --name is omitted."`
	JiraEndpoint string       `name:"jira-endpoint" env:"REMUDA_JIRA_ENDPOINT" help:"Jira base URL used by --jira context (for example https://your-domain.atlassian.net)."`
	JiraUser     string       `name:"jira-user" env:"REMUDA_JIRA_USER" help:"Jira user/email used by --jira context authentication."`
	JiraToken    string       `name:"jira-token" env:"REMUDA_JIRA_API_TOKEN,REMUDA_JIRA_TOKEN" help:"Jira API token used by --jira context authentication. Prefer env/config over direct CLI usage when possible."`
	SlackThread  []string     `name:"slack-thread" help:"Slack thread URL to import as context (repeatable, requires SLACK_TOKEN)."`
	GitHubIssue  []string     `name:"github-issue" aliases:"gh-issue" help:"GitHub issue URL or number to prepend as context (repeatable; number requires repo inference)."`
	Use          []PromptName `kong:"name=use,short=u,help='Prepend one or more saved prompts (repeatable). Custom prompts override built-ins when names collide.',env='REMUDA_USE_PROMPTS',predictor='prompt-name'"`
	NoUse        []PromptName `kong:"name=no-use,help='Exclude one or more saved prompts (repeatable).',predictor='prompt-name'"`
}

type PromptContextInput struct {
	GitHubRepoSlug string
	WrapUsePrompts bool
}

var jiraIssueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

func normalizeAndValidateJiraKeys(keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		normalizedKey := strings.ToUpper(strings.TrimSpace(key))
		if !jiraIssueKeyPattern.MatchString(normalizedKey) {
			return nil, fmt.Errorf("invalid --jira value %q: expected format ABC-123", key)
		}
		normalized = append(normalized, normalizedKey)
	}

	return normalized, nil
}

func (c ContextEngineeringOptions) AddedPromptContext(ctx Context, input PromptContextInput) ([]string, error) {
	addedContext := []string{}

	if err := c.validatePromptNames(ctx); err != nil {
		return nil, err
	}
	normalizedJira, err := normalizeAndValidateJiraKeys(c.Jira)
	if err != nil {
		return nil, err
	}

	usePrompts := c.effectiveUsePrompts()
	if len(usePrompts) > 0 {
		preface := make([]string, 0, len(usePrompts))
		for _, builtin := range usePrompts {
			prompt, err := ctx.Remuda.ShowPrompt(builtin.String())
			if err != nil {
				return nil, err
			}
			preface = append(preface, strings.TrimRight(prompt, "\n"))
		}
		prefaceContent := strings.Join(preface, "\n\n")
		if input.WrapUsePrompts {
			prefaceContent = "<context>\n" + prefaceContent + "\n</context>"
		}
		addedContext = append(addedContext, prefaceContent+"\n")
	}

	// JIRA context
	var jiraContext string
	if len(normalizedJira) > 0 {
		if setter, ok := ctx.Remuda.Jira.(jira.AuthConfigSetter); ok {
			setter.SetAuthConfigOverride(jira.AuthConfig{
				Endpoint: c.JiraEndpoint,
				User:     c.JiraUser,
				Token:    c.JiraToken,
			})
		}
		jiraContext, err = jira.BuildContext(ctx.Remuda.Jira, normalizedJira)
		if err != nil {
			return nil, errors.Wrap(err, "jira context")
		}
		addedContext = append(addedContext, jiraContext)
	}

	// Slack thread context
	var slackContext string
	if len(c.SlackThread) > 0 {
		var err error
		slackContext, err = slack.BuildSlackThreadContext(ctx.Remuda.Slack, c.SlackThread)
		if err != nil {
			return nil, errors.Wrap(err, "slack thread context")
		}
		addedContext = append(addedContext, slackContext)
	}

	// GitHub issues context
	if len(c.GitHubIssue) > 0 {
		githubContext, err := github.BuildIssueContext(ctx.Remuda.GitHub, input.GitHubRepoSlug, c.GitHubIssue)
		if err != nil {
			return nil, errors.Wrap(err, "github issue context")
		}
		addedContext = append(addedContext, githubContext)
	}

	return addedContext, nil
}

func (c ContextEngineeringOptions) validatePromptUsage(prompt string, args []string) error {
	if strings.TrimSpace(prompt) != "" {
		return nil
	}

	if len(c.Jira) > 0 || len(c.SlackThread) > 0 || len(c.GitHubIssue) > 0 {
		return errors.New("prompt context flags (--jira/--slack-thread/--github-issue/--gh-issue) require a non-empty prompt")
	}

	// Allow REMUDA_USE_PROMPTS defaults without forcing a prompt when the user omits it.
	// If --use/-u is explicitly set, fail fast to avoid silently ignoring it.
	if len(c.Use) > 0 && usesPromptPrefaceFromArgs(args) {
		return errors.New("--use/-u requires a non-empty prompt")
	}

	return nil
}

func (c ContextEngineeringOptions) effectiveUsePrompts() []PromptName {
	if len(c.Use) == 0 {
		return nil
	}
	if len(c.NoUse) == 0 {
		return c.Use
	}
	exclude := make(map[string]struct{}, len(c.NoUse))
	for _, name := range c.NoUse {
		exclude[name.String()] = struct{}{}
	}
	kept := make([]PromptName, 0, len(c.Use))
	for _, name := range c.Use {
		if _, ok := exclude[name.String()]; ok {
			continue
		}
		kept = append(kept, name)
	}
	return kept
}

func (c ContextEngineeringOptions) effectiveUsePromptNames() []string {
	usePrompts := c.effectiveUsePrompts()
	if len(usePrompts) == 0 {
		return nil
	}
	names := make([]string, 0, len(usePrompts))
	for _, prompt := range usePrompts {
		names = append(names, prompt.String())
	}
	return names
}

func (c ContextEngineeringOptions) validatePromptNames(ctx Context) error {
	if len(c.Use) == 0 && len(c.NoUse) == 0 {
		return nil
	}
	names := make([]PromptName, 0, len(c.Use)+len(c.NoUse))
	names = append(names, c.Use...)
	names = append(names, c.NoUse...)
	checked := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameStr := name.String()
		if _, ok := checked[nameStr]; ok {
			continue
		}
		checked[nameStr] = struct{}{}
		if _, err := ctx.Remuda.ShowPrompt(nameStr); err != nil {
			return err
		}
	}
	return nil
}

func (c *ContextEngineeringOptions) AfterApply(ctx *Context) error {
	normalizedJira, err := normalizeAndValidateJiraKeys(c.Jira)
	if err != nil {
		return err
	}
	c.Jira = normalizedJira
	c.JiraEndpoint = strings.TrimSpace(c.JiraEndpoint)
	c.JiraUser = strings.TrimSpace(c.JiraUser)
	c.JiraToken = strings.TrimSpace(c.JiraToken)

	if ctx == nil {
		return nil
	}

	return c.validatePromptNames(*ctx)
}

func (c ContextEngineeringOptions) hasUsePrompts() bool {
	return len(c.effectiveUsePrompts()) > 0
}

func shouldAddMainPromptMarker(wrapUsePrompts bool, usePromptsSelected bool) bool {
	return wrapUsePrompts && usePromptsSelected
}

// SessionLaunchOptions captures the common session-manager flags shared by session-like commands.
type SessionLaunchOptions struct {
	Detached bool `negatable:"" default:"true" help:"Run the session in the background with your configured terminal multiplexer."`
	NoTmux   bool `hidden:"" help:"Run without the configured session manager (detached tmux by default)."`
	Attach   bool `name:"attach" help:"Attach to the session immediately after launching (requires detached mode)."`
}

func (o SessionLaunchOptions) DetachedMode() bool {
	return o.Detached && !o.NoTmux
}

// AgentSessionOptions captures the common agent configuration flags shared by vibe commands.
type AgentSessionOptions struct {
	SessionLaunchOptions `embed:""`

	// Agent enum tag must match enums.ValidAgents; see shared_test.go for enforcement.
	Agent          string   `name:"agent" default:"codex" enum:"codex,opencode,claude,bash" help:"Built-in agent to use (codex|opencode|claude|bash)." env:"REMUDA_AGENT"`
	Model          string   `name:"model" env:"REMUDA_MODEL" help:"Specific model to use. Use agent-default to omit any model flag and let the agent CLI choose its own default." predictor:"model"`
	ReasoningLevel string   `name:"reasoning-level" env:"REMUDA_REASONING_LEVEL" help:"Reasoning level for codex/claude (none|minimal|low|medium|high|xhigh for codex; passed through to claude --effort for claude)." predictor:"reasoning-level"`
	AgentCmd       string   `name:"agent-cmd" help:"Override the agent command entirely."`
	AgentArg       []string `name:"agent-arg" sep:"none" help:"Additional argument to append to the selected built-in agent command (repeatable). Ignored when --agent-cmd is set."`
}

func (o *AgentSessionOptions) AfterApply(*Context) error {
	for i, arg := range o.AgentArg {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("--agent-arg[%d]: agent arg cannot be empty", i)
		}
	}
	return nil
}

// APIKeyOptions manages CLI flags and env fallback for agent API keys.
type APIKeyOptions struct {
	OpenAIAPIKey string `name:"openai-api-key" help:"OpenAI API key to pass to agents (overrides env lookup)." env:"OPENAI_API_KEY"`
}

// SlugifyOptions captures configuration for LLM-backed slugify.
type SlugifyOptions struct {
	SlugifyReasoningLevel string `name:"slugify-reasoning-level" env:"REMUDA_SLUGIFY_REASONING_LEVEL" default:"low" enum:"none,minimal,low,medium,high,xhigh" help:"Reasoning level for slugify (none|minimal|low|medium|high|xhigh)." predictor:"slugify-reasoning-level"`
}

type VibeContainerOptions struct {
	Container           bool     `name:"container" env:"REMUDA_CONTAINER" negatable:"" help:"Run session inside a Docker container."`
	ContainerName       string   `name:"container-name" help:"Container image to use when --container is set."`
	ContainerOpt        []string `name:"container-opt" env:"REMUDA_CONTAINER_OPTS" help:"Append raw docker run argument (repeatable)."`
	ContainerInheritEnv []string `name:"container-inherit-env" env:"REMUDA_CONTAINER_INHERIT_ENVS" help:"Forward host env var into container (repeatable)."`
}

func (o VibeContainerOptions) Validate() error {
	for _, name := range o.ContainerInheritEnv {
		if err := util.ValidateEnvVarName(name); err != nil {
			return errors.Wrap(err, "--container-inherit-env")
		}
	}
	return nil
}

func validateContainerImageSelection(containerEnabled bool, containerImage string) error {
	if !containerEnabled {
		return nil
	}
	if strings.TrimSpace(containerImage) != "" {
		return nil
	}
	return errors.New(
		"container mode requires an explicit image; pass --container-name or configure defaults.container.image (including profiles.<name>.container.image or per_repo.<slug>.defaults.container.image)",
	)
}

// ExperimentsOption captures experimental feature toggles.
//
// REMUDA_EXPERIMENTS accepts a comma- or whitespace-separated list,
// case-insensitive, e.g. "my-experiment, other".
const experimentUsePromptsContextWrapper = "use-prompts-context-wrapper"

type ExperimentsOption struct {
	Experiments string `name:"experiments" env:"REMUDA_EXPERIMENTS" help:"Enable experimental features (comma- or whitespace-separated list)."`
}

func (o ExperimentsOption) ExperimentEnabled(name string) bool {
	raw := strings.TrimSpace(o.Experiments)
	if raw == "" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(name))
	for _, f := range splitFlexibleList(raw) {
		if strings.ToLower(strings.TrimSpace(f)) == target {
			return true
		}
	}
	return false
}

func splitFlexibleList(input string) []string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		switch r {
		case ',', '\n', '\t':
			return true
		}
		return r == ' '
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

type SessionNamePickOption struct {
	Name string `kong:"required,xor=namepick,help='Session name (org/repo/<name>).',predictor='session-name'"`
	Pick bool   `kong:"required,xor=namepick,help='Use fzf to pick a session interactively when name is omitted.'"`
}

func pickSessionNames(ctx Context, multi bool) ([]string, error) {
	// Check if we have terminal access. When stdout is piped (e.g., in command
	// substitution like `cd $(remuda session path --pick)`), IsTerminal() returns
	// false, but we can still run fzf if /dev/tty is available.
	if !ctx.Remuda.IO.IsTerminal() && !hasTTY() {
		return nil, errors.New("--pick requires an interactive TTY")
	}

	selected, err := pickSessionsWithFZF(logging.FromContext(ctx.ctx), ctx.Remuda.Session, multi)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		if multi {
			return nil, errors.New("no sessions selected")
		}
		return nil, errors.New("no session selected")
	}

	if !multi {
		return []string{selected[0]}, nil
	}

	return selected, nil
}

// SessionNames resolves the selected session names, invoking fzf when --pick is set.
// The multi argument controls whether fzf permits selecting more than one session.
func (o SessionNamePickOption) SessionNames(ctx Context, multi bool) ([]string, error) {
	if o.Name == "" && !o.Pick {
		// I can't seem to get kong's xor tag to work here, so manually enforce it.
		return nil, errors.New("--name or --pick is required")
	}

	if !o.Pick {
		return []string{o.Name}, nil
	}

	return pickSessionNames(ctx, multi)
}

// SessionName returns a single resolved session name and errors if multiple selections were requested.
func (o SessionNamePickOption) SessionName(ctx Context) (string, error) {
	names, err := o.SessionNames(ctx, false)
	if err != nil {
		return "", err
	}
	return names[0], nil
}

// hasTTY returns true if /dev/tty is available for interactive use.
// This allows --pick to work even when stdout is piped (e.g., `cd $(remuda session path --pick)`).
func hasTTY() bool {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	if err := f.Close(); err != nil {
		return false
	}
	return true
}

// openTTY opens /dev/tty for interactive use. Caller must close the returned file.
func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

func pickSessionsWithFZF(
	logger zerolog.Logger,
	mgr session.SessionManager,
	multi bool,
) ([]string, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return nil, fmt.Errorf("fzf not found in PATH; please install fzf or pass a session name")
	}
	sessions, err := mgr.List()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	for _, s := range sessions {
		if !s.IsRemudaSession() {
			continue
		}
		fmt.Fprintln(&b, s.Name)
	}
	if b.Len() == 0 {
		return nil, fmt.Errorf("no sessions available to pick")
	}

	fzfCmd := "fzf"
	args := []string{}
	if multi {
		args = append(args, "--multi")
	}

	if preview := session.FZFPreviewCommand(mgr); preview != "" {
		args = append(args, "--preview", preview)
		args = append(args, "--preview-window", "up:66%")
	}

	cmd := util.CmdWithLogger(logger, fzfCmd, args...)
	cmd.Stdin = &b

	// When stdout is piped (e.g., `cd $(remuda session path --pick)`), fzf
	// cannot display its UI. Connect it to /dev/tty so the user can interact
	// with fzf even when the command's stdout is captured for substitution.
	tty, ttyErr := openTTY()
	if ttyErr == nil {
		defer func() {
			_ = tty.Close()
		}()
		cmd.Stderr = tty
	}

	out, err := cmd.Output()
	if err != nil {
		// If user cancels fzf (exit code 130), return a friendly error.
		return nil, fmt.Errorf("fzf selection error: %w", err)
	}

	sessionNames := strings.Split(strings.TrimSpace(string(out)), "\n")
	return sessionNames, nil
}

func pickOneWorkspaceWithFZF(logger zerolog.Logger, env EnvProvider, candidates []string, base string) (string, error) {
	selected, err := pickWorkspacesWithFZFMode(logger, env, candidates, base, false)
	if err != nil {
		return "", err
	}
	if len(selected) == 0 {
		return "", nil
	}
	if len(selected) > 1 {
		return "", errors.New("expected exactly one workspace selection")
	}
	return selected[0], nil
}

func pickWorkspacesWithFZFMode(logger zerolog.Logger, env EnvProvider, candidates []string, base string, multi bool) ([]string, error) {
	cmdEnv := environFromEnvProvider(env)

	var buf bytes.Buffer
	idx := map[string]string{} // display -> full path
	for _, ws := range candidates {
		org, repo, folder := util.SplitWorkspacePath(base, ws)
		if org == "" || repo == "" || folder == "" {
			continue
		}
		name := strings.Join([]string{org, repo, folder}, "/")
		idx[name] = ws
		fmt.Fprintln(&buf, name)
	}
	if buf.Len() == 0 {
		return nil, errors.New("no workspaces available to pick")
	}

	args := []string{}
	if multi {
		args = append(args, "--multi")
	}
	cmd := util.CmdWithEnvAndLogger(logger, cmdEnv, "fzf", args...)
	if cmd.Err != nil {
		return nil, fmt.Errorf("fzf not found in PATH; please install fzf or omit --pick")
	}
	cmd.Stdin = &buf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fzf selection error: %w", err)
	}

	var selected []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if ws, ok := idx[line]; ok {
			selected = append(selected, ws)
		}
	}
	return selected, nil
}
