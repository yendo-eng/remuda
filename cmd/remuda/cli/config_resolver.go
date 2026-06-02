package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/util"
)

func loadConfigV1(kctx Context) (*configfile.V1, ConfigFileDiscovery, error) {
	discovery, err := DiscoverConfigFile(kctx)
	if err != nil {
		return nil, discovery, err
	}

	if discovery.Path == "" {
		// No config file found; this is not an error.
		return nil, discovery, nil
	}

	data, err := os.ReadFile(discovery.Path)
	if err != nil {
		return nil, discovery, err
	}

	cfg, err := configfile.ParseV1(data)
	if err != nil {
		return nil, discovery, err
	}

	return cfg, discovery, nil
}

func kongOptionsFromConfig(cfg *configfile.V1, env EnvProvider) []kong.Option {
	env = envOrDefault(env)
	resolvers := []kong.Resolver{}

	if cfg != nil {
		resolvers = append(resolvers, NewConfigResolver(cfg, env))
	}
	if !isProcessEnvProvider(env) {
		resolvers = append(resolvers, NewEnvResolver(env))
	}

	if len(resolvers) == 0 {
		return nil
	}
	return []kong.Option{kong.Resolvers(resolvers...)}
}

// ConfigResolver provides default flag values from a parsed V1 config file.
// It implements kong.Resolver.
//
// Precedence (highest to lowest): flags > env > config > built-in defaults
// To achieve this, Resolve returns nil if any of the flag's env vars are set,
// allowing kong's env resolver to take precedence.
type ConfigResolver struct {
	cfg *configfile.V1
	env EnvProvider
}

// NewConfigResolver creates a resolver from a V1 config.
// If cfg is nil, the resolver returns nil for all flags (no defaults).
func NewConfigResolver(cfg *configfile.V1, env EnvProvider) *ConfigResolver {
	return &ConfigResolver{cfg: cfg, env: env}
}

// Validate validates the config against the kong application.
// Since we have our own schema validation in ParseV1, this is a no-op.
func (r *ConfigResolver) Validate(_ *kong.Application) error {
	return nil
}

// hasEnvValue returns true if any of the flag's env vars are set to a non-empty value.
func hasEnvValue(env EnvProvider, flag *kong.Flag) bool {
	for _, name := range flag.Envs {
		if val, ok := env.LookupEnv(name); ok && val != "" {
			return true
		}
	}
	return false
}

