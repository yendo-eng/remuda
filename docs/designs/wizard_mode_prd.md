# Remuda Wizard Mode — Product Requirements Document

## Implementation Status (2026-02-13)

Only per-command `--wizard` flows are implemented today. There is no top-level `remuda wizard` subcommand, and review flows use `remuda vibe-check` (with `remuda vibe check` deprecated).

## 1) Purpose

Add an interactive, TUI-driven flow (via charmbracelet/huh) to make common Remuda commands faster and less error-prone by guiding users to supply or confirm arguments and flags.

## 2) Scope (v1)

- Covered commands: `clone`, `vibe`, `vibe check`.
- Optional later: `docker sessions` and other subcommands.
- TUI engine: `github.com/charmbracelet/huh`.
- Parser: `github.com/alecthomas/kong` (existing).

## 3) Interface Options

### Option A — `--wizard` flag (per-command)

- Syntax examples:
  - `remuda clone --wizard`
  - `remuda vibe --wizard`
  - `remuda vibe check --wizard <pr-ref>`
- Behaviour: if `--wizard` is present, the command launches an interactive form that:
  - Pre-fills fields from any provided flags/env.
  - Prompts for missing values or advanced options.
  - On submit, executes the command with the collected values.
- Pros: localized to each command, discoverable via `--help`, trivial mental model.
- Cons: per-command wiring; conditional validation required to avoid failing early on missing args.

### Option B — `wizard` subcommand (aggregator)

- Syntax examples:
  - `remuda wizard` → shows a menu of flows (Clone, Vibe, Vibe Check).
  - `remuda wizard clone` → jumps directly to the clone flow.
- Pros: single entrypoint for “guided mode”, easier to expand flows, no ambiguity.
- Cons: duplicates surface area; users must learn two paths for the same capability.

### Recommendation

Ship both:
- `--wizard` on supported commands for power users and scripts.
- A `wizard` subcommand as a friendly, top-level entrypoint that can be extended.

## 4) User Flows (v1)

### Clone
- Inputs: repo URL (or pick from recent), workspace name, base dir (read-only display, env-driven).
- Actions:
  - Offer recent repos (detected from cache layout or history).
  - Validate repo URL shape; allow empty (defaults to org/repo used most often if defined in config).
- Output: absolute workspace path on stdout, consistent with `clone` contract.

### Vibe
- Inputs: prompt (required), repo URL or select from recent, agent/model, detached tmux vs inline; workspace name.
- Actions:
  - If prompt not provided on CLI, ask for it in the wizard.
  - Confirm tmux settings; show final agent command summary.
- Output: absolute workspace path; agent session launched.

- ### Vibe Check
- Inputs: PR URL or ID (+ optional `--repo-url`), workspace name, output path, agent/model, detached tmux vs inline.
- Actions:
  - Validate PR ref shape; no separate branch flag exists (branch derives from name).
- Output: absolute workspace path then absolute report path on stdout (two lines).

## 5) Behavior & Validation

- TTY detection: wizard requires a TTY. If `--wizard` is passed and stdout is not a TTY, print a concise error and exit non-zero.
- Kong integration:
  - Use XOR groups so that exactly one of `--name` or `--wizard` is required (e.g., `xor:"name_or_wizard"`).
  - Keep other fields optional and validate in the wizard forms when necessary (e.g., non-empty Prompt/Name).
- Defaults & prefill: use env, existing flags, and simple history file to pre-populate forms.

## 6) Implementation Notes

- Package layout:
  - `internal/ui/wizard`: thin TUI forms for each flow; no business logic.
  - `internal/flows`: pure functions that merge flag values + wizard selections → final command config structs (unit testable).
  - Commands call `flows.*` builders; when `Wizard` is set, pass in a `Prompt` interface implemented by the huh-based UI.
- TUI building blocks (huh):
  - Text inputs for strings; Select/Radio for enumerations (agent/model/tmux); Group forms per step.
  - Provide a summary page before execute; allow backtracking.
- Persistence (nice-to-have):
  - Write recent repos and last-used options to `$XDG_CONFIG_HOME/remuda/wizard.yaml` (opt-in). v1 can skip persistence.

## 7) CLI Examples

- Clone with wizard only:
  - `remuda clone --wizard`
- Vibe with prompt gathering:
  - `remuda vibe --wizard` (asks for prompt, repo, agent, tmux)
- Vibe check specifying PR but allowing wizard to fill the rest:
  - `remuda vibe check --wizard 123 --repo-url https://github.com/org/repo`
- Aggregator:
  - `remuda wizard` → choose flow from a menu.

## 8) Error Handling

- Non-TTY: clear error advising to rerun without `--wizard` or use a TTY.
- Cancellation: Esc/Ctrl+C returns to shell with no side-effects; exit code 130.
- Validation errors: show inline form-level errors; do not dump stack traces.

## 9) Non-Functional Requirements

- Performance: form render ≤ 50 ms; negligible overhead before executing command.
- Portability: macOS/Linux terminals; basic 80×24 works. No Windows.
- Tests: ≥ 90% coverage for `flows` package combining inputs and defaults; fake `Prompt` to avoid TUI in tests (use `stretchr/testify`).

## 10) Open Questions

- Should `wizard` remember defaults per repo/org (via config) in v1?
- Should `--wizard` imply `--no-tmux` for `vibe`/`vibe check` to keep UX in the same terminal? (Probably ask in the flow.)
- Do we want a `--yes` non-interactive mode to accept prefilled values without showing the UI when `--wizard` is set in CI? (Likely out of scope.)

## 11) Sketch: Minimal Code Shape

```go
// Root flag (available to all commands):
Wizard bool `help:"Launch interactive wizard for this command."`

// Per-command Run():
func (c *CloneCmd) Run(globals *Root) error {
  if globals.Wizard {
    if !isatty(os.Stdout) { return errors.New("--wizard requires a TTY") }
    sel, err := wizard.CloneForm(…prefills…)
    if err != nil { return err }
    c.RepoURL = coalesce(c.RepoURL, sel.RepoURL)
    c.Branch  = coalesce(c.Branch, sel.Branch)
    …
  } else {
    // validate required fields here (or via Validate()).
  }
  return c.execute()
}
```

```go
// flows package – unit tested.
func BuildCloneConfig(flags CloneFlags, env Env, sel CloneSelection) (CloneConfig, error) {
  // merge + validate; return actionable config used by execute().
}
```

```go
// tests (using stretchr/testify):
func TestBuildCloneConfig_RequiresNameOrWizard(t *testing.T) {
  _, err := BuildCloneConfig(Flags{Repo:"..."}, Env{}, Selection{})
  require.Error(t, err)
}
```
