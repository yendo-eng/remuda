package cli

import (
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/enums"
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

func RemudaPredictors(kctx Context, parser *kong.Kong) map[string]complete.Predictor {
	homeDir, _ := homeDirFromContext(kctx)
	return map[string]complete.Predictor{
		"session-name":            PredictSessionNames(kctx.Remuda),
		"prompt-name":             PredictPromptNames(kctx),
		"no-use-prompt-name":      PredictNoUsePromptNames(kctx),
		"profile-name":            PredictProfileNames(kctx),
		"repo-alias":              PredictRepoAliases(kctx.Remuda),
		"model":                   PredictModel(kctx, parser),
		"reasoning-level":         PredictReasoningLevel(kctx, parser),
		"slugify-reasoning-level": PredictSlugifyReasoningLevel(),
		"workspace-dir":           PredictWorkspaceDir(homeDir),
	}
}

func PredictWorkspaceDir(homeDir string) complete.Predictor {
	// complete.PredictDirs expects the currently typed path in Args.Last. It does not
	// expand "~", so we do a small wrapper to support common shell path forms.
	base := complete.PredictDirs("*")
	return complete.PredictFunc(func(a complete.Args) []string {
		last := a.Last

		// Only expand "~" and "~/" (not "~user").
		if last == "~" || strings.HasPrefix(last, "~/") {
			if homeDir != "" {
				expanded := homeDir
				if strings.HasPrefix(last, "~/") {
					expanded = filepath.Join(homeDir, strings.TrimPrefix(last, "~/"))
				}

				a.Last = expanded
				preds := base.Predict(a)

				homePrefix := homeDir
				if !strings.HasSuffix(homePrefix, string(filepath.Separator)) {
					homePrefix += string(filepath.Separator)
				}

				for i, p := range preds {
					if p == homeDir {
						preds[i] = "~"
						continue
					}
					if strings.HasPrefix(p, homePrefix) {
						preds[i] = "~" + strings.TrimPrefix(p, homeDir)
					}
				}
				return preds
			}
		}

		return base.Predict(a)
	})
}

func PredictSessionNames(k internal.Remuda) complete.PredictFunc {
	return func(a complete.Args) []string {
		sessions, err := k.SessionList()
		if err != nil {
			panic(err)
		}

		names := make([]string, 0, len(sessions))
		for _, s := range sessions {
			names = append(names, s.Name)
		}
		return names
	}
}

func PredictPromptNames(kctx Context) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		return allPromptNames(kctx)
	})
}

func PredictNoUsePromptNames(kctx Context) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		useDefaults := resolvedUsePromptDefaults(kctx)
		useFromFlags := promptNamesFromFlagValues(a.All, "--use", "-u")
		noUseFromFlags := promptNamesFromFlagValues(a.All, "--no-use")
		use := mergePromptNames(useDefaults, useFromFlags)

		effective := (ContextEngineeringOptions{
			Use:   use,
			NoUse: noUseFromFlags,
		}).effectiveUsePromptNames()
		if len(effective) == 0 {
			return nil
		}

		effectiveSet := make(map[string]struct{}, len(effective))
		for _, name := range effective {
			effectiveSet[name] = struct{}{}
		}

		all := allPromptNames(kctx)
		if len(all) == 0 {
			return nil
		}
		out := make([]string, 0, len(all))
		for _, name := range all {
			if _, ok := effectiveSet[name]; ok {
				out = append(out, name)
			}
		}
		return out
	})
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

func resolvedUsePromptDefaults(kctx Context) []PromptName {
	env := envFromContext(kctx)
	if envSet(env, "REMUDA_USE_PROMPTS") {
		names, err := promptNamesFromDefaults(splitFlexibleList(env.Getenv("REMUDA_USE_PROMPTS")))
		if err != nil {
			return nil
		}
		return names
	}

	cfg := kctx.ConfigFile
	if cfg == nil {
		var err error
		cfg, _, err = loadConfigV1(kctx)
		if err != nil {
			return nil
		}
	}
	if cfg == nil || cfg.Defaults == nil || cfg.Defaults.UsePrompts == nil {
		return nil
	}
	names, err := promptNamesFromDefaults(*cfg.Defaults.UsePrompts)
	if err != nil {
		return nil
	}
	return names
}

