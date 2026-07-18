# go-csmith — Specification

Source of truth for product goals and process. Decisions locked in grilling (2026-07), including the postmortem of session `019f5695-5d12-70a1-a913-c8790f96db54` and the residual-era tree.

## 1. Purpose

Go reimplementation of [Csmith](https://embed.cs.utah.edu/csmith/): a **Go library** (and eventually a **CLI**) that generates random, valid C programs for compiler / tool stress testing.

**Not goals:** shipping a C runtime/library package, replacing `csmith.h` packaging, or exposing a public C ABI.

## 2. Product surface

### 2.1 Library first, CLI last

| Phase | Surface |
|-------|---------|
| **Now** | Fair Go packages under `pkg/csmith`, built **bottom-up** from Csmith’s own modules. Public API grows as real units land. |
| **Later** | CLI binary name **`csmith`**, drop-in default generation vs golden upstream. Flag surface grows toward upstream; unimplemented non-defaults **error clearly** (no silent no-ops). |

CLI wiring (`cmd/`, `internal/cli`, flag UX) is **last**. Do not grow hollow flag parity before the generator spine is fair.

### 2.2 Eventual public API (shape)

When ProgramGenerator exists:

```go
func Generate(ctx context.Context, opts Options) (string, error)
```

- **Input:** context + options (mirrors CLI-relevant settings)
- **Output:** full C program as a `string` (same role as upstream stdout)
- **Context:** honor cancel/deadline at coarse generation boundaries
- **Not yet:** public RNG DI, plugins, partial AST export, streaming

Until that exists, `Generate` may be absent or return a clear `not implemented` error. Do not fake full programs.

## 3. Correctness / “done”

### 3.1 Product north star (emergent)

| Layer | Requirement |
|-------|-------------|
| Default options + seed `N` | Same **bit-identical C program body** as golden upstream |
| Header line (`Generator: …`) | May differ only if documented |
| Multi-seed | Defaults hold for an agreed seed range once the spine is real |

Drop-in is **emergent** from fair C++-linked ports and 1:1 function tests. It is **not** achieved by matching RNG event streams with residual burns, invent floors, or packed offline tables.

### 3.2 Golden reference

- Resolve with **`loadDotfilesEnv`** then Nix: `pkgs.csmith` / `pkgs.csmith.src`
- Pin observed at planning time:
  - Package: `csmith-2.3.0-unstable-2026-03-01`
  - Git rev: `0cdc710315cfee9035e22ef4363ca479270d1934`
  - Self-reports: **csmith 2.4.0**
- Do **not** track tip silently; bump pin deliberately when Nixpkgs moves
- Working tree may vendor/extract sources under `.build/csmith-src` (gitignored) for reading and instrumented builds

### 3.3 Daily progress metric (not the old loop)

| Do | Do not |
|----|--------|
| Port **one C++ function / path** with a same-hunk cite | Optimize `first_div` / event-count climb as the goal |
| Add **1:1 behavioral tests** for that unit | Treat green `find-rng-divergence` as acceptance |
| Grow bottom-up along Csmith’s dependency order | Ship residual multiphase to force stream match |

**Primary success signal:** “function F behaves 1:1 with upstream under tests.”

**Secondary / late:** whole-program source match and (if useful) RNG stream agreement as a **byproduct** of correct control flow — never as a license to cheat.

### 3.4 1:1 function contract

For each ported unit:

1. Identify the C++ function/method (file + name; predicate when non-obvious).
2. Define inputs: explicit args, relevant options, and RNG seed or a **fixed draw sequence** when the unit consumes randomness.
3. Define outputs: return values and **observable side effects** (pools, created variables, types, qualifiers, chosen indices, emitted fragments).
4. Test with tables and small harnesses so the same contract can be checked without generating a full program.
5. Prefer vectors captured from instrumented upstream or a narrow C++ harness when behavior is subtle; re-capture when the pin moves.

Event numbers (`eNNNN`) are **debug breadcrumbs**, not evidence and not test names of record.

### 3.5 Whole-program gates (only after ProgramGenerator exists)

When the fair spine can emit a full default program:

1. Seed **2**, defaults: **source body** match vs golden
2. Expand seed range (e.g. N = 20) for source match
3. Flag parity toward full upstream semantics

Timeouts on all generation/compare steps. Bounded filter/retry loops only where C++ has them.

Until ProgramGenerator exists, these gates are **inactive**. Do not invent residual programs to pre-satisfy them.

## 4. Options policy

| Phase | Policy |
|-------|--------|
| **Library spine** | Model `CGOptions` / defaults as in C++. Unused options stay inert or unimplemented with clear errors when exposed. |
| **CLI (last)** | Default path is the drop-in target. Non-default flags not implemented → **error clearly**. |
| **Later** | Full upstream flag semantics |

## 5. Architecture and rewrite

### 5.1 Rewrite branch — start from scratch

The residual-era implementation on `main` (including ~140k-line `generator.go`, residual packs, invent multiphase, `silenceTrace`) is **not** the base for further evolution.

| Rule | Detail |
|------|--------|
| Branch | New branch (e.g. `fair-rewrite`); delete generation/cheat mass and rebuild |
| History | Keep residual-era commits on `main` as archaeology; do not graveyard-copy cheat code into the rewrite |
| Carry over | Fair **tooling and docs**: this SPEC, `go.mod`, instrumented-upstream scripts/patch, golden pin notes, `mise.toml` as useful |
| Do not carry | Residual players, packed streams, invent floors, era sticky flags, blank-draw ladders, seed/event multiphase catalogs |
| Unsure | Delete and re-port from C++ with a cite |

### 5.2 Layout (one package, C++-named files)

Single package `pkg/csmith`. Files named after upstream units, for example:

| Go file (illustrative) | Upstream |
|------------------------|----------|
| `rng.go` / random helpers | `random.cpp`, `DefaultRndNumGenerator`, `RandomNumber` |
| `cg_options.go` | `CGOptions` |
| `probabilities.go` | `Probabilities` |
| `types.go`, `cv_qualifiers.go` | `Type`, `CVQualifiers` |
| `effect.go`, `cg_context.go` | `Effect`, `CGContext` |
| `variable.go`, `array_variable.go` | `Variable`, `ArrayVariable` |
| `variable_selector.go` | `VariableSelector` |
| `expression*.go` | `Expression*` |
| `statement*.go`, `block.go` | `Statement*`, `Block` |
| `function*.go` | `Function*`, `FunctionInvocation*` |
| `fact*.go` | `Fact*` as needed |
| `program_generator.go` | `DefaultProgramGenerator` / `AbsProgramGenerator` |

File header: upstream path(s) + pin rev. No second godfile that re-accumulates residual state machines.

### 5.3 Bottom-up build order

Implement and test in this order (do not skip upward with invent pads):

1. LCG / `DefaultRndNumGenerator` / `RandomNumber` (upto, flipcoin, pure_rnd, optional trace)
2. `CGOptions` defaults
3. `Probabilities` / tables needed for defaults
4. `Type` + `CVQualifiers`
5. `Effect` / `CGContext` skeleton
6. `Variable` / `ArrayVariable`
7. `VariableSelector`
8. `Expression*`
9. `Statement*` / `Block`
10. `Function*`
11. `Fact*` as required by real paths
12. `DefaultProgramGenerator` / output
13. **CLI last**

No empty stubs that **claim** behavior. Prefer missing symbols over fake methods.

### 5.4 Evidence bar (same-hunk cite)

Every generation branch and every RNG consumer that lands must have a **same-hunk** comment (or immediately adjacent block comment) naming:

- C++ **file**
- **function/method**
- **predicate** when non-obvious

Example shape:

```go
// VariableSelector.cpp SelectGlobal: choose_var first; GenerateNewGlobal only if empty.
```

| Evidence | Not evidence |
|----------|----------------|
| C++ file + function (+ predicate) | `eNNNN` alone |
| Same control-flow reason as C++ | “stream-grounded residual” |
| Test that locks the contract | “multi-seed still holds” while inventing pool sizes |

**No cite → treat as cheat → reject or delete.**

### 5.5 Entropy

**Default:** every `upto` / `flipcoin` / `next31` (and filtered variants) **uses** its result the way Csmith does.

**Exception:** a draw may be ignored **only if** the same C++ path discards it for the same reason, with a **same-hunk** cite. Appearance of Go-only discard is enough to reject.

## 6. Process

### 6.1 General

| Rule | Detail |
|------|--------|
| Blast radius | Durable project state in this repo (including gitignored `.build/`). Read-only Nix store / `pkgs.csmith` is fine. |
| Unit of work | One C++-linked function or narrow path + tests + cite |
| Plateau | Read C++ under `.build/csmith-src/src/`; do not invent RNG sequences |
| Stop | Human stops, or north-star gates met via fair code |
| Ralph / score loops | **Removed.** No ralph rituals; no scheduled “commit and continue” that climbs event scores |

### 6.2 Metrics vs acceptance

Parity and trace scripts (`find-rng-divergence.sh`, `compare-upstream.sh`, `parity-gate.sh`, `validate-a.sh`, instrumented builds) are **thermometers**.

| Scripts may | Scripts must not |
|-------------|------------------|
| Help debug RNG or source diffs | Alone accept a PR or mark “done” |
| Report numbers for humans | Justify residual, blank draws, or invent floors |

**Merge acceptance** always requires:

1. No looks-like-cheat shapes (§7)
2. Same-hunk C++ cites on new control/RNG paths
3. Layer **1:1 tests** green for the unit touched
4. Human or agent **read of the diff** (integrity is not scripted)

Green metrics never override a cheaty diff.

### 6.3 Agent process (stay out of the loop)

**Banned**

- Residual packs / multiphase event catalogs as a climb vehicle
- Blank entropy (`_ = r.upto`, `_ = er.pick`, untraced gap-fills without C++ discard cite)
- Inventory floors/pads inventing pool sizes
- `silenceTrace` / silent RNG to fake event counts
- Seed-literal generation branches
- Event-indexed sticky flags as primary control (`after e2151…`, `GlobalN==7 → U56`)
- “Integrity residual debt” commits that land cheat code while noting debt
- Scheduled loops whose real objective is `first_div` / `eN→eM` climb
- Commit subjects that only celebrate event scores (`seed5 e8199→8220 residual`)

**Allowed**

- Port one C++ path with cite + 1:1 tests
- Local throwaway experiments that **never** merge
- Diagnostic traces and thermometers
- Progress notes as **functions ported + tests**, not event tallies

### 6.4 Technique preference

For any mismatch or missing behavior:

1. Find the **C++ function + predicate**
2. Implement that path, including **real inventory / state** updates
3. Test the contract 1:1
4. Only then compose into higher layers

Do not “fix” a divergence by burning the upstream stream shape.

## 7. Cheat catalog (reject on sight)

Patterns observed in session `019f5695-5d12-70a1-a913-c8790f96db54` and residual-era `pkg/csmith`. **Hard reject** for merge. **Delete** on the rewrite branch. Looks-like is enough.

### 7.1 Stream replay

| Pattern | Example / smell |
|---------|-----------------|
| Packed offline event tables | `f10_late_residual_data.go`, `[]uint32` dumps of one seed’s F/U/N stream |
| Residual player as primary driver | `residualPlayer`, `burnF10LateExprResidual` replaying packs instead of Expression/Statement graph |
| Code-generated “DO NOT EDIT” stream | Headers admitting offline residual burns |

### 7.2 Trace and count faking

| Pattern | Example / smell |
|---------|-----------------|
| `silenceTrace` / `rng.silent` | Stop tracing after residual exhausts UP stream so counts “match” |
| `haltGen` after pack | Stop real generation once the taped stream ends |
| Comments like “exhausts UP seed4 stream (~106117)” | Score match without continuing fair generation |

### 7.3 Blank entropy and ladders

| Pattern | Example / smell |
|---------|-----------------|
| Blank draws | `_ = r.upto(…)`, `_ = er.pick(5)`, `_ = r.next31()` unused |
| Multiphase burn ladders | Long sequences of blank picks keyed to visit counters |
| Untraced `next31` gap-fills | Sync `rand_depth` without writing Constant digits / real API use |
| Draw then ignore to force another path | Divergent control flow + discard |

### 7.4 Invent inventory

| Pattern | Example / smell |
|---------|-----------------|
| Pool floors/pads | `if nCtrl < 37 { nCtrl = 37 }`, `nArr++`, one-shot `n = 13/18/51` |
| Hardcoded choose sizes | `.pick(4)`, `.pick(7)`, sticky `pick(21)` Global without materialised list |
| Fake choose space | Invent `Un` so choose runs when C++ pool is empty or different size |
| “Expand” pools by constant trim | Force U51/U14 without GlobalList / local_vars truth |

### 7.5 Event-era control flow

| Pattern | Example / smell |
|---------|-----------------|
| Sticky flags named for seed eras | `freeMultiIVNeedNoRhsPostEAReturnPostArrayOp…`, `nullValidatePostResidual…` |
| Visit-counter multiphase | `PLN++` / `switch un { case 0: pick(5) // e8610 }` with no C++ counter |
| One-shot “after event N do X” | Primary control is event history, not Csmith state |
| Seed literals | `if seed == 2` in generation paths |

### 7.6 Process and documentation laundering

| Pattern | Example / smell |
|---------|-----------------|
| Integrity residual debt | Land cheat, document debt in SPEC/progress, keep climbing |
| Metric-only commits | Subject/body are only `first_div` / `eN→eM` with blank-draw hunks |
| Event match + thin AST | Full event “PASS”, source FAIL, residual-driven path |
| Temporary residual “then clean later” | Never an allowed step on the path to done |
| Continue-loop gaming | Recurring “commit and continue” that optimizes scores despite anti-cheat wording |

### 7.7 Review heuristic (fail closed)

1. Removing the RNG call would not change generated structure, only the event stream → **reject** (unless same-hunk C++ discard).
2. Draw’s only role is “so the next event number matches UP” → **reject**.
3. Pool size is a constant floor/pad instead of materialised inventory → **reject**.
4. Branch would not fire for an arbitrary seed on the same C++ path → **reject** (seed overfitting).
5. No same-hunk C++ cite → **reject**.
6. Score climbed, tests are only “stream length” or absent → **reject**.

## 8. Testing

| Layer | Expectation |
|-------|-------------|
| Unit / table tests | Each ported function: 1:1 behavioral contract (§3.4) |
| Smoke tests | Small end-to-end of a **layer** (e.g. RNG sequence, type pick, selector choose) vs known upstream vectors |
| Instrumented upstream | Optional helper to capture vectors; not a substitute for cites |
| Full program parity scripts | Thermometers only; active as acceptance only after fair ProgramGenerator + read-diff |

Tests must exercise **this** implementation’s logic, not restate third-party trivia. Prefer failing tests when upstream pin behavior is not yet ported over green lies.

## 9. Explicit non-goals (current)

- Residual / invent / silenceTrace as engineering strategy
- Automated score-climbing agent loops (Ralph, scheduled first_div grind)
- Silent acceptance of unimplemented CLI flags
- Public RNG injection API (deferred)
- C library packaging of full Csmith runtime
- Preserving residual-era generator code for “compatibility”

## 10. Repo layout (target)

| Path | Role |
|------|------|
| `pkg/csmith/` | Fair library, C++-named files |
| `cmd/csmith`, `internal/cli` | CLI **last** |
| `scripts/` | Thermometers + instrumented upstream build (not integrity gates alone) |
| `third_party/csmith-rng-trace.patch` | Fair measurement aid |
| `SPEC.md` | This document |
| `.build/` | Gitignored upstream extract / instrumented build |

## 11. Changelog of decisions (grill)

### 11.1 Original product lock (2026-07, still valid where noted)

1. Goal = feature parity / drop-in replacement (not “any random C”)
2. Golden = `pkgs.csmith` pin above (not classic 2.3.0 tarball alone)
3. CLI name `csmith`; library string API; not a C package
4. Unimplemented non-defaults error when exposed
5. Blast radius in-repo; timeouts; no silent tip-tracking of upstream

### 11.2 Residual-era process (superseded)

The following were attempted and **produced metric gaming** (session `019f5695…`, residual-era tree). They are **not** the process going forward:

- Primary climb = first RNG event divergence / full event match per seed
- Residual multiphase and packed streams as temporary vehicles
- Multi-seed event hold as sufficient when invent pads were present
- “Integrity residual debt” as a mergeable state
- Scheduled commit-and-continue loops optimizing scores

Superseding integrity text that banned residual while still logging residual climbs is replaced by §3, §5–§7 of this document.

### 11.3 Fair rewrite lock (2026-07-17 grill)

| # | Decision |
|---|----------|
| 1 | **Hard reject** cheats for anything that lands; experiments only throwaway |
| 2 | **Strip all cheating**; only fair code evolves |
| 3 | Fair code is **linked to upstream C++** with evidence |
| 4 | Evidence = **same-hunk cite**; looks cheaty → delete; start from Csmith foundation |
| 5 | **Bottom-up** along Csmith modules; **CLI last** |
| 6 | Layout = one `pkg/csmith`, files named after C++ units |
| 7 | Build order = RNG → options → probs → Type/CV → Effect/CGContext → Variable → VariableSelector → Expression → Statement/Block → Function → Fact → ProgramGenerator → CLI |
| 8 | Layer tests + **1:1 smoke**; not score theater |
| 9 | **Delete all** residual-era implementation; **start from scratch on another branch** |
| 10 | Tooling/metrics kept as **thermometers** only; models will game scores if scores accept |
| 11 | **Out of the RNG-agreement loop**; daily focus = tested functions 1:1 with upstream |
| 12 | 1:1 = **behavioral contract** per function (inputs, outputs, side effects) |
| 13 | North star remains **drop-in bit-identical program body**, emergent from fair ports |
| 14 | Agent process: function-sized ports + tests; no residual/first_div continue-loops |
| 15 | SPEC rewrite records cheat catalog from session + tree so patterns stay banned |

## 12. Progress (living)

| Item | Status |
|------|--------|
| SPEC fair-rewrite lock | **Done** |
| Branch `fair-rewrite` + delete residual mass | **Done** (generator/residual/types godfiles removed) |
| Layer 1: `Rng` | **Done** — `rng.go` + tests |
| Layer 2: `CGOptions` defaults | **Done** — macros 1:1 tests; `MaxPointerDepth` fixed to 5 (`max_indirect_level`) |
| Layer 3: `Probabilities` singles + simple-types equal group | **Done** — `probabilities.go` + tests |
| Layer 4: `Type` simple + pointer wrapper | **Partial** — simple + `PointerTo` / indirect level |
| Layer 5: `Effect` + empty `CGContext` | **Partial** — purity / SE-free only |
| Layer 5b: `CVQualifiers::random_qualifiers` | **Partial** — type-based path + scalar make_* |
| Layer 6: `Variable` + `gensym` | **Partial** — CreateVariable, is_global/local/arg |
| Layer 7: `VariableSelector` | **Partial** — choose_ok_var, GenerateNewGlobal (+init), SelectGlobal |
| Layer 7b: `Constant::make_random` | **Partial** — simple + pointer |
| Layer 4b: `Type::match` / cache | **Partial** |
| Layer 8: `Expression` term pick + const/var | **Partial** |
| Layer 9: `Statement` probability + minimal Stmt emit | **Partial** |
| Layer 10: `Function` signature + `make_first` | **Partial** |
| Layer 10b: `Block::make_random` | **Partial** |
| Layer 12: `DefaultProgramGenerator` + `Generate` | **Partial** |
| Layer 9b: `StatementIf` / `StatementFor` | **Partial** |
| SelectLoopCtrlVar / ParentLocal | **Partial** |
| Layer 8b: `ExpressionFuncall` + std binary/unary | **Partial** |
| Layer 8c: `SafeOpFlags` binary/unary names | **Partial** |
| Layer 9c: `StatementAssign` ops table | **Partial** |
| Layer 4c: `SelectLType` + pointer types | **Partial** |
| Arrays select/itemize/array-op | **Partial** |
| Struct types + field expand | **Partial** |
| Layer 4e: bitfields in structs | **Partial** — make_one_bitfield, full/normal paths, `: width` emit |
| Unions / CFG back-edge / zero-pad skip | **Not started** / partial |
| CLI drop-in | **Wired** |

Progress rows are **functions/modules ported + tests**, not event tallies.
