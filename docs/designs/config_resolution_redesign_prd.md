# Config resolution redesign PRD (km-v3mg)

Status: proposal (design only — no resolver code in this change)

Related issues: km-v3mg (this design), rm-gi0p (completion/runtime unification),
km-mbbl.1/.2/.4 (resolution-code simplification), km-bd4k (`config get
--resolved`), km-b3xr (closed review of the prior kamino-only implementation),
rm-gpkz (the `--no-use` completion precedence bug this design exists to make
structurally impossible).

## Purpose

Introduce **one command-aware effective-default resolver** that computes a
`ResolvedInvocationDefaults` value for a specific invocation, and make it the
single source of truth for both **runtime** command execution and **shell
completion**. Today the same precedence rules are re-implemented in at least
four runtime code paths and again in the completion predictors; they must agree
exactly and there is no mechanism that keeps them in agreement.

> **History note (important).** An earlier implementation of this design
> (shared `resolveInvocationDefaults` / `ResolvedInvocationDefaults`,
> `effective_defaults.go`, deletion of `vibe_defaults.go` and
> `session_resume_defaults.go`) was built and reviewed on the old **kamino**
> repo, PR yendo-eng/kamino#445, and was never migrated into this repo. None of
> those symbols exist here. This document is a fresh design for **remuda**,
> informed by that attempt — in particular by the three regression classes the
> km-b3xr review caught in it (see [Hazards](#hazards-regression-classes-from-km-b3xr)).

## Background: the current resolution sites

Effective defaults are currently computed in these places, each with its own
partial copy of the precedence rules:

**Site 1 — kong parse-time (`cmd/remuda/cli/config_resolver.go`).**
`app.go` mutates the parsed config **in place** before building the kong
parser: `applyPerRepoOverlay` and `applyProfileOverlay` (driven by raw-argv
scanning via `invocation_analysis.go`) merge per-repo and profile overlays into
`cfg`, then `ConfigResolver.Resolve()` (the ~100-line switch at
`config_resolver.go:124-224`) maps flag names to config fields, returning `nil`
when an env var is set so that env beats config.

**Site 2 — vibe post-parse (`cmd/remuda/cli/vibe_defaults.go`).**
`applyPerRepoDefaultsToVibe` re-applies config defaults onto the parsed
`VibeCmd` after late repo/profile selection (`vibe.go:114-121`), guarding each
field with `flagExplicit(kctx, ...)` and `envSet(env, ...)` checks. It exists
because repo selection (and therefore the per-repo overlay) can only be known
after parsing — e.g. FTUE repo selection.

**Site 3 — session resume post-parse
(`cmd/remuda/cli/session_resume_defaults.go`).**
`applyDefaultsToSessionResume` is a near-copy of Site 2 with a different field
subset (no agent/slugify/experiments handling in the same shape), invoked after
`applyPerRepoOverlaysForPickedSessionResume` for `--pick`
(`session_resume.go:94-101`). It is supplemented by two more bespoke resolvers,
`resolveSessionResumeAgent` and `resolveSessionResumeReasoningLevel`, which
re-derive env-vs-config precedence a third way.

**Site 4 — vibe-check partial (`cmd/remuda/cli/prompt_defaults.go`).**
`applyUsePromptDefaults` handles only the `--use` merge for `vibe-check`;
everything else relies on parse-time kong resolution. `VibeCheckCmd` has **no
container or yolo flags at all**, so profile/per-repo container defaults are
silently inert for vibe-check — exactly the gap the km-b3xr review flagged in
the kamino attempt.

**Site 5 — the completion stack (`cmd/remuda/cli/completions.go`).**
The predictors carry a parallel resolution pipeline:
`resolvedUsePromptDefaultsForNoUse` (with a literal “Match runtime precedence”
comment), `applyCompletionOverlaysForUsePrompts`, `inferRepoSlugForCompletion`
(a hand-copy of the repo-slug inference in `invocation_analysis.go`), and
`defaultAgentFromConfig` / `defaultModelFromConfig` used by
`PredictModel`/`PredictReasoningLevel`. The rm-gpkz review (rm-3iyt) found and
hand-fixed a precedence divergence in exactly this code (env `--use` defaults
being merged with explicit `--use` instead of replaced). rm-gi0p tracks
eliminating this whole class.

Late-binding entry points that mutate config after parse (all of which the
resolver must serve): FTUE repo selection (`ftue_repo.go:46` →
`applyPerRepoOverlayForSelection`), `session resume --pick`
(`session_resume.go:168`), and the vibe / vibe-check wizards.

### Why this is fragile

- **Five copies of one precedence model.** Any new config field or precedence
  tweak must be applied consistently to the kong switch, the merge function,
  two-to-three post-parse appliers, and the completion stack. Missing one is a
  silent bug (rm-gpkz was this class).
- **In-place config mutation.** Overlay application destroys the original
  config, so nothing downstream can ask "what was the base value?" — which is
  what km-bd4k (`config get --resolved` with source attribution) needs.
- **Raw argv scanning.** `invocation_analysis.go` + the five scanner helpers in
  `config_resolver.go` (`commandIndex`, `invocationSubcommand`,
  `findCloneRepoURLArg`, `findSessionResumeWorkspaceArg`, `findFlagValue`)
  duplicate flag-skipping logic that must mirror the kong grammar by hand
  (km-mbbl.4).

## Goals

1. One pure(-ish) resolver, `resolveInvocationDefaults`, that computes all
   effective defaults for an invocation and is called by `vibe`, `vibe-check`,
   `session resume`, **and** the completion predictors.
2. Preserve the current precedence semantics exactly (spelled out below).
3. Produce per-field **source attribution** so km-bd4k's
   `config get --resolved` becomes a formatting exercise.
4. Subsume km-mbbl.1/.2/.4 by building the resolver around a declarative field
   registry instead of parallel switch/merge/apply code.
5. Roll out incrementally so each migration step is independently revertible
   and each km-b3xr regression class has a named test before the step that
   could reintroduce it.

## Non-goals

- Implementing `remuda config get` itself (km-bd4k).
- Changing the config file schema (`internal/configfile/v1.go`) or any
  user-visible precedence behavior.
- Consolidating repo-slug inference strategies beyond what the resolver needs
  (km-mbbl.5 stays separate; the resolver consumes `invocationAnalysis`, it
  does not rewrite it).
- Reworking `repos.base_dir` precedence (km-mbbl.3) beyond routing it through
  the same context struct.

## The precedence model to preserve

Global ordering, highest to lowest:

```
explicit flag > env var > profile / per_repo overlay > config defaults > built-in default
```

Within the overlay layer: `--profile` flag > `REMUDA_PROFILE` env >
`per_repo.<slug>.profile`, and a per-repo overlay's `defaults` apply beneath
whichever profile wins (per-repo `defaults` merge first, then the selected
profile merges on top — matching today's `applyPerRepoOverlay` →
`applyProfileOverlay` ordering in `app.go:185-190`).

The load-bearing subtleties, each of which the resolver must encode once and
tests must pin:

| # | Rule | Where it lives today |
|---|------|----------------------|
| S1 | `--use` **appends** to config `use_prompts` (deduped, defaults first), but **replaces** env `REMUDA_USE_PROMPTS` defaults. When env is set, config `use_prompts` is ignored entirely. | `vibe_defaults.go:33-43`, `prompt_defaults.go:63-80`, rm-3iyt finding |
| S2 | `--no-use` **filters** the effective use set; explicit `--no-use` replaces config `no_use` (no merge). Effective prompts = use minus no-use (`effectiveUsePromptNames`). | `vibe_defaults.go:44-54`, `shared.go` |
| S3 | Container `opts` **merge** (append) when applied from a per-repo overlay, but **replace** when applied from a profile. All other container fields replace. | `mergeOverlayV1IntoConfig` `mergeContainerOpts` param, `config_resolver.go:843-851` |
| S4 | Per-repo overlays apply only to repo-relevant invocations: `clone`, `vibe`, `vibe-check`, `session resume`. | `invocationUsesRepoForCommand` |
| S5 | Profiles are only valid for `vibe`, `vibe-check`, `session resume`. | `invocationSupportsProfileForCommand` |
| S6 | Wizard-selected values are user choices, not defaults — late default application must never overwrite them (`vibe.go:114` guards with `!c.Wizard`). | vibe wizard; km-b3xr finding 1 |
| S7 | `session resume --pick` derives repo slug (and hence per-repo overlay + per-repo profile) from the **picked workspace**, after the picker runs. FTUE repo selection similarly late-binds for `vibe`. | `session_resume.go:168`, `ftue_repo.go:46` |
| S8 | Some flags have no env var (`agent-cmd`, `container-name`, `--use`/`--no-use` have env for use only): their guard is flag-explicitness alone. | `vibe_defaults.go:30, 69` |
| S9 | Repo-alias overlays (`per_repo.<slug>.repos.aliases`) take effect for the invocation via the global alias catalog — a side effect, not a resolved value. | `applyPerRepoOverlay`, `github.MergeRepoAliases` |
| S10 | `session resume` normalizes agent to `claude`/`codex` via `resolveSessionResumeAgent` (any non-claude value → codex) — command-specific behavior that must not leak into `vibe`. | `session_resume_defaults.go:84-102` |
| S11 | Kong parse-time defaults must not drift from post-parse resolution: whatever the resolver computes as the config-layer value is what the kong resolver must return. | `ConfigResolver.Resolve` |

## Proposed design

### Core types

```go
// InvocationKind is a closed enum; the resolver's behavior is keyed on it.
type InvocationKind int // Vibe, VibeCheck, SessionResume, Clone, Other

// InvocationResolutionContext carries everything the resolver may consult.
// It is built two ways: from kong parse state (runtime) or from completion
// argv (predictors). Nothing in the resolver touches raw argv.
type InvocationResolutionContext struct {
    Kind        InvocationKind
    Config      *configfile.V1   // as parsed; NEVER mutated by the resolver
    Env         EnvProvider
    Explicit    ExplicitFlags    // interface: was this flag set by the user?
    RepoSlug    string           // normalized; empty if unknown/not applicable
    Profile     ProfileSelection // flag / env / per-repo, resolved lazily
    FlagValues  FlagValues       // typed accessors for explicit flag values the
                                 // resolver needs (use, no-use, profile, ...)
}

// ResolvedInvocationDefaults is the single output consumed by commands,
// completion, and (later) `config get --resolved`.
type ResolvedInvocationDefaults struct {
    Agent                 Resolved[string]
    Model                 Resolved[string]
    ReasoningLevel        Resolved[string]
    SlugifyReasoningLevel Resolved[string]
    AgentCmd              Resolved[string]
    Use                   Resolved[[]PromptName] // post-S1 merge
    NoUse                 Resolved[[]PromptName]
    EffectiveUse          []PromptName           // Use minus NoUse (S2)
    Experiments           Resolved[[]string]
    Yolo                  Resolved[bool]
    ContainerEnabled      Resolved[bool]
    ContainerImage        Resolved[string]
    ContainerOpts         Resolved[[]string]     // post-S3 merge
    ContainerInheritEnv   Resolved[[]string]
    RepoAliases           map[string]string      // S9: effect, applied by caller
}

// Resolved pairs a value with where it came from — this is what unblocks
// km-bd4k without re-encoding command logic.
type Resolved[T any] struct {
    Value  T
    Source ValueSource // Builtin | ConfigDefault | PerRepo | Profile | Env | Flag
}

func resolveInvocationDefaults(rctx InvocationResolutionContext) (ResolvedInvocationDefaults, error)
```

**Decision: pure compute, no config mutation.** The resolver builds an
*effective defaults view* by merging overlays into a **copy** of
`cfg.Defaults` (a shallow struct of pointers — cheap to clone field-wise via
the registry below), leaving `kctx.ConfigFile` pristine. This is the reversal
of today's `mergeOverlayV1IntoConfig`-mutates-shared-state approach, and it is
what makes source attribution and repeated resolution (late binding, wizard)
safe. The one intentional side effect, repo-alias merging (S9), is returned as
data (`RepoAliases`) and applied by the caller at well-defined points, so the
resolver itself stays idempotent.

Tradeoff: `app.go` currently relies on the mutated config for things beyond
defaults (clone hooks, `repos.base_dir`). Those reads move to explicit
accessors on the resolver's effective view during migration; until then the
old mutation path keeps running in parallel (see rollout). Full purity of the
alias catalog is *not* attempted — `github.MergeRepoAliases` remains a global,
but all calls route through one place.

### The field registry (subsumes km-mbbl.1 and km-mbbl.2)

The resolver is driven by a declarative table, one entry per resolvable field:

```go
type fieldSpec struct {
    Flag     string                          // kong flag name ("agent")
    EnvVars  []string                        // {"REMUDA_AGENT"}; empty for S8 fields
    Get      func(*configfile.DefaultsV1) any // nil-safe config getter
    Set      func(dst, src *configfile.DefaultsV1) // overlay merge (replace)
    Merge    mergeStrategy                   // Replace | AppendUnique (container opts, S3)
    Commands invocationKindSet               // which kinds this field applies to
}
```

- **km-mbbl.1**: `ConfigResolver.Resolve()` becomes a map lookup over this
  registry (against the resolver's effective view), eliminating the switch and
  the "forgot a case" bug class.
- **km-mbbl.2**: `mergeOverlayV1IntoConfig` becomes registry iteration; the
  `mergeContainerOpts` boolean turns into `Merge` strategy + the overlay
  source (per-repo → append, profile → replace), which encodes S3 in exactly
  one place. (Non-`Defaults` overlay sections — `repos`, `session` — keep a
  small hand-written merge; they are three fields and not part of the
  five-way duplication problem.)
- The registry also carries `Commands`, encoding S4/S5/S10-style scoping
  declaratively instead of via structurally different apply functions.

### Explicitness abstraction (the rm-gi0p unification)

`ExplicitFlags` is a tiny interface with two implementations:

- **Runtime**: backed by `kong.Context` (today's `flagExplicit`), plus typed
  flag values read from the parsed command struct.
- **Completion**: backed by the predictor's `complete.Args.All` tokens. This is
  where the *only* argv parsing in the completion path lives, and it parses
  only the flags the resolver asks about (`--use`, `--no-use`, `--profile`,
  `--repo`, `--repo-url`, `--in`, `--agent`, `--model`), reusing one shared
  token scanner (km-mbbl.4's `argScanner`) rather than five bespoke loops.

Completion predictors then become thin:

```
PredictNoUsePromptNames = build completion InvocationResolutionContext
                        → resolveInvocationDefaults
                        → return resolved.EffectiveUse ∩ known prompts
```

`PredictModel`/`PredictReasoningLevel` similarly read
`resolved.Agent.Value` / `resolved.Model.Value` instead of
`defaultAgentFromConfig`. Because runtime and completion now execute the same
S1/S2 logic, the rm-gpkz class of bug (completion merging where runtime
replaces) cannot recur without failing the shared parity tests.

Known inherent limitation (unchanged from rm-gpkz): completion only sees
tokens left of the cursor. That affects the *inputs* to the resolver, not the
resolution rules.

### Two resolution moments, one resolver

1. **Parse-time seed.** `app.go` builds a *provisional* context from argv
   analysis (command kind, best-known repo slug, profile signals — the
   existing `invocationAnalysis`), resolves once, and feeds the effective view
   to the kong `ConfigResolver` (S11). Kong still owns flag/env application;
   the config layer it consults is now resolver output instead of a mutated
   global config.
2. **Post-selection re-resolve.** After late bindings (FTUE repo choice,
   `session resume --pick`, wizard repo selection), the command rebuilds the
   context with the now-known repo slug and resolves again, then applies the
   result to its command struct via one shared
   `applyResolvedDefaults(cmd fieldTargets, resolved)` helper. Re-resolution
   is safe because the resolver never mutates shared state; "explicit" fields
   are naturally preserved because the apply step skips any field whose
   `ExplicitFlags` says the user set it.

**Wizard locking (S6/H1).** Wizard-selected values are not kong-explicit, so a
naive re-resolve would overwrite them — km-b3xr finding 1. Decision: the
wizard flow resolves defaults **before** presenting choices (seeding the
wizard UI with correct effective defaults, including the per-repo overlay for
the wizard-selected repo — H3), and the commands it synthesizes carry a
`defaultsLocked` marker so `applyResolvedDefaults` is a no-op on them. This
mirrors the fix that closed km-b3xr on kamino, but here it is designed in from
the start rather than patched in.

### Command migrations

| Current site | Replacement |
|---|---|
| `ConfigResolver.Resolve` switch (`config_resolver.go:124-224`) | Registry lookup against the parse-time effective view (km-mbbl.1). |
| `applyPerRepoOverlay` / `applyProfileOverlay` mutation in `app.go` | Resolver-internal overlay layering on a copied defaults view; alias side effects applied explicitly from `RepoAliases`. |
| `applyPerRepoDefaultsToVibe` (`vibe_defaults.go`) | `resolveInvocationDefaults` + shared apply in `VibeCmd.Run` (post repo selection). File deleted at the end of migration. |
| `applyDefaultsToSessionResume`, `resolveSessionResumeAgent`, `resolveSessionResumeReasoningLevel` (`session_resume_defaults.go`) | Same resolver with `Kind=SessionResume`; the claude/codex normalization (S10) becomes a `SessionResume`-scoped post-resolve rule, not a separate resolver. File deleted at the end of migration. |
| `applyUsePromptDefaults` for vibe-check (`prompt_defaults.go:63`) | Same resolver with `Kind=VibeCheck`. Prompt-name helper utilities (`promptNamesFromDefaults`, `mergePromptNames`) stay — the resolver uses them. |
| Completion stack (`resolvedUsePromptDefaultsForNoUse`, `applyCompletionOverlaysForUsePrompts`, `inferRepoSlugForCompletion`, `defaultAgentFromConfig`, `defaultModelFromConfig`) | Completion-backed `InvocationResolutionContext` + the same resolver (rm-gi0p). |

**vibe-check container decision (H2).** `VibeCheckCmd` currently has no
container/yolo flags, so resolving container defaults for it would compute
values nothing consumes — the exact trap km-b3xr finding 2 documented.
Decision: as part of the vibe-check migration step, `VibeCheckCmd` gains
`VibeContainerOptions` (it already shares the launch path via
`internal.VibeCommand`, which has container fields — the plumbing gap is only
in the CLI struct), and the resolver's `Commands` scoping marks container
fields as applicable to `VibeCheck`. If we instead chose to keep vibe-check
container-less, the registry must mark those fields *not applicable* so
`config get --resolved` doesn't lie about them; silently resolving-but-
dropping is the one option this design forbids.

### How this unblocks km-bd4k

`config get --resolved [--repo <slug>] [--profile <name>] [--command vibe]`
becomes: build an `InvocationResolutionContext` from the requested
coordinates, call the resolver, print `Resolved[T].Value` + `Source` per
field. No command-specific logic is re-encoded; the `Resolved[T]` source
metadata is populated by the same code paths every command runs. (km-bd4k also
depends on km-dr3r; this design covers the resolution half.)

## Hazards (regression classes from km-b3xr)

These three regressions all shipped in the *reviewed, careful* kamino
implementation. Each gets a named regression test that must exist **before**
the migration step that could reintroduce it, and each is called out in the
rollout table.

- **H1 — Wizard defaults overwritten.** Late re-resolution treated
  wizard-selected agent/model/use as unset and overwrote them from
  config/profile defaults (kamino `vibe_check_wizard.go` + `vibe_check.go`).
  Guard: `defaultsLocked` on wizard-synthesized commands; test resolves a
  wizard-produced command with `Agent="claude"`, `Use=["small-commits"]`
  under config `agent=codex`, `use_prompts=[make-pr]` and asserts nothing
  changes.
- **H2 — vibe-check container defaults dropped.** The resolver computed
  container defaults for vibe-check but the apply/launch path had no fields to
  put them in, so profile/per-repo container config stayed inert while tests
  of the resolver passed. Guard: e2e test that a config with
  `per_repo.<slug>.defaults.container.enabled=true` produces a container
  launch for `vibe-check` (or, if the no-container decision is taken, that the
  registry marks the field N/A and `--resolved` output reflects that).
- **H3 — per-repo overlays missing for wizard-selected repos.** The wizard
  locked defaults *before* the repo chosen inside the wizard had its
  `per_repo` overlay applied, so repo-scoped defaults never took effect.
  Guard: test that `vibe-check --wizard` selecting repo `acme/repo` with
  `per_repo["acme/repo"].defaults.container.enabled=true` yields
  `Container=true` on the synthesized command; same shape for `vibe --wizard`
  and FTUE selection.

Additional hazards specific to this repo's rollout:

- **H4 — parse-time vs post-parse drift (S11).** While Sites 1–4 migrate at
  different times, a field could resolve differently at parse time vs in the
  command's re-resolve. Guard: parity test that resolves every registry field
  both ways for a fixed config/env/argv matrix and diffs the results.
- **H5 — completion/runtime drift (rm-gpkz class).** Guard: table-driven
  parity test that runs the same scenarios through the kong-backed and
  completion-backed `ExplicitFlags` implementations and asserts identical
  `EffectiveUse` (this test replaces the hand-maintained expectations fixed in
  rm-3iyt).

## Incremental migration / rollout plan

Each phase is a separate PR-sized change, lands with its tests, and leaves the
CLI fully working. Behavior changes are confined to the phase that owns them.

**Phase 0 — Characterization tests (no production code).**
Table-driven precedence-matrix tests over the *existing* paths
(`applyPerRepoDefaultsToVibe`, `applyDefaultsToSessionResume`,
`applyUsePromptDefaults`, `ConfigResolver.Resolve`, completion predictors)
covering S1–S10: profile flag vs `REMUDA_PROFILE` vs `per_repo.<slug>.profile`;
per-repo overlays for vibe / vibe-check / `session resume --pick`;
`use_prompts` + `--use` + `--no-use` interactions incl. env replacement;
container merge-vs-replace. These become the parity oracle for every later
phase.

**Phase 1 — Introduce the resolver, wired to nothing.**
Add `InvocationResolutionContext`, `ResolvedInvocationDefaults`,
`Resolved[T]`, the field registry, and `resolveInvocationDefaults` as pure
code. Add parity tests asserting the resolver's output matches the Phase-0
characterization matrix. H4 parity harness lands here.

**Phase 2 — Migrate `vibe`.**
Replace the `applyProfileOverlayByName` + `applyPerRepoDefaultsToVibe` calls in
`vibe.go:114-121` with a post-selection re-resolve + shared apply. Wizard
locking (H1) and wizard/FTUE per-repo seeding (H3) land here **with their
regression tests in the same change**. `vibe_defaults.go` is not yet deleted
(session/vibe-check tests may reference helpers); it merely loses its caller.

**Phase 3 — Migrate `session resume`.**
Replace `applyDefaultsToSessionResume` and the two bespoke resolvers;
`--pick` re-resolves after workspace selection (S7). S10 normalization moves
into the resolver's `SessionResume` scope with a pinned test.

**Phase 4 — Migrate `vibe-check` (incl. the H2 decision).**
Adopt full resolution for `Kind=VibeCheck`; add `VibeContainerOptions` to
`VibeCheckCmd` and thread container fields through to the launch (or
explicitly mark them N/A — see decision above). H2 e2e test lands in the same
change. Wizard flow gets the same H1/H3 treatment as vibe.

**Phase 5 — Migrate completion (rm-gi0p).**
Swap the predictor internals for the completion-backed context + resolver;
delete `resolvedUsePromptDefaultsForNoUse`,
`applyCompletionOverlaysForUsePrompts`, `inferRepoSlugForCompletion`,
`defaultAgentFromConfig`, `defaultModelFromConfig`. H5 parity tests land here.
Existing rm-gpkz predictor tests must pass unmodified — they are the
acceptance bar.

**Phase 6 — Retire the parallel machinery.**
Point the kong `ConfigResolver` at the registry/effective view (km-mbbl.1),
replace `mergeOverlayV1IntoConfig` internals with registry iteration
(km-mbbl.2), collapse the argv scanners into the shared `argScanner`
(km-mbbl.4), and delete `vibe_defaults.go` and `session_resume_defaults.go`
plus wrapper-only tests. This phase is pure removal; the Phase-0 matrix and
H4 parity tests are the safety net.

Rollback story: phases 2–5 each swap one call site behind an unchanged
external behavior contract, so reverting a phase is reverting one commit;
nothing in a later phase depends on an earlier phase's *removal* (removals are
all deferred to Phase 6).

## Test strategy

Per AGENTS.md: e2e-heavy, `stretchr/testify`, no `--help`-text assertions.

1. **Precedence matrix (Phase 0, kept forever).** One table, dimensions:
   {field} × {builtin, config default, per-repo, profile-by-flag,
   profile-by-env, profile-by-per-repo, env var, explicit flag} × {vibe,
   vibe-check, session resume, session resume --pick}. Exercised e2e through
   the CLI where practical (parse + inspect the built `internal.VibeCommand` /
   `SessionResumeCommand`), and directly against the resolver where e2e can't
   reach (source attribution).
2. **Subtlety pins.** Dedicated tests per S1–S11 (e.g. S1: env set + explicit
   `--use` yields only the flag values; S3: per-repo opts append to base
   config opts while profile opts replace).
3. **Hazard regressions.** H1/H2/H3 tests as specified above, landing *before
   or with* the phase that could reintroduce them.
4. **Parity harnesses.** H4 (parse-time vs post-parse) and H5 (runtime vs
   completion) run the full matrix through both context constructions and
   diff `ResolvedInvocationDefaults`.
5. **Source attribution.** For each matrix cell, assert `Resolved[T].Source`
   names the winning layer — this doubles as the km-bd4k acceptance test.
6. **Idempotence.** Resolving twice (parse-time seed, then post-selection with
   the same inputs) yields identical output, and never mutates
   `kctx.ConfigFile` (assert deep-equality of the config before/after).

## Alternatives considered

- **Keep mutating config, but from one shared entrypoint.** Smaller diff, and
  what the kamino attempt partially did. Rejected: mutation is why source
  attribution (km-bd4k) and safe re-resolution (wizard, --pick) are hard, and
  why the completion path had to grow its own copy — predictors can't safely
  mutate the shared config either.
- **Resolve everything at parse time only.** Would eliminate the post-parse
  sites, but S7 makes it impossible: FTUE/--pick/wizard repo selection happens
  after parsing, and per-repo overlays depend on it.
- **Reflection-based merge instead of a registry (km-mbbl.2 option A).**
  Less code, but struct tags can't express command scoping (S4/S5/S10) or the
  per-source merge strategy (S3) as clearly, and the registry is needed for
  km-mbbl.1 and source attribution anyway.
- **Fix completion divergences case-by-case (status quo).** rm-gpkz shows the
  cost: a reviewer manually re-deriving runtime precedence to catch a
  completion bug, per field, forever.

## Acceptance criteria

- `vibe`, `vibe-check`, `session resume`, and the `--no-use`/model/reasoning
  completion predictors all obtain effective defaults from
  `resolveInvocationDefaults`; the five current sites are gone (Phase 6
  complete).
- The Phase-0 precedence matrix passes unchanged from before the migration.
- H1–H5 regression tests exist and pass.
- `kctx.ConfigFile` is never mutated by default resolution.
- A follow-up `config get --resolved` (km-bd4k) can be implemented by calling
  the resolver and printing `Resolved[T]` — verified by the source-attribution
  tests, not by implementing the command.
