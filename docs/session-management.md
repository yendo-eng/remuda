# Session Management

Remuda launches agents inside tmux sessions named `org/repo/<folder>`. Use the
`session` commands to inspect, attach, tail logs, and clean up sessions and their
workspaces.

`fzf` is required for interactive session picking workflows.
`session reap` targets active sessions by age; `--older-than` uses Go-style
durations (e.g. `72h`, `336h`).

## Examples

```bash
# List active Remuda sessions (filtered by naming convention)
remuda session list

# Emit JSON instead of plain text
remuda session list --json

# Attach to a session by name or pick interactively
remuda session attach acme-org/example-repo/feature-login-audit
remuda session attach --pick

# Print recent pane output (like tail -n)
remuda session readbuf acme-org/example-repo/feature-login-audit -n 500
remuda session readbuf --pick -n 200
# Print the full pane history
remuda session readbuf acme-org/example-repo/feature-login-audit -n 0

# Send a prompt to one or more sessions (use --pick to multi-select)
remuda session send acme-org/example-repo/feature-login-audit "Summarize what you've done so far"
remuda session send --pick "Continue with the next step"
remuda session send --name acme-org/example-repo/feature-login-audit --name acme-org/example-repo/feature-cache-indexing "Sync up on next steps"
remuda session send --no-newline acme-org/example-repo/feature-login-audit "export DEBUG=1"

# Open the workspace for a session in your preferred editor
remuda session edit acme-org/example-repo/feature-login-audit
remuda session edit --pick                    # fzf flow; requires $REMUDA_EDITOR/$VISUAL/$EDITOR

# Kill one or multiple sessions
remuda session kill --name acme-org/example-repo/feature-login-audit
remuda session kill --pick                # multi-select via fzf
# optionally also remove the associated workspace
remuda session kill --cleanup --name acme-org/example-repo/feature-login-audit
# optionally close the associated PR, if there is one
remuda session kill --close-pr --name acme-org/example-repo/feature-login-audit
# optionally close the PR with a closing comment
remuda session kill --close-pr="closing this PR from remuda" --name acme-org/example-repo/feature-login-audit
# optionally close the beads issue associated with the session branch
remuda session kill --close-bd --name acme-org/example-repo/feature-login-audit
# merge the associated PR before killing the session
remuda session kill --merge --name acme-org/example-repo/feature-login-audit
# override merge flags for this run (replaces config defaults)
remuda session kill --merge --merge-flag=--squash --merge-flag=--delete-branch --name acme-org/example-repo/feature-login-audit

# List all Remuda-managed workspaces (active + inactive)
remuda workspaces list

# List active workspaces (with active sessions); one path per line for scripting
remuda workspaces list --active

# List inactive workspaces (no active session); one path per line for scripting
remuda workspaces list --inactive

# Resume the most recent session in an inactive workspace
remuda session resume ~/.remuda/repos/acme-org/example-repo/feature-login-audit
remuda session resume --pick
remuda session resume --yolo ~/.remuda/repos/acme-org/example-repo/feature-login-audit
remuda session resume --agent claude --model claude-sonnet-4.6 ~/.remuda/repos/acme-org/example-repo/feature-login-audit "Continue from the last checkpoint and add tests"
remuda session resume --agent-cmd "codex resume --last" ~/.remuda/repos/acme-org/example-repo/feature-login-audit "Continue"

# Reap active sessions older than a threshold (dry-run by default)
remuda session reap --older-than 336h
remuda session reap --older-than 336h --dry-run=false
# optionally also remove the associated workspace
remuda session reap --older-than 336h --dry-run=false --cleanup
# interactively pick from eligible sessions
remuda session reap --older-than 336h --pick
```

`remuda workspaces list`, `remuda workspaces list --active`, and
`remuda workspaces list --inactive` all respect `workspaces.ignore` patterns
from config. `--active` and `--inactive` are mutually exclusive.

`remuda session resume` supports the same post-clone launch flags as `remuda vibe`:

- `--agent`, `--model`, `--reasoning-level`, `--agent-cmd`
- `--use`, `--no-use`, `--jira`, `--github-issue` (alias: `--gh-issue`)
- `--openai-api-key`
- `--profile`, `--yolo`, `--[no-]detached`, `--attach`, and container flags
- an optional trailing prompt argument

Resume is intentionally stateless. Remuda does not persist or detect the agent
that originally created a workspace session history, so you must resume with the
correct agent yourself.

Default behavior:

- Codex is used by default (`codex resume --last`).
- If `REMUDA_AGENT=claude` or profile/config defaults resolve to `claude`,
  Remuda resumes with `claude --continue` (directory-scoped latest conversation).
- Remuda intentionally does not use `claude --resume` because that command opens
  an interactive picker.
- `--agent opencode` and `--agent bash` currently return a clear "resume unsupported"
  error unless you provide a custom `--agent-cmd`.

Clone-creation flags are not available on `session resume`:
`--repo`, `--repo-url`, `--full-clone`, `--no-clone-hooks`, and wizard repo
selection remain `vibe`/`clone` concerns.

For `session kill --merge`, merge flags are selected with this precedence:
1. CLI `--merge-flag` values (repeatable; replaces config list)
2. `defaults.merge.gh_flags` in config
3. built-in default `--rebase`

## Resume Ignore Patterns

`remuda session resume --pick` skips any workspace matching `session.prune.ignore`
in the config file, so those workspaces are not offered as candidates. Patterns
match `org/repo/workspace` relative to the repos base dir, using Go `path.Match`
rules (`*` and `?` do not cross `/` boundaries).

Example:

```yaml
session:
  prune:
    ignore:
      - "acme-org/example-repo/keep-*"
      - "*/utils/archive-*"
```

## Shell Completions

Remuda generates shell completion scripts via cobra.

- Generate a script: `remuda completions <bash|zsh|fish|powershell>`
- Load it from your shell profile, e.g. `source <(remuda completions bash)` or `remuda completions fish | source`.
