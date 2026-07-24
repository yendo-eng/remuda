# Commands

This document covers the main commands provided by Remuda.

## Commands Overview

- `remuda clone --name <name> [repo_url]` (or `--wizard`): fast workspace creation from a local cache using git worktrees. Folder and branch are `<name>`; errors on collision unless `--force` is used.
- `remuda vibe [--name <name>] [<prompt>]` (or `--wizard`): start an AI coding session in a new workspace (detached session via tmux by default). Omit the prompt to start the agent in its default interactive mode. If `--name` is omitted, Remuda derives one from the prompt. Session name is `org/repo/<name>`.
- `remuda vibe-check [--name <name>] <branch>` (or `--wizard`): review a branch and produce a Markdown report in the session output.
  - Defaults `--name` to `<branch>-code-review`.
  - For PR reviews, pass `--pr <pr-ref>` (requires `gh auth status` to be authenticated).
  - Outputs: `.vibe/check/pr.json` and `.vibe/check/diff.patch` in the workspace; the Markdown report is emitted in the session output (not saved by default).
  - Note: does not forward `--yolo`. The branch (or PR head branch) is checked out in the workspace before review.
- `remuda workspaces list [--active|--inactive]`: list Remuda-managed workspaces on disk, one absolute path per line.
- `remuda workspaces remove [--dry-run] [--force] <target>...`: remove one or more explicit workspaces by absolute path or `org/repo/workspace` identifier.
- `remuda config validate`: validate the resolved config file (missing config is treated as success).
- `remuda session <subcommand>`: manage running sessions created by Remuda. Subcommands: `list`, `attach`, `readbuf`, `send`, `path`, `kill`, `inactive`, `resume`, `reap`, `shell`, `edit`. See [Session Management](session-management.md).
  - `session resume` supports the same post-clone launch flags as `vibe`: `--agent`, `--model`, `--reasoning-level`, `--agent-cmd`, `--use`, `--no-use`, `--jira`, `--github-issue` (alias: `--gh-issue`), `--openai-api-key`, `--profile`, `--yolo`, plus detached/attach/container flags and an optional trailing prompt arg.
  - `session resume` is stateless: Remuda does not detect which agent created the original session. Use the same agent that created the workspace session history.
  - `session resume` defaults to Codex unless config/env defaults resolve to Claude via existing resume-default logic.
  - Clone-creation flags (`--repo`, `--repo-url`, `--full-clone`, `--no-clone-hooks`, wizard repo-selection) are intentionally not supported on `session resume`.
  - `--experiments session-manifest` – `vibe` writes a `.remuda.json` launch manifest (agent,
    model, reasoning level, yolo, container settings, use-prompts) into the workspace root, local
    and untracked. With the same experiment enabled, `session resume` reads it back to default any
    of the flags above that weren't passed explicitly, instead of falling back to Codex.

---

## Workspaces

Manage Remuda-managed workspaces under `repos.base_dir`.

```bash
# List all Remuda-managed workspaces (active + inactive)
remuda workspaces list

# Restrict output to active workspaces only
remuda workspaces list --active

# Restrict output to inactive workspaces only
remuda workspaces list --inactive

# Dry-run removal without deleting
remuda workspaces remove --dry-run ~/.remuda/repos/acme-org/example-repo/feature-login-audit

# Force-remove a dirty/desynced linked worktree
remuda workspaces remove --force acme-org/example-repo/feature-login-audit

# Remove by identifier
remuda workspaces remove acme-org/example-repo/feature-login-audit

# Remove multiple explicit targets
remuda workspaces remove \
  acme-org/example-repo/feature-login-audit \
  acme-org/example-repo/feature-cache-index
```

Behavior:

- Output is one absolute workspace path per line.
- Discovery uses the same candidate rules as `session inactive`.
- `.repo_cache` is never listed.
- `workspaces.ignore` config patterns are applied.
- `--active` and `--inactive` cannot be combined.
- Remove targets must be absolute paths or `org/repo/workspace` identifiers.
- Remove refuses active-session workspaces and special directories such as `.repo_cache`.
- Linked worktrees with untracked/desynced state are refused unless `--force` is set.

