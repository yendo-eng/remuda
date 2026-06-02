# `km-uas` — Default Repo FTUE (Design Doc, Small)

## 1. Purpose

Reduce first-run friction by prompting for a default repository on the first
`remuda clone` or `remuda vibe` invocation when no repo is specified anywhere
(flags/args, env, or config). Optionally persist the choice if the user asks to
remember it.

## 2. Background / Problem

Historically, `remuda clone` and `remuda vibe` assumed a default repo when none
was supplied. For first-time users on a machine, this was often wrong and
required re-running the command with `--repo` or `--repo-url`.

We want an FTUE-style prompt the first time a user runs `clone`/`vibe` without
explicit repo selection, and allow saving that choice as the default.

## 3. Goals / Non-Goals

### Goals
- Prompt only when the repo is unspecified in flags/args/env/config.
- Only prompt when an interactive TTY is available.
- Allow the user to optionally persist the choice to the config file.
- Keep existing command semantics and defaults for non-interactive usage.

### Non-Goals
- No changes to `vibe-check`, `vibe-fanout`, or other commands.
- No changes to repo aliasing behavior or catalog management.
- No persistence if the user declines to remember their choice.

## 4. Proposed UX

Trigger (both `clone` and `vibe`):
- Repo selection is missing from:
  - CLI flags / args (`--repo`, `--repo-url`, positional `repo_url`)
  - environment variables (`REMUDA_DEFAULT_REPO`, `REMUDA_DEFAULT_REPO_URL`)
  - config file (`repos.default_repo`, `repos.default_repo_url`)
- A TTY is available.

Flow:
1. Prompt: "Repository"
   - Options: canonical alias list + user-defined aliases from config +
     "Custom URL"
   - If custom URL, request input for URL (validated)
2. Prompt: "Remember my choice?" (yes/no)
3. Continue execution using selected repo.
4. If remember = yes, persist as default.

Wizard interaction:
- If `--wizard` is used, do not run this FTUE prompt (wizard already selects
  the repo).

Non-interactive behavior:
- If no TTY is available, do not prompt.
- Follow normal repo resolution behavior; when repo selection is still unset,
  return the existing "repository is not configured" error.

## 5. Decision Rules (Repo Selection)

Priority order for the selected repo:
1. Explicit CLI flags/args (`--repo-url`, positional `repo_url`, `--repo`)
2. Env vars: `REMUDA_DEFAULT_REPO_URL`, then `REMUDA_DEFAULT_REPO`
3. Config file: `repos.default_repo_url`, then `repos.default_repo`
4. FTUE prompt (TTY-only, first invocation semantics)

"First invocation" is interpreted as: no defaults are present in flags/args,
env, or config, at the moment the command is run.

If the user declines "Remember my choice", they will be prompted again on
subsequent runs that still meet the trigger conditions.

## 6. Persistence (Config Update)

When the user selects "Remember my choice":
- If alias selected:
  - Set `repos.default_repo` to the alias.
  - Clear `repos.default_repo_url` (to avoid ambiguity).
- If custom URL selected:
  - Set `repos.default_repo_url` to the URL.
  - Clear `repos.default_repo`.

Config file location should follow existing discovery rules:
- If `$REMUDA_CONFIG` is set, write to that path.
- Otherwise write to the primary XDG path (`$XDG_CONFIG_HOME/remuda/config.yaml`)
  or `~/.config/remuda/config.yaml` when XDG is unset.
- Legacy `~/.remuda/config.yaml` should not be created unless it is the only
  discovered file.

If a config file exists, merge with existing settings (preserve all other keys).
If none exists, create a minimal `version: 1` file.
Ensure the parent directory exists (create if missing).

If persistence fails (e.g. unreadable `$REMUDA_CONFIG`, unwritable path), warn
and continue executing the command using the chosen repo without saving.

## 7. Implementation Notes (non-binding)

Where:
- `cmd/remuda/cli/clone.go` and `cmd/remuda/cli/vibe.go` should insert the
  FTUE prompt before calling `github.RepoOrURL`.
- Reuse existing repo selection UI (eg. `wizardSelectRepo`), but without the
  full wizard flow.

TTY handling:
- Use `ctx.Remuda.IO.IsTerminal()` as the guard for interactive prompts.

Repo detection:
- Detect if `--repo` or `--repo-url` is explicitly set vs just a default.
- Ensure no struct-tag default masks the "unspecified" state needed by FTUE.
- Non-FTUE commands that do not resolve a repo should keep the existing
  "repository is not configured" error behavior.

Config write:
- Add a small helper to load/merge/write `config.yaml` (likely in
  `internal/configfile` or `cmd/remuda/cli`).
- Writing should be atomic (write temp, then rename).
- Validation should reuse `configfile.ParseV1` after writing if feasible.

Validation:
- Custom URL must be non-empty and parseable as a Git remote (https/ssh),
  matching what `github.RepoOrURL` accepts (eg. https://github.com/org/repo.git,
  git@github.com:org/repo.git, ssh://git@github.com/org/repo.git).
- Invalid input should re-prompt in interactive mode; if prompting is not
  possible, abort with a clear error message.

## 8. Tests (e2e preferred)

Scenarios:
- TTY available + no repo default anywhere -> prompt; choose alias; no remember.
- TTY available + no repo default anywhere -> prompt; choose URL; remember -> config updated.
- Non-interactive (no TTY) + no defaults -> no prompt; command returns repository-not-configured error.
- Env/config default present -> no prompt.
- Explicit `--repo` or `--repo-url` -> no prompt.

Use stretchr/testify for assertions and favor e2e harness coverage.

## 9. Decisions

- Only `clone` and `vibe` participate.
- Prompt runs only when a TTY is available.
- Persistence is opt-in via "Remember my choice".
- Config writes follow existing discovery precedence.
- FTUE prompt is skipped when `--wizard` is used.
- No sentinel is stored; declining "Remember my choice" means repeated prompts
  on future qualifying runs.
