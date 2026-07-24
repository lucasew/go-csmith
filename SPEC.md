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
| Multi-seed | Level **B** battery exact body — **SPEC §3.5a** (seed 2 alone is not done) |

Drop-in is **emergent** from fair C++-linked ports and 1:1 function tests. It is **not** achieved by matching RNG event streams with residual burns, invent floors, or packed offline tables.

### 3.1a Event match is a consequence (lock)

**Whole-program RNG stream match and `first_div` climb are never the work item.**

| | |
|--|--|
| **Goal** | Each checklist / C++ unit behaves 1:1 under tests; control flow and entropy use match the cited C++ path |
| **Consequence** | Same flags + seed → same draws → same program body (and thus matching event streams) **if and only if** units are fair |
| **Not the goal** | Optimizing `find-rng-divergence`, event counts, or seed-N first divergence as acceptance |

If stream/body match is missing, the next step is **find the wrong or incomplete C++ unit** and fix that contract — not blank draws, residual pads, or scheduled event climbs.

Thermometers (`find-rng-divergence.sh`, traces, seed diffs) may **report** consequence; they must not **drive** what lands.

### 3.1b Checklist “100%” without match means false marks

**Invariant:** for default options and a fixed seed, if the golden upstream and this port diverge on RNG stream or program body, the checklist is **not** honestly complete — even if every box is `[x]`.

| Situation | Meaning |
|-----------|---------|
| All items checked **and** same flags+seed → same body (streams follow) | Checklist may be done |
| All items checked **but** events or body still diverge | Something was **mistakenly marked done**; it is not 1:1 with C++ |
| Response | **Uncheck** (or treat as open) the responsible unit once known; until then treat “100%” as **invalid / under audit** |

Do **not** invent residual work to force the thermometer green while leaving false `[x]` marks. The fix is re-open the wrong port and restore a real 1:1 contract.

Mass “catch-up” checks without per-item tests are the usual failure mode. A mark requires the §3.4 contract, not “a Go symbol exists.”

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

**Secondary / late (consequence only):** whole-program source match and RNG stream agreement appear **as a byproduct** of correct control flow and fair entropy use. They are never the climb target and never a license to cheat. See §3.1a.

### 3.4 1:1 function contract

For each ported unit:

1. Identify the C++ function/method (file + name; predicate when non-obvious).
2. Define inputs: explicit args, relevant options, and RNG seed or a **fixed draw sequence** when the unit consumes randomness.
3. Define outputs: return values and **observable side effects** (pools, created variables, types, qualifiers, chosen indices, emitted fragments).
4. Test with tables and small harnesses so the same contract can be checked without generating a full program.
5. Prefer vectors captured from instrumented upstream or a narrow C++ harness when behavior is subtle; re-capture when the pin moves.

Event numbers (`eNNNN`) are **debug breadcrumbs**, not evidence and not test names of record.

### 3.5 Whole-program gates (active)

The fair spine **can** emit full default programs. Gates below are **active**.

**Acceptance metric (only this):** bit-identical **program body** vs golden / instrumented upstream for the same default flags + seed.

| In the gate | Out of the gate |
|-------------|-----------------|
| From `static long __undefined` (or first program section after includes) through end of `main` | Header `Generator:` / `Git version:` lines (may differ if documented) |
| Structs, globals, functions, RW comments, blank lines as emitted | **Statistics** tail (`/************************ statistics` …) |
| Exact text, not id-normalized “close enough” | Compile checksum alone without body match |
| | RNG event stream / `first_div` as acceptance |

Timeouts on every generate/compare step. Bounded filter/retry loops only where C++ has them.

### 3.5a Multi-seed parity process (lock — agents must comply)

This section is the **mandatory work method** for multi-seed drop-in. It does **not** replace §3.1a–3.1b; it applies them to whole-program parity.

#### Goal levels

| Level | Definition | Status (update only when verified) |
|-------|------------|--------------------------------------|
| **A** | Defaults + seed **2**: exact program body vs golden | **Met** (2026-07-21; re-verify after every gen/emit change) |
| **B** | Defaults + every seed in the **§3.5a battery**: exact program body | **Met** (2026-07-22; re-verify after every gen/emit change) |
| **C** | Broader seed range (e.g. 0…N or random sample) + optional checksum thermometer | **Met** (2026-07-24; `TestBodyParityLevelC` 10m CLEAN n=458 + battery green; re-verify after every gen/emit change) |

