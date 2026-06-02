# `remuda vibe --loop` - Design Notes

## 1. Purpose

Define where the `--loop` behavior should live (inside vs outside the session)
for a new `remuda vibe --loop` flag, and recommend an MVP approach.

## 2. Background

Users asked for a "Ralph Wiggum" loop: run the agent repeatedly until a
stop condition passes. Remuda launches sessions detached by default
(tmux/zellij), and can run inside containers, so the loop placement drives UX
and reliability.

## 3. Options

### Option A: Loop inside the session (wrapper command)

Remuda starts a wrapper command inside the tmux/zellij session. The wrapper
repeats:
1) run the agent
2) run a stop hook
3) sleep/backoff

### Option B: Loop outside the session (launcher respawns)

Remuda monitors the session exit status. If the stop hook fails, it re-launches
another session (or restarts the agent within the same session).

## 4. Evaluation Criteria

- Detached vs inline behavior
- Session manager parity (tmux/zellij)
- Container mode compatibility (stop hook environment)
- Crash recovery semantics and retries
- UX for attach/readbuf and clear termination
- Safety: avoiding infinite loops, max attempts/backoff

## 5. Tradeoffs

### Option A (inside session)

Pros:
- Works the same for detached and inline runs.
- Stop hook and agent run in the same environment (same container, cwd, env).
- Minimal changes to session orchestration; easy to ship quickly.

Cons:
- Harder to observe from the parent process; loop control is buried inside the
  session.
- Attach/readbuf may include wrapper noise unless we keep logging minimal.
- Interrupt semantics are per-session (user must attach/kill session).

### Option B (outside session)

Pros:
- Remuda retains control; can enforce backoff, max attempts, and richer status.
- Central place to log failure reasons and loop state.
- Easier to add metrics/telemetry later.

Cons:
- More complex orchestration across tmux/zellij and inline mode.
- Container mode becomes more expensive (start/stop per iteration).
- More edge cases around race conditions when users attach/kill sessions.

## 6. Recommendation (MVP)

Choose Option A for the MVP.

Rationale: it is the lowest-complexity option that works for detached sessions
and container mode, and keeps the stop hook in the same environment as the
agent. It aligns with Remuda's current session model and avoids complex
cross-process orchestration.

## 7. MVP Behavior (Option A)

### CLI (proposed)

- `--loop` (bool): enable looping.
- `--loop-stop-hook <cmd>` (string, required when `--loop` is set): shell
  command that returns exit code 0 to stop, non-zero to continue.
- `--loop-max-attempts <n>` (int, default 10): guardrail to avoid accidental
  infinite looping. Use `0` only for explicit unlimited looping.
- `--loop-sleep <duration>` (duration, default 0): sleep between attempts.

### Execution

Inside the session, run a wrapper script that:

- Runs the agent command (same as normal `vibe`).
- Runs the stop hook in the workspace root with the same environment.
- Stops on hook success; otherwise continues unless max-attempts is reached.
- Prints a short, consistent prefix per attempt to reduce noise.

Inline mode (no tmux/zellij) should behave the same way: run the wrapper
directly in the current terminal instead of forcing a detached session.

### Safety / UX

- Enforce `--loop-stop-hook` validation at parse time (Kong) to prevent
  accidental infinite loops.
- Respect `--loop-max-attempts` and exit non-zero if exhausted.
- In detached mode, log loop state to `.vibe/loop/` (attempt count, last hook
  output) so users can diagnose without reattaching.
- If Remuda currently wraps detached sessions with a crash-recovery sleep,
  consider skipping or shortening that sleep when `--loop` is enabled so
  completed loops exit promptly.

## 8. Risks and Mitigations

- **Wrapper noise in session output**: keep loop logging minimal; optionally
  write detailed hook output to `.vibe/loop/`.
- **Hard to interrupt**: document that users can `remuda session kill` or
  attach and interrupt the shell.
- **Stop hook behavior in containers**: run the hook inside the same container
  so it has access to the workspace and secrets.

## 9. Follow-up Work (Option B)

If loop orchestration needs stronger guarantees or richer status reporting,
revisit Option B:

- Move loop control into Remuda, tracking attempts and stop-hook results.
- Add backoff/jitter policies and structured logs in the launcher.
- Consider per-iteration session reuse to avoid tmux/zellij churn.
