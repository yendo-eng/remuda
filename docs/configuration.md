# Configuration

Remuda can be configured via environment variables and a YAML configuration file.

## Environment Variables

Remuda inspects a handful of environment variables (and associated helper files)
to reduce setup friction. Where possible it will fall back to sensible defaults
or populate values automatically, but you can configure them explicitly:

- `OPENAI_API_KEY` – forwarded to agents and container sessions. If unset,
  no API key is inferred; pass it explicitly with `--openai-api-key`.
- `ANTHROPIC_API_KEY` – forwarded for Claude container sessions (`--agent claude`)
  so the Claude CLI can authenticate inside Docker.
- `GH_TOKEN` / `GITHUB_TOKEN` – used for GitHub API access (eg. PR review,
  pushing branches, `gh` helpers). When neither variable is present Remuda runs
  `gh auth token` and exports the result so that container sessions inherit the
  credential. Pre-setting either variable skips that lookup.
- `REMUDA_AGENT` – overrides the default agent used by `vibe`/`vibe-check` when
  you omit `--agent`. Accepts the same values as the flag (`codex`,
  `opencode`, `claude`, `bash`).
- `REMUDA_MODEL` – default model used when you omit `--model`. Use
  `agent-default` to omit any model flag and let the agent CLI choose.
- `REMUDA_REASONING_LEVEL` – default reasoning level for supported agents.
  For codex this uses `none|minimal|low|medium|high|xhigh`; for claude the
  value is forwarded to Claude Code as `--effort`.
- `REMUDA_SLUGIFY_REASONING_LEVEL` – reasoning level used when Remuda auto-
  generates workspace names from prompts.
- `REMUDA_EXPERIMENTS` – comma- or whitespace-separated list of experimental
  features to enable.
- `REMUDA_JIRA_ENDPOINT` – Jira base URL used by `--jira` context fetches
  (eg. `https://your-domain.atlassian.net`).
- `REMUDA_JIRA_USER` – Jira user/email used by `--jira` context authentication.
- `REMUDA_JIRA_API_TOKEN` / `REMUDA_JIRA_TOKEN` – Jira API token used by
  `--jira` context authentication.
- `REMUDA_PROFILE` – apply a named profile from `config.yaml` for `vibe`,
  `vibe-check`, and `session resume`. `--profile` overrides this value, and
  this env var overrides `per_repo.<slug>.profile`.
- `REMUDA_EDITOR` – command invoked by `remuda session edit`. If unset, Remuda
  falls back to `VISUAL`, then `EDITOR`. The value is passed to your shell so
  you can include flags (eg. `export REMUDA_EDITOR='code --wait'`).
- `REMUDA_USE_PROMPTS` – comma-separated list of built-in prompt names to apply
  by default when running `vibe` or `vibe-check`. Equivalent to
  passing `--use` for each entry; use `--no-use` to exclude a default prompt for
  a single run.
- `REMUDA_CONTAINER` – set to `true` to run `vibe` sessions inside a Docker container
  by default. Equivalent to passing `--container`.
- `REMUDA_CONTAINER_OPTS` – comma-separated list of extra `docker run` arguments to
  append when container mode is enabled. Equivalent to repeating `--container-opt`.
- `REMUDA_CONTAINER_INHERIT_ENVS` – comma-separated list of environment variable names
  to forward into container sessions. Equivalent to repeating `--container-inherit-env`.
- `REMUDA_YOLO` – set to `true` to enable yolo mode (ignore sandboxing/approvals for
  supported agents) by default. Equivalent to passing `--yolo` (`codex` maps to
  `--dangerously-bypass-approvals-and-sandbox`, `claude` maps to
  `--dangerously-skip-permissions`).
- `REMUDA_SESSION_MANAGER` – select the session manager used for detached runs.
  Accepts `tmux` (default) or `zellij`. Equivalent to passing
  `--session-manager`.
- `REMUDA_DEFAULT_REPO` – default repository alias used when no repo is supplied
  via flags/args. Handy for teams whose primary workspace is not the default
  configured repository.
- `REMUDA_DEFAULT_REPO_URL` – default repository URL override used when no repo
  is supplied via flags/args (takes precedence over `REMUDA_DEFAULT_REPO`).
- `REMUDA_REPOS_BASE_DIR` – overrides the root where repositories are cached and
  worktrees are created. The `<org>/<repo>` substructure is preserved, e.g.
  `$HOME/vibing/acme-org/example-repo/<workspace>`.

## Jira Ticket Context (`--jira`)

