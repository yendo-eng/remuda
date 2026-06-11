# `remuda vibe --agent claude` - Product Requirements

## 1. Purpose

Add official Anthropic Claude Code CLI support as a first-class Remuda agent (`--agent claude`) for `vibe`, `vibe-check`, and related defaults/config flows.

The goal is feature parity with existing built-in agents where practical, including container-mode operation and propagation of host OAuth session state into the container.

## 2. Goals and Non-Goals

### Goals

- Support `claude` as a built-in agent in all current agent selection surfaces.
- Pass `--model` through directly to Claude Code with no restrictive model whitelist.
- Map Remuda `--yolo` to Claude Code's `--dangerously-skip-permissions` flag.
- Map Remuda `--reasoning-level` to Claude Code's `--effort` flag.
- Support container mode for Claude Code, including:
  - installing Claude Code in the `vibe-dev` image,
  - mounting host Claude auth/session state into the container,
  - preserving OAuth-based login across host and container runs.
- Keep implementation consistent with existing Remuda architecture:
  - CLI parsing in `cmd/remuda/cli`,
  - launcher/business logic in `internal/agentlauncher`,
  - container helpers in `internal/docker`.

### Non-Goals (v1)

- Dedicated `remuda session resume` semantics tuned specifically for Claude Code.
- Enumerating/synchronizing all available Claude model IDs.
- Managing Claude login flows from Remuda UX (users can run `claude auth` directly).

## 3. User Experience

```bash
# basic
remuda vibe --agent claude "Implement pagination"

# model passthrough
remuda vibe --agent claude --model claude-opus-4-1 "Refactor auth"

# yolo mapping
remuda vibe --agent claude --yolo "Update deps and fix build"

# reasoning-level mapping
remuda vibe --agent claude --reasoning-level high "Plan schema migration"

# container mode with OAuth state propagation
remuda vibe --agent claude --container "Add tests"
```

Expected behavior:
- `--agent claude` launches the Claude Code CLI.
- If `--model` is provided, Remuda appends it to the launch command as-is.
- If `--model` is omitted, Remuda should omit any model flag and let Claude choose defaults.
- `--yolo` maps to Claude Code's `--dangerously-skip-permissions` mode.
- `--reasoning-level` maps to Claude Code's `--effort <level>`.
- In container mode, existing host login/session state remains usable in-container.

## 4. Functional Requirements

### FR-1: Agent registration across CLI/config/runtime

Add `claude` to all canonical agent registries and validation points:
- `internal/enums/enums.go` (`ValidAgents`),
- `internal/agentlauncher/agent_launcher.go` (`SupportedAgent`, `SupportedAgents()`, parser switch),
- `cmd/remuda/cli/shared.go` agent enum tag/help text,
- any config parsing/validation paths that rely on canonical agent lists.

Constraints:
- Keep `enum` values and `enums.ValidAgents` synchronized.
- Preserve backward compatibility for existing agents.

### FR-2: Claude launcher implementation

Add `internal/agentlauncher/claude.go` implementing `AgentLauncher`:
- `Name() string` returns `"claude"`.
- `Command(prompt string)` builds the Claude launch command.
- `Version()` detects installed version via `claude --version`.
- `SupportedModels()` returns `nil` to avoid false warnings from partial model lists.

Command composition (v1):
- Base binary: `claude`.
- Prompt behavior should match existing launcher semantics:
  - include prompt argument only when non-empty,
  - escape shell quoting correctly.
- Model behavior:
  - if model is non-empty and not `agent-default`, append `--model <value>`.
- Yolo behavior:
  - when enabled, append `--dangerously-skip-permissions`.

### FR-3: Model passthrough behavior

Remuda should treat Claude model values as opaque passthrough strings:
- no required format validation,
- no whitelist validation,
- rely on Claude CLI for final validation/errors.

Completion UX:
- maintain context-aware model completion behavior architecture,
- for Claude specifically, completion may be empty/unbounded in v1 (acceptable).

### FR-4: Reasoning-level behavior via `--effort`

Remuda should map Claude reasoning-level directly to Claude Code's effort flag:
- if `--reasoning-level <level>` is provided, append `--effort <level>` to the Claude command,
- if no reasoning level is provided, omit `--effort`,
- rely on Claude CLI for final value validation/errors in v1 (no extra model-coupled validation in Remuda).

### FR-5: Container image support

Ensure the container image used by `--container` includes Claude Code CLI.

Requirements:
- installation must work in the team's chosen image build workflow,
- resulting image should allow `claude --version` successfully,
- keep build reproducibility conventions in that external image pipeline.

### FR-6: Container OAuth/session propagation