**Done for multi-seed** means **level B**, not A alone. Seed 2 green is **not** permission to stop or to invent pads so other seeds “pass.”

**Body parity harness (integration, not unit marks):**

| Command | Role |
|---------|------|
| `go test ./test/bodyparity -run TestBodyParityBattery -count=1` | Level **B** battery (`testing.T`) |
| `BODYPARITY_LEVELC=10m go test ./test/bodyparity -run TestBodyParityLevelC -count=1 -timeout 15m` | Level **C** sequential random seeds (`testing.T`; no 10s fuzz-worker cap) |
| `go test ./test/bodyparity -run '^$' -fuzz=FuzzBodyParity -fuzztime=30s` | Level **C** quick fuzz (`testing.F`; single inputs >~10s may false-hang) |

Package **`./test/bodyparity`** only (not under `pkg/csmith`). Upstream path is `CSMITH_UPSTREAM` (hard fail if unset/invalid). Gate is **exact pre-stats program body** (§3.5); mismatches report a **go-cmp** line diff (`-upstream +go`). Prefer **`TestBodyParityLevelC`** for substantial level-C time; `FuzzBodyParity` is fine for short smoke. Crashers under `test/bodyparity/testdata/fuzz/` re-run until fixed or removed (delete hang-only false positives that MATCH alone).

#### Frozen battery (level B)

Compare **every** seed in this set under default options (same flags as golden CLI defaults, `-s <seed>`):

```
0, 1, 2, 3, 4, 5, 7, 10, 42, 100, 123, 999
```

| Rule | Detail |
|------|--------|
| Membership | **Do not** shrink the battery to invent a higher pass rate |
| Expansion | May **add** seeds after B is green; never remove a failing seed to claim B |
| Timeout | Generation hard-timeout per side (e.g. ≥60s; ≥300s only when debugging hang). Hang = **FAIL**, not skip |
| Both sides | Same golden pin / instrumented binary as SPEC §3.2 |

#### One-seed work unit (only allowed method)

For **one** battery seed that fails exact body match:

1. Generate Go and upstream. Diff **program body** (§3.5).
2. Take the **first real content difference** (not cosmetics listed in Phase 0 after those fixes land).
3. If types/structs already differ: **stop there** — own the type/struct/pack/field unit. Do not chase later statements or streams.
4. Map the line to a **C++ file + function (+ predicate)**; implement that path with **same-hunk cite** + **§3.4 tests**.
5. Re-check: **this seed**, **seed 2 (must stay green)**, and prefer a quick battery smoke.
6. Commit when that unit is fair — not when `first_div` moved.

| Do | Do not |
|----|--------|
| Fix the owning C++ unit | Climb `first_div` / event counts as the objective |
| Prefer longest shared body prefix among remaining fails | Seed-literal branches (`if seed == 7`) |
| Keep seed 2 green after every change | Invent may-null / pool sizes / blank draws so a seed matches |
| Hang → timeout + investigate control/FP loop | Skip hanging seeds when reporting “B” |

#### Phases (order of attack — do not skip for score)

**Phase 0 — Comparison cosmetics (emit-only, unblocks reading diffs)**

| Unit | Evidence | Action |
|------|----------|--------|
| Empty `/* --- Struct/Union Declarations --- */` always emitted when C++ does | Seeds **1**, **100** content-match once line is present | Fair header/type-list Output |
| `#pragma pack(push, 1)` vs `pack(push)` + `pack(1)` | Seeds **4**, **5** first lines | Fair struct/union pack emit (C++ cite) |

Land with cites. Re-run battery. Remaining DIFFs must be IR/content, not print noise.

**Phase 1 — Re-score after Phase 0**

Expect some seeds to flip to MATCH without more gen fixes. Gate remains **exact pre-stats body**.

**Phase 2 — Early type / struct / union**

| Pattern | Example seeds | Own |
|---------|---------------|-----|
| Go invents packed struct; UP has none | 0, 10 | When types are created / emitted |
| Field type differs | 7, 42, 123 | Type/field choose path |
| Union id/order (`U0` vs `U1`) | 999 | Type gensym / creation order |

**Phase 3 — Mid-body after shared types**