When you pass `--jira <KEY>` to `remuda vibe`/`remuda vibe-check`, Remuda uses Jira Cloud REST APIs directly (it does not shell out to the `jira` binary).

Authentication and endpoint resolution follows this precedence:

1. CLI flags: `--jira-endpoint`, `--jira-user`, `--jira-token`
2. Environment: `REMUDA_JIRA_ENDPOINT`, `REMUDA_JIRA_USER`, `REMUDA_JIRA_API_TOKEN` / `REMUDA_JIRA_TOKEN`
3. Config file: `jira.endpoint`, `jira.user`, `jira.api_token`

Compatibility note:

- Remuda does not read keyring/pass-backed tokens from go-jira config. If you rely on keyring/pass with the `jira` CLI, set `REMUDA_JIRA_API_TOKEN` explicitly for Remuda.

## Configuration File

Remuda supports a versioned YAML configuration file for defaults and per-repo overrides.
Use `remuda config validate` to verify the resolved config file; it exits 0 and prints nothing when the config is valid (or missing).

### Location & Discovery (in order)

- `$REMUDA_CONFIG` (strict; if set, only this path is used)
- `$XDG_CONFIG_HOME/remuda/config.yaml` (usually `~/.config/remuda/config.yaml`)
- `~/.config/remuda/config.yaml` (fallback when `$XDG_CONFIG_HOME` is unset)
- `~/.remuda/config.yaml` (fallback location)

Notes:
- If `REMUDA_CONFIG` is set, config discovery is strict and normal runs require the file to exist and be readable.
- When persisting the default repo selection, Remuda will create the config at `REMUDA_CONFIG` if it is missing; unreadable paths still fail and surface as warnings.

### Precedence

CLI flags/args > environment > config file > built-in defaults

### Default repo persistence (FTUE)

When running `remuda clone` or `remuda vibe` on an interactive TTY with no repo
specified via flags/args/env/config, Remuda may prompt for a default repo.
Choosing "remember my choice" persists the selection to:
- `repos.default_repo` (alias), or
- `repos.default_repo_url` (explicit URL).

The prompt is skipped when `--wizard` is used, when `remuda vibe --in` targets an
existing workspace, when defaults are already set, or when running non-interactively.

Environment variables still override the saved defaults:
`REMUDA_DEFAULT_REPO_URL` / `REMUDA_DEFAULT_REPO` > config file.

### Schema (v1) Example

```yaml
version: 1

session:
  manager: tmux   # tmux|zellij

workspaces:
  ignore:
    - "acme-org/example-repo/archive-*"

defaults:
  agent: codex    # codex|opencode|claude|bash
  model: ""
  skip_version_check: false
  use_prompts: ["small-commits"]
  no_use: ["make-pr"]
  container:      # or: container: true|false (shorthand for container.enabled)
    enabled: false
    image: ghcr.io/acme/remuda-agent:latest
    opts: ["--network=host"]
    inherit_env: ["AWS_REGION"]

jira:
  endpoint: "https://your-domain.atlassian.net"
  user: "dev@example.com"
  api_token: "replace-with-token" # Optional when provided via env vars; may be omitted or set to "".

profiles:
  "fast_codex":
    agent: codex
    model: gpt-5
    yolo: true
    container:
      enabled: true

repos:
  base_dir: "~/.remuda/repos"
  aliases:
    example-repo: "https://github.com/acme-org/example-repo.git"
  default_repo: "example-repo"
  default_repo_url: ""

per_repo:
  "acme-org/example-repo":
    profile: "fast_codex"
    defaults:
      container:
        enabled: true
    clone_hooks:
      - name: "bootstrap"
        argv: ["./scripts/bootstrap", "--fast"]
      - name: "fetch-secrets"
        argv: ["bash", "-lc", "./scripts/secrets pull"]
```

Note: `container` can also be set to a boolean (`true`/`false`) as shorthand for `container.enabled`.

### Jira Context Auth (`--jira`)

When you pass `--jira <ISSUE-KEY>` to `vibe` or `vibe-check`,
Remuda resolves Jira auth with this precedence:

1. CLI flags: `--jira-endpoint`, `--jira-user`, `--jira-token`
2. Environment: `REMUDA_JIRA_*` vars (legacy `JIRA_*` aliases also supported)
3. Config file: `jira.endpoint`, `jira.user`, `jira.api_token`

The old `.jira.d/config.yml` discovery flow is no longer used.
`jira.api_token` can be omitted (or set to `""`) when token is provided by env or CLI.

### Profiles