// Resolve returns a default value for the given flag, or nil if not configured.
// Returns nil if any of the flag's env vars are set (to preserve env > config precedence).
func (r *ConfigResolver) Resolve(_ *kong.Context, _ *kong.Path, flag *kong.Flag) (any, error) {
	if r.cfg == nil {
		return nil, nil
	}

	// If env var is set, let kong's env resolver handle it (env > config).
	if hasEnvValue(envOrDefault(r.env), flag) {
		return nil, nil
	}

	// Map flag names to config values.
	// The mapping is based on the PRD schema:
	//   - session.manager -> session-manager
	//   - jira.endpoint -> jira-endpoint
	//   - jira.user -> jira-user
	//   - jira.api_token -> jira-token
	//   - defaults.agent -> agent
	//   - defaults.model -> model
	//   - defaults.reasoning_level -> reasoning-level
	//   - defaults.slugify_reasoning_level -> slugify-reasoning-level
	//   - defaults.agent_cmd -> agent-cmd
	//   - defaults.skip_version_check -> skip-version-check
	//   - defaults.use_prompts -> use
	//   - defaults.no_use -> no-use
	//   - defaults.experiments -> experiments
	//   - defaults.container.enabled -> container
	//   - defaults.container.image -> container-name
	//   - defaults.container.opts -> container-opt
	//   - defaults.container.inherit_env -> container-inherit-env
	//   - repos.base_dir -> (environment only, not a flag)
	//   - repos.default_repo -> repo
	//   - repos.default_repo_url -> repo-url

	switch flag.Name {
	case "session-manager":
		if r.cfg.Session != nil && r.cfg.Session.Manager != nil {
			return *r.cfg.Session.Manager, nil
		}

	case "jira-endpoint":
		if r.cfg.Jira != nil && r.cfg.Jira.Endpoint != nil {
			return *r.cfg.Jira.Endpoint, nil
		}

	case "jira-user":
		if r.cfg.Jira != nil && r.cfg.Jira.User != nil {
			return *r.cfg.Jira.User, nil
		}

	case "jira-token":
		if r.cfg.Jira != nil && r.cfg.Jira.APIToken != nil {
			return *r.cfg.Jira.APIToken, nil
		}

	case "agent":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Agent != nil {
			return *r.cfg.Defaults.Agent, nil
		}

	case "model":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Model != nil {
			return *r.cfg.Defaults.Model, nil
		}

	case "reasoning-level":
		if r.cfg.Defaults != nil && r.cfg.Defaults.ReasoningLevel != nil {
			return *r.cfg.Defaults.ReasoningLevel, nil
		}

	case "slugify-reasoning-level":
		if r.cfg.Defaults != nil && r.cfg.Defaults.SlugifyReasoningLevel != nil {
			return *r.cfg.Defaults.SlugifyReasoningLevel, nil
		}

	case "agent-cmd":
		if r.cfg.Defaults != nil && r.cfg.Defaults.AgentCmd != nil {
			return *r.cfg.Defaults.AgentCmd, nil
		}

	case "skip-version-check":
		if r.cfg.Defaults != nil && r.cfg.Defaults.SkipVersionCheck != nil {
			return *r.cfg.Defaults.SkipVersionCheck, nil
		}

	case "use":
		if r.cfg.Defaults != nil && r.cfg.Defaults.UsePrompts != nil {
			// Kong expects slice values as comma-separated string for decoding.
			return strings.Join(*r.cfg.Defaults.UsePrompts, ","), nil
		}

	case "no-use":
		if r.cfg.Defaults != nil && r.cfg.Defaults.NoUse != nil {
			// Kong expects slice values as comma-separated string for decoding.
			return strings.Join(*r.cfg.Defaults.NoUse, ","), nil
		}

	case "experiments":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Experiments != nil {
			// Kong expects slice values as comma-separated string for decoding.
			return strings.Join(*r.cfg.Defaults.Experiments, ","), nil
		}

	case "yolo":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Yolo != nil {
			return *r.cfg.Defaults.Yolo, nil
		}

	case "container":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Container != nil && r.cfg.Defaults.Container.Enabled != nil {
			return *r.cfg.Defaults.Container.Enabled, nil
		}

	case "container-name":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Container != nil && r.cfg.Defaults.Container.Image != nil {
			return *r.cfg.Defaults.Container.Image, nil
		}

	case "container-opt":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Container != nil && r.cfg.Defaults.Container.Opts != nil {
			// Kong expects slice values as comma-separated string for decoding.
			return strings.Join(*r.cfg.Defaults.Container.Opts, ","), nil
		}

	case "container-inherit-env":
		if r.cfg.Defaults != nil && r.cfg.Defaults.Container != nil && r.cfg.Defaults.Container.InheritEnv != nil {
			// Kong expects slice values as comma-separated string for decoding.
			return strings.Join(*r.cfg.Defaults.Container.InheritEnv, ","), nil
		}

	case "repo":
		if r.cfg.Repos != nil && r.cfg.Repos.DefaultRepo != nil {
			return *r.cfg.Repos.DefaultRepo, nil
		}

	case "repo-url":
		if r.cfg.Repos != nil && r.cfg.Repos.DefaultRepoURL != nil {
			return *r.cfg.Repos.DefaultRepoURL, nil
		}
	}

	return nil, nil
}

