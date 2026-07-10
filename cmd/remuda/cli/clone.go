package cli

import (
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
)

type CloneCmd struct {
	NameWizardOption
	CloneHooksOption
	CloneRepoOption
	FullCloneOption

	RepoURLArg string
	Branch     string
}

func (a *app) cloneCmd() *cobra.Command {
	c := &CloneCmd{}
	var fl *flagSet
	cmd := &cobra.Command{
		Use:   "clone [repo_url]",
		Short: "Clone a repository into a local workspace.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.RepoURLArg = args[0]
			}
			err := a.prepare(cmd, prepareOpts{
				fl: fl,
				slugFn: func() string {
					c.CloneRepoOption.normalize()
					return c.repoSelection(*a.kctx, RepoResolutionOptions{RepoURLArg: c.RepoURLArg}).RepoSlug
				},
			})
			if err != nil {
				return err
			}
			c.CloneRepoOption.normalize()
			return c.Run(*a.kctx)
		},
	}

	fl = newFlagSet(cmd.Flags())
	c.NameWizardOption.register(cmd)
	c.CloneHooksOption.register(cmd)
	c.CloneRepoOption.register(cmd, fl)
	c.FullCloneOption.register(cmd, fl)
	cmd.Flags().StringVar(&c.Branch, "branch", "", "Checkout this branch instead of deriving from --name.")

	return cmd
}

func (c *CloneCmd) Run(ctx Context) error {
	if c.Wizard {
		// Require a TTY to run the wizard.
		if !ctx.Remuda.IO.IsTerminal() {
			return pkgerrors.Errorf("--wizard requires an interactive TTY")
		}

		// Launch wizard prompting, prefilled with existing values (if any).
		sel, err := cloneWizard(CloneWizardPrefill{
			RepoAlias: c.Repo,
			RepoURL:   firstNonEmpty(c.RepoURL, c.RepoURLArg),
			Name:      c.Name,
		})
		if err != nil {
			return err
		}

		sel.SkipCloneHooks = c.NoCloneHooks
		sel.Force = c.Force
		sel.FullClone = c.FullClone
		sel.Branch = c.Branch

		path, err := ctx.Remuda.Clone(sel)
		if err != nil {
			return pkgerrors.Wrap(err, "clone")
		}

		// Print only the cloned directory path to STDOUT for downstream scripts.
		ctx.Remuda.IO.Outln(path)
		return nil
	}

	cmd := internal.CloneCommand{
		Name:             c.Name,
		SkipCloneHooks:   c.NoCloneHooks,
		Force:            c.Force,
		FullClone:        c.FullClone,
		Branch:           c.Branch,
		SkipCacheRefresh: c.SkipCacheRefresh,
	}
	repoSelection, err := resolveRepoSelectionWithFTUE(ctx, c.CloneRepoOption, RepoResolutionOptions{
		AllowFallback: true,
		RepoURLArg:    c.RepoURLArg,
	}, true)
	if err != nil {
		return err
	}
	cmd.RepoURL = repoSelection.RepoURL

	path, err := ctx.Remuda.Clone(cmd)
	if err != nil {
		return pkgerrors.Wrap(err, "clone")
	}

	// Print only the cloned directory path to STDOUT for downstream scripts.
	ctx.Remuda.IO.Outln(path)
	return nil
}
