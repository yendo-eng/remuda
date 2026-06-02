# `remuda session reap` – Product Requirements Document (Draft)

## 1. Purpose

Add a safe, default-dry-run command that helps developers identify and clean up **active** Remuda sessions that are older than a configurable time window, with an option to also delete the associated workspace (like `remuda session kill --cleanup`).

This targets **active** sessions that are likely abandoned (compare with `remuda workspaces remove`, which handles inactive workspaces explicitly).

## 2. Background / Problem Statement

Remuda sessions are typically launched detached in a session manager (tmux/zellij). Over time, users can accumulate “zombie” sessions that are still running but no longer useful (e.g., a detached agent that finished, a forgotten session left open for weeks, or a session whose workspace is no longer needed).

Today, users can:
- manually kill sessions (`remuda session kill --name …` or `--pick`)
- remove inactive workspaces explicitly (`remuda workspaces remove`)

There is no workflow to automatically target *active* sessions by age in a safe way.

## 3. Goals & Non‑Goals

### Goals

- Provide a safe default (dry-run) to surface which active sessions are “old”.
- Support a configurable threshold (e.g., `--older-than 336h`).
- Support `--cleanup` to also delete the workspace + git worktree for sessions that are killed.
- Favor e2e coverage using the existing test harness.

### Non‑Goals (v1)

- A full “workspace GC” solution across all dimensions (size, branch, PR state, last git activity, etc.).
- A cross-platform, perfect definition of “idle” across tmux and zellij.
- Automatic PR closure / merge-by-default (may be optional flags if explicitly desired).

## 4. Proposed User Experience

### CLI

```bash
# Show which active sessions would be killed (default dry-run)
remuda session reap --older-than 336h

# Actually kill those sessions
remuda session reap --older-than 336h --dry-run=false

# Kill + delete the associated workspace/worktree
remuda session reap --older-than 336h --dry-run=false --cleanup

# Interactively pick from candidate sessions (see FR-4 for candidate selection)
remuda session reap --older-than 336h --pick
```

### Output (plain text)

- For dry-run:
  - `would kill <session>`
  - if `--cleanup` is also set, additionally:
    - `would remove <workspacePath>`
- Summary line(s):
  - `would kill N sessions` (and optionally `would free <size>`)

## 5. Definitions

- **Active session**: A session returned by the configured session manager (tmux or zellij) that matches Remuda’s `org/repo/<folder>` naming convention.
- **Session age** (v1): Time since the session was created (not “idle time”), as reported by the configured session manager (tmux or zellij).

## 6. Functional Requirements

### FR‑1 — Threshold flag

- `--older-than <duration>` (required)
  - Uses Go-style durations (e.g., `72h`, `336h`), consistent with `time.ParseDuration`.

### FR‑2 — Execution flags

- `--dry-run` (default `true`):
  - When `true`, prints what would happen and makes no changes.
  - When `false`, performs kills (and optional cleanup).
- `--cleanup` (default `false`):
  - When set, after killing a session, also delete the workspace + remove its git worktree (best-effort, consistent with `session kill --cleanup`).

### FR‑3 — Candidate selection

The command identifies candidate sessions as:
1. List sessions via the configured session manager.
2. Filter to Remuda session names.
3. Filter by age (`created_at <= now - olderThan`) using the chosen age source.

If `--pick` is set:
- Run an fzf-based selection UI over the filtered candidates (multi-select).
- The command errors if the user selects nothing (consistent with existing pick flows).

### FR‑4 — Age source

- `session reap` uses the session manager as the canonical age source (no extra marker files).
  - tmux: use `session_created` from `tmux list-sessions -F ...` (Unix epoch seconds).
  - zellij: use the created age reported by `zellij list-sessions` (parseable from the `--no-formatting` output).
  - If a session’s age cannot be determined, it is skipped and reported as “unknown age” (safe default).

### FR‑5 — Error handling and edge cases

- If session listing fails, return an error.
- If a session’s age cannot be determined, do not kill it; print a line indicating it was skipped.
- If workspace path resolution fails:
  - still allow killing the session
  - but `--cleanup` should print a clear warning/error path for that session.

## 7. Non‑Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR‑1 | Safety: default behavior must never kill or delete anything without explicit opt-in (`--dry-run=false`). |
| NFR‑2 | Portability: behavior should be consistent on macOS/Linux; session-manager differences must be explicit. |
| NFR‑3 | Scriptability: plain-text output should be stable and line-oriented for piping. |
| NFR‑4 | Testability: e2e tests cover dry-run and execution paths; no dependency on real tmux/zellij binaries. |

## 8. Implementation Notes (non-binding)

- Prefer implementing this as a new `session` subcommand in `cmd/remuda/cli/`.
- Reuse existing internal primitives:
  - candidate listing via `Session.List()`
  - killing/cleanup via `Remuda.SessionKill(...)` (for shared semantics with `session kill`)
- Extend session-manager listing to provide the selected “age” signal for filtering.

## 9. Decisions (resolved)

- Command name: `remuda session reap`
- Age definition: created age only
- Attached sessions: no special handling; include in candidates like any other session
- Duration format: Go-style durations (e.g., `336h`)
- PR actions: do not support `--close-pr` or `--merge`
- Threshold: `--older-than` is required
- Session managers: must work with tmux and zellij