// LoadConfigForKong discovers and parses a config file, returning options to
// wire into kong. If no config file is found, it returns nil (not an error).
func LoadConfigForKong() ([]kong.Option, error) {
	return LoadConfigForKongWithContext(Context{})
}

// LoadConfigForKongWithContext discovers and parses a config file, returning options to
// wire into kong. If no config file is found, it returns nil (not an error).
func LoadConfigForKongWithContext(kctx Context) ([]kong.Option, error) {
	cfg, _, err := loadConfigV1(kctx)
	if err != nil {
		return nil, err
	}

	if cfg == nil {
		return nil, nil
	}

	// Merge user-defined repo aliases with built-ins (overrides on collision).
	if cfg.Repos != nil && len(cfg.Repos.Aliases) > 0 {
		github.MergeRepoAliases(cfg.Repos.Aliases)
	}

	return kongOptionsFromConfig(cfg, envFromContext(kctx)), nil
}

func applyPerRepoOverlay(kctx Context, cfg *configfile.V1, args []string) error {
	if cfg == nil || len(cfg.PerRepo) == 0 {
		return nil
	}
	analysis := resolveInvocationAnalysis(kctx, cfg, args)
	if !analysis.UsesRepo {
		return nil
	}

	slug := normalizeRepoSlug(analysis.RepoSlug)
	if slug == "" {
		return nil
	}
	overlay, ok := cfg.PerRepo[slug]
	if !ok {
		return nil
	}

	mergeOverlayV1IntoConfig(cfg, overlay, true)

	// If overlays include aliases, they should take effect for this invocation.
	if overlay.Repos != nil && len(overlay.Repos.Aliases) > 0 {
		github.MergeRepoAliases(overlay.Repos.Aliases)
	}

	return nil
}

func applyProfileOverlay(kctx Context, cfg *configfile.V1, args []string) error {
	analysis := resolveInvocationAnalysis(kctx, cfg, args)
	name, slug, source, ok := selectedProfileFromInvocation(analysis, cfg, "")
	if !ok {
		return nil
	}
	if source != invocationProfileSourcePerRepo {
		return applyProfileOverlayByName(cfg, name)
	}
	return applyPerRepoProfileOverlayByName(cfg, slug, name)
}

func applyProfileOverlayByName(cfg *configfile.V1, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	if cfg == nil {
		return fmt.Errorf("profile %q requested but no config file found", trimmed)
	}
	profile, ok := cfg.Profiles[trimmed]
	if !ok {
		return fmt.Errorf("unknown profile %q; define it under profiles in config.yaml", trimmed)
	}
	mergeOverlayV1IntoConfig(cfg, configfile.OverlayV1{Defaults: &profile}, false)
	return nil
}

func applyPerRepoOverlayForSelection(cfg *configfile.V1, selection RepoSelection) error {
	if cfg == nil || len(cfg.PerRepo) == 0 {
		return nil
	}
	slug := normalizeRepoSlug(selection.RepoSlug)
	if slug == "" {
		return nil
	}
	overlay, ok := cfg.PerRepo[slug]
	if !ok {
		return nil
	}
	mergeOverlayV1IntoConfig(cfg, overlay, true)
	if overlay.Repos != nil && len(overlay.Repos.Aliases) > 0 {
		github.MergeRepoAliases(overlay.Repos.Aliases)
	}
	if overlay.Profile != nil {
		return applyPerRepoProfileOverlayByName(cfg, slug, *overlay.Profile)
	}
	return nil
}

func selectedPerRepoProfileForInvocation(kctx Context, cfg *configfile.V1, args []string) (string, string, bool) {
	analysis := resolveInvocationAnalysis(kctx, cfg, args)
	if !analysis.SupportsProfile || cfg == nil || len(cfg.PerRepo) == 0 {
		return "", "", false
	}
	if !analysis.UsesRepo {
		return "", "", false
	}

	slug := normalizeRepoSlug(analysis.RepoSlug)
	if slug == "" {
		return "", "", false
	}
	overlay, ok := cfg.PerRepo[slug]
	if !ok || overlay.Profile == nil {
		return "", "", false
	}
	trimmed := strings.TrimSpace(*overlay.Profile)
	if trimmed == "" {
		return "", "", false
	}
	return trimmed, slug, true
}

