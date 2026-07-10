package cli

import (
	"strings"

	"github.com/charmbracelet/huh"
	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/github"
)

type repoChoice struct {
	Alias string
	URL   string
}

var ftueSelectRepoFn = ftueSelectRepo

func resolveRepoSelectionWithFTUE(ctx Context, repo CloneRepoOption, opts RepoResolutionOptions, allowFTUE bool) (RepoSelection, error) {
	optsNoFallback := opts
	optsNoFallback.AllowFallback = false
	selection, err := resolveRepoSelection(ctx, repo, optsNoFallback)
	if err != nil {
		return RepoSelection{}, err
	}

	if selection.Source == RepoSourceUnspecified && allowFTUE && ctx.Remuda.IO.IsTerminal() {
		choice, remember, err := ftueSelectRepoFn()
		if err != nil {
			return RepoSelection{}, err
		}
		if remember {
			alias := strings.TrimSpace(choice.Alias)
			url := strings.TrimSpace(choice.URL)
			if alias != "" {
				url = ""
			}
			if _, err := persistDefaultRepoSelection(ctx, alias, url); err != nil {
				ctx.Remuda.IO.Errf("warning: failed to persist default repo: %v\n", err)
			}
		}
		selection, err = repoSelectionFromChoice(choice)
		if err != nil {
			return RepoSelection{}, err
		}
		if err := ctx.ApplyRepoOverlays(selection.RepoSlug); err != nil {
			return RepoSelection{}, err
		}
		return selection, nil
	}

	if selection.Source == RepoSourceUnspecified && opts.AllowFallback {
		return resolveRepoSelection(ctx, repo, opts)
	}

	return selection, nil
}

func ftueSelectRepo() (repoChoice, bool, error) {
	choice, err := wizardSelectRepo("", "")
	if err != nil {
		return repoChoice{}, false, err
	}

	remember := false
	if err := huh.NewConfirm().
		Title("Remember my choice").
		Description("Save this repository as the default for future runs").
		Value(&remember).
		Run(); err != nil {
		return repoChoice{}, false, pkgerrors.Wrap(err, "wizard cancelled or failed")
	}

	return choice, remember, nil
}

func applyRepoSelection(repoOpt *CloneRepoOption, selection repoChoice) {
	if selection.Alias != "" {
		repoOpt.Repo = selection.Alias
		repoOpt.RepoURL = ""
		return
	}
	repoOpt.Repo = ""
	repoOpt.RepoURL = strings.TrimSpace(selection.URL)
}

func repoSelectionFromChoice(choice repoChoice) (RepoSelection, error) {
	if choice.Alias != "" {
		url, err := github.RepoOrURL("", choice.Alias)
		if err != nil {
			return RepoSelection{}, err
		}
		return RepoSelection{
			RepoURL:  url,
			RepoSlug: repoSlugFromURL(url),
			Source:   RepoSourceExplicit,
		}, nil
	}
	url := strings.TrimSpace(choice.URL)
	return RepoSelection{
		RepoURL:  url,
		RepoSlug: repoSlugFromURL(url),
		Source:   RepoSourceExplicit,
	}, nil
}
