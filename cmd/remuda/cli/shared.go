package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/slack"
	"github.com/yendo-eng/remuda/internal/util"
)

// NameWizardOption groups CLI switches that require either --name or --wizard.
type NameWizardOption struct {
	Name   string
	Wizard bool
}

func (o *NameWizardOption) register(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&o.Name, "name", "", "Workspace name; reused as branch (and session) name.")
	fs.BoolVar(&o.Wizard, "wizard", false, "Launch interactive wizard for this command (requires a TTY).")
	cmd.MarkFlagsMutuallyExclusive("name", "wizard")
	cmd.MarkFlagsOneRequired("name", "wizard")
}

// CloneRepoOption groups the CLI switches for selecting which repo to clone.
type CloneRepoOption struct {
	Repo             string
	RepoURL          string
	Force            bool
	SkipCacheRefresh bool
}

func (o *CloneRepoOption) register(cmd *cobra.Command, fl *flagSet) {
	fs := cmd.Flags()
	fs.StringVar(&o.Repo, "repo", "", "Shorthand repository alias to clone; expands to a full URL. Alias values come from config (repos.aliases) or environment-resolved defaults. If omitted and no defaults are set, interactive TTY runs may prompt to choose a default repo (skipped for --wizard/--in or non-interactive).")
	fs.StringVar(&o.RepoURL, "repo-url", "", "Direct git repository URL to clone. Overrides alias; if neither is set, interactive TTY runs may prompt to choose a default repo (skipped for --wizard/--in or non-interactive).")
	fs.BoolVar(&o.Force, "force", false, "Replace existing workspace if it exists.")
	fs.BoolVar(&o.SkipCacheRefresh, "skip-cache-refresh", false, "Skip refreshing the repo cache before cloning. May be out of date with the upstream.")
	fl.bind("repo", bindEnvs("REMUDA_DEFAULT_REPO"), bindKey("repos.default_repo"))
	fl.bind("repo-url", bindEnvs("REMUDA_DEFAULT_REPO_URL"), bindKey("repos.default_repo_url"))
	registerRepoAliasCompletion(cmd, "repo")
}

// normalize expands a shorthand repo URL after flag resolution.
func (o *CloneRepoOption) normalize() {
	if strings.TrimSpace(o.RepoURL) != "" {
		o.RepoURL = github.ExpandRepoURL(o.RepoURL)
	}
}

// repoSelection derives the repo selection (and slug for per_repo overlays)
// from the resolved flag values, without interactive fallbacks.
func (o CloneRepoOption) repoSelection(ctx Context, opts RepoResolutionOptions) RepoSelection {
	opts.AllowFallback = false
	selection, err := resolveRepoSelection(ctx, o, opts)
	if err != nil {
		return RepoSelection{}
	}
	return selection
}

type FullCloneOption struct {
	FullClone bool
}

func (o *FullCloneOption) register(cmd *cobra.Command, fl *flagSet) {
	cmd.Flags().BoolVar(&o.FullClone, "full-clone", false, "Clone the entire repository instead of creating a linked worktree (slower, higher disk usage).")
	fl.negatable("full-clone")
}

// CloneHooksOption exposes the shared toggle for skipping built-in clone hooks.
type CloneHooksOption struct {
	NoCloneHooks bool
}

func (o *CloneHooksOption) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.NoCloneHooks, "no-clone-hooks", false, "Skip running all post-clone hooks (built-in and config-defined).")
}

// ContextEngineeringOptions captures the common flags to help add context to agent
// sessions.
type ContextEngineeringOptions struct {
	Jira         []string
	JiraEndpoint string
	JiraUser     string
	JiraToken    string
	SlackThread  []string
	GitHubIssue  []string
	ghIssueAlias []string
	Use          []string
	NoUse        []string
}

