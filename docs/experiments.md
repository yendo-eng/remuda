# Experiments

Remuda puts some experimental functionality behind configuration.

To enable an experiment, either add its name as an entry to
`defaults.experiments` in your config, or supply a comma-separated list of
experiment names in `REMUDA_EXPERIMENTS` or the `--experiments` flag.

List of current experiments:

- `use-prompts-context-wrapper` – wrap the saved prompts selected with
  `--use`/`--no-use` in a `<context>...</context>` block before injecting them
  into the agent prompt. This affects `vibe`, `vibe-check`, and `session
  resume`; fetched Jira, Slack, and GitHub issue context is not wrapped.
- `cow-clone` – use copy-on-write clones of the repository cache when creating
  `--full-clone` workspaces with `clone`, `vibe`, or `vibe-check`. Supported
  filesystems use shared blocks; unsupported filesystems transparently fall
  back to a regular copy.
- `session-manifest` – have `vibe` write a local, untracked `.remuda.json`
  launch manifest into the workspace. When enabled, `session resume` reads the
  manifest to determine what settings to use when launching the agent.
- `aggregate-multiplexer` – make session discovery and operations search all
  supported multiplexer backends (`tmux`, `zellij`, and `herdr`) while new
  sessions continue to launch in the configured backend. Unavailable backends
  are skipped during discovery.

