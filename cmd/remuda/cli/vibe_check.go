package cli

import (
	"fmt"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
	igit "github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/logging"
)

type VibeCheckCmd struct {
	Name    string
	Wizard  bool
	Profile string

	// Clone selection
	CloneRepoOption
	CloneHooksOption
	FullClone bool

	// Agent / session flags (subset of vibe)
	AgentSessionOptions
	APIKeyOptions
	ExperimentsOption
	ContextEngineeringOptions

	PRRef string

	Branch string
}

func (a *app) vibeCheckCmd() *cobra.Command {
	c := &VibeCheckCmd{}
	var fl *flagSet
	cmd := &cobra.Command{
		Use:   "vibe-check [branch]",
		Short: "Review a pull request with AI assistance.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.Branch = args[0]
			}
			err := a.prepare(cmd, prepareOpts{
				fl:       fl,
				profiled: true,
				slugFn: func() string {
					c.CloneRepoOption.normalize()
					if strings.TrimSpace(c.PRRef) != "" && strings.TrimSpace(c.RepoURL) == "" {
						if prURLRepo := github.RepoURLFromPR(c.PRRef); prURLRepo != "" {
							return repoSlugFromURL(prURLRepo)
						}
					}
					return c.repoSelection(*a.kctx, RepoResolutionOptions{}).RepoSlug
				},
			})
			if err != nil {
				return err
			}
			c.CloneRepoOption.normalize()
			if err := c.AgentSessionOptions.afterApply(); err != nil {
				return err
			}
			if err := c.ContextEngineeringOptions.afterApply(*a.kctx); err != nil {
				return err
			}
			return c.Run(*a.kctx)
		},
	}

	fl = newFlagSet(cmd.Flags())
	c.CloneRepoOption.register(cmd, fl)
	c.CloneHooksOption.register(cmd)
	c.AgentSessionOptions.register(cmd, fl)
	c.APIKeyOptions.register(cmd, fl)
	c.ExperimentsOption.register(cmd, fl)
	c.ContextEngineeringOptions.register(cmd, fl)

	fs := cmd.Flags()
	fs.StringVar(&c.Name, "name", "", "Workspace name; defaults to <branch>-code-review (or PR head branch + -code-review when --pr is set).")
	fs.BoolVar(&c.Wizard, "wizard", false, "Launch interactive wizard for this command (requires a TTY).")
	fs.BoolVar(&c.FullClone, "full-clone", true, "Clone the entire repository instead of creating a linked worktree (slower, higher disk usage).")
	fl.negatable("full-clone")
	fs.StringVar(&c.PRRef, "pr", "", "GitHub PR URL (https://github.com/org/repo/pull/N) or PR number when --repo-url/--repo is set. When set, reviews the PR instead of a branch.")
	registerProfileFlag(cmd, &c.Profile)

	return cmd
}

