package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yendo-eng/remuda/internal/configfile"
	"github.com/yendo-eng/remuda/internal/session"
)

type SessionKillNamePickOption struct {
	Name string `kong:"name='name',help='Session name (org/repo/<name>).',predictor='session-name'"`
	Pick bool   `kong:"name='pick',help='Use fzf to pick a session interactively when name is omitted.'"`
}

func (o SessionKillNamePickOption) Validate() error {
	if o.Name == "" && !o.Pick {
		return errors.New("--name or --pick is required")
	}
	return nil
}

// SessionNames resolves session names for session kill. If both --name and --pick
// are set, --name takes precedence and --pick is ignored.
func (o SessionKillNamePickOption) SessionNames(ctx Context, multi bool) ([]string, error) {
	if o.Name != "" {
		return []string{o.Name}, nil
	}
	return pickSessionNames(ctx, multi)
}

type SessionKillCmd struct {
	SessionKillNamePickOption `embed:""`
	Cleanup                   bool               `name:"cleanup" help:"Also remove the workspace and git worktree for killed sessions."`
	CloseBD                   bool               `name:"close-bd" help:"Close the beads issue associated with the session branch."`
	ClosePR                   OptionalStringFlag `name:"close-pr" help:"Close the GitHub PR associated with the session via gh, if present. Optionally provide a closing comment via --close-pr=COMMENT."`
	MergePR                   bool               `name:"merge" help:"Rebase-and-merge the GitHub PR associated with the session via gh before killing."`
	MergeFlag                 []string           `name:"merge-flag" help:"Flag to pass to gh pr merge when --merge is set. Repeatable; when provided, replaces defaults.merge.gh_flags config."`
}

func (c SessionKillCmd) Validate() error {
	if err := c.SessionKillNamePickOption.Validate(); err != nil {
		return err
	}
	for i, mergeFlag := range c.MergeFlag {
		if strings.TrimSpace(mergeFlag) == "" {
			return fmt.Errorf("--merge-flag[%d] cannot be empty", i)
		}
	}
	return nil
}

func (c SessionKillCmd) configuredMergeFlags(cfg *configfile.V1) []string {
	if len(c.MergeFlag) > 0 {
		return append([]string(nil), c.MergeFlag...)
	}
	if cfg != nil &&
		cfg.Defaults != nil &&
		cfg.Defaults.Merge != nil &&
		cfg.Defaults.Merge.GHFlags != nil &&
		len(*cfg.Defaults.Merge.GHFlags) > 0 {
		return append([]string(nil), (*cfg.Defaults.Merge.GHFlags)...)
	}
	return []string{"--rebase"}
}

func (c SessionKillCmd) configuredMergeFlagsForSession(ctx Context, sessionName string) []string {
	if len(c.MergeFlag) > 0 {
		return append([]string(nil), c.MergeFlag...)
	}

	mergedCfg := sessionKillConfigWithPerRepoOverlay(ctx, sessionName)
	return c.configuredMergeFlags(mergedCfg)
}

func sessionKillConfigWithPerRepoOverlay(ctx Context, sessionName string) *configfile.V1 {
	if ctx.ConfigFile == nil {
		return nil
	}

	cfg := &configfile.V1{}
	if ctx.ConfigFile.Defaults != nil {
		defaultsCopy := *ctx.ConfigFile.Defaults
		cfg.Defaults = &defaultsCopy
	}

	if len(ctx.ConfigFile.PerRepo) == 0 {
		return cfg
	}

	slug := repoSlugFromSessionName(sessionName)
	if slug == "" {
		baseDir := reposBaseDirForOverlay(ctx, ctx.ConfigFile)
		workspace, err := session.SessionInfo{Name: strings.TrimSpace(sessionName)}.WorkspacePath(baseDir)
		if err == nil {
			slug = normalizeRepoSlug(repoSlugFromWorkspacePath(ctx, ctx.ConfigFile, workspace))
		}
	}
	if slug == "" {
		return cfg
	}

	overlay, ok := ctx.ConfigFile.PerRepo[slug]
	if !ok {
		return cfg
	}
	mergeOverlayV1IntoConfig(cfg, overlay, true)
	return cfg
}

func repoSlugFromSessionName(name string) string {
	parts := strings.Split(strings.TrimSpace(name), "/")
	if len(parts) < 3 {
		return ""
	}
	if parts[0] == "" || parts[1] == "" {
		return ""
	}
	return normalizeRepoSlug(parts[0] + "/" + parts[1])
}

func (c *SessionKillCmd) Run(ctx Context) error {
	names, err := c.SessionNames(ctx, true)
	if err != nil {
		return err
	}

	for _, name := range names {
		mergeFlags := c.configuredMergeFlagsForSession(ctx, name)

		var closePRComment *string
		if c.ClosePR.Enabled() {
			comment := c.ClosePR.Value()
			closePRComment = &comment
		}

		if err := ctx.Remuda.SessionKill(name, c.Cleanup, closePRComment, c.MergePR, mergeFlags, c.CloseBD); err != nil {
			return err
		}

		ctx.Remuda.IO.Outf("Killed session %q\n", name)
	}

	return nil
}