func applyPerRepoProfileOverlayByName(cfg *configfile.V1, slug, profile string) error {
	trimmed := strings.TrimSpace(profile)
	if trimmed == "" {
		return nil
	}
	normalizedSlug := normalizeRepoSlug(slug)
	if cfg == nil {
		return fmt.Errorf("per_repo[%q].profile %q requested but no config file found", normalizedSlug, trimmed)
	}
	if _, ok := cfg.Profiles[trimmed]; !ok {
		return fmt.Errorf("per_repo[%q].profile references unknown profile %q; define it under profiles in config.yaml", normalizedSlug, trimmed)
	}
	return applyProfileOverlayByName(cfg, trimmed)
}

func invocationSupportsProfile(args []string) bool {
	return invocationSupportsProfileForCommand(invocationCommand(args), invocationSubcommand(args))
}

func selectedProfileForInvocation(args []string, env EnvProvider) (string, bool) {
	analysis := resolveInvocationAnalysisWithEnv(Context{}, nil, args, env)
	if !analysis.SupportsProfile {
		return "", false
	}
	if strings.TrimSpace(analysis.ExplicitProfile) != "" {
		return strings.TrimSpace(analysis.ExplicitProfile), true
	}
	if strings.TrimSpace(analysis.EnvProfile) != "" {
		return strings.TrimSpace(analysis.EnvProfile), true
	}
	return "", false
}

func findProfileFlagValue(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return "", false
		}
		if arg == "--profile" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1], true
			}
			return "", false
		}
		if strings.HasPrefix(arg, "--profile=") {
			return strings.TrimPrefix(arg, "--profile="), true
		}
	}
	return "", false
}

func invocationUsesRepo(args []string) bool {
	return invocationUsesRepoForCommand(invocationCommand(args), invocationSubcommand(args))
}

func inferRepoSlugForInvocation(kctx Context, cfg *configfile.V1, args []string) string {
	analysis := resolveInvocationAnalysis(kctx, cfg, args)
	return normalizeRepoSlug(analysis.RepoSlug)
}

func normalizeRepoSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func repoSlugFromWorkspacePath(kctx Context, cfg *configfile.V1, workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
	}
	if strings.HasPrefix(workspace, "~") {
		home, homeErr := homeDirFromContext(kctx)
		if expanded, err := expandHomePath(workspace, home, homeErr); err == nil && expanded != "" {
			workspace = expanded
		}
	}
	if abs := absPathFromContext(workspace, kctx); abs != "" {
		workspace = abs
	}

	baseDir := reposBaseDirForOverlay(kctx, cfg)
	if baseDir == "" {
		return ""
	}
	if strings.HasPrefix(baseDir, "~") {
		home, homeErr := homeDirFromContext(kctx)
		if expanded, err := expandHomePath(baseDir, home, homeErr); err == nil && expanded != "" {
			baseDir = expanded
		}
	}
	if abs := absPathFromContext(baseDir, kctx); abs != "" {
		baseDir = abs
	}

	if !workspaceWithinBase(baseDir, workspace) {
		return ""
	}

	org, repo, _ := util.SplitWorkspacePath(baseDir, workspace)
	if org == "" || repo == "" {
		return ""
	}
	return normalizeRepoSlug(org + "/" + repo)
}

func workspaceWithinBase(baseDir, workspace string) bool {
	rel, err := filepath.Rel(baseDir, workspace)
	if err != nil {
		return false
	}
	sep := string(filepath.Separator)
	return rel != ".." && !strings.HasPrefix(rel, ".."+sep)
}