---

## Clone

Create a workspace without launching an agent. Useful for worktree management.

```bash
# Clone the configured default repo into folder+branch feature-auth-hardening
remuda clone --name feature-auth-hardening

# ➜ ~/.remuda/repos/acme-org/example-repo/feature-auth-hardening
```

It is recommended to set `REMUDA_REPOS_BASE_DIR` so that no matter where you run
remuda, the worktrees show up in a consistent place in your filesystem.

Options:

- `--name <name>` – required unless you use `--wizard`; used for folder and branch.
- `--branch <name>` – checkout this git branch (workspace folder still derives from `--name`). Defaults to the name when omitted.
- `--repo <alias>` – shorthand for a configured repository alias. Alias values
  come from merged repo-alias configuration (defaults plus `repos.aliases` in
  config). Use `remuda repo list` to see aliases available in your environment.
- `--force` – replace the existing workspace if it already exists.
- `--full-clone` – copy the cache into the workspace instead of creating a linked worktree.
- `--experiments cow-clone` – populate a `--full-clone` workspace with copy-on-write clones of
  the cache, so the workspace shares blocks with it instead of duplicating them. Requires APFS,
  btrfs, XFS with `reflink=1` or bcachefs; other filesystems (ext4, overlayfs) fall back to a
  plain copy.
- `--no-clone-hooks` – skip running all post-clone hooks (built-in and config-defined).
- `[repo_url]` – clone a different repository instead of the configured default URL.

First-run repo prompt:
- If no repo is specified via flags/args/env/config and you're on an interactive TTY, `remuda clone`
  will prompt you to pick a default repo and optionally remember it (persisting to
  `repos.default_repo` or `repos.default_repo_url`).
- The prompt is skipped when `--wizard` is used, when defaults are already set, or when running
  non-interactively.

---

## Vibe

Launch an AI agent in a new workspace. This is the most common workflow.

```bash
remuda vibe --name feature/pagination "Implement pagination for transactions"

# Creates a new workspace and starts a detached session running Codex.
```

```bash
remuda vibe --name feature/pagination

# Creates a new workspace and starts the agent without an initial prompt
# (agent CLI decides its default interactive behavior).
```

If `--name` is omitted, Remuda derives a workspace name from the prompt (and
falls back to `session` if slug generation fails).

Run `remuda vibe --help` for more options. Common flags:

- `--agent codex|opencode|claude|bash` – pick between built-in agents (default: `codex`).
- `--profile <name>` – apply a named profile from `config.yaml` as defaults (or set `REMUDA_PROFILE`). When neither is set, `per_repo.<slug>.profile` may select one based on the resolved repo.
- `--model <m>` – pass a model override to the chosen agent when supported.
- `--reasoning-level <level>` – set agent reasoning effort. For Codex this maps to
  `--config model_reasoning_effort=<level>`. Completion suggests
  `none|minimal|low|medium|high|xhigh` for older models and also `max|ultra` for
  `gpt-5.6*`; unknown values are passed through with a warning for Codex to
  validate. For Claude this maps to `--effort <level>`.
- `--yolo` – enable agent-specific approval/sandbox bypass behavior. Codex maps
  this to `--dangerously-bypass-approvals-and-sandbox` plus
  `--dangerously-bypass-hook-trust`; Claude maps this to
  `--dangerously-skip-permissions`.
- `--remote` – request agent remote control when supported. For Claude this maps
  to `--remote-control <session-name>`, where Remuda uses the run's session
  name (`org/repo/workspace`) as the remote-control name. For non-Claude
  agents, Remuda logs a warning and continues without remote control.
- `--agent-arg <arg>` – append an extra argument to the selected built-in agent
  command (repeatable). Config defaults can be set per agent with
  `defaults.agent_args.<agent>` (and per-repo overlays); CLI `--agent-arg`
  values append after resolved config defaults. Ignored when `--agent-cmd` is set.
