package prompts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yendo-eng/remuda/internal/env"
)

const promptsDirEnv = "REMUDA_PROMPTS_DIR"

// Minimal built-in prompt bank (v1): small-commits only.

// Prompt describes a reusable snippet.
type Prompt struct {
	Name        string
	Description string
	Content     string
	Builtin     bool
}

var smallCommits = Prompt{
	Name:        "small-commits",
	Description: "Encourage incremental changes and tight loops with git.",
	Content: `Please work in small, verifiable steps. Use git to manage your changes.

- Prefer committing cohesive changes that work incrementally unless directed otherwise.
- After each change, run tests and make a git commit.
  - Stage all intended changes and commit with a short, imperative summary.
  - Example: git add -A && git commit -m "feat: add pagination for transactions list".
  - Use clear commit messages; Follow the style of existing commits in the codebase or any direction.
- Separate refactors from feature changes into distinct commits when possible.
- Keep messages concise; call out risks and rollback points.
- Aim to leave the branch in a PR-ready state with a readable commit history.`,
	Builtin: true,
}

var makePR = Prompt{
	Name:        "make-pr",
	Description: "When you're done, open a GitHub PR using gh; assume you're already on a feature branch (no new branch).",
	Content: `When you finish the requested work, open a GitHub Pull Request using the GitHub CLI (gh).

- Do not create or switch branches; assume you're already on a feature branch.
- Use a clear, imperative title and a concise body summarizing the changes and key risks.
- Preferred command:
  gh pr create --title "<concise, imperative title>" --body "<short summary>" --fill --draft
- If the remote isn't set for the current branch, set the appropriate upstream and then create the PR.
- If a PR already exists for this branch, update it instead of creating a new one (e.g., gh pr view --web or gh pr edit).`,
	Builtin: true,
}

var updateDocs = Prompt{
	Name:        "update-docs",
	Description: "Keep documentation aligned with code changes.",
	Content: `Make sure documentation keeps pace with the code you ship.

- Identify every user- or teammate-facing behavior you've changed; note where it lives today.
- Update READMEs, quick-start guides, and runbooks that reference the touched feature or workflow.
- Refresh internal design docs or ADRs so future contributors understand why the change exists and when to revisit it.
- Flag gaps you can't close right now, capture TODOs with owners, and surface them in the PR description.
- Confirm rendered Markdown/examples build cleanly (lint, formatting, code blocks).
- Highlight new docs in your PR summary so reviewers can verify them quickly.`,
	Builtin: true,
}

var refactorCohesion = Prompt{
	Name:        "refactor-cohesion",
	Description: "Improve cohesion and shared patterns while refactoring.",
	Content: `Refactor to reinforce cohesion across the codebase as you go.

- Map the affected call paths and modules so the new structure still fits the existing architecture.
- Consolidate duplicate logic into shared helpers or packages; remove drift in naming and conventions.
- Update tests alongside refactors to lock in intent.
- Call out mechanical moves versus behavior changes so reviewers can follow along.`,
	Builtin: true,
}

var minimalChange = Prompt{
	Name:        "minimal-change",
	Description: "Keep edits scoped to what the request strictly needs.",
	Content: `Keep scope tight and touch only what the request requires.

- Capture the expected behavior change before you edit; resist opportunistic cleanups.
- Inspect diffs for unrelated formatting or drive-by edits and roll them back.
- Prefer targeted fixes over rewrites; if deeper refactors emerge, split them into a separate plan.
- Add or adjust the smallest set of tests that prove the change; avoid cascading broad updates.
- Before finishing, review git status for strays and remove incidental files or generated artifacts.`,
	Builtin: true,
}

var prototype = Prompt{
	Name:        "prototype",
	Description: "Favor quick proofs of concept over production-hardening.",
	Content: `Prove the idea works before hardening the edges.

- Target the happy path to demonstrate end-to-end viability; skip defensive branches unless they unlock the demo.
- Keep scaffolding thin: stub integrations, hard-code sample data, and avoid over-engineered abstractions.
- Focus on readability of the core flow so teammates can react quickly and iterate.
- Mark follow-up polish or validation with TODOs so future passes know what to shore up.
- Defer exhaustive error handling, logging, and configuration until the prototype earns a green light.
- Call out assumptions, shortcuts, and risks in your notes or commit message so reviewers know what's intentional.`,
	Builtin: true,
}

// registry of built-ins by name.
var builtins = []Prompt{
	smallCommits,
	makePR,
	updateDocs,
	refactorCohesion,
	minimalChange,
	prototype,
}

// List returns all prompts, including any custom prompts discovered on disk.
func List() ([]Prompt, error) {
	return ListWithEnv(env.Default())
}

