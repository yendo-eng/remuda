# `remuda vibe check` – Product Requirements Document

## Implementation Status (2026-02-13)

The command shipped as `remuda vibe-check` (with `remuda vibe check` deprecated). `--name` is optional with a derived default, and detached mode is controlled by `--[no-]detached` rather than `--no-tmux`.

## 1. Purpose

Add a `vibe check` subcommand to Remuda that reviews a Pull Request (PR) using an AI code reviewer. The command fetches PR context (diff + metadata), prepares a local workspace, runs a non‑destructive review, and outputs a structured Markdown report. The agent responds in the chat/session; no PR comments in v1.

## 2. Background / Problem Statement

Engineers want quick, consistent AI assistance on PRs to surface correctness, security, performance, and testing issues before or alongside human review. Today, teammates manually assemble diffs and prompts in ad‑hoc agent sessions. This is inconsistent, time‑consuming, and hard to share.

Remuda already provides `remuda clone` and `remuda vibe` to spin up agent sessions. A dedicated `vibe check` can standardize PR review: clone or check out the PR branch, gather metadata, create a review prompt, run an agent in a controlled mode (no edits), and emit a high‑signal report — optionally posted back to the PR.

## 3. Goals & Non‑Goals

### Goals

- Provide a one‑liner to review a PR: `remuda vibe check <pr-url-or-id>`.
- Generate a structured Markdown report with actionable findings and suggested tests.
- Support GitHub PRs in v1 (GitHub only).
- Keep behavior non‑destructive: no code changes, no force pushes.
- Integrate smoothly with existing `clone` and `vibe` workflows (tmux defaults, env handling).
  

### Non‑Goals (v1)

- Inline file/line comments across the diff (can be a follow‑up).
- Full provider parity (start with GitHub; GitLab later).
- Auto‑approving or changing PR states.
- Attempting large‑scale repo indexing beyond the PR’s diff and a small set of nearby files.

## 4. Personas

- Developer (author): pre‑flight check for issues and missing tests before asking for review.
- Reviewer: quick summary of risk and hotspots to focus manual review.
- Release engineer: confidence report on backports/hotfixes.

## 5. User Stories

| ID | Description |
|----|-------------|
| US‑1 | As a developer, I run `remuda vibe check https://github.com/org/repo/pull/123` and get a Markdown review in the agent session without additional setup. |
| US‑2 | As a developer, I can run the review inline with `--no-tmux` or detached (default) in tmux. |
| US‑4 | As a power user, I choose the agent/model via `--agent`/`--model` or supply `--agent-cmd`. |
| US‑5 | As a developer, I get a concise, structured report surfaced directly in the agent chat. |

## 6. Functional Requirements

### FR‑1 — CLI syntax

```
remuda vibe check --name <name> <pr-ref> [flags]
```

- Required — `--name <name>`: unified naming. Maps to folder/branch/session.
- Positional — `<pr-ref>`: a GitHub PR URL (`https://github.com/org/repo/pull/123`) or an ID/number when `--repo-url` is supplied (e.g., `123`).
- Flag — `--repo-url <url>`: override or supply repository when `<pr-ref>` is not a URL.
- Other flags: `--agent`, `--model`, `--agent-cmd`, `--no-tmux`, `--repo-url`, `--repo`, `--jira`.
- New flag: `--gh-issue <ref>` — repeatable; accepts GitHub issue URLs, `owner/repo#123`, or bare numbers (uses repo context). Injects issue metadata into the review prompt.
- New flag: `--attach` — immediately attach to the session after launch; only valid when the command runs detached (default behaviour).

### FR‑2 — Environment & Secrets

1. GitHub: use the `gh` CLI exclusively in v1, requiring `gh auth status` to be authenticated. No direct REST calls in v1. Use `gh pr checkout <ref>` to determine and check out the PR head branch.
2. OpenAI: use existing `OPENAI_API_KEY` env when set; do not infer a key.
3. Do not log tokens. Redact secrets in error paths.