func (c VibeCheckCmd) Run(ctx Context) error {
	cmds := []VibeCheckCmd{c}
	if c.Wizard {
		if !ctx.Remuda.IO.IsTerminal() {
			return pkgerrors.Errorf("--wizard requires an interactive TTY")
		}

		wizardCmds, err := launchVibeCheckWizard(logging.FromContext(ctx.ctx), c)
		if err != nil {
			return err
		}

		cmds = wizardCmds
	}

	total := len(cmds)
	for idx, cmd := range cmds {
		if total > 1 {
			if strings.TrimSpace(cmd.PRRef) != "" {
				ctx.Remuda.IO.Errf("[%d/%d] Reviewing PR %s with workspace %s\n", idx+1, total, strings.TrimSpace(cmd.PRRef), cmd.Name)
			} else {
				ctx.Remuda.IO.Errf("[%d/%d] Reviewing branch %s with workspace %s\n", idx+1, total, strings.TrimSpace(cmd.Branch), cmd.Name)
			}
		}
		if err := cmd.run(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c VibeCheckCmd) run(ctx Context) error {
	// If a GitHub PR URL is provided, default repo selection should follow the URL
	// (even when a repo alias default is set via env/config).
	sourceHint := RepoSourceUnspecified
	if strings.TrimSpace(c.PRRef) != "" && strings.TrimSpace(c.RepoURL) == "" {
		if prURLRepo := github.RepoURLFromPR(c.PRRef); prURLRepo != "" {
			c.RepoURL = prURLRepo
			sourceHint = RepoSourceDerived
		}
	}

	repoSelection, err := resolveRepoSelectionWithFTUE(ctx, c.CloneRepoOption, RepoResolutionOptions{
		AllowFallback: true,
		SourceHint:    sourceHint,
	}, false)
	if err != nil {
		return err
	}
	c.RepoURL = repoSelection.RepoURL

	var repoSlug string
	if len(c.GitHubIssue) > 0 {
		repoSlug = repoSelection.RepoSlug
	}

	if strings.TrimSpace(c.PRRef) != "" && strings.TrimSpace(c.Branch) != "" {
		return pkgerrors.Errorf("cannot use both branch argument and --pr")
	}

	if strings.TrimSpace(c.PRRef) == "" && strings.TrimSpace(c.Branch) == "" {
		return pkgerrors.Errorf("branch is required unless --pr or --wizard is provided")
	}

	if strings.TrimSpace(c.PRRef) == "" {
		if err := igit.ValidateBranchName(c.Branch); err != nil {
			return err
		}
	}

	// Resolve the head branch (and base, when known) before launching vibe so
	// the workspace lands on the right branch and the agent can run a
	// well-formed `git diff origin/<base>...HEAD`.
	headBranch := strings.TrimSpace(c.Branch)
	baseBranch := ""
	var prMeta map[string]any
	if strings.TrimSpace(c.PRRef) != "" {
		if err := ctx.Remuda.GitHub.CheckAuthStatus(); err != nil {
			return err
		}
		prRepoSlug, _ := github.RepoSlugFromURL(c.RepoURL)
		view, err := ctx.Remuda.GitHub.PRViewWithRepo(prRepoSlug, c.PRRef)
		if err != nil {
			return pkgerrors.Wrap(err, "fetching PR details")
		}
		head, _ := view["headRefName"].(string)
		if strings.TrimSpace(head) == "" {
			return pkgerrors.Errorf("gh pr view output missing headRefName")
		}
		headBranch = strings.TrimSpace(head)
		if base, ok := view["baseRefName"].(string); ok {
			baseBranch = strings.TrimSpace(base)
		}
		prMeta = view
	}
	if err := igit.ValidateBranchName(headBranch); err != nil {
		return err
	}

	if !c.Wizard && strings.TrimSpace(c.Name) == "" {
		c.Name = defaultReviewName("", headBranch)
	}
	if !c.Wizard && strings.TrimSpace(c.Name) == "" {
		return pkgerrors.Errorf("--name is required unless --wizard is provided")
	}

	usePromptIDs := c.effectiveUsePromptNames()
	wrapUsePrompts := c.ExperimentEnabled(experimentUsePromptsContextWrapper)
	usePromptsSelected := len(usePromptIDs) > 0

	addedContext, err := c.AddedPromptContext(ctx, PromptContextInput{
		GitHubRepoSlug: repoSlug,
		WrapUsePrompts: wrapUsePrompts,
	})
	if err != nil {
		return pkgerrors.Wrap(err, "adding prompt context")
	}
	agentArgs := effectiveAgentArgsFromKoanf(ctx.EffectiveConfig(), c.Agent, c.AgentArg)

	cmd := internal.VibeCommand{
		Name:           c.Name,
		Agent:          c.Agent,
		Model:          c.Model,
		ReasoningLevel: c.ReasoningLevel,
		AgentCmd:       c.AgentCmd,
		AgentArgs:      agentArgs,
		Detached:       c.DetachedMode(),
		Attach:         c.Attach,
		UsePromptIDs:   usePromptIDs,
		Prompt:         buildVibeCheckPrompt(headBranch, baseBranch, prMeta),
		Clone: internal.CloneCommand{
			Name:           c.Name,
			RepoURL:        c.RepoURL,
			Branch:         headBranch,
			Force:          c.Force,
			SkipCloneHooks: c.NoCloneHooks,
			FullClone:      c.FullClone,
		},
	}
	cmd.BeforePrompt = append(cmd.BeforePrompt, addedContext...)
	if shouldAddMainPromptMarker(wrapUsePrompts, usePromptsSelected) {
		cmd.BeforePrompt = append(cmd.BeforePrompt, "Main prompt:")
	}

	err = ctx.Remuda.Vibe(ctx.ctx, cmd)
	return pkgerrors.Wrap(err, "vibe check")
}

// buildVibeCheckPrompt builds the review instructions handed to the agent.
// When baseBranch is empty (branch-mode without --pr), the agent is told how
// to discover the default base via origin/HEAD.
func buildVibeCheckPrompt(headBranch, baseBranch string, prMeta map[string]any) string {
	var b strings.Builder
	b.WriteString("Pull Request Review\n\n")

	if prMeta != nil {
		if url, ok := prMeta["url"].(string); ok && strings.TrimSpace(url) != "" {
			b.WriteString("URL: ")
			b.WriteString(url)
			b.WriteString("\n")
		}
		if title, ok := prMeta["title"].(string); ok && strings.TrimSpace(title) != "" {
			b.WriteString("Title: ")
			b.WriteString(title)
			b.WriteString("\n")
		}
		if body, ok := prMeta["body"].(string); ok && strings.TrimSpace(body) != "" {
			b.WriteString("Body:\n")
			b.WriteString(strings.TrimRight(body, "\n"))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if baseBranch != "" {
		fmt.Fprintf(&b, "Review the changes on branch `%s` relative to `%s`.\n\n", headBranch, baseBranch)
		fmt.Fprintf(&b, "To see the unified diff, run from the workspace root:\n  git diff origin/%s...HEAD\n\n", baseBranch)
	} else {
		fmt.Fprintf(&b, "Review the changes on branch `%s`.\n\n", headBranch)
		b.WriteString("To see the unified diff, first determine the default base branch and diff against it:\n")
		b.WriteString("  base=$(git symbolic-ref --short refs/remotes/origin/HEAD | sed 's@^origin/@@')\n")
		b.WriteString("  git diff \"origin/$base...HEAD\"\n\n")
	}

	b.WriteString("Focus on correctness, security, performance, reliability, and tests.\n")
	b.WriteString("Output a Markdown report with sections: Summary, Risk, Findings (High/Med/Low with file:line), Suggested Tests, Follow-ups, Verdict.\n")

	return b.String()
}
