# Container Mode

Use `--container` to run `vibe` inside a Docker container, mounting the newly
created workspace at a unique path under `/workspaces/` (derived from the host
workspace folder name, with a short hash for uniqueness).

When combined with `--yolo`, the agent is empowered to do more without human
supervision while keeping the host system relatively safe.

## Options

- Set the container image with `--container-name <image>`.
- Remuda requires an explicit image when container mode is enabled. If no image
  is provided from `--container-name` or config (`defaults/profiles/per_repo`),
  the command exits with an error.
- Pass extra docker args via `--container-opt` (repeatable).
- Forward additional env vars via `--container-inherit-env` (repeatable).

## Image Setup

Use a published image (or your own image build pipeline) and pass it with
`--container-name`.

```bash
# Pull or publish the image you want to run
docker pull ghcr.io/acme/vibe-dev:latest

# Pass the image explicitly on each run
remuda vibe --container --container-name ghcr.io/acme/remuda-agent:latest "Implement pagination"
```

## Examples

```bash
# Run Codex inside your chosen image
remuda vibe --container --container-name ghcr.io/acme/remuda-agent:latest "Implement pagination for transactions"

# Specify an image and pass extra docker options
remuda vibe --container \
  --container-name ghcr.io/acme/remuda-agent:latest \
  --container-opt "--gpus all" \
  --name feature/validation \
  "Add structured validation errors"

# Run inline without tmux (still in the container)
remuda vibe --container --no-detached "Investigate flaky tests"
```

## Technical Notes

- The workspace is bind-mounted read/write at a unique path under `/workspaces/` and used as the working directory via `-w <that-path>`.
- For linked worktrees, the shared repository cache (`.repo_cache`) is bind-mounted at its real host path so the worktree's `.git` pointer resolves inside the container. This holds for `--tmp` sessions too: the worktree lives under the OS temp dir while its cache is mounted from `REMUDA_REPOS_BASE_DIR` (under `$HOME`).
- With `--tmp --container` on macOS, the OS temp dir (`/var/folders/...`) is not shared with Docker Desktop by default, so Remuda fails early with guidance. Share the path under Docker Desktop → Settings → Resources → File Sharing, or set `REMUDA_TMP_DIR` to a directory under `$HOME`.
- `OPENAI_API_KEY`, `GH_TOKEN`, and `GITHUB_TOKEN` are forwarded so agents can reach LLM and GitHub APIs. When the tokens are missing Remuda will call `gh auth token` before launch; pre-setting them skips that lookup.
- For Claude runs (`--agent claude`), `ANTHROPIC_API_KEY` is forwarded into the container.
- `--container-inherit-env <NAME>` forwards additional host environment variables into the container via `docker run -e <NAME>`.
  Use this for opt-in forwarding of sensitive values such as `GOPRIVATE` (for example: `--container-inherit-env GOPRIVATE`).
- If an SSH agent is available (`SSH_AUTH_SOCK` on Linux, Docker Desktop's agent on macOS), it is forwarded read-only along with `~/.ssh` and `~/.config/gh` to support git+ssh and `gh` operations inside the container.
- For Claude runs (`--agent claude`), host `~/.claude` and `~/.claude.json` are mounted read/write when present so Claude OAuth/session state can be reused.
- Custom Codex prompts (`~/.codex/prompts`) are mounted read-only at `/root/.codex/prompts` so container sessions can use your slash commands.
- Codex rules (`~/.codex/rules`) are mounted read-only at `/root/.codex/rules`.
- Codex skills (`~/.codex/skills`) are mounted read-only at `/root/.codex/skills`.
- Codex state (`~/.codex/history.jsonl` and `~/.codex/sessions`) is mounted read/write at `/root/.codex` so Codex can persist history and sessions across container runs.
- No privileged flags or Docker socket mounts are used by default.