func (c *ContextEngineeringOptions) register(cmd *cobra.Command, fl *flagSet) {
	fs := cmd.Flags()
	fs.StringSliceVar(&c.Jira, "jira", nil, "JIRA ticket ID to prepend as context. Repeatable. For vibe, this also drives default name derivation when --name is omitted.")
	fs.StringVar(&c.JiraEndpoint, "jira-endpoint", "", "Jira base URL used by --jira context (for example https://your-domain.atlassian.net).")
	fs.StringVar(&c.JiraUser, "jira-user", "", "Jira user/email used by --jira context authentication.")
	fs.StringVar(&c.JiraToken, "jira-token", "", "Jira API token used by --jira context authentication. Prefer env/config over direct CLI usage when possible.")
	fs.StringSliceVar(&c.SlackThread, "slack-thread", nil, "Slack thread URL to import as context (repeatable, requires SLACK_TOKEN).")
	fs.StringSliceVar(&c.GitHubIssue, "github-issue", nil, "GitHub issue URL or number to prepend as context (repeatable; number requires repo inference).")
	fs.StringSliceVar(&c.ghIssueAlias, "gh-issue", nil, "Alias for --github-issue.")
	fs.Lookup("gh-issue").Hidden = true
	fs.StringSliceVarP(&c.Use, "use", "u", nil, "Prepend one or more saved prompts (repeatable). Custom prompts override built-ins when names collide.")
	fs.StringSliceVar(&c.NoUse, "no-use", nil, "Exclude one or more saved prompts (repeatable).")
	fl.bind("jira-endpoint", bindEnvs("REMUDA_JIRA_ENDPOINT"), bindKey("jira.endpoint"))
	fl.bind("jira-user", bindEnvs("REMUDA_JIRA_USER"), bindKey("jira.user"))
	fl.bind("jira-token", bindEnvs("REMUDA_JIRA_API_TOKEN", "REMUDA_JIRA_TOKEN"), bindKey("jira.api_token"))
	fl.bind("use", bindEnvs("REMUDA_USE_PROMPTS"), bindKey("defaults.use_prompts"), bindMergeConfigSlice())
	fl.bind("no-use", bindKey("defaults.no_use"))
	registerPromptNameCompletion(cmd, "use")
	registerNoUsePromptNameCompletion(cmd, "no-use")
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
			return nil, pkgerrors.Errorf("invalid --jira value %q: expected format ABC-123", key)
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
			prompt, err := ctx.Remuda.ShowPrompt(builtin)
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
			return nil, pkgerrors.Wrap(err, "jira context")
		}
		addedContext = append(addedContext, jiraContext)
	}

	// Slack thread context
	var slackContext string
	if len(c.SlackThread) > 0 {
		var err error
		slackContext, err = slack.BuildSlackThreadContext(ctx.Remuda.Slack, c.SlackThread)
		if err != nil {
			return nil, pkgerrors.Wrap(err, "slack thread context")
		}
		addedContext = append(addedContext, slackContext)
	}

	// GitHub issues context
	if len(c.GitHubIssue) > 0 {
		githubContext, err := github.BuildIssueContext(ctx.Remuda.GitHub, input.GitHubRepoSlug, c.GitHubIssue)
		if err != nil {
			return nil, pkgerrors.Wrap(err, "github issue context")
		}
		addedContext = append(addedContext, githubContext)
	}

	return addedContext, nil
}

func (c ContextEngineeringOptions) validatePromptUsage(ctx Context, prompt string) error {
	if strings.TrimSpace(prompt) != "" {
		return nil
	}

	if len(c.Jira) > 0 || len(c.SlackThread) > 0 || len(c.GitHubIssue) > 0 {
		return pkgerrors.New("prompt context flags (--jira/--slack-thread/--github-issue/--gh-issue) require a non-empty prompt")
	}

	// Allow REMUDA_USE_PROMPTS defaults without forcing a prompt when the user omits it.
	// If --use/-u is explicitly set, fail fast to avoid silently ignoring it.
	if len(c.Use) > 0 && ctx.FlagExplicit("use") {
		return pkgerrors.New("--use/-u requires a non-empty prompt")
	}

	return nil
}