| Pattern | Example seeds | Own |
|---------|---------------|-----|
| Qualifier / volatile on same `g_N` | 4 | CV / create global |
| Array itemize / index form | 5 | Itemize / collective / member Output |
| Shared prefix then size/lattice drift | various | Same class as pre–seed-2 fair unit fixes (facts, visit, Effect) |

**Phase 4 — Hang / pathological gen**

| Seed | Rule |
|------|------|
| **3** (Go hung >300s while UP finished) | Separate bug: finish in the same order of magnitude as UP, **then** body-diff. Do not claim B while any battery seed hangs. |

**Phase 5 — Gate**

1. Level **B**: entire battery exact body.
2. Only then level **C** / CI sample expansion.
3. Checksum / compile scripts remain **thermometers** unless body already matches.

#### Uncheatable reporting

Any agent report of multi-seed status **must** include a per-seed table:

| seed | exact body MATCH/DIFF/TIMEOUT | first real body line (if DIFF) |

| Forbidden report | Why |
|------------------|-----|
| “Seed 2 matches → multi-seed done” | A ≠ B |
| Pass rate after removing hard seeds | Battery is frozen |
| “Events match further” as success | §3.1a — consequence only |
| Id-normalized or RW-stripped body as MATCH | Gate is exact body (§3.5) |
| “Checklist 100%” while battery has DIFF | §3.1b — false marks |

#### Anti-patterns specific to multi-seed (reject)

- `if seed == N` or seed-era sticky flags to force one battery seed
- Residual / invent / blank entropy to force multi-seed streams
- Shrinking or redefining the battery to improve a dashboard
- Treating pack/section cosmetics as “close enough” forever without Phase 0 fixes
- Closing level B while seed 3 (or any battery seed) times out

#### Relation to checklist

CHECKLIST marks stay unit contracts (§3.4). Multi-seed body match is the **consequence** that proves marks are not false (§3.1b). When battery seed S diverges, the incomplete unit is **open** even if its checklist box is checked — re-open that unit; do not invent residual to green the seed.

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
- Multi-seed work that ignores **§3.5a** (battery shrink, seed-literal gen, reporting A as B)

**Allowed**

- Port one C++ path with cite + 1:1 tests
- Multi-seed via **§3.5a** only: first body diff → C++ unit → cite + tests; keep seed 2 green
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
| Full program parity scripts | Thermometers + battery report aid; **acceptance** = exact body (§3.5 / §3.5a), not script exit alone |
| `./test/bodyparity` (`TestBodyParityBattery` / `FuzzBodyParity`) | Upstream vs Go **exact program body** (B/C); go-cmp; `CSMITH_UPSTREAM` required; out of core package tests |

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
| `test/bodyparity/` | Upstream body parity battery + fuzz (slow; not core unit tests) |
| `cmd/csmith`, `internal/cli` | CLI **last** |
| `scripts/` | Thermometers + instrumented upstream build (not integrity gates alone) |
| `third_party/csmith-rng-trace.patch` | Fair measurement aid |
| `SPEC.md` | This document (process + multi-seed §3.5a) |
| `AGENTS.md` | Always-on pointer for agents → SPEC locks |
| `CHECKLIST.md` | Unit inventory; process pointer to SPEC |
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
| 16 | Multi-seed parity process locked in §3.5a (battery, phases, exact body, anti-cheat); agents must not rewrite into stream climb |

## 12. Progress (living)

| Item | Status |
|------|--------|
| SPEC fair-rewrite lock | **Done** |
| SPEC §3.5a multi-seed process lock | **Done** (2026-07-21) |
| Whole-program level **A** (seed 2 exact body) | **Met** — re-verify after gen/emit changes |
| Whole-program level **B** (battery exact body) | **Met** (2026-07-22) — frozen battery exact pre-stats body |
| Whole-program level **C** (expanded range) | **Met** (2026-07-24) — `TestBodyParityLevelC` 10m CLEAN n=458 + battery green; re-verify after gen/emit changes |
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
| Structs + bitfields + field expand | **Partial** |
| Layer 4f: unions | **Partial** — make_random_union, first-field init, LType, decls |
| CFG back-edge gotos | **Not started** |
| CLI drop-in | **Wired** |

Progress rows are **functions/modules ported + tests**, not event tallies.
