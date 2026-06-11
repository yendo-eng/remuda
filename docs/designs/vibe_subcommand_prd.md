# `remuda vibe` - Product Requirements Document

## 1. Purpose

`remuda vibe` is Remuda's primary entry point for starting AI coding sessions.
The command:

- `--name` is optional. When omitted, Remuda derives a workspace name from the
  first Jira ticket title (when `--jira` is provided) or from the prompt,
  falling back to `session`.
- Detached mode is controlled by `--[no-]detached`. A hidden `--no-tmux` alias
  is also supported.
- Detached launches use the configured session manager (`tmux` by default,
  `zellij` also supported), not tmux only.
- Built-in agents are `codex`, `opencode`, `claude`, and `bash`.
- Prompt enrichment supports Jira tickets, Slack threads, GitHub issues, and
  saved prompt-bank entries.
- Container mode, profiles, reasoning-level configuration, and first-run repo
  selection are part of the command surface.
- Jira context is fetched via Remuda's native Jira HTTP client. GitHub issue
  context is fetched through the configured `gh` CLI.

1. Provision a fresh workspace for a selected repository, or reuse an existing
   workspace.
2. Construct and launch a configured coding agent.
3. Start the session inline or detached under the configured session manager.
4. Enrich the prompt with saved prompts and external context before launch.

## 2. Background / Problem Statement

Launching an AI coding session involves several related concerns:

- repository selection
- workspace provisioning or reuse
- prompt and context construction
- agent command construction
- detached or inline execution

`remuda vibe` centralizes that workflow inside Remuda so repo selection,
workspace creation, prompt construction, agent launch, and session management
all follow one implementation path with consistent validation and defaults.

## 3. Goals & Non-Goals

### Goals

- Provide a single command for launching AI-assisted development sessions.
- Support both fresh workspaces and existing workspaces.
- Keep detached launches ergonomic while also supporting inline execution.
- Make prompt/context engineering a first-class part of session startup.
- Honor config, env, and profile-driven defaults without hiding important
  launch behavior.

### Non-Goals

- Windows support.
- A generic free-form "pass arbitrary extra args to any agent" surface.
- Replacing repo/worktree lifecycle behavior owned by `remuda clone`.
- Designing every future workflow under `vibe` (for example, review and fanout
  remain separate commands).

## 4. Personas

- Backend / full-stack developers who need multiple isolated workspaces quickly.
- Reviewers or prototypers who want reproducible agent startup defaults.
- Power users who want config/profile-driven setup with low launch ceremony.

## 5. User Stories

| ID | Description |
| --- | --- |
| **US-1** | As a developer, I can run `remuda vibe "Implement pagination"` and get a derived workspace name plus a detached Codex session without first inventing a branch/workspace slug. |
| **US-2** | As a developer, I can run `remuda vibe --in /path/to/workspace` to reuse an existing checkout instead of cloning again. |
| **US-3** | As a power user, I can select an agent, model, reasoning level, profile, and session manager without rewriting the rest of the launch flow. |
| **US-4** | As a developer, I can prepend Jira, Slack, GitHub issue, and saved prompt context before the main prompt is sent to the agent. |
| **US-5** | As a developer, I can run the session in a Docker container when I want more isolation from the host environment. |

## 6. Functional Requirements

### FR-1 - CLI shape

```bash
remuda vibe [--name <name> | --wizard] [<prompt>] [flags]
```

- `<prompt>` is optional. If omitted, Remuda launches the agent without a main
  prompt so the agent CLI can enter its own default interactive behavior.
- Supplying `-` as the prompt reads the prompt from STDIN.
- `--wizard` launches an interactive TTY-only flow for repo, prompt, context,
  agent, detached mode, and container selection.

### FR-2 - Workspace naming and reuse

- `--name <name>` sets the workspace name seed.
- When `--name` is omitted and `--in` is not used, Remuda derives the workspace
  name from:
  1. the first Jira ticket title, if `--jira` is present and slugification
     succeeds;
  2. otherwise the prompt text;
  3. otherwise `session`.
- `--in <path>` reuses an existing workspace and skips cloning.
- `--in` is incompatible with `--wizard`, `--name`, `--branch`, and
  `--full-clone`.
- Session names are derived from the workspace path as `org/repo/<folder>`.

Note on slash-containing names:

- The workspace folder uses the basename of `--name`.
- If callers want a branch name that differs from the workspace folder, they
  should pass `--branch <branch>` explicitly.

### FR-3 - Repository resolution and cloning

- Repo selection supports:
  - `--repo <alias>`
  - `--repo-url <url>`
  - env/config defaults
  - first-run interactive selection on TTYs when no repo is otherwise set
- `--repo-url` also accepts GitHub shorthand such as
  `github.com/org/repo`, which is expanded to the canonical HTTPS URL.
- When cloning is required, `remuda vibe` delegates workspace provisioning to
  the same underlying clone flow used by `remuda clone`.
- Clone-related flags supported by `vibe`:
  - `--branch <name>`
  - `--force`
  - `--[no-]full-clone`
  - `--no-clone-hooks`
- `--force` replaces an existing workspace and also kills any detached session
  with the same derived session name before relaunch.

### FR-4 - Agent selection and launch command construction

- Built-in agents:
  - `codex` (default)
  - `opencode`
  - `claude`
  - `bash`
- `--agent-cmd <cmd>` overrides the built-in launcher entirely. When a prompt is
  present, Remuda appends it as the final quoted argument.
- `--model <model>` overrides the model when supported.
- `--model agent-default` suppresses Remuda's explicit model flag so the agent
  CLI can choose its own default.
- `--profile <name>` applies a named config profile before final flag/env
  precedence is resolved for agent, model, yolo, container, and related
  defaults.