func (c ContextEngineeringOptions) effectiveUsePrompts() []string {
	if len(c.Use) == 0 {
		return nil
	}
	if len(c.NoUse) == 0 {
		return c.Use
	}
	exclude := make(map[string]struct{}, len(c.NoUse))
	for _, name := range c.NoUse {
		exclude[name] = struct{}{}
	}
	kept := make([]string, 0, len(c.Use))
	for _, name := range c.Use {
		if _, ok := exclude[name]; ok {
			continue
		}
		kept = append(kept, name)
	}
	return kept
}

func (c ContextEngineeringOptions) effectiveUsePromptNames() []string {
	return c.effectiveUsePrompts()
}

func (c ContextEngineeringOptions) validatePromptNames(ctx Context) error {
	if len(c.Use) == 0 && len(c.NoUse) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Use)+len(c.NoUse))
	names = append(names, c.Use...)
	names = append(names, c.NoUse...)
	checked := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, ok := checked[name]; ok {
			continue
		}
		checked[name] = struct{}{}
		if _, err := ctx.Remuda.ShowPrompt(name); err != nil {
			return err
		}
	}
	return nil
}

func (c *ContextEngineeringOptions) afterApply(ctx Context) error {
	if len(c.ghIssueAlias) > 0 {
		c.GitHubIssue = append(c.GitHubIssue, c.ghIssueAlias...)
		c.ghIssueAlias = nil
	}

	normalizedJira, err := normalizeAndValidateJiraKeys(c.Jira)
	if err != nil {
		return err
	}
	c.Jira = normalizedJira
	c.JiraEndpoint = strings.TrimSpace(c.JiraEndpoint)
	c.JiraUser = strings.TrimSpace(c.JiraUser)
	c.JiraToken = strings.TrimSpace(c.JiraToken)

	return c.validatePromptNames(ctx)
}

func shouldAddMainPromptMarker(wrapUsePrompts bool, usePromptsSelected bool) bool {
	return wrapUsePrompts && usePromptsSelected
}

// SessionLaunchOptions captures the common session-manager flags shared by session-like commands.
type SessionLaunchOptions struct {
	Detached bool
	NoTmux   bool
	Attach   bool
}

func (o *SessionLaunchOptions) register(cmd *cobra.Command, fl *flagSet) {
	fs := cmd.Flags()
	fs.BoolVar(&o.Detached, "detached", true, "Run the session in the background with your configured terminal multiplexer.")
	fs.BoolVar(&o.NoTmux, "no-tmux", false, "Run without the configured session manager (detached tmux by default).")
	fs.Lookup("no-tmux").Hidden = true
	fs.BoolVar(&o.Attach, "attach", false, "Attach to the session immediately after launching (requires detached mode).")
	fl.negatable("detached")
}

func (o SessionLaunchOptions) DetachedMode() bool {
	return o.Detached && !o.NoTmux
}

// AgentSessionOptions captures the common agent configuration flags shared by vibe commands.
type AgentSessionOptions struct {
	SessionLaunchOptions

	Agent          string
	Model          string
	ReasoningLevel string
	AgentCmd       string
	AgentArg       []string
}

