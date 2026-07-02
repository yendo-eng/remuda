package cli

import (
	"strings"

	"github.com/alecthomas/kong"
	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/util"
)

// RepoSelectionSource describes where a repo selection came from.
type RepoSelectionSource string

const (
	RepoSourceExplicit    RepoSelectionSource = "flag"
	RepoSourceEnv         RepoSelectionSource = "env"
	RepoSourceConfig      RepoSelectionSource = "config"
	RepoSourceWorkspace   RepoSelectionSource = "workspace"
	RepoSourceUnspecified RepoSelectionSource = "unspecified"
	RepoSourceDerived     RepoSelectionSource = "derived"
)

type RepoSelection struct {
	RepoURL  string
	RepoSlug string
	Source   RepoSelectionSource
}

type RepoResolutionOptions struct {
	AllowFallback     bool
	ExistingWorkspace string
	RepoURLArg        string
	SourceHint        RepoSelectionSource
	ReposBaseDir      string
}

func resolveRepoSelection(ctx Context, kctx *kong.Context, repo CloneRepoOption, opts RepoResolutionOptions) (RepoSelection, error) {
	invocation := invocationAnalysis{}
	if kctx != nil {
		invocation = resolveInvocationAnalysis(ctx, nil, kctx.Args)
	}

	repoURL := github.ExpandRepoURL(derefString(repo.RepoURL))
	repoAlias := strings.TrimSpace(derefString(repo.Repo))
	repoURLArg := github.ExpandRepoURL(opts.RepoURLArg)
	explicitRepoURLFlag := flagExplicit(kctx, "repo-url")
	usedRepoURLArg := false

	if repoURLArg != "" && !explicitRepoURLFlag {
		repoURL = repoURLArg
		usedRepoURLArg = true
	}

	if repoURL != "" {
		source := repoSourceForRepoURL(envFromContext(ctx), kctx, usedRepoURLArg, repo.RepoURL, opts.SourceHint)
		return RepoSelection{
			RepoURL:  repoURL,
			RepoSlug: repoSlugFromURL(repoURL),
			Source:   source,
		}, nil
	}

	if repoAlias != "" {
		url, err := github.RepoOrURL("", repoAlias)
		if err != nil {
			return RepoSelection{}, err
		}
		source := repoSourceForAlias(envFromContext(ctx), kctx, repo.Repo)
		return RepoSelection{
			RepoURL:  url,
			RepoSlug: repoSlugFromURL(url),
			Source:   source,
		}, nil
	}

	if strings.TrimSpace(opts.ExistingWorkspace) != "" {
		repoSlug := ""
		if invocation.RepoSource == invocationRepoSlugSourceWorkspacePath {
			repoSlug = normalizeRepoSlug(invocation.RepoSlug)
		}
		if repoSlug == "" {
			baseDir := reposBaseDirForExistingWorkspace(ctx, opts.ReposBaseDir)
			repoSlug = repoSlugFromExistingWorkspace(ctx, baseDir, opts.ExistingWorkspace)
		}
		return RepoSelection{RepoSlug: repoSlug, Source: RepoSourceWorkspace}, nil
	}

	if opts.AllowFallback {
		return RepoSelection{}, errRepoSelectionRequired()
	}

	return RepoSelection{Source: RepoSourceUnspecified}, nil
}

func errRepoSelectionRequired() error {
	return pkgerrors.Errorf(
		"repository is not configured: specify --repo-url/--repo, set REMUDA_DEFAULT_REPO_URL or REMUDA_DEFAULT_REPO, or set repos.default_repo_url/repos.default_repo in config",
	)
}

func repoSourceForRepoURL(env EnvProvider, kctx *kong.Context, repoURLArg bool, repoURL *string, hint RepoSelectionSource) RepoSelectionSource {
	if hint != "" {
		return hint
	}
	if repoURLArg || flagExplicit(kctx, "repo-url") {
		return RepoSourceExplicit
	}
	if envSet(env, "REMUDA_DEFAULT_REPO_URL") {
		return RepoSourceEnv
	}
	if repoURL != nil && strings.TrimSpace(derefString(repoURL)) != "" {
		return RepoSourceConfig
	}
	return RepoSourceUnspecified
}

func repoSourceForAlias(env EnvProvider, kctx *kong.Context, repo *string) RepoSelectionSource {
	if flagExplicit(kctx, "repo") {
		return RepoSourceExplicit
	}
	if envSet(env, "REMUDA_DEFAULT_REPO") {
		return RepoSourceEnv
	}
	if repo != nil && strings.TrimSpace(derefString(repo)) != "" {
		return RepoSourceConfig
	}
	return RepoSourceUnspecified
}

func flagExplicit(kctx *kong.Context, name string) bool {
	if kctx == nil {
		return false
	}
	for _, el := range kctx.Path {
		if el.Flag != nil && el.Flag.Name == name && !el.Resolved {
			return true
		}
	}
	return false
}

func envSet(env EnvProvider, name string) bool {
	if val, ok := env.LookupEnv(name); ok {
		return strings.TrimSpace(val) != ""
	}
	return false
}

func derefString(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}

func optionalString(val string) *string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func repoSlugFromURL(repoURL string) string {
	slug, err := github.RepoSlugFromURL(repoURL)
	if err != nil {
		return ""
	}
	return normalizeRepoSlug(slug)
}

func repoSlugFromExistingWorkspace(ctx Context, baseDir, workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
	}
	if strings.HasPrefix(workspace, "~") {
		home, homeErr := homeDirFromContext(ctx)
		if expanded, err := expandHomePath(workspace, home, homeErr); err == nil && expanded != "" {
			workspace = expanded
		}
	}
	if abs := absPathFromContext(workspace, ctx); abs != "" {
		workspace = abs
	}

	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return ""
	}
	if strings.HasPrefix(baseDir, "~") {
		home, homeErr := homeDirFromContext(ctx)
		if expanded, err := expandHomePath(baseDir, home, homeErr); err == nil && expanded != "" {
			baseDir = expanded
		}
	}
	if abs := absPathFromContext(baseDir, ctx); abs != "" {
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

func reposBaseDirForExistingWorkspace(ctx Context, baseDir string) string {
	if trimmed := strings.TrimSpace(baseDir); trimmed != "" {
		return trimmed
	}
	return reposBaseDirForOverlay(ctx, nil)
}