### FR‑3 — Data acquisition

- Resolve repo + PR metadata using `gh`:
  - `gh pr view <ref> --json ...` for title, author, base/head branches, URL, labels.
  - `gh pr diff <ref>` for the unified patch used for review.
  - `gh pr checkout <ref>` to materialize the PR locally (inside the workspace).
- Clone/checkout:
  - Delegate workspace creation to `remuda clone --name <name>`.
  - Inside the workspace, use `gh pr checkout <ref>` to materialize the PR head, then create/switch to local branch `<name>` at the current HEAD.
  - Use a full clone by default to avoid shared worktree collisions; users can pass `--no-full-clone` to opt back into worktrees. Full clones copy the refreshed `.repo_cache` locally (no network round-trip) so performance stays predictable.
- Save artifacts under workspace:
  - `.vibe/check/pr.json` — metadata snapshot.
  - `.vibe/check/diff.patch` — unified diff used for review.
  - `.vibe/check/context.txt` — additional derived context (e.g., changed file list, language mix, LOC stats).
  - When `--gh-issue` is provided, record the issue metadata that was injected to the prompt.

### FR‑4 — Prompt construction & agent launch

1. Compose a review prompt that instructs the agent to:
   - Summarize the PR’s intent and scope.
   - Identify correctness, security, performance, reliability, and concurrency issues.
   - Flag API/contract changes and migration risks.
   - Propose focused tests to add or update.
   - Call out style/readability only when it harms maintainability.
   - Output a structured Markdown report with sections: Summary, Risk, Findings (High/Med/Low with file:line), Suggested Tests, Follow‑ups, Verdict.
2. Input material:
   - PR title + body, base/head info, labels.
   - Unified diff from `gh pr diff` (may be chunked internally by the agent if needed).
   - Optional: `.jira_context.txt` from `--jira` handling (reused from `vibe`).
   - Optional repo signals when present: `CONTRIBUTING.md`, `SECURITY.md`, `CODEOWNERS`, `.golangci.yml`, etc. (best‑effort small excerpts).
3. Execution mode:
   - Non‑interactive by default; the agent’s review appears in the chat (tmux window or current terminal).
   - Map `--agent`/`--model`/`--agent-cmd` using the same semantics as `vibe`. If `--agent-cmd` is provided, append the prompt path as the final arg. Do not forward `--yolo` in `vibe check`.
   - Detached tmux session by default (consistent with `vibe`); `--no-tmux` streams output inline. Session name follows `org/repo/<folder>`.

### FR‑5 — Output & exit semantics

- Success: print the absolute workspace path on stdout (first line, consistent with `vibe`). The full review content is shown in the agent chat/session.
- Failure: propagate non‑zero exit codes from acquisition (clone/gh), or agent run.

## 7. Non‑Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR‑1 | Performance: Added latency over `vibe` must be dominated by diff retrieval and model latency; command setup overhead ≤ 150 ms on typical dev machines. |
| NFR‑2 | Portability: macOS and Linux; relies on `gh` CLI when available for GitHub. |
| NFR‑3 | Privacy: Never transmit secrets or unrequested host files; only the diff and selected small context are sent to the model. |
| NFR‑4 | Testability: ≥ 90% unit coverage for parsing, filtering, command construction, and comment‑update selection. No external network required for tests. |

## 8. Open Questions

1. What default agent/mode best suits summarization (e.g., `opencode` vs `codex` non‑interactive)?
2. How should we chunk very large diffs, if needed, without new flags — internal size heuristics or per‑file batching?

## 9. Appendix — Example Flows

### A) Minimal: local report only

```
$ remuda vibe check --name feature-login-audit https://github.com/acme-org/example-repo/pull/123
# stdout line 1: /abs/.remuda/repos/acme-org/example-repo/feature-login-audit
# …review appears in the agent session…
```

### B) Custom model and inline execution

```
$ remuda vibe check 123 \
  --repo-url https://github.com/acme-org/example-repo \
  --model gpt-4o-mini --no-tmux
```
