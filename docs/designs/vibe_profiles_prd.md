# Vibe Profiles - Design Doc (km-11f)

## 1. Purpose

Add reusable "profiles" to Remuda so users can select a named bundle of agent
configuration when running `vibe`, `vibe-check`, or `session resume`. Profiles
live in `config.yaml` and act like defaults that are overridden by explicit
flags.

## 2. Goals / Non-Goals

### Goals

- Allow users to define named profiles under the top-level `profiles` key in
  `config.yaml`.
- Select a profile with `--profile <name>` (and optionally via an env var).
- Apply profiles to `vibe`, `vibe-check`, and `session resume`.
- Profile values override config defaults and per-repo overlays.
- CLI flags and env vars override profile values.
- Validate profile names and provide clear errors for unknown profiles.

### Non-Goals (v1)

- No profile support for `vibe-fanout`.
- No commands for listing or showing profiles.
- No profile-specific prompts or custom logic beyond field defaults.

## 3. User Stories

- As a user, I can run `remuda vibe --profile opus --name feature/xyz` and
  automatically use my preferred agent/model defaults without retyping flags.
- As a reviewer, I can run `remuda vibe-check --profile opus <branch>` and get
  the same agent defaults used for code reviews.
- As a user, I can set `REMUDA_PROFILE=opus` and then override the agent for
  one run with `--agent codex`.

## 4. CLI Changes

### New flag (supported commands)

- `--profile <name>` (string)
  - `vibe`
  - `vibe-check`
  - `session resume`

No support in `vibe-fanout` (flag not defined). If the flag is used there,
Kong should return an unknown-flag error, which is acceptable for v1.

### Optional env var

- `REMUDA_PROFILE` (string)
  - Applies to the same commands that accept `--profile`.
  - If both env and flag are set, the flag wins.

## 5. Config Schema

### Example

```yaml
version: 1

profiles:
  opus:
    agent: opencode
    model: claude-opus-4.5
  fast_codex:
    agent: codex
    model: gpt-5
    use_prompts: ["small-commits"]
    yolo: true
    container:
      enabled: true
      image: vibe-dev
```

### Schema (additive)

```yaml
profiles:
  <name>:
    agent: string
    model: string
    agent_cmd: string
    skip_version_check: bool
    use_prompts: [string]
    experiments: [string]
    yolo: bool
    container:
      enabled: bool
      image: string
      opts: [string]
      inherit_env: [string]
```

Notes:
- The profile object mirrors `defaults` plus `yolo`.
- `experiments` is supported to keep parity with defaults, but remains
  a no-op unless wired elsewhere in the codebase.
- `profiles` is a map; keys are profile names.

## 6. Profile Name Rules

- Allowed characters: ASCII letters, digits, `_`, `-`, `/`.
- No spaces.
- No empty segments (no leading or trailing `/`, no `//`).

If a name violates these rules, parsing should fail with a clear error.

## 7. Precedence and Merge Order

Effective precedence (highest to lowest):

1. CLI flags/args
2. Environment variables (existing env tags)
3. Selected profile values
4. Per-repo overlay values
5. Config defaults
6. Built-in defaults

Rationale:
- Flags and env vars remain the strongest user intent.
- Profiles are an explicit, user-selected bundle and should take precedence
  over inferred defaults (including per-repo overlays).

### Slice fields

- For `use_prompts`, `container.opts`, and `container.inherit_env`, profile
  values replace any earlier values (no merge/append). This matches the current
  overlay behavior for defaults.

## 8. Command Behavior

### `remuda vibe`

- `--profile`/`REMUDA_PROFILE` set defaults for:
  - `--agent`, `--model`, `--agent-cmd`, `--skip-version-check`
  - `--use` (via config defaults + existing prompt resolver)
  - `--yolo`
  - container flags (`--container`, `--container-name`, `--container-opt`,
    `--container-inherit-env`)
- Explicit flags override profile values.

### `remuda vibe-check`

- Profiles set defaults for agent, model, agent-cmd, skip-version-check, use
  prompts, and container flags.
- `yolo` is ignored because `vibe-check` does not expose a yolo flag today.

### `remuda session resume`

- Profiles set defaults for `--yolo` and container flags.
- Agent-related fields are ignored (resume is Codex-only in v1).

## 9. Validation and Errors

- Unknown profile name: error with a message like
  `unknown profile "opus"; define it under profiles in config.yaml`.
- Profile name format violation: error during config parsing.
- Profile selected but config file missing: error (only for commands that accept
  `--profile` or when `REMUDA_PROFILE` is set).
- No profile selected: behavior unchanged.

## 10. Implementation Plan (High Level)

### 10.1 Config types (internal/configfile)

- Add `Profiles map[string]DefaultsV1` to `configfile.V1`.
- Extend `DefaultsV1` with `Yolo *bool` (additive field).
- Validate profile names during config parsing.
- Reuse existing validation for agent, container, prompts.

### 10.2 Early profile selection (cmd/remuda/cli)

- Add `--profile` flag to `VibeCmd`, `VibeCheckCmd`, `SessionResumeCmd`.
- Add optional env var `REMUDA_PROFILE` via kong tags on those flags.
- Before kong.Parse:
  - Determine selected profile name by scanning args for `--profile` or
    `--profile=<name>`.
  - Respect `--` terminator: ignore any tokens after `--`.
  - Only apply `REMUDA_PROFILE` for commands that actually define `--profile`
    (vibe, vibe-check, session resume).
  - Apply per-repo overlay (existing behavior).
  - Apply profile overlay, if selected, after per-repo overlay.
- If profile does not exist, return an error before parsing.

### 10.3 Kong resolver

- Reuse existing config resolver; after applying the profile overlay the
  resolver will naturally source defaults from the updated config.
- Add mapping for the `yolo` flag to `cfg.Defaults.Yolo`.

### 10.4 Use prompts behavior

- `use_prompts` from the profile should behave like config defaults.
- `--use` adds to the defaults; `--no-use` removes from the combined list.

## 11. Testing Strategy

Prefer e2e tests where possible. Add focused unit tests only where mocking would
otherwise be required.

### Suggested tests

- Parse tests:
  - `vibe --profile <name>` and `REMUDA_PROFILE` both work; flag overrides env.
  - Unknown profile yields a clear error.
  - Invalid profile name in config yields a parse error.
- Resolver tests:
  - Profile values override per-repo overlay and defaults.
  - `--yolo` defaults from profile apply to vibe and session resume.
- E2E (if feasible):
  - `remuda vibe --profile ...` launches with expected agent command.

## 12. Documentation Updates

- `docs/configuration.md`: add `profiles` section with schema and examples.
- `docs/commands.md`: mention `--profile` for `vibe`, `vibe-check`, and
  `session resume`.

## 13. Rollout and Compatibility

- Additive change; existing configs remain valid.
- If a user adds `profiles` with invalid names, config parsing will fail with a
  clear error. This is consistent with strict config validation behavior.
