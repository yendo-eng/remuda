package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/cmd/remuda/cli/forms"
	"github.com/yendo-eng/remuda/internal/prompts"
)

type VibeStartWizardPrefill struct {
	Prompt        string
	Use           []string
	NoUse         []string
	RepoURL       string
	RepoAlias     string
	Name          string
	Agent         string
	Model         string
	AgentCmd      string
	Yolo          bool
	Jira          []string
	SlackThreads  []string
	NoTmux        bool
	Container     bool
	ContainerName string
}

func launchVibeStartWizard(ctx Context, pref VibeCmd) (VibeCmd, error) {
	if !ctx.Remuda.IO.IsTerminal() {
		return VibeCmd{}, errors.New("wizard mode requires a TTY")
	}

	sel := pref
	explicitNameProvided := strings.TrimSpace(pref.Name) != ""

	promptList, err := prompts.List()
	if err != nil {
		return VibeCmd{}, fmt.Errorf("failed to load prompts: %w", err)
	}
	promptOptions := make([]huh.Option[PromptName], 0, len(promptList))
	for _, p := range promptList {
		title := p.Name
		if d := strings.TrimSpace(p.Description); d != "" {
			title = title + " — " + d
		}
		promptOptions = append(promptOptions, huh.NewOption(title, PromptName(p.Name)))
	}

	jiraJoined := strings.Join(pref.Jira, ",")
	slackJoined := strings.Join(pref.SlackThread, ",")
	issueJoined := strings.Join(pref.GitHubIssue, ",")

	// Step A: choose repository first.
	repoSelection, err := wizardSelectRepo(derefString(sel.Repo), derefString(sel.RepoURL))
	if err != nil {
		return VibeCmd{}, fmt.Errorf("repo selection: %w", err)
	}
	applyRepoSelection(&sel.CloneRepoOption, repoSelection)

	// Step B: collect prompt/context inputs first.
	stepContext := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Prompt").
				Value(&sel.Prompt).
				Placeholder("What's the task?").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("prompt is required")
					}
					return nil
				}),
		),
		// Built-in prompt bank selection (optional)
		huh.NewGroup(
			huh.NewMultiSelect[PromptName]().
				Title("Add saved prompts? (space to select) ").
				Options(promptOptions...).
				Value(&sel.Use),
		),
		huh.NewGroup(
			huh.NewMultiSelect[PromptName]().
				Title("Exclude saved prompts? (space to select) ").
				Options(promptOptions...).
				Value(&sel.NoUse),
		),
		huh.NewGroup(
			huh.NewInput().Title("JIRA tickets (comma-separated)").Value(&jiraJoined),
		),
		huh.NewGroup(
			huh.NewInput().Title("Slack thread URLs (comma-separated)").Value(&slackJoined),
		),
		huh.NewGroup(
			huh.NewInput().Title("GitHub issues (URL, owner/repo#id, or number)").Value(&issueJoined),
		),
	).WithWidth(forms.DefaultWidth).WithShowHelp(false)
	if err := stepContext.Run(); err != nil {
		return VibeCmd{}, fmt.Errorf("wizard cancelled or failed: %w", err)
	}
	applyVibeWizardContextSelections(&sel, jiraJoined, slackJoined, issueJoined)

	// If the user has not explicitly set a name, suggest one from the first Jira key they provided.
	if !explicitNameProvided {
		if suggestedName, err := suggestVibeWizardNameFromJira(ctx, sel, explicitNameProvided); err != nil {
			return VibeCmd{}, fmt.Errorf("derive jira-based name suggestion: %w", err)
		} else if strings.TrimSpace(suggestedName) != "" {
			sel.Name = suggestedName
		}
	}

	// Step C: name remains editable regardless of prefill source.
	stepName := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Workspace name (also branch)").
				Value(&sel.Name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("name is required")
					}
					return nil
				}),
		),
	).WithWidth(forms.DefaultWidth).WithShowHelp(false)
	if err := stepName.Run(); err != nil {
		return VibeCmd{}, fmt.Errorf("wizard cancelled or failed: %w", err)
	}

	agent, model, cmd, yolo, err := wizardAgentFlow(sel.Agent, sel.Model, sel.AgentCmd, true)
	if err != nil {
		return VibeCmd{}, fmt.Errorf("wizard cancelled or failed: %w", err)
	}
	sel.Agent = agent
	sel.Model = model
	sel.AgentCmd = cmd
	sel.Yolo = yolo

	// Step: choose whether or not to use tmux
	detached, err := wizardDetachedFlow(sel.DetachedMode())
	if err != nil {
		return VibeCmd{}, fmt.Errorf("wizard cancelled or failed: %w", err)
	}
	sel.Detached = detached

	// Step: container mode?
	err = huh.NewConfirm().Title("Run in Docker container").Value(&sel.Container).Run()
	if err != nil {
		return VibeCmd{}, fmt.Errorf("wizard cancelled or failed: %w", err)
	}
	if sel.Container {
		err = huh.NewInput().Title("Container image").Description("(optional)").Placeholder("vibe-dev").Value(&sel.ContainerName).Run()
		if err != nil {
			return VibeCmd{}, fmt.Errorf("wizard cancelled or failed: %w", err)
		}
	}

	if strings.TrimSpace(sel.Name) == "" {
		return VibeCmd{}, fmt.Errorf("name is required")
	}
	return sel, nil
}

func applyVibeWizardContextSelections(sel *VibeCmd, jiraJoined, slackJoined, issueJoined string) {
	if strings.TrimSpace(jiraJoined) != "" {
		sel.Jira = splitCSV(jiraJoined)
	} else {
		sel.Jira = nil
	}
	if strings.TrimSpace(slackJoined) != "" {
		sel.SlackThread = splitCSV(slackJoined)
	} else {
		sel.SlackThread = nil
	}
	if strings.TrimSpace(issueJoined) != "" {
		sel.GitHubIssue = splitCSV(issueJoined)
	} else {
		sel.GitHubIssue = nil
	}
}

func suggestVibeWizardNameFromJira(ctx Context, cmd VibeCmd, explicitNameProvided bool) (string, error) {
	if explicitNameProvided {
		return strings.TrimSpace(cmd.Name), nil
	}
	suggestion, ok, err := deriveWorkspaceNameFromJira(ctx, cmd.ContextEngineeringOptions, cmd.SlugifyReasoningLevel)
	if err != nil {
		return "", err
	}
	if !ok {
		return strings.TrimSpace(cmd.Name), nil
	}
	return suggestion, nil
}
