# `--no-use` prompt exclusion PRD

## Purpose
Allow users to selectively disable specific saved prompts that would otherwise be applied via defaults or explicit `--use`, without changing their global configuration.

## Background
Remuda supports default prompt prefaces via `REMUDA_USE_PROMPTS` and config `defaults.use_prompts`, plus explicit `--use` flags. Today there is no easy way to disable a single prompt (e.g., `make-pr`) for a one-off run.

## Goals
- Provide a CLI-only opt-out mechanism that removes specific prompts by name.
- Keep parsing validation consistent with `--use` (unknown names error early).
- Ensure precedence is intuitive: exclusions win over inclusions.
- Make the feature available on any command that already supports `--use`.

## Non-goals
- A global env/config toggle for exclusions.
- A blanket “disable all prompts” switch (for now).
- New prompt templating or ordering semantics beyond current behavior.

## User experience

### CLI
- New flag: `--no-use <name>[,<name>...]` (repeatable; accepts comma-separated values).
- Available wherever `--use` is available today (e.g., `vibe`, `vibe-check`, `vibe-fanout`, wizard flows).

### Examples
```bash
# Defaults include make-pr and small-commits. Remove make-pr for this run.
REMUDA_USE_PROMPTS=make-pr,small-commits \
  remuda vibe --no-use make-pr --name spike "Investigate X"

# Explicit + default, but remove one that appears in either.
remuda vibe --use small-commits --use make-pr \
  --no-use make-pr --name spike "Investigate X"

# Comma-separated values
remuda vibe --no-use make-pr,small-commits --name spike "Investigate X"
```

## Behavior
- `--no-use` removes matching prompt names from the effective `use` list.
- Precedence: exclusions win. Example: `--use a,b --no-use b` results in `a` only.
- Exclusions apply to all sources of `use` prompts (CLI, env, config).
- If an excluded prompt is not in the effective `use` list, it is ignored.
- Unknown prompt names in `--no-use` are errors (same behavior as `--use`).

## Validation and errors
- `--no-use` requires at least one prompt name when present.
- Unknown prompt name: error at parse time (reusing the existing prompt resolver).
- No warnings for unmatched exclusions.
- If the user prompt is empty, `--no-use` should not trigger a prompt-required error; the exclusion is effectively ignored in that run.

## Tests (stretchr/testify)
- Parsing: `--no-use` accepts valid prompt names and rejects unknown ones.
- Precedence: explicit `--use` plus `--no-use` removes the excluded prompt.
- Defaults: env/config `use` combined with `--no-use` filters correctly.
- No-prompt flow: `--no-use` does not force a non-empty prompt in `vibe`/`vibe-fanout`.
- Multi-command coverage: `vibe`, `vibe-check`, and `vibe-fanout` apply exclusions consistently.

## Implementation notes
- Extend `ContextEngineeringOptions` with `NoUse []PromptName` and a new `--no-use` flag.
- Filter the resolved `Use` list inside `AddedPromptContext` to ensure all commands get the same behavior.
- Keep validation close to parse time by reusing `PromptName.UnmarshalText` (same as `--use`).
- Update command help text and shell completions to include `--no-use`.
- No config or env additions.
