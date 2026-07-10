package cli

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/prompts"
	"github.com/yendo-eng/remuda/internal/util"
)

const claudeHelpTimeout = 5 * time.Second

var (
	claudeCompletionCacheMu sync.RWMutex
	claudeCompletionCache   = map[string]claudeCompletionHints{}
)

type claudeCompletionHints struct {
	EffortSuggestions []string
}

// completionsCmd generates shell completion scripts. Source the output from
// your shell profile, e.g. `source <(remuda completions bash)`.
func (a *app) completionsCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:       "completions <bash|zsh|fish|powershell>",
		Short:     "Generate shell completions for remuda.",
		Long:      "Generate a shell completion script for remuda.\n\nLoad it in your shell profile, for example:\n  source <(remuda completions bash)\n  source <(remuda completions zsh)\n  remuda completions fish | source",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := a.kctx.Remuda.IO.Out
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			default:
				return pkgerrors.Errorf("unsupported shell %q", args[0])
			}
		},
	}
}

func noFileComp(values []string) ([]string, cobra.ShellCompDirective) {
	return values, cobra.ShellCompDirectiveNoFileComp
}

func registerFlagCompletion(cmd *cobra.Command, flag string, fn func(cmd *cobra.Command, toComplete string) []string) {
	_ = cmd.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return noFileComp(fn(cmd, toComplete))
	})
}

func registerStaticCompletion(cmd *cobra.Command, flag string, values []string) {
	registerFlagCompletion(cmd, flag, func(*cobra.Command, string) []string {
		return append([]string(nil), values...)
	})
}

func registerSessionNameCompletion(cmd *cobra.Command, flag string) {
	registerFlagCompletion(cmd, flag, func(c *cobra.Command, _ string) []string {
		kctx := contextFromCompletion(c)
		if kctx == nil {
			return nil
		}
		sessions, err := kctx.Remuda.SessionList()
		if err != nil {
			return nil
		}
		names := make([]string, 0, len(sessions))
		for _, s := range sessions {
			names = append(names, s.Name)
		}
		return names
	})
}

