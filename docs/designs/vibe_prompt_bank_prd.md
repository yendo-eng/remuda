# `remuda vibe` — Prompt Bank PRD (v1, built‑ins only)

## Implementation Status (2026-02-13)

Prompt discovery matches the PRD, but listing and inspection are exposed via `remuda prompts list` and `remuda prompts show` rather than `--list-prompts` / `--show-prompt` flags.

## Purpose
Add a simple “prompt bank” to `remuda vibe` so users can prepend curated, reusable prompts to their own message with a single flag.

Core UX:
- `remuda vibe --use <name> [--use <name> ...] --name <branch> "<user prompt>"`

In v1 the bank contains only built‑in prompts embedded in the binary. No filesystem or environment lookups.

## Scope
- Built-ins plus user-provided prompts. Drop text files into `~/.remuda/prompts`
  (or set `REMUDA_PROMPTS_DIR`) and reference them by filename via `--use`.
  Built-ins still ship in the binary; custom prompts are merged in at runtime.
- Prepend mode only (no append option).
- No custom separators; the concatenated text is the built-in prompt(s), two newlines, then the user prompt.

## Non-Goals (v1)
- No templating/variables.
- No custom formatting or headers.

## CLI (Kong)
- `--use <name>` (repeatable, alias `-u`): include one or more built‑in prompts. Values outside the built‑in set are an error.
- `--list-prompts` (bool): print available built‑in prompt names with a one‑line description; exit 0.
- `--show-prompt <name>`: print the full text of a built‑in prompt; exit 0.
- Help must show at least one `--use` example.

## Behavior
- Concatenate selected prompts in CLI order, then two newlines, then the user’s prompt.
- Unknown names → friendly error with suggestion to run `--list-prompts`; exit code 2.
- The first stdout line from `vibe` remains the workspace path (unchanged contract).

## Built‑ins (initial)
- `small-commits`
  - Description: Encourage incremental changes, tight loops, and frequent checkpoints.
  - Draft content (for reference):
    - Make changes in small, verifiable steps with clear rationale.
    - Prefer surgical diffs over broad refactors unless asked.
    - After each change: run tests and make a git commit with a short, imperative message (Conventional Commits style preferred). Example: `git add -A && git commit -m "feat: add pagination for transactions list"`.
    - Separate refactors from feature changes into distinct commits when possible.
    - If a larger change is required, propose a short plan and confirm before proceeding.
    - Keep messages concise; call out risks and roll-back points; aim for a PR-ready branch.

## Tests (stretchr/testify)
- Flag parsing for `--use` (repeatable) and interaction with existing `vibe` flags.
- Error on unknown prompt name; message suggests `--list-prompts`.
- `--list-prompts` includes `small-commits` with description.
- `--show-prompt small-commits` prints the expected content.
- Composition order and double‑newline join before the user prompt.

## Implementation Notes
- Store built-in prompts under `internal/prompts` using simple string constants (no `//go:embed`).
- Load custom prompts by scanning `~/.remuda/prompts` (configurable via `REMUDA_PROMPTS_DIR`). Files are matched by filename; content is copied verbatim.
- Provide a resolver that exposes `List()`, `Resolve(name)`, and `Compose(names, userPrompt)` over the combined set.
- Wire resolution into `vibe` before agent command construction; only the combined text is passed to the agent (no format changes elsewhere).