func (o *AgentSessionOptions) register(cmd *cobra.Command, fl *flagSet) {
	o.SessionLaunchOptions.register(cmd, fl)
	fs := cmd.Flags()
	fs.StringVar(&o.Agent, "agent", "codex", "Built-in agent to use (codex|opencode|claude|bash).")
	fs.StringVar(&o.Model, "model", "", "Specific model to use. Use agent-default to omit any model flag and let the agent CLI choose its own default.")
	fs.StringVar(&o.ReasoningLevel, "reasoning-level", "", "Reasoning level for codex/claude (none|minimal|low|medium|high|xhigh for codex; passed through to claude --effort for claude).")
	fs.StringVar(&o.AgentCmd, "agent-cmd", "", "Override the agent command entirely.")
	fs.StringArrayVar(&o.AgentArg, "agent-arg", nil, "Additional argument to append to the selected built-in agent command (repeatable). Ignored when --agent-cmd is set.")
	fl.bind("agent", bindEnvs("REMUDA_AGENT"), bindKey("defaults.agent"), bindEnum(enums.ValidAgents...))
	fl.bind("model", bindEnvs("REMUDA_MODEL"), bindKey("defaults.model"))
	fl.bind("reasoning-level", bindEnvs("REMUDA_REASONING_LEVEL"), bindKey("defaults.reasoning_level"))
	fl.bind("agent-cmd", bindKey("defaults.agent_cmd"))
	registerStaticCompletion(cmd, "agent", enums.ValidAgents)
	registerModelCompletion(cmd)
	registerReasoningLevelCompletion(cmd)
}

func (o *AgentSessionOptions) afterApply() error {
	for i, arg := range o.AgentArg {
		if strings.TrimSpace(arg) == "" {
			return pkgerrors.Errorf("--agent-arg[%d]: agent arg cannot be empty", i)
		}
	}
	return nil
}

// APIKeyOptions manages CLI flags and env fallback for agent API keys.
type APIKeyOptions struct {
	OpenAIAPIKey string
}

func (o *APIKeyOptions) register(cmd *cobra.Command, fl *flagSet) {
	cmd.Flags().StringVar(&o.OpenAIAPIKey, "openai-api-key", "", "OpenAI API key to pass to agents (overrides env lookup).")
	fl.bind("openai-api-key", bindEnvs("OPENAI_API_KEY"))
}

// SlugifyOptions captures configuration for LLM-backed slugify.
type SlugifyOptions struct {
	SlugifyReasoningLevel string
}

func (o *SlugifyOptions) register(cmd *cobra.Command, fl *flagSet) {
	cmd.Flags().StringVar(&o.SlugifyReasoningLevel, "slugify-reasoning-level", "low", "Reasoning level for slugify (none|minimal|low|medium|high|xhigh).")
	fl.bind("slugify-reasoning-level",
		bindEnvs("REMUDA_SLUGIFY_REASONING_LEVEL"),
		bindKey("defaults.slugify_reasoning_level"),
		bindEnum(enums.ValidSlugifyReasoningLevels...),
	)
	registerStaticCompletion(cmd, "slugify-reasoning-level", enums.ValidSlugifyReasoningLevels)
}

type VibeContainerOptions struct {
	Container           bool
	ContainerName       string
	ContainerOpt        []string
	ContainerInheritEnv []string
}

func (o *VibeContainerOptions) register(cmd *cobra.Command, fl *flagSet) {
	fs := cmd.Flags()
	fs.BoolVar(&o.Container, "container", false, "Run session inside a Docker container.")
	fs.StringVar(&o.ContainerName, "container-name", "", "Container image to use when --container is set.")
	fs.StringSliceVar(&o.ContainerOpt, "container-opt", nil, "Append raw docker run argument (repeatable).")
	fs.StringSliceVar(&o.ContainerInheritEnv, "container-inherit-env", nil, "Forward host env var into container (repeatable).")
	fl.negatable("container")
	fl.bind("container", bindEnvs("REMUDA_CONTAINER"), bindKey("defaults.container.enabled"))
	fl.bind("container-name", bindKey("defaults.container.image"))
	fl.bind("container-opt", bindEnvs("REMUDA_CONTAINER_OPTS"), bindKey("defaults.container.opts"))
	fl.bind("container-inherit-env", bindEnvs("REMUDA_CONTAINER_INHERIT_ENVS"), bindKey("defaults.container.inherit_env"))
}

