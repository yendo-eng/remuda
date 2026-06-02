package cli

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/github"
)

type invocationRepoSlugSource string

const (
	invocationRepoSlugSourceNone                  invocationRepoSlugSource = ""
	invocationRepoSlugSourceRepoURLFlag           invocationRepoSlugSource = "repo-url-flag"
	invocationRepoSlugSourceRepoFlag              invocationRepoSlugSource = "repo-flag"
	invocationRepoSlugSourceClonePositionalRepo   invocationRepoSlugSource = "clone-positional-repo-url"
	invocationRepoSlugSourceWorkspacePath         invocationRepoSlugSource = "workspace-path"
	invocationRepoSlugSourceVibeCheckPRURL        invocationRepoSlugSource = "vibe-check-pr-url"
	invocationRepoSlugSourceConfigDefaultRepoURL  invocationRepoSlugSource = "config-default-repo-url"
	invocationRepoSlugSourceConfigDefaultRepo     invocationRepoSlugSource = "config-default-repo"
	invocationRepoSlugSourceSessionResumePickOnly invocationRepoSlugSource = "session-resume-pick"
)

type invocationProfileSource string

const (
	invocationProfileSourceNone    invocationProfileSource = ""
	invocationProfileSourceFlag    invocationProfileSource = "flag"
	invocationProfileSourceEnv     invocationProfileSource = "env"
	invocationProfileSourcePerRepo invocationProfileSource = "per_repo"
)

type invocationAnalysis struct {
	Command string

	Subcommand string

	UsesRepo        bool
	SupportsProfile bool

	RepoSlug   string
	RepoSource invocationRepoSlugSource

	WorkspacePath string
	ReposBaseDir  string

	ExplicitProfile string
	EnvProfile      string
}

func resolveInvocationAnalysis(kctx Context, cfg *configfile.V1, args []string) invocationAnalysis {
	return resolveInvocationAnalysisWithEnv(kctx, cfg, args, envFromContext(kctx))
}

func resolveInvocationAnalysisWithEnv(kctx Context, cfg *configfile.V1, args []string, env EnvProvider) invocationAnalysis {
	command := invocationCommand(args)
	subcommand := invocationSubcommand(args)

	analysis := invocationAnalysis{
		Command:         command,
		Subcommand:      subcommand,
		UsesRepo:        invocationUsesRepoForCommand(command, subcommand),
		SupportsProfile: invocationSupportsProfileForCommand(command, subcommand),
	}

	if value, ok := findProfileFlagValue(args); ok {
		analysis.ExplicitProfile = strings.TrimSpace(value)
	}
	if analysis.SupportsProfile {
		env = envOrDefault(env)
		if value, ok := env.LookupEnv("REMUDA_PROFILE"); ok {
			analysis.EnvProfile = strings.TrimSpace(value)
		}
	}

	if analysis.UsesRepo {
		slug, source, workspace, baseDir := inferRepoSlugForInvocationWithMetadata(kctx, cfg, args, command, subcommand)
		analysis.RepoSlug = normalizeRepoSlug(slug)
		analysis.RepoSource = source
		analysis.WorkspacePath = strings.TrimSpace(workspace)
		analysis.ReposBaseDir = strings.TrimSpace(baseDir)
	}

	return analysis
}

func selectedProfileFromInvocation(analysis invocationAnalysis, cfg *configfile.V1, repoSlugOverride string) (string, string, invocationProfileSource, bool) {
	if !analysis.SupportsProfile {
		return "", "", invocationProfileSourceNone, false
	}
	if trimmed := strings.TrimSpace(analysis.ExplicitProfile); trimmed != "" {
		return trimmed, "", invocationProfileSourceFlag, true
	}
	if trimmed := strings.TrimSpace(analysis.EnvProfile); trimmed != "" {
		return trimmed, "", invocationProfileSourceEnv, true
	}

	if cfg == nil || len(cfg.PerRepo) == 0 || !analysis.UsesRepo {
		return "", "", invocationProfileSourceNone, false
	}

	slug := normalizeRepoSlug(repoSlugOverride)
	if slug == "" {
		slug = normalizeRepoSlug(analysis.RepoSlug)
	}
	if slug == "" {
		return "", "", invocationProfileSourceNone, false
	}

	overlay, ok := cfg.PerRepo[slug]
	if !ok || overlay.Profile == nil {
		return "", "", invocationProfileSourceNone, false
	}
	trimmed := strings.TrimSpace(*overlay.Profile)
	if trimmed == "" {
		return "", "", invocationProfileSourceNone, false
	}
	return trimmed, slug, invocationProfileSourcePerRepo, true
}

