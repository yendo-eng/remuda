# Remuda codebase review — June 10, 2026

Scope: full-repo architectural and tactical review of master as of `780f326`
(~17k LoC non-test Go across 263 files, plus 123 test files). Method: direct
reading of the core flow (`internal/vibe.go`, `cmd/remuda/cli/config_resolver.go`,
`internal/app.go`, `cmd/remuda/main.go`), plus systematic sweeps of the CLI
layer, the `internal/` tree, and the test/CI setup. Findings marked **[verified]**
were confirmed against source by hand; the rest were confirmed at the file/line
level during the sweep.

## Executive summary

This is a healthy codebase. It builds clean, `go vet` is silent, the full test
suite passes in under 10 seconds with `-race` in CI, the e2e harness has genuinely
excellent environment isolation, and dependency injection through the `Remuda`
facade keeps the core testable without heavy mocking. The stated architecture
(porcelain in `cmd/remuda/cli`, business logic in `internal/`) is real and mostly
respected for simple commands.

The two structural problems are both gravity-related: **config resolution has
accreted in the CLI layer** (precedence logic spread across 5+ files, with an
837-line manual flag→config mapping that must be edited every time a flag is
added), and **shell-string composition is the load-bearing mechanism for
launching everything** (tmux/zellij start commands, docker run options) with
four independent quoting helpers and ad-hoc `fmt.Sprintf` splicing. Neither is
broken today, but both are where bugs will come from as the tool grows.

Tactically, there are a handful of real bugs (comma-splitting of slice config
values, a copy-pasted dead condition in model validation, an orphaned-workspace
leak on launch failure, an API key written to an uncleaned temp dir) — all small
and cheap to fix.

## Architecture

### What's working

- **Composition root + DI.** `internal.NewRemuda` (internal/app.go) takes
  interfaces for git, session manager, jira, docker, and github, with functional
  options for IO/env/logger. Tests swap in mocks without monkey-patching. The
  `Remuda` struct is a facade, not a god object — subpackages (`session`, `git`,
  `github`, `jira`, `docker`, `configfile`, `prompts`, `logging`) own their
  domains.
- **Kong integration.** Config discovery → resolver chain → parse → context
  binding (cmd/remuda/cli/app.go) is well-structured, and Kong's `Validate()`
  hook is used for genuine parse-time validation in most commands
  (e.g. session_kill.go, vibe.go flag-conflict checks).
- **Config schema validation** (internal/configfile/v1.go) is strict and
  thoughtful: unknown-key rejection via `KnownFields`, enum validation,
  case-insensitive per-repo key normalization with duplicate detection, and
  polymorphic `container: true | {image: ...}` unmarshalling.
- **Locking is correct.** `repo_lock_unix.go` uses `flock` with proper
  defer/`errors.Join` release; the clone-path TOCTOU between `os.Stat` and
  clone/pull is neutralized because the mutation lock spans the whole operation.
- **HTTP hygiene** in `internal/jira/http_client.go`: context timeouts, body
  cleanup, status→error mapping with graceful fallback parsing.

### Finding A1 — Config precedence logic lives in the CLI layer and is scattered (high)

Intended precedence is flags > env > config > defaults, but the implementation
is spread across at least five files: the Kong resolver chain setup and the
manual flag→config switch in `cmd/remuda/cli/config_resolver.go:92-233`
**[verified]**, per-repo/profile overlay application
(`config_resolver.go:261-314`, `mergeOverlayV1IntoConfig` at 742-837), duplicate
flag/env/config checks in `session_resume_defaults.go`, repo-source detection in
`repo_resolution.go:102-129`, and `repos.base_dir` handling in `app.go:58-89`.

Adding one configurable flag currently requires touching the Kong struct tag,
the `Resolve()` switch, and potentially the overlay/defaults files — with
nothing (no test, no exhaustiveness check) catching a missed site. This is also
a layering violation by the repo's own AGENTS.md: overlay merging and repo
inference are business logic, not porcelain.

*Recommendation:* extract an `internal/config` (or grow `internal/configfile`)
package that owns merge + precedence, and drive the flag→config mapping from a
single table (flag name → config getter) instead of a hand-written switch.

### Finding A2 — Shell-string composition is the core mechanism, with 4 quoting helpers (high)

