package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
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
