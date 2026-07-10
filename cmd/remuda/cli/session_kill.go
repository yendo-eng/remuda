package cli

import (
	"strconv"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal/session"
)

// OptionalStringFlag parses --flag (optionally: --flag=VALUE).
//
// Bare --flag enables it; --flag=VALUE sets a value; if VALUE looks like a
// bool (eg. "true"/"false"), it is treated as a toggle (and clears Value
// when false).
type OptionalStringFlag struct {
	set     bool
	enabled bool
	value   string
}

func (f *OptionalStringFlag) Set(raw string) error {
	f.set = true
	f.enabled = true

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if b, err := strconv.ParseBool(trimmed); err == nil {
		f.enabled = b
		if !b {
			f.value = ""
		}
		return nil
	}

	f.value = raw
	return nil
}

func (f OptionalStringFlag) String() string { return f.value }
func (f OptionalStringFlag) Type() string   { return "string" }

func (f OptionalStringFlag) Enabled() bool { return f.set && f.enabled }
func (f OptionalStringFlag) Value() string { return f.value }

type SessionKillNamePickOption struct {
	Name string
	Pick bool
}

func (o *SessionKillNamePickOption) register(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&o.Name, "name", "", "Session name (org/repo/<name>).")
	fs.BoolVar(&o.Pick, "pick", false, "Use fzf to pick a session interactively when name is omitted.")
	registerSessionNameCompletion(cmd, "name")
}

func (o SessionKillNamePickOption) Validate() error {
	if o.Name == "" && !o.Pick {
		return pkgerrors.New("--name or --pick is required")
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
	SessionKillNamePickOption
	Cleanup   bool
	CloseBD   bool
	ClosePR   OptionalStringFlag
	MergePR   bool
	MergeFlag []string
}

func (a *app) sessionKillCmd() *cobra.Command {
	c := &SessionKillCmd{}
	cmd := &cobra.Command{
		Use:   "kill",
		Short: "Kill one or all sessions (optionally clean up workspace).",
		Args:  cobra.NoArgs,
	}
	c.SessionKillNamePickOption.register(cmd)
	fs := cmd.Flags()
	fs.BoolVar(&c.Cleanup, "cleanup", false, "Also remove the workspace and git worktree for killed sessions.")
	fs.BoolVar(&c.CloseBD, "close-bd", false, "Close the beads issue associated with the session branch.")
	fs.Var(&c.ClosePR, "close-pr", "Close the GitHub PR associated with the session via gh, if present. Optionally provide a closing comment via --close-pr=COMMENT.")
	fs.Lookup("close-pr").NoOptDefVal = "true"
	fs.BoolVar(&c.MergePR, "merge", false, "Rebase-and-merge the GitHub PR associated with the session via gh before killing.")
	fs.StringSliceVar(&c.MergeFlag, "merge-flag", nil, "Flag to pass to gh pr merge when --merge is set. Repeatable; when provided, replaces defaults.merge.gh_flags config.")
	return a.simpleCmd(cmd, nil, func([]string) error {
		if err := c.Validate(); err != nil {
			return err
		}
		return c.Run(*a.kctx)
	})
}

func (c SessionKillCmd) Validate() error {
	if err := c.SessionKillNamePickOption.Validate(); err != nil {
		return err
	}
	for i, mergeFlag := range c.MergeFlag {
		if strings.TrimSpace(mergeFlag) == "" {
			return pkgerrors.Errorf("--merge-flag[%d] cannot be empty", i)
		}
	}
	return nil
}

// configuredMergeFlagsForSession resolves gh pr merge flags for one session:
// explicit --merge-flag wins, then per_repo/defaults merge.gh_flags from
// config (keyed by the session's repo slug), then the built-in --rebase.
func (c SessionKillCmd) configuredMergeFlagsForSession(ctx Context, sessionName string) []string {
	if len(c.MergeFlag) > 0 {
		return append([]string(nil), c.MergeFlag...)
	}

	slug := repoSlugFromSessionName(sessionName)
	if slug == "" {
		baseDir := reposBaseDirForOverlay(ctx, ctx.ConfigFile)
		workspace, err := session.SessionInfo{Name: strings.TrimSpace(sessionName)}.WorkspacePath(baseDir)
		if err == nil {
			slug = normalizeRepoSlug(repoSlugFromWorkspacePath(ctx, ctx.ConfigFile, workspace))
		}
	}

	eff, err := newEffectiveConfig(ctx.ConfigFile, slug, profileRef{})
	if err == nil {
		if flags, ok := effectiveStrings(eff, "defaults.merge.gh_flags"); ok && len(flags) > 0 {
			return flags
		}
	}
	return []string{"--rebase"}
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
