# Remuda Jira Cloud Client Design

## Implementation Status (2026-03-26)

Shipped. Remuda now uses a native Jira Cloud HTTP client at runtime and no
longer shells out to the `jira` CLI. The legacy shell implementation
(`internal/jira/shell_jira.go`) has been removed.

## 1. Purpose

Replace the shell-out to the `jira` CLI for `--jira` context with a native Jira
Cloud client library inside Remuda. The new client must fetch an issue and its
comments, format them as prompt-ready text, and preserve existing UX and
failure semantics.

## 2. Background / Problem Statement

Remuda historically shelled out to the `jira` CLI (`jira view <ID>`) to build
prompt context. This added an external binary dependency and leaked
behaviors/config that we do not control. The goal is to embed a Jira Cloud
client to remove the shell dependency while keeping the CLI behavior the same
for users.

## 3. Goals & Non-Goals

### Goals

- Preserve the `--jira` flag behavior (repeatable, ordered) while removing shelling out.
- Jira Cloud only, using API token authentication.
- Fetch issue details and comments and format them in a human-readable prompt block.
- Use a library-style Jira client inside `internal/jira` that can be reused.
- Do not log tokens or credentials.
- No caching.

### Non-Goals

- Jira Server/Data Center support.
- Full parity with `jira view` output and templates.
- Keyring/pass/gopass integration (token retrieval is env/config only).
- Rendering every Jira rich-text edge case; focus on reliable plain text.

## 4. User Stories

| ID | Description |
|----|-------------|
| US-1 | As a developer, I can run `remuda vibe --jira ABC-123 "..."` and Remuda prepends a readable Jira summary + comments. |
| US-2 | As a developer, I can pass multiple `--jira` flags and the output is ordered to match input. |
| US-3 | As a developer, I can keep using my existing Jira Cloud token without installing the `jira` binary. |
| US-4 | As a developer, failures to fetch Jira data fail fast before cloning/launching. |

## 5. Functional Requirements

### FR-1 CLI behavior

- No CLI surface change: `--jira <ID>` remains repeatable in `vibe` and `vibe-check`.
- BuildContext remains the single call path (`jira.BuildContext`).
- Fail fast when any Jira fetch fails, same as today.

### FR-2 Jira Cloud authentication & config

- Support Jira Cloud API token authentication.
- Token source priority:
  1. CLI overrides (`--jira-endpoint`, `--jira-user`, `--jira-token`).
  2. Environment (`REMUDA_JIRA_ENDPOINT`, `REMUDA_JIRA_USER`, `REMUDA_JIRA_API_TOKEN` and legacy `JIRA_*` aliases).
  3. Remuda config file (`jira.endpoint`, `jira.user`, `jira.api_token`).
- Base URL and user/email resolution:
  - Load from Remuda config discovery (`REMUDA_CONFIG`, XDG config path, then legacy `~/.remuda/config.yaml`).
  - Override from env and per-invocation CLI flags using the precedence above.
- If any required values are missing (endpoint/user/token), return a clear error with next steps.

Notes:
- Remuda does not read `.jira.d/config.yml`, keyring/pass-backed go-jira settings, or executable go-jira config scripts.

### FR-3 Config discovery

- Discover Jira defaults from Remuda config using the same lookup order as other CLI config:
  - `REMUDA_CONFIG` (strict, single path)
  - `$XDG_CONFIG_HOME/remuda/config.yaml` (or `~/.config/remuda/config.yaml`)
  - `~/.remuda/config.yaml` (legacy fallback)
- Resolve Jira auth as CLI overrides > env > config file values.

### FR-4 Data retrieval

- For each issue key:
  - Fetch issue fields (summary, status, assignee, reporter, priority, created, updated, description).
  - Fetch comments (all, via pagination).
- Jira Cloud REST API v3 endpoints (explicit):
  - Issue: `GET /rest/api/3/issue/{key}` with query params:
    - `fields=summary,status,assignee,reporter,priority,created,updated,description,issuetype`
    - `expand=renderedFields` (required to enable HTML fallback if ADF parsing fails)
  - Comments: `GET /rest/api/3/issue/{key}/comment` with query params:
    - `startAt=<offset>` and `maxResults=<pageSize>`
    - Note: `orderBy=created` is not guaranteed; perform client-side sort by `created` if ordering is needed.
- Pagination:
  - Loop while `startAt + maxResults < total` (using response `total`).
  - Use a bounded page size (eg. 50) to avoid large payloads.
- Set a reasonable HTTP timeout (eg. 10s) per request.

### FR-5 Text formatting

- Return a single text blob for each issue that includes:
  - Key + summary
  - Status, type, priority, assignee, reporter
  - Description (plain text)
  - Comments, ordered by creation time, with author + timestamp + body