- `--agent-cmd <cmd>` – provide a complete custom command; your prompt will be
  appended as the final argument.
- `--use <prompt-name>` – apply a saved prompt (repeatable); `--no-use` excludes
  selected prompts.
- `--use-position before|after` – place saved prompts before or after the main
  prompt (default: `before`). Jira, Slack, and GitHub issue context remains
  before the main prompt in either mode.
- `--jira <ID>` – prepend Jira issue context (repeatable; format `ABC-123`).
- `--jira-endpoint <url>` – override Jira base URL for `--jira` context fetches.
- `--jira-user <email>` – override Jira user/email for `--jira` context fetches.
- `--jira-token <token>` – override Jira API token for `--jira` context fetches
  (prefer env/config in normal usage).
- `--in <path>` – launch inside an existing workspace instead of cloning a new one.
- `--repo <alias>` – shorthand for a configured repo alias; expands to a full
  URL. Alias values come from merged alias configuration (defaults plus
  `repos.aliases` in config). Use `remuda repo list` to print the current
  catalog with aliases and URLs.
- `--repo-url <url>` – explicit git URL override (takes precedence over `--repo`). Also accepts `github.com/<org>/<repo>` shorthand and expands it to `https://github.com/<org>/<repo>.git`.
- `--branch <name>` – checkout this git branch (workspace folder still derives from `--name`). Defaults to the name when omitted.
- `--force` – replace the existing workspace if it already exists.
- `--full-clone` – copy the cache into the workspace instead of creating a linked worktree.
- `--experiments cow-clone` – copy the cache copy-on-write; see the [Clone](#clone) options.
- `--no-clone-hooks` – skip running all post-clone hooks (built-in and config-defined).
- `--[no-]detached` – run the agent in the current terminal instead of using the configured session manager.
- `--session-manager tmux|zellij` – override the session manager for this invocation (tmux remains the default).
- `REMUDA_SESSION_MANAGER=tmux|zellij` – environment override for the default session manager (falls back to `tmux`).
- Containerized Claude runs forward `ANTHROPIC_API_KEY` and mount `~/.claude`
  plus `~/.claude.json` when present so Claude auth/session state is reused.
- Containerized Codex runs persist ChatGPT account logins (not API-key auth):
  when `OPENAI_API_KEY` is unset and `~/.codex/auth.json` exists, the whole
  `~/.codex` directory is mounted read-write so the account login (and its
  token refreshes) carry over across sessions. When `OPENAI_API_KEY` is set,
  a synthesized read-only `auth.json` is used instead and no account auth is
  mounted.

First-run repo prompt:
- If no repo is specified via flags/args/env/config and you're on an interactive TTY, `remuda vibe`
  will prompt you to pick a default repo and optionally remember it (persisting to
  `repos.default_repo` or `repos.default_repo_url`).
- The prompt is skipped when `--wizard` or `--in` is used, when defaults are already set, or when
  running non-interactively.

### Prompts

Remuda has a variety of saved prompts (built-in and custom) to influence the
behavior of the agent. To use a prompt, add `--use <prompt-name>` (repeatable).
To exclude a prompt for a single run, pass `--no-use <prompt-name>` (repeatable);
exclusions win over inclusions and apply to defaults from `REMUDA_USE_PROMPTS`.
These prompt flags are shared by `vibe`, `vibe-check`, and `session resume`.

Notes:
- `--no-use` accepts comma-separated prompt names and can be repeated.
- `--use-position after` places only the saved prompts after the main prompt;
  fetched Jira, Slack, and GitHub issue context stays before it.
- Exclusions are applied after combining `--use` with defaults from `REMUDA_USE_PROMPTS`.
- Config-based exclusions are available via `defaults.no_use`, `per_repo.<repo>.defaults.no_use`, and `profiles.<name>.no_use`.
- Unknown prompt names in `--no-use` are errors, matching `--use`.
- `--no-use` does not require a prompt, so it can be used when starting an empty prompt run.
- Shell completion for `--no-use` only sees tokens left of the cursor. Put `--use` before `--no-use` when you want those values to influence `--no-use` suggestions.

Example:

```bash
# Start a session with a small-commits coaching preface.
# The agent will build the feature incrementally, making git commits along the way.
remuda vibe --use small-commits --name feature/pagination "Implement pagination for transactions"

# Put saved coaching instructions after the task while keeping fetched context first.
remuda vibe --use small-commits --use-position after --name feature/pagination "Implement pagination for transactions"

# You can set your favorite prompts (built-in or custom) as defaults with
# REMUDA_USE_PROMPTS. make-pr will instruct the agent to try and open a PR with
# the gh cli when it is done with its work.
export REMUDA_USE_PROMPTS 'make-pr,small-commits'
remuda vibe --name feature/pagination "Implement pagination for transactions"

# Exclude a default prompt for a single run.
remuda vibe --no-use make-pr --name feature/pagination "Implement pagination for transactions"

# List all available prompts (built-in + custom). If a custom prompt has the
# same name as a built-in, it overrides the built-in and is marked as such.
remuda prompts list

# Dump the resolved prompt content (what --use will actually apply)
remuda prompts show small-commits

# Bring your own prompts by dropping text files into `~/.remuda/prompts`.
# If a custom prompt name collides with a built-in prompt, the custom prompt wins.
mkdir -p ~/.remuda/prompts
cat <<'EOF' > ~/.remuda/prompts/tell-jokes
Please end every task by telling a joke related to the work you just did.
EOF

remuda vibe --use tell-jokes --name feature/pagination "Implement pagination for transactions"

# Point to an alternate directory with REMUDA_PROMPTS_DIR when you want to
# sync prompts via git or keep them alongside a repo.
export REMUDA_PROMPTS_DIR=$PWD/prompts
```

---

## Vibe-Check

Review a branch or Pull Request and produce a code review report.

Use `vibe-check` to produce a code review report for either:

- a branch (default): compares the branch against the repo default branch (`origin/HEAD`)
- a PR (`--pr`): fetches PR metadata + diff via the GitHub CLI (`gh`)

Requirements:

- For PR reviews: GitHub CLI (`gh`) installed and authenticated (`gh auth status`).

Notes:

- Artifacts are written under `.vibe/check/` (pr.json + diff.patch); the Markdown report is emitted in the session output.
- `vibe-check` defaults to a full clone (`--full-clone`); use `--no-full-clone` to review via linked worktrees.
- `--experiments cow-clone` copies the cache copy-on-write; see the [Clone](#clone) options.
- `--no-clone-hooks` is supported and skips all post-clone hooks (built-in + config-defined).

Examples:

```bash
# Wizard mode if you want to find the PR interactively
remuda vibe-check --wizard

# Minimal: review a branch
remuda vibe-check feature/checkout-logging

# PR by URL
remuda vibe-check --name feature-login-audit --pr https://github.com/org/repo/pull/123

# PR by number with explicit repo URL and inline execution
remuda vibe-check --pr 123 \
  --repo-url https://github.com/acme-org/example-repo \
  --no-detached

# Choose a model (when supported by the selected agent)
remuda vibe-check --pr https://github.com/org/repo/pull/123 --model gpt-4o-mini
```

Many `vibe` session flags still apply (including `--agent-arg`). For detailed flags, run
`remuda vibe-check --help`.

Profile defaults are supported here as well: `--profile <name>` (or
`REMUDA_PROFILE`) applies a named profile from `config.yaml` for agent/container
defaults. If neither is set, `per_repo.<slug>.profile` can auto-select one for
the resolved repo.

---

## Config

Validate the resolved config file using the same discovery rules as other
commands. Missing config is treated as success and produces no output.

```bash
remuda config validate
```
