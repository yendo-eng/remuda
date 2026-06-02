# Design: decouple env/logger from process globals

## Summary

Remuda currently relies on process-global environment variables and a global
zerolog logger. This makes tests harder to parallelize and creates hidden
coupling between CLI parsing, config resolution, and runtime behavior.

This design proposes introducing an explicit runtime context (env, logger, IO,
cwd, clock) that is constructed at process entrypoints and passed through the
CLI and internal layers. The end state eliminates `os.Getenv`, `os.Setenv`,
`os.Chdir`, and `zerolog/log` global usage outside of a small adapter layer.

## Decision / status

- We are pursuing in-package parallelism by removing process-global env/cwd
  dependencies (Option C). This is now the primary plan for e2e
  parallelization.

### Why not split/shard e2e packages?

- CI sharding (Option A) and package splitting (Option B) were de-scoped to
  avoid test movement and duplicated orchestration; the effort is now focused
  on making the existing `e2e` package safe for `t.Parallel()`.

## Goals

- Make env, logger, cwd, and IO explicit dependencies.
- Ensure CLI parsing and command execution use the injected env/logger.
- Enable e2e tests to run safely in parallel by avoiding process-global state.
- Preserve CLI behavior and config precedence (flags > env > config > defaults).

## Non-goals

- Large-scale behavior changes to Remuda commands.
- Full replacement of `zerolog` or logging format changes.
- Immediate refactor of every helper to be fully pure (phased migration is ok).

## Current state (selected pain points)

- `cmd/remuda/cli/app.go` initializes a global zerolog logger.
- Many internal packages call `zerolog/log` directly.
- CLI and internal code read environment via `os.Getenv` / `os.LookupEnv`.
- e2e tests frequently use `t.Setenv` and occasionally `t.Chdir`.
- `cli.Run(...)` is executed in-process and reads process-wide env and cwd.

## Proposed architecture

### 1) Introduce an explicit runtime/context container

Add an `internal/runtime` (or `internal/appctx`) package containing:

```go
type Env interface {
    Lookup(key string) (string, bool)
    Get(key string) string
    Environ() []string
    Set(key, value string) error
    Unset(key string) error
}

type FS interface {
    Getwd() (string, error)
    // No Chdir in production runtime; avoid process-wide cwd mutation.
}

type Clock interface {
    Now() time.Time
}

type Runtime struct {
    Env    Env
    Logger zerolog.Logger
    IO     internal.IO
    FS     FS
    Clock  Clock
}
```

Production implementations:
- `Env`: OS-backed implementation (`os.LookupEnv`, `os.Setenv`, etc).
- `FS`: OS-backed implementation (`os.Getwd` only).
- `Clock`: time.Now wrapper.
- `Logger`: constructed once at entrypoint, passed by value.

Test implementations:
- `Env`: map-backed, isolated per test.
- `FS`: in-memory or temp-dir backed (only if needed by tests).
- `Clock`: fixed-time or fake clock for determinism.

### 2) Thread runtime through CLI and internal layers

#### CLI entrypoint

- `cmd/remuda/main.go` creates a `Runtime` and passes it into the CLI runner.
- The CLI runner binds the runtime into Kong so command `Run(...)` methods can
  accept it directly.

#### internal.Remuda

- Extend `internal.NewRemuda` with `WithRuntime(...)` or explicit setters for
  Env/Logger/FS/Clock.
- Ensure all internal components use the injected runtime and avoid package
  globals.

### 3) Logging strategy

Stop using the global `zerolog/log` logger.

- Replace `log.X().Msg(...)` calls with a `Logger` field on structs or functions
  that already take a `*internal.Remuda` or `Runtime`.
- For helpers that are currently free functions, add a `Logger` parameter or
  turn them into methods on a struct that has `Logger`.

### 4) Env + subprocess strategy

Stop reading or mutating process env directly in core logic, and ensure
subprocesses inherit the injected env (not the process env).

- Replace `os.Getenv` / `os.LookupEnv` with `runtime.Env`.
- Replace `os.Setenv` in helper functions with `runtime.Env.Set(...)`.
- Provide a standard `runtime.Environ()` (or `runtime.Env.Environ()`) and require
  all subprocess spawns to set `cmd.Env` explicitly.
- For environment-based config resolution, switch to a Kong resolver based on
  the injected env rather than process env.

### 5) Kong integration

Kong supports:
- A `Resolver(...)` option for default values from external sources.
- `Bind(...)` and `Context.Run(...)` bindings for injecting values into `Run`
  methods.

We can implement an env-backed resolver that reads from `Runtime.Env` and
provide the `Runtime` (or just `Env` + `Logger`) via `kong.Bind(...)`.

Example sketch:

```go
resolver := newEnvResolver(runtime.Env)
k, err := kong.New(&cli, kong.Resolver(resolver), kong.Bind(runtime))
```

This keeps current precedence while ensuring no process-global env is read.

### 6) Config discovery + path expansion

Config discovery and `~` expansion should use the runtime env/FS, not
`os.UserHomeDir` or process env:

- Replace `REMUDA_CONFIG`, `XDG_CONFIG_HOME`, `HOME`, `VIBE_REPOS_BASE_DIR` lookups
  with `runtime.Env`.
- Replace `os.UserHomeDir` usage with a `runtime.HomeDir()` helper that consults
  runtime env (and falls back to OS only in the outermost entrypoint).
- Ensure config discovery code accepts a `Runtime` or `Env` dependency.

### 7) Working directory strategy

Avoid `os.Chdir` in production logic. Instead:

- Keep cwd on the runtime (`FS.Getwd`).
- Use explicit working directories when spawning subprocesses.
- Where `t.Chdir` is required for tests, replace with explicit `RunIn(dir)` or
  use a test-only fake FS. Avoid `Chdir` in prod runtime entirely.

## Migration plan (phased)

### Phase 1: Introduce runtime + adapters (no behavior change)

- Add `internal/runtime` package and OS-backed implementations.
- Plumb runtime through CLI entrypoints and `internal.NewRemuda`.
- Add `WithRuntime`/`WithLogger`/`WithEnv` options.
- Add Kong resolver that reads from runtime env.

### Phase 2: Migrate high-level CLI/internal usage

- Replace `zerolog/log` usage in core CLI and `internal` packages.
- Replace `os.Getenv`/`os.LookupEnv` in core path with runtime env.
- Stop writing to process env from inside internal code.
- Ensure all subprocess spawns set `cmd.Env` from runtime env.
- Migrate config discovery and `~` expansion to runtime env/FS.

### Phase 3: Update tests + e2e harness

- Update e2e harness to construct a `Runtime` with isolated env/logger.
- Replace `t.Setenv` and `t.Chdir` usage where feasible.
- Enable package-level parallelism once env/cwd isolation is complete.

### Phase 4: Cleanup + enforcement

- Add lint or build tags to prevent new `os.Getenv` usage in core packages.
- Add a test that fails if `zerolog/log` global is referenced outside entrypoints.

## Testing impact

- e2e tests should create a runtime with an isolated env map.
- Unit tests can mock env/logger without mutating global state.
- Existing behavior should remain the same by using OS-backed runtime in
  production.

## Risks and tradeoffs

- Touching CLI parsing and config precedence is risky; keep a careful test
  matrix.
- Some helper functions may need to change signatures to accept runtime.
- Ensuring no hidden dependency on process env requires careful auditing.

## Open questions

- Should runtime be a first-class dependency in `internal.Remuda` or passed per
  command?
- Do we want to standardize on a small `Logger` interface instead of passing
  zerolog directly?
- Should we add an explicit `Context` struct for `Run(...)` methods?