- Prepend BuildContext ticket fence lines exactly as today:
  - `---------- Ticket <KEY> ----------`
- Ensure trailing newline at end of each ticket block.

### FR-6 Input validation

- Validate Jira issue keys as early as possible in CLI parsing (Kong).
  - Use a simple pattern (`^[A-Z][A-Z0-9]+-\d+$`) and normalize to upper case.
  - Validation should happen in `AfterApply` on `ContextEngineeringOptions` to align with existing Kong hooks.

### FR-7 Error handling

- Translate Jira HTTP errors into actionable CLI errors:
  - 401/403: missing/invalid token or insufficient permissions.
  - 404: issue not found or no access.
  - 429: rate limited (include retry hint).
- Do not log tokens or include Authorization headers in error strings.

## 6. Architecture / Design

### 6.1 Package layout

- `internal/jira/config.go`
  - Loads config from Remuda config discovery + env overrides.
  - Resolves endpoint, user/login, token.
- `internal/jira/client.go`
  - Defines structured types: `Issue`, `Comment`, `User`, `Status`, etc.
  - Interface: `Client` with `GetIssue(ctx, key)` and `GetComments(ctx, key)`.
- `internal/jira/http_client.go`
  - HTTP implementation for Jira Cloud REST API v3.
  - Handles auth, pagination, timeouts.
- `internal/jira/formatter.go`
  - Converts `Issue` + comments into plain text for `GetTicket`.
- `internal/jira/jira.go`
  - Keep `Jira` interface for compatibility; `GetTicket` delegates to client + formatter.

### 6.2 Auth details

- Use HTTP Basic auth with `user:token` (Jira Cloud API token flow).
- The `user` should be email (from `login` or `user` config field).
- Never emit or log the token.

### 6.3 Jira Cloud data format

- Description and comment bodies are returned as Atlassian Document Format (ADF).
- Primary rendering path: minimal ADF-to-text renderer.
  - Supported nodes: `doc`, `paragraph`, `text`, `heading`, `bulletList`, `orderedList`, `listItem`, `codeBlock`, `hardBreak`.
  - Unknown nodes should degrade gracefully (skip content, or insert a short placeholder like `[unsupported content]`).
- Fallback path (deterministic): if ADF parsing fails or produces empty output for a non-empty field, fall back to `renderedFields` HTML.
  - Because of this fallback, `expand=renderedFields` is always included on the issue request.
  - HTML fallback should be converted to text by stripping tags and decoding entities; links can be preserved as plain URLs if present in anchor text.

## 7. Output Format (Example)

```
---------- Ticket ABC-123 ----------
ABC-123: Fix payment retry backoff
Status: In Progress | Type: Bug | Priority: P1
Assignee: Jane Doe | Reporter: John Roe
Created: 2025-01-12 09:32 UTC | Updated: 2025-01-30 14:10 UTC

Description:
- Retry backoff is too aggressive for provider X.
- Adjust jitter parameters.

Comments:
- 2025-01-15 10:01 UTC Jane Doe:
  Working on a patch; adding feature flag.
- 2025-01-20 16:44 UTC John Roe:
  Verified issue in staging.
```

## 8. Testing Strategy

- Unit tests for:
  - Config resolution precedence (CLI override > env > Remuda config).
  - ADF-to-text renderer (input fixtures -> text output).
  - Pagination handling for comments.
- Use `httptest.Server` to avoid external Jira dependency.
- E2E tests remain unchanged; `jira.Mock` should still be used where appropriate.
- Use `stretchr/testify` with `require` for fatal assertions and `assert` for non-fatal checks.

## 9. Rollout Status

- Landed: runtime wiring in `cmd/remuda/main.go` uses `NewHTTPJira()`.
- Landed: the legacy `NewShellJira()` implementation has been removed.
- `jira.Mock` remains the test seam for context-related tests.

## 10. Risks / Pitfalls

- Jira Cloud returns ADF for description/comment bodies; without a renderer the output is unreadable. The renderer is the highest-risk complexity.
- go-jira config supports keyring/pass token storage; we will not read those, so users relying on keyring must set `JIRA_API_TOKEN` explicitly.
- API pagination for comments can silently truncate output if not implemented.
- Jira permissions may hide fields or comments; formatter must tolerate missing data.
- Jira rate limits (429) can cause failures when multiple `--jira` IDs are used.

## 11. Open Questions

1. Resolved: keep Cloud-only token auth in Remuda config/env/flags; no `.jira.d` bearer-token support.
2. Resolved: do not support `.jira.d` executable configs.
3. Should the renderer include attachments and links, or keep text-only output?
