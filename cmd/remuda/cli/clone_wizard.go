package cli

import (
	"strings"

	"github.com/charmbracelet/huh"
	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/cmd/remuda/cli/forms"
	"github.com/yendo-eng/remuda/internal"
)

// CloneWizardPrefill represents initial values passed to the wizard.
type CloneWizardPrefill struct {
	RepoAlias string
	RepoURL   string
	Name      string
}

// launchCloneWizard renders a TUI flow for clone with early custom URL prompt.
func cloneWizard(prefill CloneWizardPrefill) (internal.CloneCommand, error) {
	cmd := internal.CloneCommand{
		Name: prefill.Name,
	}

	// Unified repo selection flow
	selection, err := wizardSelectRepo(prefill.RepoAlias, prefill.RepoURL)
	if err != nil {
		return cmd, err
	}
	cmd.RepoURL = selection.URL

	// Step B: name (required).
	step2 := forms.New(
		huh.NewInput().
			Title("Workspace name (also branch)").
			Value(&cmd.Name).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return pkgerrors.New("name is required")
				}
				return nil
			}),
	)
	if err := step2.Run(); err != nil {
		return cmd, pkgerrors.Wrap(err, "wizard cancelled or failed")
	}

	return cmd, nil
}
