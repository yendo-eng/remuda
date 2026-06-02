# Remuda Config File — Product Requirements (v1)

## 1) Purpose

Add a user-editable configuration file to provide defaults for common flags and
to support per-repo customization (eg. preferred agent/model, container mode).

This is intended to reduce repeated flag/env configuration while preserving
explicit CLI control.

## 2) Goals / Non-Goals

### Goals

- Support a single config file with predictable precedence relative to env vars
  and flags.
- Use the config strictly for **defaults** (same mental model as env vars): CLI
  flags always win.
- Allow per-repo overrides keyed by `owner/repo` (derived from `--repo-url` or
  inferred from PR URLs when possible).
- Keep the schema minimal and aligned with existing flags/env vars.

### Non-Goals (v1)

- Encrypted secrets management. API keys remain env/file-driven (as today).
- Remote config fetching or profiles.
- General-purpose automation frameworks beyond scoped post-clone hooks.

## 3) File Location & Discovery

### Default search paths (in order)
1. `$REMUDA_CONFIG` (explicit override; if set, only this path is used)
2. `$XDG_CONFIG_HOME/remuda/config.yaml` (usually `~/.config/remuda/config.yaml`)
3. `~/.config/remuda/config.yaml` (fallback when `$XDG_CONFIG_HOME` is unset)
4. `~/.remuda/config.yaml` (legacy; supported for backwards compatibility)

If no file is found, Remuda behaves exactly as it does today.

If multiple files are present, Remuda prefers the XDG path(s) over the legacy
`~/.remuda` path to encourage migration.

## 4) Precedence (merging rules)

Effective value resolution order for a given field:

1. CLI flags / args (highest priority)
2. Environment variables (existing `env:"..."` tags)
3. Config file (`config.yaml`)
4. Built-in defaults in code (lowest priority)

For nested objects in the config file, merging is **field-wise**:
unspecified fields do not overwrite defaults.

## 5) Format

### Encoding

- YAML (`config.yaml`)
- Versioned schema via a required `version` field.

### Proposed schema (v1)

```yaml
version: 1

repos:
  # Mirrors REMUDA_REPOS_BASE_DIR (preferred) and VIBE_REPOS_BASE_DIR (legacy).
  base_dir: "~/.remuda/repos"

  # Mirrors REMUDA_DEFAULT_REPO / REMUDA_DEFAULT_REPO_URL.
  default_repo: "myrepo"
  default_repo_url: ""

  # Optional user-defined aliases.
  aliases:
    myrepo: "https://github.com/acme/myrepo.git"

session:
  # Mirrors REMUDA_SESSION_MANAGER.
  manager: "tmux" # tmux|zellij

defaults:
  # Mirrors REMUDA_AGENT and the common agent flags.
  agent: "codex" # codex|opencode|bash
  # When empty, Remuda may supply a per-agent default model for determinism.
  # Use "agent-default" to omit any model flag and let the agent CLI choose.
  model: ""
  agent_cmd: ""
  skip_version_check: false

  # Mirrors REMUDA_USE_PROMPTS.
  use_prompts: ["small-commits"]

  # Mirrors REMUDA_EXPERIMENTS (comma/whitespace separated via env var).
  # Reserved for future experiments; currently unused.
  experiments: []

  # Mirrors REMUDA_CONTAINER / REMUDA_CONTAINER_OPTS and container flags.
  container:
    enabled: false
    image: "vibe-dev"
    opts: ["--network=host"]
    inherit_env: ["AWS_REGION", "FOO_BAR"]

per_repo:
  # Keys are GitHub slugs: "owner/repo" (as displayed by Remuda).
  "acme-org/example-repo":
    defaults:
      agent: "codex"
      model: "gpt-5"
      container:
        enabled: true
    # Optional post-clone hooks run on host after clone.
    clone_hooks:
      - name: "bootstrap"
        argv: ["./scripts/bootstrap", "--fast"]
      - argv: ["bash", "-lc", "./scripts/secrets pull"]
```

Notes:
- `defaults` are shared across commands that embed `AgentSessionOptions`,
  `ContextEngineeringOptions`, and `VibeContainerOptions`.
- `per_repo` overlays only apply when Remuda can resolve a repo slug. If slug
  inference fails, Remuda ignores `per_repo` for that invocation.
- `per_repo.<slug>.clone_hooks` run after built-ins, in declared order, and are
  skipped when `--no-clone-hooks` is set.

## 6) Validation & Errors

- Unknown top-level keys should error with a concise message pointing at the
  offending key path.
- Enum-like fields (`session.manager`, `defaults.agent`) should validate against
  the same sets as flags.
- If `$REMUDA_CONFIG` is set but the file is unreadable or invalid, Remuda
  should exit non-zero (user explicitly requested that file).
- If no config file is found in the default search paths, it is not an error.

## 7) Implementation Notes (Kong)

Kong supports configuration-backed defaults via `kong.Configuration(loader,
paths...)`. YAML support is provided by `github.com/alecthomas/kong-yaml`.

We should wire the parser roughly like:

- Build `parser := kong.Must(&cli, kong.Name("remuda"), kong.UsageOnError(), ...)`
- Add a YAML configuration resolver using the discovered path(s)
- Keep existing `env:"..."` tags as-is so env vars continue to override config

This keeps the config file strictly in the “defaults” tier of precedence.

## 8) Decisions (v1)

- Standardize on YAML only for the config file format.
- `repos.aliases` defines user-managed alias mappings in config.
- Treat `VIBE_REPOS_BASE_DIR` as a legacy alias for `REMUDA_REPOS_BASE_DIR`
  (prefer `REMUDA_REPOS_BASE_DIR` when both are set).
