package cli

import (
	"bytes"
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/cmd/remuda/cli/forms"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/util"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/pkg/errors"
)

const (
	sentinelCustomURL = "__custom_url__"
)

var errWizardFZFUnavailable = stdErrors.New("fzf not available")

type wizardPRItem struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	URL         string `json:"url"`
}

type wizardPRSelection struct {
	Ref         string
	HeadRefName string
}

// wizardSelectRepo prompts for a repository (canonical alias + Custom URL).
func wizardSelectRepo(initAlias, initURL string) (repoChoice, error) {
	choices := canonicalAliasKeys()
	if len(choices) == 0 {
		url, err := promptCustomRepoURL(initURL)
		if err != nil {
			return repoChoice{}, fmt.Errorf("wizard cancelled or failed: %w", err)
		}
		return repoChoice{URL: url}, nil
	}

	opts := make([]huh.Option[string], 0, len(choices)+1)
	for _, k := range choices {
		opts = append(opts, huh.NewOption(k, k))
	}
	opts = append(opts, huh.NewOption("Custom URL", sentinelCustomURL))

	alias := initialRepoAliasSelection(choices, initAlias, initURL)

	err := huh.NewSelect[string]().Title("Repository").
		Description("GitHub repository to clone").
		Options(opts...).Value(&alias).
		Run()
	if err != nil {
		return repoChoice{}, fmt.Errorf("wizard cancelled or failed: %w", err)
	}

	// If custom URL, ask for it now.
	if alias == sentinelCustomURL {
		url, err := promptCustomRepoURL(initURL)
		if err != nil {
			return repoChoice{}, fmt.Errorf("wizard cancelled or failed: %w", err)
		}
		return repoChoice{URL: url}, nil
	}
	// Map alias to URL.
	if u, ok := github.ExpandRepoAlias(alias); ok {
		return repoChoice{Alias: alias, URL: u}, nil
	}
	return repoChoice{}, fmt.Errorf("unknown repository alias: %s", alias)
}

// wizardAgentFlow prompts for agent selection and returns agent/model/cmd/yolo.
// If custom agent is chosen, asks for the full command and suppresses model/yolo prompts.
func wizardAgentFlow(initAgent, initModel, initCmd string, askYolo bool) (agent, model, cmd string, yolo bool, err error) {
	agent = initAgent
	model = initModel
	cmd = initCmd

	err = forms.AgentSelect(&agent).Run()
	if err != nil {
		return
	}

	if agent == forms.UserSelectedCustomAgent {
		err = huh.NewInput().
			Title("Custom agent command").
			Value(&cmd).
			Description("Full command; prompt will be appended").
			Run()
		if err != nil {
			return
		}

		// Normalize agent; runtime uses AgentCmd when set.
		agent = "codex"
		model = ""
		yolo = false
		return
	}

	// Non-custom agents: optional model (+ optional YOLO).
	if askYolo {
		err = huh.NewConfirm().Title("YOLO mode").Description("full autonomy with no sandboxing").Value(&yolo).Run()
		if err != nil {
			return
		}
	}

	return
}

// wizardDetachedFlow prompts for detached mode (tmux/zellij).
func wizardDetachedFlow(detached bool) (bool, error) {
	detachedMode := detached
	err := huh.NewConfirm().
		Title("Detached").
		Description("Launch in the background with a terminal multiplexer").
		Value(&detachedMode).
		Run()
	if err != nil {
		return false, err
	}

	return detachedMode, nil
}

