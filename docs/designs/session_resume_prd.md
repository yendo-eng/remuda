# `remuda session resume` — Product Requirements (v1)

Beads tracking: `remuda-5dw`

## 1. Purpose

Provide an officially supported way to resume the most recent **Codex** session in an existing, **inactive** Remuda workspace.

This is equivalent in spirit to running `remuda vibe --in <workspace-dir> --agent=codex --agent-cmd "codex resume --last"`,
but with:

- a stable, documented command,
- explicit eligibility checks (must be inactive),
- an interactive `--pick` flow for choosing from inactive workspaces.

## 2. Goals / Non-Goals

### Goals

- Resume Codex sessions in an existing workspace directory.
- Support an interactive pick flow to choose from inactive workspaces under `ReposBaseDir`.
- Refuse to resume workspaces that have an active Remuda session.
- Execution semantics match normal Remuda sessions (detached by default, `--attach` supported, container support honored).

### Non-Goals (v1)

- Resuming non-Codex agents (e.g. opencode).
- Resuming arbitrary shell commands beyond `codex resume --last`.
- Scanning directories outside the configured `ReposBaseDir`.

## 3. CLI

```
remuda session resume <workspace-dir>
remuda session resume --pick
```

### Args and flags

- Exactly one of:
  - `<workspace-dir>` (positional argument), or
  - `--pick` (interactive selection)
- Session run flags should match normal session runs:
  - `--attach`
  - `--detached/--no-detached`
  - `--yolo/--no-yolo` (Codex only; enables approval/sandbox bypass)
  - Container flags: `--container`, `--container-name`, `--container-opt`, `--container-inherit-env`

If both `<workspace-dir>` and `--pick` are provided, it is a CLI validation error.

## 4. Workspace eligibility

### Workspace path validation

The provided/picked workspace must be:

- within `ReposBaseDir`, and
- exactly at depth 3: `<org>/<repo>/<folder>`,
- not a special directory such as `.repo_cache`.

Workspace validation enforces these rules at the eligibility check stage.

### Inactive requirement

The workspace must be “inactive”:

- it is not associated with any currently active Remuda session.

If the workspace is active, resume must fail with a clear error and must not start any new session.

## 5. `--pick` behavior

- The candidate list is inactive workspaces under `ReposBaseDir`.
- Only scans `ReposBaseDir`.
- Uses `fzf` for interaction.
- Single-select (resume is for one workspace at a time).
- Display format: `org/repo/folder`, mapping back to the absolute workspace path.

## 6. Execution behavior

- Only supported agent: Codex (v1).
- Command executed in the workspace directory:
  - `codex resume --last`
- When `--yolo` is set, append the Codex yolo flags (`--dangerously-bypass-approvals-and-sandbox` plus the env allowlist override).
- Session naming uses the existing workspace-derived convention: `org/repo/folder`.
- Execution should go through the same launch path as normal sessions so that:
  - detached session managers (tmux/zellij) behave consistently,
  - crash-recovery behavior is consistent,
  - container configuration is honored the same way as `vibe` (including Codex mounts and auth forwarding).

## 7. Failure modes

- `--pick` without interactive TTY => error.
- `fzf` not in `PATH` when `--pick` is used => error.
- No inactive workspaces available => user-friendly message (no-op, exit 0).
- Provided `<workspace-dir>` not eligible (outside base dir / wrong depth / special directory) => error.
- Workspace is active => error (refuse to resume).

## 8. Acceptance criteria

- `remuda session resume <workspace-dir>` starts a Remuda session and runs `codex resume --last` from that workspace.
- `remuda session resume --pick` lists only inactive workspaces under `ReposBaseDir`, then resumes the selected workspace.
- Resuming an active workspace fails with a clear error.
- Container flags/config behave the same as normal session runs.
- Tests pass: `go test ./...`.
