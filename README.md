# Remuda – agent orchestration tool

**Remuda** is a command line application that makes many agentic programming
workflows easier:

- create disposable working copies of Git repositories
- launch agents in them
- containerize your agents for safer unattended workflows
- reuse prompt snippets and inject context from tools like Jira and Slack
- broadcast messages to swarms of agents

## Why Remuda?

I would frequently hear coworkers voice some problem or feature idea and think
"(current frontier model) could oneshot that", but then I knew I'd have to clone
the repo, assemble the prompt, configure all the flags, manage the terminal
window, etc. There was enough activation energy still there that I just wouldn't
go through with it most of the time.

Most agentic coding tools take care of managing single agents with the human in
the loop, but not a lot of them expose a good interface for managing lots of
agents working on unrelated tasks. The ones that do are often ecosystem locked
to only one agent harness, despite there being at this point few meaningful
differences between how they fundamentally work.

I wanted one that could do all that and fit with my own tooling preferences:

- session management with a terminal multiplexer (e.g. tmux)
- little to no config required
- strong integration with my other CLI tools
- portable binary

I believe coding agents are strongest when managed asynchronously. For the busy
developer, they are a powerful weapon against codebase rot, allowing you to
delegate smaller tasks while you focus on big ones. Remuda attempts to make this
workflow as frictionless as possible.

Remuda was the answer. I started with the goal of being able to kick something
off with just `vibe "please do this thing I don't have time for"` and it grew
from there.

## Installation

Requirements:

- Required to build/install (via `go install`):
  - Go 1.24+ (any platform supported by the Go toolchain)
  - Git
- Strongly recommended:
  - `tmux` for the default detached-session flow (`zellij` is a supported alternative when configured)
- Optional (feature-dependent):
  - [fzf](https://github.com/junegunn/fzf) for interactive selection flows
  - Agent harness (at least one is required to actually run an agent). Currently supported:
    - [Codex](https://github.com/openai/codex)
    - [OpenCode](https://github.com/anomalyco/opencode)
    - [Claude Code](https://code.claude.com/docs/en/overview)
    - You can also provide a custom agent command

### Build from source

```bash
go install github.com/yendo-eng/remuda/cmd/remuda@latest

# Verify installation
remuda --help
```

---

## Quick Start

By default, remuda stores workspaces under `~/.remuda/repos`. You can override
the root via the `REMUDA_REPOS_BASE_DIR` environment variable.

**Tip — Interactive wizard:** Add `--wizard` to most commands for a guided, TUI-based flow.

Here's the most generally useful workflow for feature development:

```bash
# Use a published image for container sessions
IMAGE=ghcr.io/acme/vibe-dev:latest

remuda vibe \
  --name '<your-session-name-goes-here>' \
  --container \
  --container-name "$IMAGE" \
  --yolo \
  --use make-pr \
  --use small-commits \
  "please build the feature"
```

---

## Documentation

For detailed usage information, see:

- [Commands](docs/commands.md) – vibe, clone, vibe-check
- [Session Management](docs/session-management.md) – managing tmux sessions
- [Container Mode](docs/container-mode.md) – running agents in Docker
- [Configuration](docs/configuration.md) – environment variables and config file

### Session Merge Flags

`remuda session kill --merge` uses `gh pr merge` under the hood.

- Configure default merge flags in config: `defaults.merge.gh_flags`
- Override per run with repeatable `--merge-flag` (replaces config list)
- If neither is set, Remuda keeps the current default behavior: `--rebase`

---

## License

Remuda is licensed under the [MIT License](LICENSE).