func reposBaseDirForOverlay(kctx Context, cfg *configfile.V1) string {
	env := envFromContext(kctx)
	if base := strings.TrimSpace(env.Getenv("REMUDA_REPOS_BASE_DIR")); base != "" {
		return base
	}
	if cfg != nil && cfg.Repos != nil && cfg.Repos.BaseDir != nil {
		if base := strings.TrimSpace(*cfg.Repos.BaseDir); base != "" {
			return base
		}
	}
	return reposBaseDirFromContext(kctx)
}

func reposBaseDirFromContext(kctx Context) string {
	if strings.TrimSpace(kctx.Remuda.Config.ReposBaseDir) != "" {
		return kctx.Remuda.Config.ReposBaseDir
	}
	return internal.ConfigFromEnvWithProvider(kctx.Remuda.Env).ReposBaseDir
}

func invocationCommand(args []string) string {
	index := commandIndex(args)
	if index == -1 {
		return ""
	}
	return args[index]
}

func invocationSubcommand(args []string) string {
	cmd := invocationCommand(args)
	if cmd != "session" {
		return ""
	}
	cmdIndex := commandIndex(args)
	if cmdIndex == -1 {
		return ""
	}
	for i := cmdIndex + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, "--session-manager=") {
			continue
		}
		if arg == "--session-manager" {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func commandIndex(args []string) int {
	if len(args) == 0 {
		return -1
	}

	for _, arg := range args {
		// Avoid changing resolver behavior when running help.
		if arg == "--help" || arg == "-h" {
			return -1
		}
	}

	// Find the command token (first non-flag), skipping known root flags that take values.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return i + 1
			}
			return -1
		}
		if strings.HasPrefix(arg, "--session-manager=") {
			continue
		}
		if arg == "--session-manager" {
			// Skip the value.
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return i
	}

	return -1
}

func findSessionResumeWorkspaceArg(args []string) string {
	cmdIndex := commandIndex(args)
	if cmdIndex == -1 || args[cmdIndex] != "session" {
		return ""
	}
	subcommand := invocationSubcommand(args)
	if subcommand != "resume" {
		return ""
	}

	subIndex := -1
	for i := cmdIndex + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				subIndex = i + 1
			}
			break
		}
		if strings.HasPrefix(arg, "--session-manager=") {
			continue
		}
		if arg == "--session-manager" {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		subIndex = i
		break
	}
	if subIndex == -1 {
		return ""
	}

	for i := subIndex + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "=") {
				continue
			}
			if sessionResumeFlagTakesValue(arg) {
				if i+1 < len(args) {
					i++
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func sessionResumeFlagTakesValue(flag string) bool {
	switch flag {
	case "--container-name", "--container-opt", "--container-inherit-env", "--session-manager", "--profile":
		return true
	default:
		return false
	}
}

func findFlagValue(args []string, name string) (string, bool) {
	prefix := "--" + name
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == prefix {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1], true
			}
			return "", false
		}
		if strings.HasPrefix(arg, prefix+"=") {
			return strings.TrimPrefix(arg, prefix+"="), true
		}
	}
	return "", false
}