// ListWithEnv returns all prompts, including any custom prompts discovered on disk.
func ListWithEnv(provider env.Provider) ([]Prompt, error) {
	provider = env.OrDefault(provider)
	out := make([]Prompt, 0, len(builtins))
	out = append(out, builtins...)
	custom, err := loadCustomPrompts(provider)
	if err != nil {
		return nil, err
	}
	return append(out, custom...), nil
}

// Get returns a built-in prompt by name.
func Get(name string) (Prompt, bool) {
	for _, p := range builtins {
		if p.Name == name {
			return p, true
		}
	}

	return Prompt{}, false
}

// Resolve returns a prompt by name, searching user-defined prompt files under
// ~/.remuda/prompts (or REMUDA_PROMPTS_DIR) first, then falling back to built-ins.
func Resolve(name string) (Prompt, error) {
	return ResolveWithEnv(name, env.Default())
}

// ResolveWithEnv returns a prompt by name, searching user-defined prompt files under
// ~/.remuda/prompts (or REMUDA_PROMPTS_DIR) first, then falling back to built-ins.
func ResolveWithEnv(name string, provider env.Provider) (Prompt, error) {
	if err := validatePromptName(name); err != nil {
		return Prompt{}, err
	}
	provider = env.OrDefault(provider)
	if prompt, err := loadCustomPromptByName(provider, name); err == nil {
		return prompt, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Prompt{}, err
	}
	if p, ok := Get(name); ok {
		return p, nil
	}
	return Prompt{}, ErrUnknownPrompt(name)
}

// Compose concatenates the selected built-in prompts in order, then two
// newlines, then the user prompt.
func Compose(names []string, userPrompt string) (string, error) {
	if len(names) == 0 {
		return userPrompt, nil
	}
	// Compute size and build efficiently.
	var parts []string
	parts = make([]string, 0, len(names)+1)
	for _, n := range names {
		p, err := Resolve(n)
		if err != nil {
			return "", err
		}
		parts = append(parts, p.Content)
	}
	// Append user prompt last with required separation.
	composed := joinWithDoubleNewline(parts...) + "\n\n" + userPrompt
	return composed, nil
}

// ErrUnknownPrompt represents a request for an undefined built-in.
type ErrUnknownPrompt string

func (e ErrUnknownPrompt) Error() string { return "unknown prompt: " + string(e) }

// ErrInvalidPromptName surfaces invalid custom prompt names (eg. path traversal).
type ErrInvalidPromptName string

func (e ErrInvalidPromptName) Error() string {
	return "invalid prompt name: " + string(e)
}

func joinWithDoubleNewline(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	// avoid strings.Builder to keep it simple and adequate here
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "\n\n" + parts[i]
	}
	return out
}

func loadCustomPrompts(provider env.Provider) ([]Prompt, error) {
	dir, err := promptsDir(provider)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	prompts := make([]Prompt, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := validatePromptName(name); err != nil {
			continue
		}
		prompt, err := loadPromptFile(dir, name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})
	return prompts, nil
}

func loadCustomPromptByName(provider env.Provider, name string) (Prompt, error) {
	dir, err := promptsDir(provider)
	if err != nil {
		return Prompt{}, err
	}
	if dir == "" {
		return Prompt{}, fs.ErrNotExist
	}
	return loadPromptFile(dir, name)
}

func loadPromptFile(dir, name string) (Prompt, error) {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return Prompt{}, err
	}
	content := string(data)
	desc := describeCustomPrompt(content)
	return Prompt{Name: name, Description: desc, Content: content, Builtin: false}, nil
}

func describeCustomPrompt(content string) string {
	if strings.TrimSpace(content) == "" {
		return "Custom prompt"
	}
	firstLine := content
	if idx := strings.Index(firstLine, "\n"); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstLine = strings.TrimSpace(strings.TrimSuffix(firstLine, "\r"))
	if firstLine == "" {
		return "Custom prompt"
	}
	return firstLine
}

func promptsDir(provider env.Provider) (string, error) {
	provider = env.OrDefault(provider)
	if override := strings.TrimSpace(provider.Getenv(promptsDirEnv)); override != "" {
		return expandHome(provider, override)
	}
	home, err := provider.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".remuda", "prompts"), nil
}

func expandHome(provider env.Provider, path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	provider = env.OrDefault(provider)
	home, err := provider.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	sep := string(filepath.Separator)
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+sep) {
		return filepath.Join(home, path[2:]), nil
	}
	return "", ErrInvalidPromptName(path)
}

func validatePromptName(name string) error {
	if name == "" {
		return ErrInvalidPromptName("name cannot be empty")
	}
	if name == "." || name == ".." {
		return ErrInvalidPromptName(name)
	}
	if strings.ContainsAny(name, "/\\") {
		return ErrInvalidPromptName(name)
	}
	return nil
}
