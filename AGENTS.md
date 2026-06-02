Read docs/\* for more context about the application.

Please read https://raw.githubusercontent.com/alecthomas/kong/refs/heads/master/README.md
for the full docs on Kong. It is the core framework for CLI argument parsing that we use.

Please use stretchr/testify for test assertions. Use require or assert where it
makes sense.

Do not add tests that assert CLI `--help` text (including Kong struct-tag help
strings). Focus tests on command behavior and other public runtime outcomes.

The base branch is `master`, not `main`.

## general design of remuda's codebase:

It's in a loose Clean Architecture style:

- All porcelain/CLI concerns go in ./cmd/remuda/cli
- Business logic goes in ./internal
- Other dependencies go in subpackages of ./internal

Input validation should be done as close to parse time as possible.

An e2e-heavy testing regime is favored. Unit test only if mocking is not required.
Do not add new unit tests that assert Dockerfile contents; Dockerfile coverage is intentionally being phased out because the Dockerfile will be removed from this repo.

## Issue Tracking with br (beads)

**IMPORTANT**: This project uses **br (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why br?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**
```bash
br ready --json
```

**Create new issues:**
```bash
br create "Issue title" -t bug|feature|task -p 0-4 --json
br create "Issue title" -p 1 --deps discovered-from:bd-123 --json
br create "Subtask" --parent <epic-id> --json  # Hierarchical subtask (gets ID like epic-id.1)
```

**Claim and update:**
```bash
br update km-42 --status in_progress --claim --json
br update km-42 --priority 1 --json
```

**Complete work:**
```bash
br close km-42 --reason "Completed" --json
```

**Add comments:**
```bash
br comments add km-42 --message "Investigated --in parsing issue; fixed in docs"
br comments add km-42 --file /path/to/comment.txt
br comments add km-42 "Investigated --in parsing issue; fixed in docs"
```
Note: Always quote comment text (or use `--file`). Unquoted strings like `--in`
or `--no-foo` can be interpreted by `br` as flags (whether passed positionally
or via `--message`), so wrap them in quotes.

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `br ready` shows unblocked issues
2. **Claim your task**: `br update <id> --status in_progress --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `br create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Request review**: create a new br issue for code review
   - Include the work item ID in the review issue title (example: `Review <work-id>: <branch>`)
   - Include branch name + remote, PR link (if any), related issue IDs, and a brief blurb about the issue under review (eg. its title) in the body
   - Link it to the work item: `br create "Review <work-id>: <branch>" -t task -p 2 --deps discovered-from:<work-id> --json`
6. **Sync branch only**: Do NOT commit `.beads/issues.jsonl` to your feature branch. br is configured to sync issues to a dedicated sync branch.

**Note**: Do not use `review` status. Keep the work issue open/in_progress and file a separate review issue; only use `br close` after the PR has been approved and merged.

### Code Review Workflow

- Add review feedback as comments on the **issue under review** (not the review issue).
- Only the reviewer closes the review issue; implementation authors must not close their own review issues.
- Keep the review issue open while findings remain; only close it after all findings are resolved or there were none.
- If there are no findings, the reviewer closes the review issue (eg. if issue `km-123` is to review `km-abc` and there were no findings, close `km-123`).

### Auto-Sync

br automatically syncs issues to the configured sync branch:
- Keeps `.beads/issues.jsonl` in sync after changes (5s debounce)
- Imports from the sync branch when newer (e.g., after `git pull`)
- No manual commit of `.beads/issues.jsonl` on feature branches

### Remuda worktrees + shared `.beads` store

When Remuda runs agents in containers, the working directory is often a git worktree whose
`.git` points at a shared “host” repo dir (see `.git` containing a `gitdir:` path). In this
setup, `br` is expected to read/write issues from the host-shared Beads store (commonly
`.../.beads_worktree/.beads/issues.jsonl`) so multiple worktrees/containers stay in sync.

Implications:
- The `.beads/issues.jsonl` file in *this* worktree may not be the file `br` is currently
  operating on. That is normal in Remuda-managed worktrees.
- If you need to refresh the repo-tracked mirror for inspection, export from the canonical
  store: `br --no-db export -o .beads/issues.jsonl` (do not commit to feature branches).

### GitHub Copilot Integration

If using GitHub Copilot, also create `.github/copilot-instructions.md` for automatic instruction loading.
Run `br onboard` to get the content, or see step 2 of the onboard instructions.

### MCP Server (Recommended)

If using Claude or MCP-compatible clients, install the beads MCP server:

```bash
pip install beads-mcp
```

Add to MCP config (e.g., `~/.config/claude/config.json`):
```json
{
  "beads": {
    "command": "beads-mcp",
    "args": []
  }
}
```

Then use `mcp__beads__*` functions instead of CLI commands.

### Managing AI-Generated Planning Documents

AI assistants often create ephemeral planning notes during development:
- PLAN.md, IMPLEMENTATION.md, ARCHITECTURE.md
- DESIGN.md, CODEBASE_SUMMARY.md, INTEGRATION_PLAN.md
- TESTING_GUIDE.md, TECHNICAL_DESIGN.md, and similar files

**Best Practice: Use a dedicated directory for these ephemeral files**

**Recommended approach:**
- Create a `history/` directory in the project root
- Store ephemeral AI planning docs in `history/`
- Keep the repository root clean and focused on permanent project files
- Only access `history/` when explicitly asked to review past planning
- Store durable design docs in `docs/designs/` instead of `history/`

**Example .gitignore entry (optional):**
```
# AI planning documents (ephemeral)
history/
```

**Benefits:**
- ✅ Clean repository root
- ✅ Clear separation between ephemeral and permanent documentation
- ✅ Easy to exclude from version control if desired
- ✅ Preserves planning history for archeological research
- ✅ Reduces noise when browsing the project

### CLI Help

Run `br <command> --help` to see all available flags for any command.
For example: `br create --help` shows `--parent`, `--deps`, `--assignee`, etc.

### Important Rules

- ✅ Use br for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `br ready` before asking "what should I work on?"
- ✅ Store ephemeral AI planning docs in `history/` directory
- ✅ Store durable design docs in `docs/designs/`
- ✅ Run `br <cmd> --help` to discover available flags
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems
- ❌ Do NOT clutter repo root with planning documents

For more details, see README.md and QUICKSTART.md.

When making git commits, if you are doing so as part of work on
implementing a beads issue, ALWAYS mention the issue ID somewhere
in the commit headline.