Launch commands are built by string concatenation:
`fmt.Sprintf("cd %s && %s %s", ...)`, `"export BD_ACTOR=%s; %s"`, docker options
as strings like `"-e BD_ACTOR='...'"` and `fmt.Sprintf("-v %q:...", path)`
(internal/vibe.go:231-252, 330, 418-426) **[verified]**. There are four
independent single-quote helpers: `internal/util/shell/quote.go:11` (canonical),
`internal/vibe.go:374`, `internal/docker/run.go:138`, and
`internal/session_edit.go:44` **[verified]**.

Two concrete hazards inside this pattern:
- `%q` is Go quoting, not shell quoting — it happens to work for paths without
  backslashes/double-quote-adjacent characters, but it is the wrong escaping
  function for a `bash -lc` string and will misbehave on exotic paths.
- `fmt.Sprintf("unset %s; ...", name)` / `"export %s=..."` in vibe.go:245-248
  interpolates env-var names with no validation at the usage site (validation
  exists upstream in config parsing, but nothing enforces that every caller
  went through it).

Some of this is irreducible — tmux/zellij and `docker run` inside a multiplexer
pane genuinely require a shell string at the boundary. But the *assembly* can be
done as `[]string` argv plus a single, tested `shell.Join`/`SingleQuote` at the
final boundary.

*Recommendation:* delete the three duplicate quote helpers, build docker options
as argv slices rather than pre-joined strings, and quote exactly once at the
session-start boundary.

### Finding A3 — Fat commands: resolution/inference logic in `cmd/remuda/cli` (medium)

Beyond config: `invocation_analysis.go:121-205` infers repo slug from raw args +
env + workspace state; `repo_resolution.go:38-94` implements the repo fallback
chain with FTUE hooks; `vibe_check.go:67-185` builds the review prompt and PR
context before calling into `internal`. Simple commands (session_attach,
session_list, prompts) follow the intended thin-wrapper pattern, so the
convention exists — these files just predate or outgrew it. Moving the
inference/resolution into `internal/` would also make it e2e-testable without
going through the full CLI.

### Finding A4 — `Vibe()` does too much in one pass with no rollback (medium)

`internal/vibe.go:88-262` resolves the agent, assembles the prompt, clones the
workspace, composes the launch command, mutates env, and starts the session. If
`Clone` succeeds but `startSessionWithEnv` fails (vibe.go:253), the cloned
workspace is orphaned on disk with nothing to reclaim it (session_prune won't
see a session for it). Repeated failed launches accumulate workspaces.

*Recommendation:* split prompt assembly / workspace acquisition / launch into
helpers, and either clean up the fresh clone on launch failure or print the
orphan path so the user can.

## Tactical findings

| # | Sev | Location | Issue |
|---|-----|----------|-------|
| T1 | high | cmd/remuda/cli/config_resolver.go:176-219 **[verified]** | Slice config values (`use`, `no_use`, `experiments`, `container.opts`, `container.inherit_env`) are re-encoded for Kong as comma-joined strings. Any element containing a comma — plausible for a docker opt like `--mount type=bind,src=...` — is silently split into multiple values. |
| T2 | medium | internal/vibe.go:268-273 **[verified]** | `checkModelSupported` has the identical `if model == agentlauncher.ModelAgentDefault { return }` block twice — copy-paste artifact; either dead code or a missing second condition. |
| T3 | medium | internal/vibe.go:400-446 **[verified]** | `codexDockerVolumeMountOptions` writes `auth.json` containing `OPENAI_API_KEY` into an `os.MkdirTemp` dir that is never removed — the key persists in /tmp after the session ends. The redundant `os.MkdirAll(tmpDir)` right after `MkdirTemp` has its error ignored. The function is also 6 levels deep and tracks four `*Applied` booleans by hand; it wants to be a loop over (mount, condition) pairs. |
| T4 | medium | internal/vibe.go:142,253 | Orphaned workspace on launch failure (see A4). |
| T5 | medium | repo-wide (30 files) **[verified]** | Error wrapping uses the archived `github.com/pkg/errors`, mixed with stdlib `fmt.Errorf` in the CLI layer. Pick one: stdlib `%w` does everything used here. |
| T6 | medium | internal/ (3 files use ctx) **[verified]** | `context.Context` reaches `Vibe()` but is used only for logger extraction; all subprocess execution goes through `util.CmdWithLogger` → `exec.Command`, never `exec.CommandContext`. Ctrl-C/timeout cancellation can't propagate to child git/docker/gh processes. |
| T7 | low | internal/vibe.go:593,600-602 **[verified]** | `logLaunchingAgent` sets the `model` field twice on the same zerolog event (unconditionally at 593, again at 600), producing a duplicate key in JSON output. |
| T8 | low | internal/vibe.go:152-155,316-319 | `filepath.Abs` errors swallowed with silent fallback to the relative path; fine in practice, but it masks a broken CWD. |
| T9 | low | cmd/remuda/cli/completions.go:24-27 | Package-global completion cache (mutex + map) with no invalidation; harmless for a short-lived CLI but it's the only mutable package global in the layer. |
| T10 | low | cmd/remuda/cli/completions.go:222-270 | `defaultAgentFromConfig`/`defaultModelFromConfig` re-derive defaults duplicating session_resume_defaults.go logic. |
| T11 | nit | internal/vibe.go vs session_shell.go | Agent-specific behavior keyed on scattered `strings.EqualFold(cmd.Agent, "claude"/"codex"/"bash")` checks in `composeLaunchCommand` (vibe.go:336-353); these belong as capabilities on the `AgentLauncher` interface so adding an agent is one file, not a grep. |