// wizardSelectPR lists recent PRs for the given owner/repo and prompts for selection.
// Returns the PR ref (number as string) and the head branch name. Falls back to manual
// input when listing fails or is empty. When entered manually, attempts to resolve the
// head branch via `gh pr view`.
func wizardSelectPR(logger zerolog.Logger, ownerRepo string, initRef string) ([]wizardPRSelection, error) {
	// Try gh pr list
	var prs []wizardPRItem
	const prLimit = "100"
	if ownerRepo != "" {
		err := spinner.New().
			Title("Fetching PRs...").
			ActionWithErr(func(ctx context.Context) error {
				out, err := util.RunCmdOutputWithLogger(logger, "gh", "pr", "list", "--repo", ownerRepo, "--limit", prLimit, "--json", "number,title,headRefName,url")
				if err != nil {
					return errors.Wrap(err, "fetch PRs")
				}

				err = json.Unmarshal([]byte(out), &prs)
				return errors.Wrap(err, "unmarshal PRs")
			}).
			Run()
		if err != nil {
			return nil, fmt.Errorf("fetch PRs: %w", err)
		}
	}

	if len(prs) > 0 {
		selections, manualChoice, err := wizardSelectPRWithFZF(logger, prs, initRef)
		if err != nil {
			if stdErrors.Is(err, errWizardFZFUnavailable) {
				selections, manualChoice, err = wizardSelectPRWithMenu(prs, initRef, prLimit)
			}
		}
		if err != nil {
			return nil, err
		}
		if !manualChoice {
			return selections, nil
		}
	}

	return wizardSelectPRManual(logger, ownerRepo, initRef)
}

func wizardSelectPRWithFZF(logger zerolog.Logger, prs []wizardPRItem, initRef string) (selections []wizardPRSelection, manualChoice bool, err error) {
	if _, lookErr := exec.LookPath("fzf"); lookErr != nil {
		return nil, false, errWizardFZFUnavailable
	}
	const manualOption = "__manual_pr__"
	var input bytes.Buffer
	headByRef := map[string]string{}
	for _, pr := range prs {
		value := fmt.Sprintf("%d", pr.Number)
		headByRef[value] = pr.HeadRefName
		label := fmt.Sprintf("#%d %s [%s]", pr.Number, pr.Title, pr.HeadRefName)
		fmt.Fprintf(&input, "%s\t%s\n", value, label)
	}
	fmt.Fprintf(&input, "%s\t%s\n", manualOption, "Enter PR ref manually")

	args := []string{"--with-nth=2..", "--delimiter", "\t", "--prompt", "PR> ", "--header", "Select Pull Request(s)", "--multi"}
	if q := strings.TrimSpace(initRef); q != "" {
		args = append(args, "--query", q)
	}
	cmd := util.CmdWithLogger(logger, "fzf", args...)
	cmd.Stdin = &input
	out, cmdErr := cmd.Output()
	if cmdErr != nil {
		return nil, false, fmt.Errorf("wizard cancelled or failed: %w", cmdErr)
	}
	selection := strings.TrimSpace(string(out))
	if selection == "" {
		return nil, false, fmt.Errorf("wizard cancelled or failed: empty selection")
	}
	lines := strings.Split(selection, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		value := parts[0]
		if value == manualOption {
			return nil, true, nil
		}
		head := headByRef[value]
		selections = append(selections, wizardPRSelection{Ref: value, HeadRefName: head})
	}
	if len(selections) == 0 {
		return nil, false, fmt.Errorf("wizard cancelled or failed: empty selection")
	}
	return selections, false, nil
}

func wizardSelectPRWithMenu(prs []wizardPRItem, initRef string, prLimit string) (selections []wizardPRSelection, manualChoice bool, err error) {
	options := []huh.Option[string]{}
	headByRef := map[string]string{}
	const manualOption = "__manual_pr__"
	for _, pr := range prs {
		value := fmt.Sprintf("%d", pr.Number)
		headByRef[value] = pr.HeadRefName
		label := fmt.Sprintf("#%d %s [%s]", pr.Number, pr.Title, pr.HeadRefName)
		options = append(options, huh.NewOption(label, value))
	}
	options = append(options, huh.NewOption("Enter PR ref manually", manualOption))

	vals := []string{}
	if q := strings.TrimSpace(initRef); q != "" {
		vals = append(vals, q)
	}
	err = huh.NewMultiSelect[string]().
		Title("Select Pull Request(s)").
		Description("Space to toggle; Enter to confirm (up to " + prLimit + ")").
		Options(options...).
		Value(&vals).
		WithWidth(72).
		WithHeight(20).
		Run()
	if err != nil {
		return nil, false, fmt.Errorf("wizard cancelled or failed: %w", err)
	}
	if len(vals) == 0 {
		return nil, false, fmt.Errorf("wizard cancelled or failed: no selection")
	}
	for _, value := range vals {
		if value == manualOption {
			return nil, true, nil
		}
		head := headByRef[value]
		selections = append(selections, wizardPRSelection{Ref: value, HeadRefName: head})
	}
	return selections, false, nil
}

