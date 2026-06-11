# `remuda vibe --agent pi` – Product Requirements

## 1. Purpose

Add [pi-coding-agent](https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent) (`@mariozechner/pi-coding-agent`) as a supported agent in Remuda. Pi is a multi-provider terminal coding agent with extensible architecture and no approval gates by design.

## 2. Goals and Non-Goals

### Goals

- Pi as a first-class agent with feature parity to Codex
- Container support with `~/.pi/agent/` state persistence
- Model selection via `provider/model` format (e.g., `anthropic/claude-sonnet-4`)
- Map `--reasoning-level` to pi's `--thinking` flag
- Forward relevant API keys (ANTHROPIC_API_KEY, GEMINI_API_KEY) in container mode

### Non-Goals (v1)

- Extension/package management via Remuda
- OAuth/subscription flows (users handle `/login` directly in pi)
- Print mode (`-p`) for non-interactive use
- Session resume (defer to v2)

## 3. User Experience

```bash
# Basic usage
remuda vibe --agent pi "Implement pagination"

# With model selection (provider/model format required)
remuda vibe --agent pi --model anthropic/claude-sonnet-4 "Refactor auth"

# With thinking level
remuda vibe --agent pi --reasoning-level high "Design schema"

# Container mode
remuda vibe --agent pi --container "Add tests"
```

## 4. Functional Requirements

### FR-1: Agent Registration

Add `pi` to:
- `internal/enums/enums.go` – ValidAgents
- `internal/agentlauncher/agent_launcher.go` – SupportedAgent const, SupportedAgents(), ParseWithReasoning()
- `cmd/remuda/cli/shared.go` – Agent field enum tag
- `docs/configuration.md`, `docs/commands.md`

### FR-2: Pi Launcher

New file: `internal/agentlauncher/pi.go`

Implement `AgentLauncher` interface:
- `Command()` builds `pi --provider X --model Y --thinking Z 'prompt'`
- Model format: require `provider/model`, fail fast at parse time if missing `/`
- No yolo flag needed (pi has no approvals by design)
- `SupportedModels()` should return `nil` (not a partial list) to avoid false "model may not be supported" warnings for valid pi models

### FR-3: Model Validation

In `ParseWithReasoning()`:
- If `--agent pi` and `--model` is set, validate it contains `/`
- Reject with clear error if invalid (pi silently ignores `--model` without `--provider`)

### FR-4: Reasoning Level Support

Update:
- `internal/reasoning_level.go` – add pi to `AgentSupportsReasoningLevel()`
- `cmd/remuda/cli/shared.go` – update `--reasoning-level` help text to include pi
- Map `none` → `off` (pi uses "off" instead of "none")
- Pi supports: off, minimal, low, medium, high, xhigh

### FR-5: Container Mode

New file: `internal/docker/pi.go`

- Mount `~/.pi/agent/` → `/root/.pi/agent/` (RW)
- Respect `PI_CODING_AGENT_DIR` env var if set (use `env.Provider` for consistency with other container mounts)
- Forward ANTHROPIC_API_KEY, GEMINI_API_KEY (others already forwarded)

Wire in `internal/vibe.go` alongside existing agent-specific mount logic.

### FR-6: Container image pipeline

Update the team's container image build pipeline to include `@mariozechner/pi-coding-agent`.

**Note**: Pi requires Node >= 20. Ensure the chosen container base/runtime meets that requirement.

## 5. Non-Functional Requirements

- Pi startup latency comparable to Codex/OpenCode
- Works on macOS and Linux, Docker Desktop and Docker Engine
- Clear error if pi not installed
- Keep launch behavior consistent with existing agent defaults

## 6. Security

- Pi runs without approval gates (equivalent to `--yolo`)
- API keys forwarded via `-e` flags
- State directory created with 0700 permissions

## 7. Decisions

| Decision | Rationale |
|----------|-----------|
| Agent name: `pi` | Matches CLI binary |
| Require `provider/model` format | Pi ignores `--model` without `--provider` |
| Fail fast on invalid model | Better UX than silent fallback |
| No default model | Pi auto-selects based on available auth |
| Mount entire `~/.pi/agent/` | Captures sessions, auth, extensions |
| Map `none` → `off` | Pi uses "off" for thinking level |
| `SupportedModels()` returns `nil` | Avoids false warnings for valid models |
| Session resume: directory-scoped | Pi's `-c` continues recent session for current cwd; defer to v2 via `remuda session resume` |

## 8. Open Questions

1. **Version output format**: What does `pi --version` output? Need for version parsing.

## 9. Acceptance Criteria

- [ ] `remuda vibe --agent pi "hello"` launches pi in tmux session
- [ ] `remuda vibe --agent pi --container "hello"` runs with state persistence
- [ ] `remuda vibe --agent pi --model anthropic/claude-sonnet-4` passes provider and model correctly
- [ ] `remuda vibe --agent pi --reasoning-level high` maps to `--thinking high`
- [ ] `remuda vibe --agent pi --model invalid` fails with clear error
- [ ] API keys available in container
- [ ] Unit tests pass
- [ ] Docs updated