Profiles are named bundles of defaults that you can apply with `--profile` or
the `REMUDA_PROFILE` environment variable, or select automatically per repo with
`per_repo.<slug>.profile`. For profile-capable commands (`vibe`, `vibe-check`,
`session resume`), profile selection precedence is:

1. `--profile`
2. `REMUDA_PROFILE`
3. `per_repo.<slug>.profile`
4. none

After selection, the chosen profile overrides top-level defaults and per-repo
defaults, while explicit command flags still win.

Allowed fields (same as `defaults`, plus `yolo`):

```yaml
profiles:
  <name>:
    agent: string
    model: string
    reasoning_level: string
    slugify_reasoning_level: string
    agent_cmd: string
    skip_version_check: bool
    use_prompts: [string]
    no_use: [string]
    experiments: [string]
    yolo: bool
    container:
      enabled: bool
      image: string
      opts: [string]
      inherit_env: [string]
```

Profile name rules:
- ASCII letters, digits, `_`, `-`, `/`
- No leading/trailing `/`, no `//`, no spaces

### Per-Repo Clone Hooks

You can define post-clone hooks per repo under `per_repo.<slug>.clone_hooks`.
Each hook uses argv semantics (no implicit shell), runs with working directory
set to the cloned workspace, and executes on the host.

Example:

```yaml
per_repo:
  "acme-org/example-repo":
    clone_hooks:
      - name: "bootstrap"
        argv: ["./scripts/bootstrap", "--fast"]
      - argv: ["bash", "-lc", "./scripts/secrets pull"]
```

Behavior:
- Built-in hooks run first, then config-defined hooks.
- Hooks run sequentially in config order.
- Hook failures abort the clone.
- `--no-clone-hooks` skips all post-clone hooks (built-in + config-defined).
- Hook processes receive these injected env vars:
  `REMUDA_REPO_URL`, `REMUDA_REPO_ORG`, `REMUDA_REPO_NAME`,
  `REMUDA_REPO_SLUG`, `REMUDA_CACHE_DIR`, `REMUDA_WORKTREE_DIR`.

### Session Resume Ignore Patterns

`session.prune.ignore` is a list of glob patterns matched against the relative
workspace path `org/repo/workspace` (forward slashes). Patterns use Go
`path.Match` rules, so `*` and `?` do not cross `/` boundaries.

Matching workspaces are excluded from `remuda session resume --pick` candidate
listing, so they are not offered as options to resume.

Example:

```yaml
session:
  prune:
    ignore:
      - "acme-org/example-repo/keep-*"
      - "*/utils/archive-*"
```

### Workspace List Ignore Patterns

`workspaces.ignore` is a list of glob patterns matched against the relative
workspace path `org/repo/workspace` (forward slashes). Patterns use Go
`path.Match` rules, so `*` and `?` do not cross `/` boundaries.

Matching workspaces are hidden from:
- `remuda workspaces list`
- `remuda workspaces list --active`
- `remuda workspaces list --inactive`

Example:

```yaml
workspaces:
  ignore:
    - "acme-org/example-repo/archive-*"
    - "*/utils/scratch-*"
```

### Notes

- `per_repo` overlays only apply when Remuda can resolve a repo slug (eg. from `--repo-url` or a PR URL). If slug inference fails, `per_repo` is ignored for that run.
- `per_repo.<slug>.profile` can pick a profile by repo for `vibe`, `vibe-check`, and `session resume`; explicit `--profile` and `REMUDA_PROFILE` override it.
- When `per_repo.defaults.container.opts` is set, its values are appended to `defaults.container.opts`. Use an explicit empty list (`opts: []`) to clear inherited container opts for a repo.
- `per_repo.<slug>.clone_hooks` validates that `argv` is non-empty and `argv[0]` is non-blank at parse time; runtime exec/exit errors still surface when hooks run.
- `session.prune.ignore` (used by `session resume --pick`) applies globally (not per-repo).
- `workspaces.ignore` applies globally (not per-repo).
- Unknown keys and invalid enum values (eg. `session.manager`, `defaults.agent`) are rejected with clear errors.

### Migration to XDG Path

Preferred location is XDG: `~/.config/remuda/config.yaml` (or `$XDG_CONFIG_HOME/remuda/config.yaml`).
`~/.remuda/config.yaml` is still read, but when both files exist the XDG file wins. To migrate:

```bash
mkdir -p ~/.config/remuda
if [ -f ~/.remuda/config.yaml ]; then
  mv ~/.remuda/config.yaml ~/.config/remuda/config.yaml
fi
```
