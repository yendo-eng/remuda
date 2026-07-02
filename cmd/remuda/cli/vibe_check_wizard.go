package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/cmd/remuda/cli/forms"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/prompts"
)

func launchVibeCheckWizard(logger zerolog.Logger, pref VibeCheckCmd) ([]VibeCheckCmd, error) {
	sel := pref

	jiraJoined := strings.Join(pref.Jira, ",")
	issueJoined := strings.Join(pref.GitHubIssue, ",")

	promptList, err := prompts.List()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to load prompts")
	}
	promptOptions := make([]huh.Option[PromptName], 0, len(promptList))
	for _, p := range promptList {
		title := p.Name
		if d := strings.TrimSpace(p.Description); d != "" {
			title = title + " — " + d
		}
		promptOptions = append(promptOptions, huh.NewOption(title, PromptName(p.Name)))
	}

	// First select repository and then PR from list.
	selection, err := wizardSelectRepo(derefString(sel.Repo), derefString(sel.RepoURL))
	if err != nil {
		return nil, pkgerrors.Wrap(err, "repo selection")
	}
	sel.Repo = nil
	sel.RepoURL = optionalString(selection.URL)
	org, repo, perr := github.ParseRepo(selection.URL)
	if perr != nil {
		return nil, perr
	}
	ownerRepo := fmt.Sprintf("%s/%s", org, repo)
	prSelections, perr := wizardSelectPR(logger, ownerRepo, sel.PRRef)
	if perr != nil {
		return nil, perr
	}
	if len(prSelections) == 0 {
		return nil, pkgerrors.Errorf("no pull requests selected")
	}
	singleSelection := len(prSelections) == 1
	if singleSelection {
		// Default workspace name to selected PR's head branch with review suffix when available.
		sel.Name = defaultReviewName(sel.Name, prSelections[0].HeadRefName)
	}

	// Rest of form after repo resolved
	groups := []*huh.Group{}
	if singleSelection {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title("Workspace name (also branch)").
				Value(&sel.Name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return pkgerrors.New("name is required")
					}
					return nil
				}),
		))
	}
	groups = append(groups,
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
			huh.NewInput().Title("GitHub issues (URL, owner/repo#id, or number)").Value(&issueJoined),
		),
	)
	stepRest := huh.NewForm(groups...).WithWidth(forms.DefaultWidth).WithShowHelp(false)
	if err := stepRest.Run(); err != nil {
		return nil, pkgerrors.Wrap(err, "wizard cancelled or failed")
	}

	agent, model, cmd, _, err := wizardAgentFlow(sel.Agent, sel.Model, sel.AgentCmd, false)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "wizard cancelled or failed")
	}
	sel.Agent = agent
	sel.Model = model
	sel.AgentCmd = cmd

	// Step: choose No tmux.
	detached, err := wizardDetachedFlow(sel.DetachedMode())
	if err != nil {
		return nil, pkgerrors.Wrap(err, "wizard cancelled or failed")
	}
	sel.Detached = detached

	if strings.TrimSpace(jiraJoined) != "" {
		sel.Jira = splitCSV(jiraJoined)
	} else {
		sel.Jira = nil
	}
	if strings.TrimSpace(issueJoined) != "" {
		sel.GitHubIssue = splitCSV(issueJoined)
	} else {
		sel.GitHubIssue = nil
	}

	cmds, err := buildVibeCheckWizardCommands(sel, prSelections)
	if err != nil {
		return nil, err
	}

	return cmds, nil
}

// defaultReviewName returns the initial name if provided; otherwise it derives
// a name from the PR head branch and appends "-code-review" (unless already present).
func defaultReviewName(initial, headBranch string) string {
	if s := strings.TrimSpace(initial); s != "" {
		return s
	}
	hb := strings.TrimSpace(headBranch)
	if hb == "" {
		return ""
	}
	// For derived workspace names, avoid path separators that would create
	// surprising workspace/session names or truncate via filepath.Base.
	hb = strings.NewReplacer("/", "-", "\\", "-").Replace(hb)

	const reviewSuffix = "-code-review"
	if strings.HasSuffix(hb, reviewSuffix) {
		return hb
	}
	return hb + reviewSuffix
}

// buildVibeCheckWizardCommands clones the base command for each selected PR and
// returns the finalized list of commands the wizard should execute.
func buildVibeCheckWizardCommands(base VibeCheckCmd, selections []wizardPRSelection) ([]VibeCheckCmd, error) {
	if len(selections) == 0 {
		return nil, pkgerrors.Errorf("no pull requests selected")
	}

	single := len(selections) == 1
	if single && strings.TrimSpace(base.Name) == "" {
		return nil, pkgerrors.Errorf("name is required")
	}

	cmds := make([]VibeCheckCmd, 0, len(selections))
	nameUsage := map[string]int{}
	for _, sel := range selections {
		cmd := base
		cmd.Wizard = false
		cmd.PRRef = strings.TrimSpace(sel.Ref)
		if cmd.PRRef == "" {
			return nil, pkgerrors.Errorf("--pr is required")
		}
		cmd.Branch = ""
		if single {
			cmd.Name = base.Name
		} else {
			cmd.Name = generateBatchWorkspaceName(sel, nameUsage)
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// generateBatchWorkspaceName deterministically assigns a unique workspace name
// for each selected PR when batching.
func generateBatchWorkspaceName(sel wizardPRSelection, usage map[string]int) string {
	base := defaultReviewName("", sel.HeadRefName)
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("%s-code-review", sanitizePRRef(sel.Ref))
	}
	count := usage[base]
	name := base
	if count > 0 {
		name = fmt.Sprintf("%s-%d", base, count+1)
	}
	usage[base] = count + 1
	return name
}

var (
	prURLPattern    = regexp.MustCompile(`/pull/(\d+)`)
	nonAlnumPattern = regexp.MustCompile(`[^a-zA-Z0-9-]+`)
)

// sanitizePRRef normalizes various PR ref formats (numbers, URLs, etc.) into a
// filesystem-safe slug suitable for derived workspace names.
func sanitizePRRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "review"
	}
	if n, err := strconv.Atoi(ref); err == nil {
		return fmt.Sprintf("pr-%d", n)
	}
	if match := prURLPattern.FindStringSubmatch(ref); len(match) == 2 {
		return fmt.Sprintf("pr-%s", match[1])
	}
	sanitized := nonAlnumPattern.ReplaceAllString(ref, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "review"
	}
	return sanitized
}