Add Claude-specific container mounts/env handling so host authentication and state carry into container sessions.

State propagation requirements:
- Mount host `~/.claude/` into container `/root/.claude/` (rw), when available.
- Mount host `~/.claude.json` into container `/root/.claude.json` (rw), when available.
- Prefer resilient behavior:
  - if files/dirs are missing, continue without hard failure,
  - log warning/debug context when mounts cannot be prepared.

Environment propagation requirements:
- Ensure Anthropic-related credentials can be forwarded in container runs when present.
- Keep existing token forwarding behavior intact.

Wiring requirements:
- integrate Claude mount options in `internal/vibe.go` container option composition,
- mirror existing style used for Codex/OpenCode state mounts.

### FR-7: Documentation updates

Update user-facing docs where built-in agents are enumerated:
- `docs/commands.md`,
- `docs/configuration.md`,
- any other command/config snippets that currently list `codex|opencode|bash`.

Doc guidance:
- mention container-mode OAuth/session propagation behavior,
- call out deferred items (session-resume specialization).

### FR-8: Tests

Use `stretchr/testify` assertions (`require`/`assert`) and favor e2e coverage where practical.

Unit tests:
- `internal/agentlauncher/claude_test.go`
  - command generation with/without prompt,
  - model passthrough,
  - yolo flag mapping,
  - reasoning-level to `--effort` mapping,
  - quoting safety.
- `internal/docker/*_test.go`
  - Claude state mount detection/selection logic,
  - missing-path graceful behavior.
- targeted parser/config tests for new agent enum acceptance.

E2E tests:
- launch flow with `--agent claude` via `--agent-cmd`-style harness patterns already used in repo,
- container launch path verifies Claude-related mounts/env are present in composed command.

## 5. Non-Functional Requirements

- No regressions for existing agents (`codex`, `opencode`, `bash`).
- Maintain deterministic command assembly and safe shell quoting.
- Keep container startup behavior within current operational expectations.
- Preserve existing logging and error-wrapping patterns.

## 6. Security Considerations

- `--yolo` maps to `--dangerously-skip-permissions`; this is expected and explicit.
- OAuth/session files contain sensitive data:
  - do not log file contents or tokens,
  - avoid embedding secrets in error strings,
  - mount only necessary paths.
- Keep existing GitHub credential handling unchanged unless intentionally modified.

## 7. Decisions

| Decision | Rationale |
|----------|-----------|
| Agent name is `claude` | Matches Claude Code binary and expected CLI UX |
| `--model` is pure passthrough | Avoid brittle validation and stay compatible with CLI evolution |
| `SupportedModels()` returns `nil` | Prevent false unsupported-model warnings from incomplete lists |
| `--yolo` maps to `--dangerously-skip-permissions` | Align Remuda safety toggle semantics across agents |
| `--reasoning-level` maps to `--effort` | Reuse existing Remuda reasoning control with Claude-native flagging |
| Container mode mounts `~/.claude` + `~/.claude.json` | Preserves OAuth/session continuity in container workflows |
| Session-resume specialization is deferred | Keep v1 focused on core launch/container integration |

## 8. Deferred Follow-ups (Nice to Have)

1. Claude-specific reasoning-level completion and compatibility hints for `--effort` values.
2. Claude-aware session resume behavior parity with other agents.
3. Optional dynamic model completions sourced from local Claude CLI capabilities.

## 9. Implementation Risks and Pitfalls

- CLI flag drift: Claude CLI flags may evolve; command building should be isolated in launcher tests for quick updates.
- Container auth assumptions: host machines may have partial Claude state; mount logic must tolerate missing files.
- Over-validation risk: adding strict model validation would conflict with passthrough goal.
- Regression risk in enum updates: ensure all agent lists are updated consistently (CLI tags, config validation, completions/tests).

## 10. Acceptance Criteria

- [ ] `remuda vibe --agent claude "hello"` launches Claude Code in a session.
- [ ] `remuda vibe --agent claude --model <value>` forwards `<value>` directly to Claude command.
- [ ] `remuda vibe --agent claude --yolo` includes `--dangerously-skip-permissions` in the Claude launch command.
- [ ] `remuda vibe --agent claude --reasoning-level <level>` includes `--effort <level>` in the Claude launch command.
- [ ] `remuda vibe --agent claude --container "hello"` runs using image with Claude installed.
- [ ] Container run can reuse host Claude OAuth/session state when `~/.claude` and/or `~/.claude.json` exist.
- [ ] Existing agent flows remain green in tests.
- [ ] User docs list `claude` among supported built-in agents and describe relevant behavior.