func registerRepoAliasCompletion(cmd *cobra.Command, flag string) {
	registerFlagCompletion(cmd, flag, func(*cobra.Command, string) []string {
		aliases := []string{}
		for alias := range github.RepoAliases() {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		return aliases
	})
}

func registerProfileNameCompletion(cmd *cobra.Command, flag string) {
	registerFlagCompletion(cmd, flag, func(c *cobra.Command, _ string) []string {
		kctx := contextFromCompletion(c)
		if kctx == nil || kctx.ConfigFile == nil || len(kctx.ConfigFile.Profiles) == 0 {
			return nil
		}
		names := make([]string, 0, len(kctx.ConfigFile.Profiles))
		for name := range kctx.ConfigFile.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	})
}

func registerPromptNameCompletion(cmd *cobra.Command, flag string) {
	registerFlagCompletion(cmd, flag, func(c *cobra.Command, _ string) []string {
		kctx := contextFromCompletion(c)
		if kctx == nil {
			return nil
		}
		return allPromptNames(*kctx)
	})
}

// registerNoUsePromptNameCompletion completes --no-use with the prompts that
// are currently in effect (explicit --use plus env/config defaults), i.e.
// only prompts that excluding would actually remove.
func registerNoUsePromptNameCompletion(cmd *cobra.Command, flag string) {
	registerFlagCompletion(cmd, flag, func(c *cobra.Command, _ string) []string {
		kctx := contextFromCompletion(c)
		if kctx == nil {
			return nil
		}

		useFromFlags, _ := c.Flags().GetStringSlice("use")
		noUseFromFlags, _ := c.Flags().GetStringSlice("no-use")

		env := envFromContext(*kctx)
		var use []string
		if envSet(env, "REMUDA_USE_PROMPTS") {
			// Match runtime precedence: explicit --use replaces env defaults.
			if len(useFromFlags) > 0 {
				use = useFromFlags
			} else {
				use = splitFlexibleList(env.Getenv("REMUDA_USE_PROMPTS"))
			}
		} else {
			var configUse []string
			if kctx.ConfigFile != nil && kctx.ConfigFile.Defaults != nil && kctx.ConfigFile.Defaults.UsePrompts != nil {
				configUse = *kctx.ConfigFile.Defaults.UsePrompts
			}
			use = mergeUnique(configUse, useFromFlags)
		}

		effective := (ContextEngineeringOptions{Use: use, NoUse: noUseFromFlags}).effectiveUsePromptNames()
		if len(effective) == 0 {
			return nil
		}

		effectiveSet := make(map[string]struct{}, len(effective))
		for _, name := range effective {
			effectiveSet[name] = struct{}{}
		}

		all := allPromptNames(*kctx)
		out := make([]string, 0, len(all))
		for _, name := range all {
			if _, ok := effectiveSet[name]; ok {
				out = append(out, name)
			}
		}
		return out
	})
}

func registerModelCompletion(cmd *cobra.Command) {
	registerFlagCompletion(cmd, "model", func(c *cobra.Command, _ string) []string {
		kctx := contextFromCompletion(c)
		if kctx == nil {
			return nil
		}
		agent, _, err := agentlauncher.Parse(completionAgentName(c, *kctx), "", false)
		if err != nil {
			return nil
		}
		return agent.SupportedModels()
	})
}

func registerReasoningLevelCompletion(cmd *cobra.Command) {
	registerFlagCompletion(cmd, "reasoning-level", func(c *cobra.Command, _ string) []string {
		kctx := contextFromCompletion(c)
		if kctx == nil {
			return nil
		}
		agentName := completionAgentName(c, *kctx)

		model, _ := c.Flags().GetString("model")
		if strings.TrimSpace(model) == "" {
			model = strings.TrimSpace(envFromContext(*kctx).Getenv("REMUDA_MODEL"))
		}
		if model == "" {
			model = strings.TrimSpace(defaultModelFromConfig(*kctx))
		}
		model = agentlauncher.EffectiveModel(agentName, model)

		if strings.EqualFold(agentName, string(agentlauncher.AgentClaude)) {
			return claudeHintsForContext(*kctx).EffortSuggestions
		}

		return agentlauncher.SuggestedReasoningLevels(agentName, model)
	})
}

func registerWorkspaceDirCompletion(cmd *cobra.Command, flag string) {
	_ = cmd.RegisterFlagCompletionFunc(flag, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
}

func registerWorkspaceDirPositionalCompletion(cmd *cobra.Command) {
	cmd.ValidArgsFunction = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}
}

// contextFromCompletion resolves the CLI Context for completion callbacks.
func contextFromCompletion(cmd *cobra.Command) *Context {
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	if kctx, ok := ctx.Value(completionContextKey{}).(*Context); ok {
		return kctx
	}
	return nil
}

type completionContextKey struct{}

func completionAgentName(c *cobra.Command, kctx Context) string {
	agentName, _ := c.Flags().GetString("agent")
	agentName = strings.TrimSpace(agentName)
	if agentName == "" || agentName == "codex" && !c.Flags().Changed("agent") {
		if fromEnv := strings.TrimSpace(envFromContext(kctx).Getenv("REMUDA_AGENT")); fromEnv != "" {
			return fromEnv
		}
		if fromConfig := strings.TrimSpace(defaultAgentFromConfig(kctx)); fromConfig != "" {
			return fromConfig
		}
		return "codex"
	}
	return agentName
}

func allPromptNames(kctx Context) []string {
	provider := kctx.Remuda.Env
	promptList, err := prompts.ListWithEnv(provider)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(promptList))
	for _, prompt := range promptList {
		names = append(names, prompt.Name)
	}
	return names
}

func defaultAgentFromConfig(kctx Context) string {
	cfg := kctx.ConfigFile
	if cfg == nil || cfg.Defaults == nil || cfg.Defaults.Agent == nil {
		return ""
	}
	return *cfg.Defaults.Agent
}