## Testing & CI

The test story is the strongest part of the repo.

- **Hermeticity is exemplary.** `e2e/testutils/harness.go` isolates HOME, XDG,
  locale, TZ, and git config (`GIT_CONFIG_NOSYSTEM`, redirected global/system
  config) per test; ~309 `t.TempDir`/`t.Cleanup` uses; mocks for session
  manager, GitHub, Slack, Docker, Jira; e2e tests drive the real `cli.Run`
  entrypoint rather than calling internals.
- **Parallel and fast.** 272 `t.Parallel()` uses, one intentional 100ms sleep in
  the whole suite, full run < 10s. Concurrency is actually tested
  (8-goroutine concurrent clone exercising the repo lock).
- **CI** runs `go test -race` and golangci-lint pinned at v2.11.4 with a
  near-`default: all` linter config — strict by Go-community standards.

Gaps, in order:

1. **Platform code is untested and CI is ubuntu-only.** `repo_lock_unix.go` and
   `repo_lock_other.go` have no tests; macOS/Windows paths are never exercised.
   If non-Linux is a supported target, add an OS matrix; if not, say so in the
   README and delete `repo_lock_other.go` speculation.
2. **Mock fidelity is thin** for GitHub/Slack/Jira (e.g. `MockGitHub` barely
   implements PRView/issue fetching), so the context-engineering paths
   (`--jira`, `--github-issue`, Slack threads) are weakly covered relative to
   their complexity.
3. `internal/session_shell_test.go:19-39` has four commented-out tests —
   either restore or delete them; commented tests rot.
4. No coverage reporting in CI. Not urgent given the e2e style, but a trend
   line is cheap.
5. **Dev-environment friction:** the lint config requires golangci-lint
   v2.11.4+ (it references linters that don't exist in older releases), so a
   contributor with the widely-installed 2.4.x can't run lint locally at all —
   it errors on config parse. Worth a `make lint` that downloads the pinned
   version (or `go run` shim) so local == CI.

## Dependencies

Direct deps are lean and appropriate (kong, huh, zerolog, testify, yaml.v3).
Two notes: `github.com/pkg/errors` is archived (see T5), and
`github.com/openai/openai-go/v3` is a direct dependency pulled in for one
feature (`cli/llm.go` name suggestions) — fine, but worth knowing it's the
single largest dep brought in by the smallest feature.

## Prioritized recommendations

1. **Fix the small bugs now** (one PR): T1 comma-splitting, T2 duplicate
   condition, T3 temp-dir/API-key cleanup, T7 duplicate log field.
2. **Consolidate shell quoting** (A2): one helper in `util/shell`, argv-slice
   docker opts, quote once at the boundary.
3. **Extract config precedence into `internal/`** (A1): table-driven
   flag→config mapping; this removes the biggest add-a-flag hazard.
4. **Clean up `Vibe()`** (A4/T4): extract helpers, handle the orphaned-clone
   path.
5. **Decide on platform support** and either add an OS matrix or prune the
   dead platform stubs; restore or delete the commented session_shell tests.
6. **Migrate pkg/errors → stdlib** and thread `exec.CommandContext` through
   `util.CmdWithLogger` (mechanical, low risk, do opportunistically).
