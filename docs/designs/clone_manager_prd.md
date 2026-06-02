# Remuda CLI – Product Requirements Document

## Implementation Status (2026-02-13)

`remuda clone` now supports `--force`, `--full-clone`, and `--branch`, so the v1 non-goals and flag list below are out of date.

## 1. Purpose

Provide a fast, single-command way for developers to spin up a local working copy of a Git repository **without** repeatedly cloning from the remote origin.

## 2. Background / Problem Statement

Engineers frequently work on multiple features, hot-fixes, or experiments in parallel and therefore need isolated check-outs of the same codebase. Performing a fresh `git clone` each time is slow and network-intensive.  
A Python script (`clone_manager.py`) already exists that caches the repository locally and copies from that cache, but:

* The folder-naming logic can still collide with existing directories.
* The cache lives in a root-level hidden directory, making the workspace messy.
* Branch selection is not supported.

We need to formalise and extend the tool so that it is reliable day-to-day infrastructure for the team.

## 3. Goals & Non-Goals

### Goals

* Speed up creation of local repo instances by **≥ 80 %** compared with full `git clone`.
* Guarantee that no existing local directory is overwritten.
* Keep the command surface minimal and intuitive.

### Non-Goals

* Managing virtual-environments or other dependencies inside cloned folders.
* Windows support (explicitly **out of scope**).
* Force-overwriting existing folders (no `--force` flag).

## 4. Personas

* **Backend / Full-stack developer** – needs several simultaneous workspaces for different branches.
* **Release engineer** – generates deterministic builds from various commits.

## 5. User Stories

| ID | Description |
|----|-------------|
| **US-1** | As a developer, I can run `remuda clone --name feature-auth-hardening` and get a workspace folder named `feature-auth-hardening`. If that folder already exists, the command errors (no auto-numbering). |
| **US-3** | As a developer on a slow or offline connection, subsequent runs utilise the local cache rather than the network. |
| **US-4** | As a developer, the local branch is automatically created and checked out with the same name as the workspace (`--name`). |

## 6. Functional Requirements

### FR-1 CLI Parameters

1. **Positional** – `[repo_url]` (optional, defaults to the configured repo
   when one is set via `REMUDA_DEFAULT_REPO_URL`, `REMUDA_DEFAULT_REPO`,
   or config).
2. **Flags (XOR required)** – exactly one must be supplied:
   - `--name <n>` – workspace folder (and branch) name.
   - `--wizard` – interactive flow that collects the name and repo.
3. **Optional flags**
   - `--repo <alias>` – shorthand for a configured repository alias; expands to
     the corresponding git URL (see `remuda repo list`).
   - `--no-clone-hooks` – skip running built-in post-clone hooks for the target repository.

### FR-2 Storage Layout & Local Cache

* **Base directory:** `<baseRoot>/<org>/<repo>/` derived from `repo_url`.
  * `baseRoot` defaults to `~/.remuda/repos` and can be overridden via the optional
    environment variable `REMUDA_REPOS_BASE_DIR` (preferred; `VIBE_REPOS_BASE_DIR` is a legacy alias).
* **Repo cache:** `.repo_cache` sub-folder inside the repo’s base directory.
  * If missing, perform full `git clone` into `.repo_cache`.
  * If present, run `git -C .repo_cache pull` to refresh.

### FR-3 Folder-Name & Collision

* The workspace folder equals the provided `--name` (e.g., `feature-auth-hardening`).
* If a folder with that exact name already exists under the repo base, the command fails with a clear error. No automatic numbering.

### FR-4 Clone Creation Workflow (UPDATED)

1. Use `git worktree add` to create `~/.remuda/repos/<org>/<repo>/<target_folder>` **linked to** the local cache at `~/.remuda/repos/<org>/<repo>/.repo_cache`.
   * This shares objects with the cache, drastically reducing disk usage and wall-clock time compared to a full copy.
2. Inside the new worktree directory:
   * Create/switch to local branch `<name>` (tracks remote if it already exists).
   * `git pull` is attempted (best-effort) to fast-forward.
3. Run any built-in post-clone hooks registered for the repository (unless `--no-clone-hooks` was provided). Hooks run sequentially and failures abort the clone.
4. (Implementation note) The previous step that copied `.aider.conf.yml`/symlinked conventions is currently out-of-scope and may be revisited later.

### FR-5 Logging & UX

* Plain, colour-less console messages: *caching*, *updating*, *cloning*, *error*.
* On invalid CLI usage → concise help with examples.

### FR-6 Error Handling

* Non-zero `git` exit codes abort the current clone with explanation.
* Copy/symlink failures skip the affected clone but continue with remaining clones.

## 7. Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| **NFR-1** | Performance: each additional clone after the first completes in < 3 s (SSD workstation). |
| **NFR-2** | Portability: macOS / Linux with Python 3.8 +. |
| **NFR-3** | Idempotency: running the same command repeatedly never corrupts existing folders. |
| **NFR-4** | Code quality: PEP-8 compliant, strong typing (mypy) in future, ≥ 95 % unit-test coverage around name-generation logic. |

## 8. Metrics / Success Criteria

* Median time to create a clone vs `git clone` baseline.
* Zero reported incidents of overwritten working directories.
* ≥ 90 % internal developer adoption for multi-branch workflows within two weeks of release.

## 9. Open Questions

1. Should we allow checkout of a **specific commit hash** in addition to branch?
2. Any need for configurable base directory beyond `~/.remuda/repos`? — Resolved: use
    optional env var `REMUDA_REPOS_BASE_DIR` to set the base root (preferred; `VIBE_REPOS_BASE_DIR` is a legacy alias).

## 10. Out-of-Scope / Future Enhancements

* Retention / GC of old caches (explicitly **not** in v1).
* Force-overwrite functionality.
* Windows support.
* Automatic dependency setup.
* Parallel copy operations for extremely large repositories.

## 11. Appendix – Example Flows

### A) Minimal

```bash
$ remuda clone --name feature-auth-hardening
# → ~/.remuda/repos/acme-org/example-repo/feature-auth-hardening on branch feature-auth-hardening
```

> Note: the tool now always creates **one** clone per execution. Re-run the command to obtain additional workspaces if needed.