func findCloneRepoURLArg(args []string) string {
	cmdIndex := -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" || arg == "-h" {
			return ""
		}
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, "--session-manager=") {
			continue
		}
		if arg == "--session-manager" {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		cmdIndex = i
		break
	}
	if cmdIndex == -1 {
		return ""
	}
	if args[cmdIndex] != "clone" {
		return ""
	}

	for i := cmdIndex + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "=") {
				continue
			}
			if cloneFlagTakesValue(arg) {
				if i+1 < len(args) {
					i++
				}
				continue
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func cloneFlagTakesValue(flag string) bool {
	switch flag {
	case "--name", "--branch", "--repo", "--repo-url":
		return true
	default:
		return false
	}
}

func mergeOverlayV1IntoConfig(cfg *configfile.V1, overlay configfile.OverlayV1, mergeContainerOpts bool) {
	if cfg == nil {
		return
	}

	if overlay.Repos != nil {
		if cfg.Repos == nil {
			cfg.Repos = &configfile.ReposV1{}
		}
		if overlay.Repos.BaseDir != nil {
			cfg.Repos.BaseDir = overlay.Repos.BaseDir
		}
		if overlay.Repos.DefaultRepo != nil {
			cfg.Repos.DefaultRepo = overlay.Repos.DefaultRepo
		}
		if overlay.Repos.DefaultRepoURL != nil {
			cfg.Repos.DefaultRepoURL = overlay.Repos.DefaultRepoURL
		}
		if len(overlay.Repos.Aliases) > 0 {
			if cfg.Repos.Aliases == nil {
				cfg.Repos.Aliases = map[string]string{}
			}
			for k, v := range overlay.Repos.Aliases {
				cfg.Repos.Aliases[k] = v
			}
		}
	}

	if overlay.Session != nil {
		if cfg.Session == nil {
			cfg.Session = &configfile.SessionV1{}
		}
		if overlay.Session.Manager != nil {
			cfg.Session.Manager = overlay.Session.Manager
		}
	}

	if overlay.Defaults != nil {
		if cfg.Defaults == nil {
			cfg.Defaults = &configfile.DefaultsV1{}
		}
		if overlay.Defaults.Agent != nil {
			cfg.Defaults.Agent = overlay.Defaults.Agent
		}
		if overlay.Defaults.Model != nil {
			cfg.Defaults.Model = overlay.Defaults.Model
		}
		if overlay.Defaults.ReasoningLevel != nil {
			cfg.Defaults.ReasoningLevel = overlay.Defaults.ReasoningLevel
		}
		if overlay.Defaults.SlugifyReasoningLevel != nil {
			cfg.Defaults.SlugifyReasoningLevel = overlay.Defaults.SlugifyReasoningLevel
		}
		if overlay.Defaults.AgentCmd != nil {
			cfg.Defaults.AgentCmd = overlay.Defaults.AgentCmd
		}
		if overlay.Defaults.SkipVersionCheck != nil {
			cfg.Defaults.SkipVersionCheck = overlay.Defaults.SkipVersionCheck
		}
		if overlay.Defaults.UsePrompts != nil {
			cfg.Defaults.UsePrompts = overlay.Defaults.UsePrompts
		}
		if overlay.Defaults.NoUse != nil {
			cfg.Defaults.NoUse = overlay.Defaults.NoUse
		}
		if overlay.Defaults.Experiments != nil {
			cfg.Defaults.Experiments = overlay.Defaults.Experiments
		}
		if overlay.Defaults.Yolo != nil {
			cfg.Defaults.Yolo = overlay.Defaults.Yolo
		}
		if overlay.Defaults.Container != nil {
			if cfg.Defaults.Container == nil {
				cfg.Defaults.Container = &configfile.ContainerV1{}
			}
			if overlay.Defaults.Container.Enabled != nil {
				cfg.Defaults.Container.Enabled = overlay.Defaults.Container.Enabled
			}
			if overlay.Defaults.Container.Image != nil {
				cfg.Defaults.Container.Image = overlay.Defaults.Container.Image
			}
			if overlay.Defaults.Container.Opts != nil {
				if mergeContainerOpts && cfg.Defaults.Container.Opts != nil && len(*overlay.Defaults.Container.Opts) > 0 {
					merged := append([]string{}, (*cfg.Defaults.Container.Opts)...)
					merged = append(merged, (*overlay.Defaults.Container.Opts)...)
					cfg.Defaults.Container.Opts = &merged
				} else {
					cfg.Defaults.Container.Opts = overlay.Defaults.Container.Opts
				}
			}
			if overlay.Defaults.Container.InheritEnv != nil {
				cfg.Defaults.Container.InheritEnv = overlay.Defaults.Container.InheritEnv
			}
		}
	}
}