func (o VibeContainerOptions) Validate() error {
	for _, name := range o.ContainerInheritEnv {
		if err := util.ValidateEnvVarName(name); err != nil {
			return pkgerrors.Wrap(err, "--container-inherit-env")
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
	return pkgerrors.New(
		"container mode requires an explicit image; pass --container-name or configure defaults.container.image (including profiles.<name>.container.image or per_repo.<slug>.defaults.container.image)",
	)
}

// ExperimentsOption captures experimental feature toggles.
//
// REMUDA_EXPERIMENTS accepts a comma- or whitespace-separated list,
// case-insensitive, e.g. "my-experiment, other".
const experimentUsePromptsContextWrapper = "use-prompts-context-wrapper"

type ExperimentsOption struct {
	Experiments string
}

func (o *ExperimentsOption) register(cmd *cobra.Command, fl *flagSet) {
	cmd.Flags().StringVar(&o.Experiments, "experiments", "", "Enable experimental features (comma- or whitespace-separated list).")
	fl.bind("experiments", bindEnvs("REMUDA_EXPERIMENTS"), bindKey("defaults.experiments"))
}

// registerProfileFlag adds the --profile flag shared by profile-aware
// commands. REMUDA_PROFILE and per_repo profile selection are handled by
// selectProfile, not by flag binding.
func registerProfileFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "profile", "p", "", "Config profile name to apply from config.yaml (profiles section).")
	registerProfileNameCompletion(cmd, "profile")
}

// registerYoloFlag adds the shared --yolo/--no-yolo flag.
func registerYoloFlag(cmd *cobra.Command, fl *flagSet, target *bool) {
	cmd.Flags().BoolVar(target, "yolo", false, "Ignore sandboxing/approvals for supported agents (Codex/Claude).")
	fl.negatable("yolo")
	fl.bind("yolo", bindEnvs("REMUDA_YOLO"), bindKey("defaults.yolo"))
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
	Name string
	Pick bool
}

func (o *SessionNamePickOption) register(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&o.Name, "name", "", "Session name (org/repo/<name>).")
	fs.BoolVar(&o.Pick, "pick", false, "Use fzf to pick a session interactively when name is omitted.")
	cmd.MarkFlagsMutuallyExclusive("name", "pick")
	cmd.MarkFlagsOneRequired("name", "pick")
	registerSessionNameCompletion(cmd, "name")
}

func pickSessionNames(ctx Context, multi bool) ([]string, error) {
	// Check if we have terminal access. When stdout is piped (e.g., in command
	// substitution like `cd $(remuda session path --pick)`), IsTerminal() returns
	// false, but we can still run fzf if /dev/tty is available.
	if !ctx.Remuda.IO.IsTerminal() && !hasTTY() {
		return nil, pkgerrors.New("--pick requires an interactive TTY")
	}

	selected, err := pickSessionsWithFZF(logging.FromContext(ctx.ctx), ctx.Remuda.Session, multi)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		if multi {
			return nil, pkgerrors.New("no sessions selected")
		}
		return nil, pkgerrors.New("no session selected")
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
		// Callers like `session readbuf` construct this option directly, so
		// enforce the one-of requirement here as well as at flag level.
		return nil, pkgerrors.New("--name or --pick is required")
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
		return nil, pkgerrors.Errorf("fzf not found in PATH; please install fzf or pass a session name")
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
		return nil, pkgerrors.Errorf("no sessions available to pick")
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
		return nil, pkgerrors.Wrap(err, "fzf selection error")
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
		return "", pkgerrors.New("expected exactly one workspace selection")
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
		return nil, pkgerrors.New("no workspaces available to pick")
	}

	args := []string{}
	if multi {
		args = append(args, "--multi")
	}
	cmd := util.CmdWithEnvAndLogger(logger, cmdEnv, "fzf", args...)
	if cmd.Err != nil {
		return nil, pkgerrors.Errorf("fzf not found in PATH; please install fzf or omit --pick")
	}
	cmd.Stdin = &buf
	out, err := cmd.Output()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "fzf selection error")
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