func promptNamesFromFlagValues(args []string, flags ...string) []PromptName {
	rawValues := flagValues(args, flags...)
	if len(rawValues) == 0 {
		return nil
	}
	out := make([]PromptName, 0, len(rawValues))
	for _, raw := range rawValues {
		parsed, err := promptNamesFromDefaults(splitFlexibleList(raw))
		if err != nil {
			continue
		}
		out = append(out, parsed...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func flagValues(args []string, flags ...string) []string {
	if len(args) == 0 || len(flags) == 0 {
		return nil
	}
	out := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		for _, flag := range flags {
			if arg == flag {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					out = append(out, args[i+1])
				}
				break
			}
			if strings.HasPrefix(arg, flag+"=") {
				out = append(out, strings.TrimPrefix(arg, flag+"="))
				break
			}
			if flag == "-u" && strings.HasPrefix(arg, "-u") && len(arg) > 2 {
				out = append(out, arg[2:])
				break
			}
		}
	}
	return out
}

func PredictProfileNames(kctx Context) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		cfg := kctx.ConfigFile
		if cfg == nil {
			var err error
			cfg, _, err = loadConfigV1(kctx)
			if err != nil {
				return nil
			}
		}
		if cfg == nil || len(cfg.Profiles) == 0 {
			return nil
		}

		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	})
}

func PredictRepoAliases(k internal.Remuda) complete.PredictFunc {
	return func(a complete.Args) []string {
		aliases := []string{}
		aliasMap := github.RepoAliases()
		for alias := range aliasMap {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		return aliases
	}
}

func PredictModel(kctx Context, parser *kong.Kong) complete.PredictFunc {
	return func(a complete.Args) []string {
		agentName := strings.TrimSpace(lastFlagValue(a.All, "--agent"))
		if agentName == "" {
			agentName = strings.TrimSpace(envFromContext(kctx).Getenv("REMUDA_AGENT"))
		}
		if agentName == "" {
			agentName = strings.TrimSpace(defaultAgentFromConfig(kctx))
		}
		if agentName == "" {
			agentName = "codex"
		}

		agent, _, err := agentlauncher.Parse(agentName, "", false)
		if err != nil {
			return nil
		}

		return agent.SupportedModels()
	}
}

func PredictReasoningLevel(kctx Context, parser *kong.Kong) complete.PredictFunc {
	return func(a complete.Args) []string {
		agentName := strings.TrimSpace(lastFlagValue(a.All, "--agent"))
		if agentName == "" {
			agentName = strings.TrimSpace(envFromContext(kctx).Getenv("REMUDA_AGENT"))
		}
		if agentName == "" {
			agentName = strings.TrimSpace(defaultAgentFromConfig(kctx))
		}
		if agentName == "" {
			agentName = "codex"
		}

		model := strings.TrimSpace(lastFlagValue(a.All, "--model"))
		if model == "" {
			model = strings.TrimSpace(envFromContext(kctx).Getenv("REMUDA_MODEL"))
		}
		if model == "" {
			model = strings.TrimSpace(defaultModelFromConfig(kctx))
		}
		model = agentlauncher.EffectiveModel(agentName, model)

		if strings.EqualFold(agentName, string(agentlauncher.AgentClaude)) {
			return claudeHintsForContext(kctx).EffortSuggestions
		}

		return agentlauncher.SuggestedReasoningLevels(agentName, model)
	}
}

func PredictSlugifyReasoningLevel() complete.PredictFunc {
	return func(a complete.Args) []string {
		return append([]string(nil), enums.ValidSlugifyReasoningLevels...)
	}
}

func lastFlagValue(args []string, flag string) string {
	for i := len(args) - 1; i >= 0; i-- {
		arg := args[i]
		if strings.HasPrefix(arg, flag+"=") {
			return strings.TrimPrefix(arg, flag+"=")
		}
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func defaultAgentFromConfig(kctx Context) string {
	cfg, _, err := loadConfigV1(kctx)
	if err != nil || cfg == nil {
		return ""
	}

	// Keep repo alias catalog in sync with config for the remainder of startup.
	// This is safe to call repeatedly and makes completions consistent with parsing.
	if cfg.Repos != nil && len(cfg.Repos.Aliases) > 0 {
		github.MergeRepoAliases(cfg.Repos.Aliases)
	}

	if cfg.Defaults == nil || cfg.Defaults.Agent == nil {
		return ""
	}

	return *cfg.Defaults.Agent
}

func defaultModelFromConfig(kctx Context) string {
	cfg, _, err := loadConfigV1(kctx)
	if err != nil || cfg == nil {
		return ""
	}

	if cfg.Defaults == nil || cfg.Defaults.Model == nil {
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
