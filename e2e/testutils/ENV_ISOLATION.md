# e2e environment isolation contract

These tests run in-process (they call `cli.Run` directly), so any host environment
variables or host config files can accidentally affect behavior and make local
runs flaky.

This directory defines a baseline contract for e2e that we can apply consistently
in:

- `e2e/main_test.go` (suite-level, once per package)
- helper functions that spawn subprocesses (`git`, `gh`, etc) via `exec.Cmd.Env`
- the e2e harness (per-test env maps passed into CLI contexts)

## Contract (baseline)

### Cleared by default

- All `REMUDA_*` and `VIBE_*` env vars (tests opt in explicitly via the harness
  env map, e.g. `h.SetEnv(...)`).

### Forced by default

- User dirs: `HOME` + `XDG_*` are forced to a temp directory so config discovery
  is deterministic.
- Locale/TZ: `TZ=UTC`, and `LANG`/`LC_ALL` forced to a UTF-8 "C" locale when
  available (eg `C.UTF-8`/`C.utf8`), else `C`, to stabilize output/timestamps.
- Git config isolation:
  - Clear all `GIT_CONFIG_*` and `GIT_ATTR_*` inherited vars.
  - Force:
    - `GIT_CONFIG_NOSYSTEM=1`
    - `GIT_CONFIG_GLOBAL=$DEVNULL` (uses `os.DevNull`)
    - `GIT_CONFIG_SYSTEM=$DEVNULL` (uses `os.DevNull`)
    - `GIT_ATTR_NOSYSTEM=1`

### Allowlisted for subprocesses

When constructing `exec.Cmd.Env`, carry through only a small set from the parent
process and then apply the forced vars above:

- `PATH`, `TERM`, `TMPDIR`, `TMP`, `TEMP`

If a specific test needs more, it should opt in by passing explicit overrides
when creating the subprocess env (or by expanding the contract deliberately).

Common opt-ins (keep explicit in tests/harness helpers):

- SSH: `SSH_AUTH_SOCK` (git over ssh)
- Proxies: `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`

Windows note: the default subprocess allowlist is slightly larger on Windows
(`SystemRoot`, `ComSpec`, `WINDIR`, `PATHEXT`) to help external tools start.

## Implementation

See `e2e/testutils/env.go` for `DefaultE2EEnvIsolationContract` and
`E2EEnvIsolationContract.SanitizeProcessEnv` / `E2EEnvIsolationContract.SanitizeSubprocessEnv`.

`ApplyE2EEnvIsolationToCmd` now takes an explicit base env map so tests can
control the subprocess environment without mutating the process env.
