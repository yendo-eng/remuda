# `remuda vibe --container` – Product Requirements

## 1. Purpose

Allow developers to run AI coding sessions inside a Docker container while keeping the cloned workspace on the host. This provides a reproducible, dependency‑isolated runtime (with Codex or OpenCode CLIs available) without changing the existing workflow of cloning to `~/.remuda/repos/<org>/<repo>/<folder>`.

## 2. Background

Today, `remuda vibe` launches the chosen agent on the host (optionally detached via tmux). Remuda supports launching the session inside a container, and teams provide the image via their own build/publish flow or an existing registry image. Teams want:

- Consistent agent runtime across machines.
- Minimal host dependencies beyond Docker.
- A clean mapping of the newly created workspace into the container.

We avoid Docker-in-Docker. The host Docker daemon runs the container; the workspace is bind‑mounted.

## 3. Goals and Non‑Goals

### Goals

- Run `remuda vibe` sessions inside a specified container image with a single flag.
- Ensure the newly created workspace is bind‑mounted and set as the working directory in the container.
- Pass required environment (e.g., `OPENAI_API_KEY`) into the container.
- Preserve current UX: detached by default, `--no-detached` supported.
- Keep an escape hatch for power users to pass extra `docker run` options.

### Non‑Goals (v1)

- Managing Docker image builds or publishing images.
- Reusing/rendezvousing with existing running containers; v1 creates ephemeral containers.
- Full `docker compose` support.
- Mounting the Docker socket by default (opt‑in only).

## 4. User Experience

### CLI

```
remuda vibe --container --name feature/pagination "Implement pagination for transactions"

# Equivalent with more control
remuda vibe \
  --container \
  --container-name vibe-dev \
  --container-opt "--gpus all" \
  --name feature/pagination \
  "Add pagination API + tests"
```

### Behavior

1. `remuda` clones the workspace as it does today and prints the path.
2. Instead of executing the agent command directly, it runs `docker run` with:
   - `--rm -it` for ephemeral, interactive sessions.
   - `-v <absolute-workspace>:/workspace` bind mount (default target `/workspace`).
   - When the workspace is a Git worktree (default for `remuda clone`), also bind‑mount the cache repo directory `<baseDir>/.repo_cache` to the identical absolute path inside the container so the worktree’s `.git` indirection remains valid.
   - `-w /workspace` as working directory.
   - `-e OPENAI_API_KEY` forwarded from host when set.
   - Optional env passthrough from repeated `--container-inherit-env` (each becomes `-e <NAME>`).
   - Optional additional flags from repeated `--container-opt`.
   - Image name from `--container-name`.
   - Entrypoint `bash -lc "<agent command>"` so shell features work.
3. In detached mode, the configured session manager runs the `docker run ...` command. With `--no-detached`, it runs in the current terminal.

## 5. Functional Requirements

### FR‑1 New flags on `vibe`

- `--container`: Enable containerized session (uses default image `vibe-dev`).
- `--container-name <image>`: Override the default image name.
- `--container-opt <arg>` (repeatable): Append raw additional arguments to `docker run` (e.g., `--gpus all`, `--network host`).
- `--container-inherit-env <name>` (repeatable): Forward host env var into the container (adds `docker run -e <name>`).

Notes:
- Relevant flags `--agent`, `--model`, `--agent-cmd`, `--no-detached`, `--jira`, `--name`, and `--repo-url` continue to work. Suffix/branch flags have been removed in favor of unified `--name`.

### FR‑2 Command composition

Given a workspace path `/abs/.remuda/repos/acme-org/example-repo/feature-login-audit`, constructed command:

```
docker run --rm -it \
  -v "/abs/.remuda/repos/acme-org/example-repo/feature-login-audit:/workspace" \
  -w /workspace \
  -e OPENAI_API_KEY \
  -e AWS_REGION \
  [extra args from --container-opt] \
  <image> \
  bash -lc "<agent command built from existing logic>"
```

Quoting rules: the agent command is wrapped in `bash -lc "..."` and must be properly escaped. Workspace paths with spaces are supported by quoting the `-v` argument as a single value.

### FR‑3 Environment handling

- Forward `OPENAI_API_KEY` when it is already set in the host environment.
- Allow additional env forwarding via repeated `--container-inherit-env <NAME>` (each becomes `-e <NAME>`).

### FR‑4 Tmux integration

- Detached default: the configured session manager runs the full `docker run ...` command.
- `--no-detached`: run `docker run ...` inline.
- Session naming: default is `org/repo/<folder>` (no absolute paths).

### FR‑5 Failure modes

- If Docker is not available or the image cannot be pulled, print a concise error and exit non‑zero.
- If path bind‑mount fails (e.g., missing workspace), abort with explanation.

## 6. Non‑Functional Requirements

- Zero additional latency beyond Docker startup; cloning speed remains unchanged.
- Works on macOS and Linux with Docker Desktop / Docker Engine.
- Use Docker’s default container user (typically root) in v1.
- No privileged containers by default.

## 7. Security Considerations

- No default mounting of sensitive host directories (SSH, Docker socket) unless explicitly requested.
- Image trust is the user’s responsibility; document that `--container` should point to a trusted image.

## 8. Implementation Plan

1. Extend `internal/commands/vibe.go`:
   - Add new flags and config struct fields.
   - Build a `dockerRunCommand(workspace string) string` when `--container` is set; otherwise preserve existing host flow.
   - Quote/escape arguments safely (reuse `escapeSingleQuotes`, add helpers for shell/volume quoting).
   - Append to existing `agentCmd` string: final exec becomes `bash -lc "<agentCmd>"` inside the container.
2. Wire detached/no‑detached branches to use the composed docker command.
3. Tests:
   - Unit tests to validate command composition for typical permutations (envs, sockets, names).
   - No integration tests that require Docker; tests assert built strings only.
4. Docs:
   - Update README with a short “Run in container” section and examples.

## 9. Decisions (resolved)

1. `--container` is a boolean flag that enables containerized execution. Initial launch omitted direct `docker exec`, but we now provide `remuda session shell` for interactive access to the running container.
2. Default image is `vibe-dev`; users can override via `--container-name <image>`.
3. Use Docker’s default user (no `--user` mapping in v1).
4. Keep scope minimal: `--container`, `--container-name`, `--container-opt`, and `--container-inherit-env` are supported (no extra mounts/user flags in v1).
5. No Docker socket mount convenience flag in v1.

## 10. Acceptance Criteria

- `remuda vibe --container [--container-name <image>] "hello"` clones the repo, prints the workspace path, and starts a detached session running a `docker run` command with the workspace mounted at `/workspace` and the agent executing successfully inside the container.
- `--no-detached` runs interactively in the current terminal with the same `docker run` invocation.
- `OPENAI_API_KEY` is available to the agent inside the container when it is already set on the host.
- Command composition unit tests pass without requiring Docker.

## 11. Example Commands

Minimal:

```
remuda vibe --container "Refactor payment validation"  # uses default image vibe-dev
```

With unified name:

```
remuda vibe --container --container-name ghcr.io/acme/vibe-dev:latest \
  --name feature/validation \
  "Add structured validation errors"
```

With extra docker opts and no detached:

```
remuda vibe --container --container-name vibe-dev \
  --container-opt "--gpus all" \
  --no-detached \
  "Investigate flaky tests"
```

Drop into the running container for debugging:

```
remuda session shell acme-org/example-repo/example-repo_feature-login-audit
# or interactively pick:
remuda session shell --pick
```