// canonicalAliasKeys returns canonical repo keys (prefer full repo name over short aliases).
func canonicalAliasKeys() []string {
	byURL := map[string][]string{}
	for k, url := range github.RepoAliases() {
		byURL[url] = append(byURL[url], k)
	}
	var out []string
	for url, keys := range byURL {
		repo := repoNameFromURL(url)
		pick := ""
		for _, k := range keys {
			if k == repo {
				pick = k
				break
			}
			if len(k) > len(pick) {
				pick = k
			}
		}
		if pick != "" {
			out = append(out, pick)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func repoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return url
	}
	return parts[len(parts)-1]
}

func initialRepoAliasSelection(choices []string, initAlias, initURL string) string {
	alias := strings.TrimSpace(initAlias)
	if contains(choices, alias) {
		return alias
	}
	if canonical, ok := canonicalChoiceForAlias(choices, alias); ok {
		return canonical
	}
	if strings.TrimSpace(initURL) != "" {
		return sentinelCustomURL
	}
	if len(choices) == 0 {
		return sentinelCustomURL
	}
	return choices[0]
}

func canonicalChoiceForAlias(choices []string, alias string) (string, bool) {
	if strings.TrimSpace(alias) == "" {
		return "", false
	}
	repoURL, ok := github.ExpandRepoAlias(alias)
	if !ok {
		return "", false
	}
	for _, choice := range choices {
		choiceURL, choiceOK := github.ExpandRepoAlias(choice)
		if !choiceOK {
			continue
		}
		if choiceURL == repoURL {
			return choice, true
		}
	}
	return "", false
}

func promptCustomRepoURL(initURL string) (string, error) {
	url := strings.TrimSpace(initURL)
	err := huh.NewInput().
		Title("Custom repository URL").
		Value(&url).
		Validate(func(input string) error {
			trimmed := strings.TrimSpace(input)
			if trimmed == "" {
				return stdErrors.New("repository URL is required")
			}
			return github.ValidateRepoURL(trimmed)
		}).
		Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(url), nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// splitCSV splits on commas and trims whitespace, discarding empties.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func wizardSelectPRManual(logger zerolog.Logger, ownerRepo string, initRef string) ([]wizardPRSelection, error) {
	refInput := strings.TrimSpace(initRef)
	prompt := "PR ref(s) (URL or number; comma-separated for multiple)"
	err := huh.NewInput().Title(prompt).Value(&refInput).Run()
	if err != nil {
		return nil, fmt.Errorf("wizard cancelled or failed: %w", err)
	}
	refs := splitFlexibleList(refInput)
	if len(refs) == 0 {
		return nil, fmt.Errorf("wizard cancelled or failed: no PR selected")
	}
	selections := make([]wizardPRSelection, 0, len(refs))
	for _, ref := range refs {
		selections = append(selections, wizardPRSelection{
			Ref:         ref,
			HeadRefName: resolveHeadBranch(logger, ownerRepo, ref),
		})
	}
	return selections, nil
}

func resolveHeadBranch(logger zerolog.Logger, ownerRepo, ref string) string {
	type headOnly struct {
		HeadRefName string `json:"headRefName"`
	}
	args := []string{"gh", "pr", "view", ref, "--json", "headRefName"}
	if strings.TrimSpace(ownerRepo) != "" {
		args = []string{"gh", "pr", "view", "--repo", ownerRepo, ref, "--json", "headRefName"}
	}
	cmd := util.CmdWithLogger(logger, args[0], args[1:]...)
	if out, err := cmd.Output(); err == nil {
		var ho headOnly
		if jsonErr := json.Unmarshal(out, &ho); jsonErr == nil {
			return ho.HeadRefName
		}
	}
	return ""
}
