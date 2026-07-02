package cli

import (
	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal"
)

type CloneCmd struct {
	NameWizardOption `embed:""`
	CloneHooksOption `embed:""`
	CloneRepoOption  `embed:""`
	FullCloneOption  `embed:""`

	RepoURLArg string `arg:"" optional:"" name:"repo_url" help:"Direct git repository URL to clone."`

	// Branch overrides the default of using --name for git branch checkout.
	Branch string `name:"branch" help:"Checkout this branch instead of deriving from --name."`
}

func (c *CloneCmd) Run(ctx Context) error {
	if c.Wizard {
		// Require a TTY to run the wizard.
		if !ctx.Remuda.IO.IsTerminal() {
			return pkgerrors.Errorf("--wizard requires an interactive TTY")
		}

		// Launch wizard prompting, prefilled with existing values (if any).
		sel, err := cloneWizard(CloneWizardPrefill{
			RepoAlias: derefString(c.Repo),
			RepoURL:   firstNonEmpty(derefString(c.RepoURL), c.RepoURLArg),
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
	repoSelection, err := resolveRepoSelectionWithFTUE(ctx, ctx.KongCtx, c.CloneRepoOption, RepoResolutionOptions{
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