- `--reasoning-level <level>` is supported as follows:
  - Codex: validated against `none|minimal|low|medium|high|xhigh`
  - Claude: forwarded to `claude --effort <level>`
  - OpenCode: ignored with a warning
  - Bash: ignored
- `--yolo` maps to agent-specific approval/sandbox bypass behavior:
  - Codex: `--dangerously-bypass-approvals-and-sandbox`
  - Claude: `--dangerously-skip-permissions`

### FR-5 - Prompt and context engineering

- Explicit prompt/context inputs are prepended before the main prompt in this
  order:
  1. saved prompts selected by `--use`
  2. Jira ticket context from `--jira`
  3. Slack thread context from `--slack-thread`
  4. GitHub issue context from `--gh-issue`
  5. the main prompt
- `--use <prompt-name>` is repeatable and resolves built-in plus custom prompts.
- `--no-use <prompt-name>` excludes prompt defaults or explicitly selected saved
  prompts.
- `--jira <ABC-123>` is repeatable and validated near parse time.
- Jira auth/endpoint overrides:
  - `--jira-endpoint`
  - `--jira-user`
  - `--jira-token`
- `--slack-thread <url>` is repeatable and requires Slack auth to be configured.
- `--gh-issue <ref>` is repeatable and accepts:
  - issue URLs
  - `owner/repo#123`
  - bare issue numbers when a repo can be inferred
- Prompt context flags require a non-empty prompt.
- Explicit `--use` on the CLI also requires a non-empty prompt.

### FR-6 - Session management behavior

- Detached mode is enabled by default.
- `--no-detached` runs the agent inline in the current terminal.
- Hidden `--no-tmux` is also supported for inline execution.
- `--attach` attaches immediately after starting a detached session and is only
  meaningful in detached mode.
- The active session manager is selected by the global
  `--session-manager tmux|zellij` flag or `REMUDA_SESSION_MANAGER`.
- Detached session startup includes crash-recovery sleep so users can inspect
  the buffer after unexpected agent exit.

### FR-7 - Container mode

- `--container` enables containerized session launch.
- Container mode requires an explicit image via `--container-name` or config.
- `--container-opt` appends raw `docker run` arguments.
- `--container-inherit-env` forwards additional host env vars by name.
- Container launches mount the workspace and forward the auth/state needed for
  supported agents and GitHub operations.
- Claude container sessions additionally forward Claude auth/session state when
  present on the host.

### FR-8 - Environment and secrets

- `--openai-api-key` overrides `OPENAI_API_KEY` for agents that use it.
- If `OPENAI_API_KEY` is unset and no override is supplied, Remuda does not try
  to infer one.
- GitHub authentication for issue-context and container workflows relies on the
  existing `gh` CLI setup and/or `GH_TOKEN` / `GITHUB_TOKEN`.
- Detached tmux launches explicitly export relevant env vars so long-lived tmux
  servers do not retain stale values.

### FR-9 - Output and exit semantics

- On success, the workspace path is printed as the first stdout line.
- Inline (`--no-detached`) runs stream the agent process stdout/stderr directly.
- Failures from repo resolution, cloning, context fetching, agent validation, or
  agent execution surface as actionable errors.
- Clone failures should not leave partially created workspaces behind; cleanup is
  delegated to the underlying clone flow.

## 7. Non-Functional Requirements

| ID | Requirement |
| --- | --- |
| **NFR-1** | Input validation should happen as close to parse time as practical, especially for incompatible flags and malformed context references. |
| **NFR-2** | Detached and inline launches should share one command-construction path so behavior stays consistent across execution modes. |
| **NFR-3** | The command should remain e2e-testable, with behavior-oriented tests preferred over help-text assertions. |
| **NFR-4** | Supported host platforms remain macOS and Linux. |

## 8. Metrics / Success Criteria

- `remuda vibe` remains the default entry point for new coding sessions.
- Session startup remains dominated by clone/network/model latency rather than
  CLI overhead.
- Users can launch the same workflow via explicit flags, env defaults, or
  profiles without observable behavioral drift.

## 9. Open Questions

1. Should `--attach` become a validated error when combined with inline mode
   instead of being merely non-useful there?
2. Should repo/folder naming semantics be clarified or adjusted for slash-heavy
   `--name` inputs?
3. Should prompt-context features gain first-class support for promptless
   interactive sessions?

## 10. Out of Scope / Future Enhancements

- Windows support.
- A generic pass-through flag bag for arbitrary downstream agent arguments.
- Collapsing `vibe`, `vibe-check`, and `vibe-fanout` into one command surface.
- Replacing the underlying clone/worktree strategy owned by `remuda clone`.

## 11. Appendix - Example Flows

### A) Minimal default launch with derived name

```bash
$ remuda vibe "Implement pagination for transactions"
# -> prints the workspace path
# -> launches a detached session using the configured session manager
```

### B) Explicit workspace folder plus explicit branch

```bash
$ remuda vibe --name pagination --branch feature/pagination "Implement pagination"
# -> workspace folder .../pagination
# -> checked-out branch feature/pagination
```

### C) Reuse an existing workspace inline

```bash
$ remuda vibe --in ~/src/acme/widgets/pagination --no-detached
# -> runs the selected agent in the current terminal without cloning
```

### D) Containerized Claude run

```bash
$ remuda vibe \
    --agent claude \
    --container \
    --container-name ghcr.io/acme/remuda-agent:latest \
    --name auth-hardening \
    "Harden token refresh flows"
```

### E) Jira + Slack + GitHub issue context

```bash
$ remuda vibe \
    --jira ABC-123 \
    --slack-thread https://acme.slack.com/archives/C123/p1712345678901234 \
    --gh-issue acme/widgets#42 \
    --use small-commits \
    "Finish the retry redesign"
```