func defaultModelFromConfig(kctx Context) string {
	cfg := kctx.ConfigFile
	if cfg == nil || cfg.Defaults == nil || cfg.Defaults.Model == nil {
		return ""
	}
	return *cfg.Defaults.Model
}

func claudeHintsForContext(kctx Context) claudeCompletionHints {
	cacheKey := strings.TrimSpace(envFromContext(kctx).Getenv("PATH"))
	if cacheKey == "" {
		cacheKey = "__empty_path__"
	}

	claudeCompletionCacheMu.RLock()
	if cached, ok := claudeCompletionCache[cacheKey]; ok {
		claudeCompletionCacheMu.RUnlock()
		return cached
	}
	claudeCompletionCacheMu.RUnlock()

	hints := claudeCompletionHints{
		EffortSuggestions: nil,
	}
	helpText := loadClaudeHelpText(kctx)
	hints.EffortSuggestions = mergeEffortSuggestions(
		parseClaudeEffortSuggestions(helpText),
		agentlauncher.ClaudeEffortLevels,
	)

	claudeCompletionCacheMu.Lock()
	// Prefer first-write in case concurrent callers race a cache miss.
	if cached, ok := claudeCompletionCache[cacheKey]; ok {
		claudeCompletionCacheMu.Unlock()
		return cached
	}
	claudeCompletionCache[cacheKey] = hints
	claudeCompletionCacheMu.Unlock()
	return hints
}

func loadClaudeHelpText(kctx Context) string {
	baseCtx := kctx.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := context.WithTimeout(baseCtx, claudeHelpTimeout)
	defer cancel()

	cmdEnv := environFromEnvProvider(envFromContext(kctx))
	baseCmd := util.CmdWithEnv(cmdEnv, "claude", "--help")
	if baseCmd.Err != nil {
		return ""
	}

	//nolint:gosec // G204: this intentionally executes the resolved claude binary for local completion hints.
	cmd := exec.CommandContext(ctx, baseCmd.Path, baseCmd.Args[1:]...)
	cmd.Args[0] = "claude"
	cmd.Env = cmdEnv
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

func parseClaudeEffortSuggestions(helpText string) []string {
	line := findLineContaining(helpText, "--effort <level>")
	if line == "" {
		return nil
	}

	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open == -1 || close <= open {
		return nil
	}

	inside := line[open+1 : close]
	parts := strings.Split(inside, ",")
	if len(parts) == 0 {
		return nil
	}

	valid := map[string]struct{}{}
	for _, level := range agentlauncher.ClaudeEffortLevels {
		valid[level] = struct{}{}
	}

	levels := make([]string, 0, len(parts))
	for _, part := range parts {
		candidate := strings.TrimSpace(strings.Trim(part, `"'`))
		if _, ok := valid[candidate]; !ok {
			continue
		}
		levels = append(levels, candidate)
	}
	return uniqueNonEmpty(levels)
}

func mergeEffortSuggestions(preferred []string, fallback []string) []string {
	merged := make([]string, 0, len(preferred)+len(fallback))
	seen := map[string]struct{}{}

	for _, level := range preferred {
		level = strings.TrimSpace(level)
		if level == "" {
			continue
		}
		if _, ok := seen[level]; ok {
			continue
		}
		merged = append(merged, level)
		seen[level] = struct{}{}
	}

	for _, level := range fallback {
		level = strings.TrimSpace(level)
		if level == "" {
			continue
		}
		if _, ok := seen[level]; ok {
			continue
		}
		merged = append(merged, level)
		seen[level] = struct{}{}
	}

	return merged
}

func findLineContaining(text string, needle string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func uniqueNonEmpty(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := map[string]struct{}{}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		unique = append(unique, value)
		seen[value] = struct{}{}
	}

	if len(unique) == 0 {
		return nil
	}
	return unique
}