func inferRepoSlugForInvocationWithMetadata(
	kctx Context,
	cfg *configfile.V1,
	args []string,
	command string,
	subcommand string,
) (string, invocationRepoSlugSource, string, string) {
	if command == "" {
		command = invocationCommand(args)
	}
	if subcommand == "" {
		subcommand = invocationSubcommand(args)
	}

	if repoURL, ok := findFlagValue(args, "repo-url"); ok {
		if slug := repoSlugFromURL(github.ExpandRepoURL(repoURL)); slug != "" {
			return normalizeRepoSlug(slug), invocationRepoSlugSourceRepoURLFlag, "", ""
		}
	}

	if repoAlias, ok := findFlagValue(args, "repo"); ok {
		if repoURL, ok := github.ExpandRepoAlias(repoAlias); ok {
			if slug := repoSlugFromURL(repoURL); slug != "" {
				return normalizeRepoSlug(slug), invocationRepoSlugSourceRepoFlag, "", ""
			}
		}
	}

	if command == "clone" {
		if repoURL := findCloneRepoURLArg(args); repoURL != "" {
			if slug := repoSlugFromURL(github.ExpandRepoURL(repoURL)); slug != "" {
				return normalizeRepoSlug(slug), invocationRepoSlugSourceClonePositionalRepo, "", ""
			}
		}
	}

	if command == "vibe" {
		if workspace, ok := findFlagValue(args, "in"); ok {
			baseDir := reposBaseDirForOverlay(kctx, cfg)
			if slug := repoSlugFromWorkspacePath(kctx, cfg, workspace); slug != "" {
				return normalizeRepoSlug(slug), invocationRepoSlugSourceWorkspacePath, workspace, baseDir
			}
		}
	}

	if command == "session" && subcommand == "resume" {
		workspace := findSessionResumeWorkspaceArg(args)
		if workspace == "" {
			return "", invocationRepoSlugSourceSessionResumePickOnly, "", ""
		}
		baseDir := reposBaseDirForOverlay(kctx, cfg)
		if slug := repoSlugFromWorkspacePath(kctx, cfg, workspace); slug != "" {
			return normalizeRepoSlug(slug), invocationRepoSlugSourceWorkspacePath, workspace, baseDir
		}
	}

	// PR URL inference is only relevant for vibe-check, where the positional arg is a PR ref.
	// Avoid scanning all args for PR URLs to prevent false positives (eg. in freeform prompts).
	if command == "vibe-check" {
		for _, arg := range args {
			if repoURL := github.RepoURLFromPR(arg); repoURL != "" {
				if slug := repoSlugFromURL(repoURL); slug != "" {
					return normalizeRepoSlug(slug), invocationRepoSlugSourceVibeCheckPRURL, "", ""
				}
			}
		}
	}

	if cfg != nil && cfg.Repos != nil {
		if cfg.Repos.DefaultRepoURL != nil && strings.TrimSpace(*cfg.Repos.DefaultRepoURL) != "" {
			if slug := repoSlugFromURL(github.ExpandRepoURL(*cfg.Repos.DefaultRepoURL)); slug != "" {
				return normalizeRepoSlug(slug), invocationRepoSlugSourceConfigDefaultRepoURL, "", ""
			}
		}
		if cfg.Repos.DefaultRepo != nil && strings.TrimSpace(*cfg.Repos.DefaultRepo) != "" {
			if repoURL, ok := github.ExpandRepoAlias(*cfg.Repos.DefaultRepo); ok {
				if slug := repoSlugFromURL(repoURL); slug != "" {
					return normalizeRepoSlug(slug), invocationRepoSlugSourceConfigDefaultRepo, "", ""
				}
			}
		}
	}

	return "", invocationRepoSlugSourceNone, "", ""
}

func invocationUsesRepoForCommand(command, subcommand string) bool {
	switch command {
	case "clone", "vibe", "vibe-check":
		return true
	case "session":
		return subcommand == "resume"
	default:
		return false
	}
}

func invocationSupportsProfileForCommand(command, subcommand string) bool {
	switch command {
	case "vibe", "vibe-check":
		return true
	case "session":
		return subcommand == "resume"
	default:
		return false
	}
}
