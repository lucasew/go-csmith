# go-csmith — Specification

Decisions locked in grilling (2026-07). This is the source of truth for product and process goals.

## 1. Purpose

Go reimplementation of [Csmith](https://embed.cs.utah.edu/csmith/): a **CLI** and **Go library** that generate random, valid C programs for compiler / tool stress testing.

**Not goals:** shipping a C runtime/library package, replacing `csmith.h` packaging, or exposing a public C ABI.

## 2. Product surface

### 2.1 CLI

- Binary name: **`csmith`**
- Drop-in *behavior* for default generation relative to golden upstream (see §3)
- Flag surface should grow toward upstream; see §4 for unimplemented flags

### 2.2 Go API (minimal first)

Primary entrypoint (shape; exact names may match existing code and evolve slightly):

```go
func Generate(ctx context.Context, opts Options) (string, error)
```

- **Input:** context + options struct (mirrors CLI-relevant settings)
- **Output:** full C program as a `string`, same role as upstream Csmith stdout
- **Context:** honor cancel/deadline at coarse generation boundaries (not every RNG draw)
- **Not yet:** public RNG dependency injection (may add later)
- **Not yet:** rich plugins, partial AST export, streaming

Current code may still be `Generate(opts Options) (string, error)` until ctx is wired.

## 3. Correctness / “done”

### 3.1 Drop-in meaning

| Layer | Requirement |
|-------|-------------|
| Default flags + seed `N` | Same **RNG decision stream** as golden upstream |
| Same | Same **bit-identical C source** (see header note) |
| Multi-seed | Defaults hold for a fixed seed range (see gates) |

Header line (`Generator: …`) may intentionally differ only if documented; **program body** must match for source gate.

### 3.2 Golden reference

- Resolve with **`loadDotfilesEnv`** then Nix: `pkgs.csmith` / `pkgs.csmith.src`
- Pin observed at planning time:
  - Package: `csmith-2.3.0-unstable-2026-03-01`
  - Git rev: `0cdc710315cfee9035e22ef4363ca479270d1934`
  - Self-reports: **csmith 2.4.0**
- Do **not** track tip silently; bump pin deliberately when Nixpkgs moves

### 3.3 Engineering metric

- Primary climb metric: **first RNG event divergence** (higher match prefix is better; full stream match is success)
- Then: **source text match** for the same invocation
- Stock `pkgs.csmith` has no RNG trace; use an **instrumented build of the same source** (Nix + in-repo patch, store/gitignored artifacts only)

### 3.4 Acceptance gates (order)

1. Re-baseline tooling against golden + instrumented binary (discard stale scores)
2. Seed **2**, defaults: full **RNG event** match
3. Seed **2**, defaults: **source** match
4. Seeds over **N = 20** (defaults): event + source (or agreed script equivalent)
5. Toward full flag parity (§4 → C)

All generation/compare steps use **timeouts**. Filter/retry loops must be bounded.

## 4. Options policy

| Phase | Policy |
|-------|--------|
| **Now (B)** | Default `Options` path is the parity target. Non-default options that are **not implemented** must **error clearly** (no silent no-ops). |
| **Later (C)** | Full upstream flag semantics |

## 5. Process

| Rule | Detail |
|------|--------|
| Blast radius | Prefer all durable project state **in this repo** (including gitignored build dirs). Do not pollute unrelated host/projects. Read-only use of Nix store / `pkgs.csmith` is fine. |
| Driver | **Agent iterates** measure → one hypothesis → minimal patch → re-measure. |
| Plateau | Stop blind guessing; read **C++** under `pkgs.csmith.src` / `.build/csmith-src` for the failing path; **align Go call flow to Csmith**. |
| Stop | Goal gates met, or human stops the work. **No** “max iterations then abandon.” |
| Ralph loop | **Removed.** No `ralph-loop.sh`, no agent `PROMPT.md` / `MEMORY.md` ritual. |
| Measurement scripts | `find-rng-divergence.sh`, `compare-upstream.sh`, `parity-gate.sh`, `validate-a.sh` (metrics only — not integrity) |
| Integrity | **Not scripted.** Reviewers/agents **read the implementer’s code** and reject gaming (see §5.2). Event match alone is not acceptance. |

### 5.1 Technique — Csmith control flow (required)

**Implementers MUST mirror Csmith’s control flow**, not invent an independent generator that only matches LCG outputs for one seed.

**Goal of a patch:** make Go take the **same branch decisions for the same reasons** as C++ (types, inventory, effects, depth, visit_facts), so RNG *and* source generalize across seeds.

1. Open the relevant C++ under `.build/csmith-src/src/` (e.g. `Expression.cpp`, `FunctionInvocation.cpp`, `VariableSelector.cpp`, `StatementAssign.cpp`, `Lhs.cpp`, `CVQualifiers.cpp`, `FactPointTo.cpp`).
2. Trace the **same function/method sequence** for the failing event (SITE lines / callsites in traces).
3. Patch Go so the **same RNG consumers** run in the **same order** because the **same predicates** hold (e.g. empty `local_vars` → create; `!qfer` → `random_qualifiers`; NewArray → `create_array_and_itemize`).
4. Prefer shared helpers that name the upstream API (`createOnDemand` ≈ `GenerateNewParentLocal` / `CreateArrayVariable`) over ad-hoc coin sequences.
5. Multi-seed: fixes must be **structural** (options, effects, inventory, depth, visit_facts), not seed literals and not event-index multiphase catalogs.
6. **After every structural patch:** re-check **at least two other seeds** (e.g. seed2 + the climb seed). A fix that raises seed5 but breaks seed2 is invalid.

### 5.1.1 Overfitting ban (seed multiphase residual)

**Do not “overfit” a single seed’s event stream.** The following are **rejected** even if `first_div` climbs:

| Reject | Why |
|--------|-----|
| Sticky flags named for **event numbers** or one-shot eras (`postAggGlobalU24AfterArrayOpDone`, `nullValidatePostResidualForBodyAssign`, “after e2151 do U12…”) as the *primary* control mechanism | Encodes seed4/seed5 history, not Csmith state |
| Multiphase counters that switch U/F catalogs by `n`th visit (`GlobalN==7 → U56`, `PLN==11 → residual ladder`) **without** a C++ state variable that increments the same way | Event pack dressed as generation |
| Growing a free-invent residual burn to “exhaust” one seed’s UP stream (`silenceTrace` after ~106k draws) | Event match without real AST / multi-seed path |
| Patch description that only cites `eNNNN` and not the C++ function + predicate | Metric gaming |

**Allowed structural signals** (mirror real C++ state, not event index):

- `expr_depth` / max depth filters, `effectSEFree`, empty params, empty block locals, `max_funcs` reached, type kind (pointer/simple/struct), `must_read` / visit_facts failure, stack size from real function stack model.

**Review test:** “Would this branch fire for **any** seed that hits the same C++ path?” If it only fires because we remembered seed5 event 2151, reject.

### 5.2 Integrity review (read the code — no integrity scripts)

**Do not add scripts that grep for bans as a gate.** Humans and agent **reviewers** open the implementer’s diff and reject work that games metrics. Review is **must-read-diff**, not green CI.

When reviewing a climb/commit, **read** `pkg/csmith/*.go` (and related) and fail the review if you find:

| Reject if present | Why |
|-------------------|-----|
| Packed residual event tables (`f10_late_residual_data.go`, `f10LateResidualPacked`, offline `[]uint32` stream dumps of one seed) | Replays a seed offline instead of generating |
| `residualPlayer` / `burnF10LateExprResidual` as primary stream driver | RNG burns without Csmith Expression/Statement graph |
| Event-indexed / multiphase residual catalogs as primary driver (§5.1.1) | Overfits one seed’s stream |
| `silenceTrace` / `rng.silent` to stop tracing | Fakes event-count match after residual exhausts |
| Seed-literal branches (`if seed == 2`) in generation paths | Non-portable hardcodes |
| Event match with **thin/wrong source** (Go C ≪ upstream structure) while claiming “done” | Events advanced without materializing real AST |
| Coin sequences with no corresponding C++ call path | Unmotivated hacks |
| **Anything that looks like discarding entropy** (see below) | Advances LCG without using the draw |

#### No discarding entropy (unless upstream does too)

**Default:** every `upto` / `flipcoin` / `next31` (and filtered variants) must **use** its result for the same purpose Csmith does: choose a term, accept/reject a filter, build a constant digit, pick a pool index, etc.

**Exception — only when upstream discards too:** a draw may be ignored in Go **if and only if** the corresponding Csmith C++ call path also consumes RNG without using that value for generation (same call site / same reason). The **same hunk** (comment or adjacent code) must cite the C++ function and the reason (e.g. failed candidate still advanced `rand_depth_`, short-circuit after genrand, DFS rollback that re-draws). Matching an untraced LCG step that Csmith performs for a real API (e.g. `RandomHexDigits` digits written into the constant) is **not** discard — the value is used.

##### Automatic reject: looks like entropy discard

**Appearance is enough.** Reviewers and implementers **must reject** a patch (or the offending hunk) if the new/changed code *looks like* Go-only entropy discard — **even if** `first_div` climbs and multi-seed still “holds.” Do not grant benefit of the doubt; do not accept “temporary residual to climb then clean later.” No C++ citation in the same hunk → reject.

| Looks like discard → **REJECT** | Why |
|---------------------------------|-----|
| `_ = r.upto(…)` / `_ = r.flipcoin(…)` / `_ = r.next31()` / `_ = er.pick(…)` / `_ = er.fallback.…` where the blank identifier means the value is unused | Classic discarded draw |
| Assigned then ignored: `v := r.upto(n); _ = v` or draw used only to advance LCG before a forced branch | Same as blank discard |
| Inventory / pool **floors and pads** that invent `n` for `upto` without materialising that many C++-visible vars: `nArr++`, `if nArr < 5 { nArr = 5 }`, `if nCtrl < floor { nCtrl = floor }`, `n = 13` / `n = 18` / `n = 37` one-shots, sticky “pad Global U21 forever” | Fake choose space; not GlobalList / local_vars |
| Multiphase residual ladders: sequences of blank draws keyed to `eNNNN` / phase counters (`for` of `_ = r.upto` after “UP did U4 U1 U2 F0…”) | Event pack without AST / inventory |
| Untraced `next31` “gap fills” to match `rand_depth` without writing digits / calling the real Constant path | Fake LCG sync |
| `burn…Residual` / one-shot sticky flags whose body is only RNG burns and not a real create/select that updates inventory | Residual as primary driver |
| Drawing a value then ignoring it to force a different path when C++ uses the value | Divergent control flow + discard |
| Commit/PR that only cites event numbers and score, with blank-draw hunks and no C++ path | Metric gaming |

**Does not look like discard (allowed when true):**

- Draw stored and used: index into a live pool, bool for branch, digit appended to constant, type pick that changes emitted AST.
- `uptoWithFilter` / VectorFilter retries where **tries** match upstream and the **accepted** index drives selection.
- Real create/select that fails visit_facts and retries (upstream does this) — the failed attempt’s draws were used for that attempt’s structure.
- Csmith discards at that site — Go implements **that same path**, with a same-hunk C++ cite (not a free-standing pad).
- Untraced `RandomHexDigits` → digits written into constant text (value used).

**Review heuristic (fail closed):**

1. If removing the RNG call would not change generated C (only the event stream) → **reject** as discard, unless the same hunk cites matching C++ discard.
2. If the draw’s only role is “so the next event number matches UP” → **reject**.
3. If pool size is a constant floor / pad / `++` instead of counting materialised inventory → **reject**.
4. Prefer implementing the real C++ path (predicate + callee + inventory update) over any pad.

**Implementer rule:** do not land a climb that contains looks-like-discard hunks. If the only way you can raise `first_div` is blank draws or floors, **stop and re-diagnose** the C++ inventory/control path — the climb is invalid even when multi-seed still passes.

**Review process:** after a patch, a separate pass **reads the code** (diff + call graph around the fix) and confirms it maps to Csmith C++ methods — not only that `find-rng-divergence` score improved. Explicitly scan for looks-like-discard shapes above. **Reject the whole patch** if any new looks-like-discard hunk lacks a same-hunk upstream-discard cite. Score alone never overrides this.

Allowed: temporary debug prints; timeouts; bounded retries that still call real create/select APIs; instrumented upstream for measurement only.

### 5.3 Technique constraints

Any remaining technique must:

1. Stay inside the repo’s blast radius
2. Have timeouts against non-termination
3. Converge by consulting C++ (not unmotivated RNG hacks)
4. **No discarding entropy unless upstream does too** — every RNG consumer either feeds a real decision/materialised value or mirrors a documented C++ discard at the same site (§5.2)
5. **No seed overfitting** — prefer C++ predicates and shared helpers; reject event-indexed multiphase residual as the main strategy (§5.1.1)

**Order of preference for a divergence:**

1. Identify the **C++ function + predicate** that should fire (inventory empty? depth block? SE-free? visit_facts fail?).
2. Implement or fix that predicate and the real callee (create/select/Expression), including **inventory materialisation** (GlobalList / local_vars / derived_types) so later `upto(n)` uses a true pool size.
3. **Do not** ship temporary residual multiphase / blank-draw pads. Diagnostics only (prints, traces) — never `_ = r.…` packs as a climb vehicle.

Residual multiphase catalogs that only burn stream without a C++ counterpart are **out**. Reimplement the C++ path that produces those draws. Anything that **looks like** entropy discard (§5.2) is **rejected** even if first_div rises.

## 6. Explicit non-goals (current)

- C library / packaging of full Csmith runtime as a replaceable install tree (users may compile output against existing `csmith.h` from `pkgs.csmith` or upstream)
- Automated Claude “Ralph” score loops
- Silent acceptance of unimplemented CLI flags
- Public RNG injection API (deferred)

## 7. Repo layout (relevant)

| Path | Role |
|------|------|
| `cmd/csmith` | CLI |
| `pkg/csmith` | Generator library |
| `internal/cli` | Flag wiring |
| `scripts/` | Parity / validation (no Ralph) |
| `SPEC.md` | This document |

## 8. Changelog of decisions (grill)

### 8.1 Original lock (2026-07)

1. Goal = feature parity, drop-in replacement (not “random C is fine”)
2. Bit-identical default output via RNG stream alignment
3. Golden = current `pkgs.csmith` / `.src` (not classic 2.3.0 tarball alone)
4. Observe upstream RNG via instrumented same-src build (Nix-friendly, clean tree)
5. Flexible technique; in-repo; timeouts; no give-up
6. CLI + Go string API; not C package
7. Minimal Go API first; RNG DI later
8. Unimplemented non-defaults error (B → C later)
9. Gates: seed 2 events → seed 2 source → 20 seeds
10. Agent-driven iteration; remove Ralph loop
11. Binary name: `csmith`

### 8.2 Closed after SPEC §5.1.1 / §5.2 hardening (post-`cf7198d` / `eaad659`, 2026-07-16)

**Anchor:** last pre-closure SPEC integrity edits were `cf7198d` (looks-like entropy discard fail-closed in §5.2) and `eaad659` (residual climb vehicle banned; living integrity line). Everything below was locked in **continued grilling while climbing seed5** after those commits. They **do not reopen** §8.1 or product goals; they close process gaps that showed up in agent execution.

#### What grilling closed (durable)

| # | Decision | Status |
|---|----------|--------|
| 12 | **Looks-like entropy discard is fail-closed forever** — Appearance is enough to reject (`_ = r.upto`, invent floors, multiphase residual ladders, untraced gap-fills). No “temporary pad to climb then clean later.” Reaffirmed when a climb pack (`U2 U7 F50…` invent choose) was introduced after §5.2: **strip the pack**; do not argue score. | **CLOSED** |
| 13 | **No residual as climb vehicle** — Preference order is only: C++ predicate → materialise inventory + real callee. Diagnostics = traces/prints only. Blank-draw multiphase is never a step on the path to “done.” | **CLOSED** (`eaad659` + reaffirmation) |
| 14 | **Integrity = read the diff** — No integrity scripts as acceptance. Multi-seed hold + `first_div` climb are **metrics**, not a substitute for code review against §5.1–5.2. | **CLOSED** (reaffirmed) |
| 15 | **Deviations are often non-RNG** — When GO and UP share the same raw but differ on `n` (e.g. U1 vs U3), diagnose **C++ state** (inventory, `Function::stack`, effect filters), not invent pool size. **GDB / instrumented C++ inspection is preferred** over smoke unit tests of isolated Go helpers for parity. | **CLOSED** |
| 16 | **Parity measure stays the instrumented stream** — Climb with `scripts/find-rng-divergence.sh` (and related parity scripts). Do **not** substitute ad-hoc C++ unit “smoke” or Go-only unit tests as the proof of 1:1. | **CLOSED** |
| 17 | **Multi-seed hold is mandatory after every patch** — At least seeds **2, 4, 6** (and 3 when cheap) must keep full event match when climbing seed5. A seed5 climb that breaks 2/4 is **invalid** even if first_div rises. | **CLOSED** |
| 18 | **`pure_rnd` / DefaultRndNumGenerator untraced `next31` is not invent pad** when it mirrors C++ `pure_rnd_upto` / hex digit generation at the same API site (value is used by upstream). Still reject free-standing next31 packs with no C++ cite. | **CLOSED** |
| 19 | **Done = COUNT=20 SEED_START=2 parity-gate exit 0** — Keep grinding until that gate; no “good enough first_div” stop. Source match and residual-debt strip remain open work after events. | **CLOSED** (goal; work ongoing) |
| 20 | **Scheduled continue loops are process only** — Recurring agent “continue” / “commit and continue” jobs do not change integrity rules; each fire is still subject to §5.1–5.2. | **CLOSED** |
| 21 | **Invent packs are stripped on sight** — If a patch lands blank invent choose / multiphase `U2 U7 F50…` (or similar) to force stream match, **revert/strip** it even when `first_div` rose. Do not leave it as temporary debt. | **CLOSED** (reaffirmation of 12–13; applied after e6316 invent attempt) |
| 22 | **Empty create vs live choose is an inventory bug** — Class: UP `choose_ok_var Un` vs GO empty `create` F50 (e.g. seed5 e6316). Fix is **materialise** the C++-visible pool so choose is live, **or** true empty → full `GenerateNewParentLocal` / create path with **used** draws. Not invent-`Un` floors. | **CLOSED** (diagnosis rule; fix work open) |
| 23 | **`select_array` / loop-ctrl pool size = materialised lists** — GDB on C++ `select_array` is authoritative (GlobalList + ParentLocal arrays after effect filters). Invent `nArr++` / floors that break seeds 2/4 are invalid even if seed5 climbs. Same rule for `SelectLoopCtrl` / `Function::stack` sizes (`parentStackPick` must reflect real stack depth). | **CLOSED** (e6263–e6286 class) |
| 24 | **1:1 means events then source under structural path** — “Matched through eN” is progress only. Full drop-in requires event match **and** bit-identical program body for the gate seeds, via C++-like control flow — not residual exhaust + thin AST. | **CLOSED** (reaffirmation of §3 + §5.1) |

#### Explicit rejections locked in the same sessions (do not reopen)

- Residual multiphase as primary climb strategy (§5.1.1) — still banned.
- `silenceTrace` / seed-literal branches / event-only “done” — still banned.
- “Why discarding entropy?” answer locked: **we do not**, except same-hunk C++ discard cite; looks-like is enough to reject.
- Unit/smoke tests of isolated Go helpers as proof of parity — rejected in favor of instrumented stream + GDB on C++ state.
- Stopping at a high seed5 `first_div` without COUNT=20 — rejected.

**Explicitly still open (not closed by grill):** seed5 full event stream (living first_div ~8106); seeds 7–21 events; seed2/4 **source** match; stripping legacy residual/`silenceTrace` debt already in tree; full flag parity (§4 phase C). Work continues under the closed rules above.


## 9. Progress (living)

| Gate | Status |
|------|--------|
| Instrumented upstream build | `scripts/build-instrumented-upstream.sh` → `.build/csmith-instrumented/` |
| Seed 2 re-baseline | Running vs golden `0cdc710` / csmith 2.4.0 |
| Seed 2 event match | **PASS** — full **37939/37939** |
| Seed 2 source match | **FAIL** — residual-driven path; not full Csmith-flow AST |
| Seed 3 event match | **PASS** — full **64/64** |
| Seed 4 event match | **PASS** — full **106117/106117** (historical residual climb; integrity debt remains) |
| Seed 6 event match | **PASS** — full **23/23** (SelectParentLocal empty create + Block max=0 append_return) |
| Seeds 5,7–21 event | **FAIL** — seed5 first_div **~8506** (post invent residual FuncPLU1 * qfer + NewArray F20 F20)|
| 20-seed gate | **OPEN** — COUNT=20 SEED_START=2; event-only seed2/3/4/**6** PASS |

**Integrity:** reviewers **read the implementer diff** (no integrity scripts). Reject residual packs, event-indexed multiphase overfitting (§5.1.1), `silenceTrace`, seed hardcodes, event-only climbs, and **anything that looks like entropy discard** (§5.2 — blank `_ = r.…`, inventory floors/pads, multiphase residual ladders, untraced gap-fills). Appearance is enough to reject; same-hunk C++ discard cite required to keep a blank draw. Require call flow aligned with Csmith C++ **predicates + methods**, not seed event numbers; draws must be **used** or mirror documented upstream discard at the same site. `first_div` climb alone is **never** acceptance.




**seed5 e8199→8220 climbed — residual VS empty-params filter multiphase:**
1. Capture: e8199 U100 tries=2 v=0 (raw 361144200) vs GO Statement AssignOps U120; intermediate raws 85/83 are ParentParam rejects.
2. C++: VariableSelectFilter rejects ParentParam when params empty (VariableSelector.cpp); then Global choose U2 + further VS/stack U1 U4 ladder before free Assign e8209.
3. GO: inventArrayOpExprEmptyParamsVS + variableScopePickFromER (tries=2) + U2 U100 U1 U4×… through e8208. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8220 U4 vs GO U7 (live Global int16 pool after stripping permanent ParamU7 invent floor at invent residual arm).

**Integrity — clear ParamU7 invent floor at invent residual then-body arm:**
1. nullValidatePostResidualParamU7 sticky pick(7) from e1068 survived through invent residual (e8130+); permanent invent floor.
2. Clear ParamU7 when invent residual then-body arms; e2050 era still has pad. Seeds 2/4/6 held.

**seed5 e8220→8230 climbed — invent residual PP U4 visit-fail Expression retry:**
1. Capture: e8220 UP U4 vs sticky post-Return invent pick(7) sole → Lhs F80; after U4, UP Expression U120 (visit fail) vs GO accept→Lhs.
2. C++: ExpressionVariable ParentParam choose_ok_var then visit_facts miss → Expression do-while term retry U120.
3. GO: clear post-Return invent residual at invent residual arm; termVariable PP burn U4 + restoreGenSnapshot continue exprTries. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8230 — climbed.

**seed5 e8230→8238 climbed — invent residual Global U5 visit-fail VS reselect:**
1. Capture: e8230 UP Global choose U5 then VS reselect U100 PP + U4 + Expression U120; GO live eFlexible GlobalList ~41 overcount accept→Lhs.
2. C++: ExpressionVariable do-while after visit_facts miss reselects VariableSelector (U100) not term U120 first.
3. GO: one-shot after invent residual PP path — burn U5 + VS reselect + PP U4 + continue exprTries. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8238 — climbed.

**seed5 e8238→8240 climbed — invent residual Lhs SelectDeref empty → VS PP sole:**
1. Capture: e8237 F80=1 then UP U100=92 (VS) vs GO U5 pointer choose (same raw). UP empty ok_vars (no choose U, create skipped) → VS ParentParam sole → free Expression U120=118 Comma.
2. C++: Lhs.cpp SelectDerefPointerProb then select_deref_pointer empty → VariableSelector::select; e8163-class PP sole (no PP→PL U2 create).
3. GO: after Global U5 residual arm empty SelectDeref (no invent Un floor); re-arm LhsPPSole + SkipCommaType for free Comma left (skip AllTypes U16). **Integrity residual debt** — inventory overcount compensation. Seeds 2/4/6 held.
4. Next: e8240 — climbed.

**seed5 e8240→8367 climbed — invent residual free Comma nested Assign Lhs SelectDeref empty:**
1. Capture: e8239 U120=118 Comma; UP nested Assign Lhs F80 empty multiphase (VS Global U8→U7, empty create F50 F10 F50 F20 F20 U3/U4, NewArray CreateArray U99, F80 U3 / VS PL U1 U3…) vs GO Comma left Function term.
2. C++: ExpressionComma left ExpressionAssign need_no_rhs Lhs.cpp do-while select_deref empty + create (VariableSelector.cpp:1266–1315).
3. GO: termComma invent residual SkipCommaType burns Lhs SelectDeref empty multiphase then Comma rhs. **Integrity residual debt** — multiphase residual pack. Seeds 2/4/6 held.
4. Next: e8367 — climbed.

**seed5 e8367→8476 climbed — extend post-CreateArray Lhs F80 do-while + NewValue create:**
1. Capture: e8367 F80=0 Lhs continue vs residual early exit to Comma rhs U120; then LCG-driven F80 U3 / VS PL U1 U3 / Global U6·U5 / NewValue PL create F10 U1 U14 F50 F20 F50 F50 U3 + SafeOpFlags F50 U4.
2. C++: Lhs.cpp do-while after CreateArray visit fails reselects SelectDeref/VS until NewValue create accepts; need_no_rhs SafeOpFlags.
3. GO: extend post-CreateArray loop (no j>=3 early break); NewValue→PL create + SafeOpFlags then Comma rhs. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8476 — climbed.

**seed5 e8476→8494 climbed — post-NewValue VS multiphase before Comma rhs Function:**
1. Capture: e8476 UP U100 VS ladder (PL empty reselect, tries=1/8, Global U12) vs GO Comma rhs term U120 same raw.
2. C++: VariableSelector multiphase after create tail before free Expression term Function (e8492 U120=62).
3. GO: sequential VS residual with scopePickTries (uptoWithFilter tries+1 call-path) then Comma rhs. **Integrity residual debt** — event-sequential multiphase. Seeds 2/4/6 held.
4. Next: e8494 — climbed.

**seed5 e8494→8499 climbed — Comma rhs Function useExisting inventory undercount F20 U16:**
1. Capture: e8493 F50=1 useExisting; UP F20 + U16 choose_func then param Expression; GO sole/empty inventory skipped choose → Variable U100.
2. C++: Function::choose_func BuiltinFunctionProb-shaped F20 + get_one_function U(n) among ok_funcs then build_invocation params.
3. GO: invent residual FuncF20U16 residual on useExisting (even if GO candidates non-empty undercount); skip ThenBody F30 under nested userFuncNest. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8499 — climbed.

**seed5 e8499→8504 climbed — Function-fail PP miss → PL stack U1 + int* qfer:**
1. Capture: e8498 U100=68 PP; UP empty miss → stack U1 + GenerateNewParentLocal * qfer F50 F10×2 F20 F20; GO sticky ThenBodyEver pick(4) on Function-fail path (paramCands non-empty).
2. C++: SelectParentParam empty/miss → SelectParentLocal (VariableSelector.cpp:1052–59) stack + create.
3. GO: Function-fail PP path — skip ThenBodyEver U4 when FuncPLU1; force stack U1 + create int* qferMode 1 retype=false. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8504 — climbed.

**seed5 e8504→8506 climbed — FuncPLU1 PL create * levels clamp (not GlobalU21 **):**
1. Capture: after U1, UP * qfer F50 F10 + self F50 F10 then NewArray F20 F20; GO sticky freeMultiIVNeedNoRhsPostEAGlobalU21 floored levels to 2 (extra F50 F10) then NewArray.
2. C++: GenerateNewParentLocal for int* uses 1-level random_qualifiers + self then make_init NewArray.
3. GO: invent residual FuncPLCreateLvl1 one-shot levels=1 before GlobalU21/StmtEra floors. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8506 F80 F10 F50 F20 F20 residual after address init vs GO Expression U120.

**seed5 e8190→8199 climbed — second-create post-Constant multiphase (char hex + VS U2 U2 + Constant):**
1. Capture: after eLongLong+2 Constants, UP e8190 F50=0 depth gap 2 (eChar hex), U100 Global, U2 U2, Constant small U3, eChar hex, U100; GO residual ended → Statement early.
2. C++: Constant.cpp GenerateRandomCharConstant RandomHexDigits(2); Lhs do-while continues VS/choose without F80 gap (same class as e8178).
3. GO: burnSimpleConstant eChar; variableScopePick + pick(2)×2 + Constant + eChar. **Integrity residual debt** — invent multiphase. Seeds 2/4/6 held.
4. Next: e8199 — climbed.

**seed5 e8188→8190 climbed — second NewValue create eLongLong hex next31×16:**
1. Capture: e8183 F50=0 depth gap 16 next31 then small U3 raw 1566741184; GO vol F50 + uint8/int32 under-burn desynced LCG → F50 mismatch at e8188.
2. C++: Constant.cpp GenerateRandomLongLongConstant RandomHexDigits(16); second create no WRITE vol before Constant (qfer set).
3. GO: drop invent vol; burnSimpleConstant eLongLong hn=16 then two more Constants. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8190 F50 residual — climbed.

**seed5 e8178→8188 climbed — invent residual NewValue PL create hex/LCG + multiphase:**
1. Capture: after free Expression nest, Lhs F80=0 NewValue PL create; Constant hex next31 desynced LCG; second create visit-fail multiphase stack U2.
2. C++: GenerateNewParentLocal eSimple retype + Constant RandomHexDigits; Lhs do-while continues after visit fail.
3. GO: eSimple pickSimpleNonVoid + burnSimpleConstant (hex next31); second VS Global miss → NewValue stack U2 + Constants. **Integrity residual debt**. Seeds 2/4/6 held.
4. Next: e8188 second Constant F50 residual — climbed.

**seed5 e8157→8178 climbed — invent residual For body Global multiphase + free Expression nest:**
1. Capture: 2nd For-body Function-fail Global U10 F0 reselect U9; Lhs F80 U4 → VS PP sole; free Expression Function CREATE F50 F30 F0 + Constant; parent Lhs F80=0 NewValue PL create.
2. C++: ExpressionVariable do-while visit fail; StatementAssign Lhs ParentParam; free Expression after Lhs; parent ExpressionAssign Lhs SelectDeref F80=0 → NewValue.
3. GO: multiphase ForGlobalN (U5 then U10 F0 U9); LhsEmptyCreate / LhsDerefU4 / LhsPPSole; freeMultiIVForLhsExprContinue + user Function CREATE residual; post-nest F80=0 NewValue PL. **Integrity residual debt** — pads/floors. Seeds 2/4/6 held.
4. Next: e8178 LCG/create residual — climbed.

**seed5 e8130→8157 climbed — invent residual then-body For body Global U5 + Lhs empty create:**
1. Capture: For body Assign Function-fail → ExpressionVariable Global UP choose U5 then Lhs SelectDeref empty create F80 F10 F50 F20 F20; GO live Global U3 then Lhs live pointer U5.
2. C++: Function-fail ExpressionVariable SelectGlobal eFlexible ~5 GlobalList; Lhs empty create random_add (VariableSelector.cpp).
3. GO: sticky ForBody first Global U5; arm LhsEmptyCreateN=12 (F80 F10 F50 F20 F20 only). **Integrity residual debt** — U5 pad. Seeds 2/4/6 held.
4. Next: e8157 Global multiphase — climbed.

**seed5 e8106→8130 climbed — invent residual then-body Global residual ends Expression → Statement For:**
1. Capture: after Global expand F50 U64 U4, UP StatementProbability For (U100=29) + SelectLoopCtrl U10 + loop_control; GO continued free Expression (binary Function wanted RHS).
2. C++: Expression nest ends after residual Variable (parent binaries complete); then-body Statement For SelectLoopCtrlVar among ~10 int visibles + make_random_loop_control.
3. GO: after residual, arm `postAggUnwindBinaryAfterExprVar` + `ppPostPadSkipParentExprN` (e4332/e2126 family) so nested binary Function operands return without SafeOp/RHS RNG → Statement For; one-shot SelectLoopCtrl U10 (not sticky post-Return U27). **Integrity residual debt** — U10 floor + invent F50 U64 U4; replace with real inventory/visit path. Seeds 2/4/6 held.
4. Next: e8130 For body Global U5 — climbed.

**seed5 e8105→8106 climbed — invent residual Global expand post-choose F50 U64 U4:**
1. Capture: after expand U51, UP residual F50 U64 U4 then Statement For; GO Expression ended after U51.
2. C++: residual multiphase after Global expand (e7433 F50 U64 family) then Expression ends → Statement.
3. GO: after expand choose, residual F50 U64 U4 (not chooseOKVar itemize-only). **Integrity residual debt** — replace with visit_facts/itemize structural path. Seeds 2/4/6 held.
4. Next: e8106 Statement For — climbed.

**seed5 e8102→8105 climbed — invent residual then-body Global expand U51:**
1. Capture: free Expression Variable Global UP choose_ok_var U51 (eFlexible expand) vs GO thin U4/U53.
2. C++: SelectGlobal Nonvolatiles + expand_struct_union_vars (e7429 class GlobalList).
3. GO: invent residual then-body reuses inventArrayOp Global expand pool; trim overcount to 51. Seeds 2/4/6 held.
4. Next: e8105 post-choose residual — climbed.

**seed5 e8082→8102 climbed — invent residual then-body PL stack U1 create:**
1. Capture: free Expression Variable PL after free Expression nest — UP stack U1 + retype U14 create; GO sticky freeMultiIVNeedNoRhsIfBody U5 + choose U3 sole.
2. C++: Function::stack.size()=1 after residual nest (not IfBody overcount U5).
3. GO: inventArrayOpPostNestBreakThenBodyPLStackU1 overrides needNoRhsIfPL U5 U3 with U1 + empty create retype+qfer. Seeds 2/4/6 held.
4. Next: e8102 Global U51 — climbed.

**seed5 e8036→8082 climbed — invent residual free Expression nest NewValue + multiphase Constant + Lhs F80:**
1. Capture: after depthBlock Variable, UP VS empty-params NewValue F10 PL retype U14 qfer create; then Constant×2; Lhs SelectDeref F80 F20 F20 U4 before then-body Statement.
2. C++: VariableSelectFilter rejects ParentParam when params empty; NewValue always creates (not sticky U15 choose); free Expression residual continues before then-body.
3. GO: nullValidateEmptyParamsVS for first nest Expression only; postEAReturnPL scopePick=4 force create retype+qfer; multiphase Constant with emptyParamsDepthBlock cleared; Lhs F80 residual. Seeds 2/4/6 held.
4. Next: e8082 then-body PL U1 — climbed.

**seed5 e8035→8036 climbed — invent residual free Expression nest after If condition Constant:**
1. Capture: after condition Constant residual, UP free Expression U120 tries=9 Variable (depthBlock); GO jumped to then-body Statement U100.
2. C++: residual SelectDeref / high expr depth continues free Expression before then-body Statements.
3. GO: after invent residual Constant residual, arm depthBlock + forceNoFunc and burn nested randomTypedExprDepthFlags. Seeds 2/4/6 held.
4. Next: e8036 empty-params NewValue — climbed.

**seed5 e8027→8035 climbed — invent residual If condition forceNoFunc + PL U3 empty → Constant:**
1. Capture: free Statement IfElse condition Expression UP U120 tries=1 Variable (Function filtered) then PL U1 U3 empty → Constant tries=4 F50 F50 U20; GO Function tries=0 stdfunc / U15 live PL.
2. C++: residual SelectDeref multiphase leaves Function disallowed; PL choose_ok_var empty → Expression term retry Constant.
3. GO: ppPostPadForceNoFunc after GlobalU51 residual; inventArrayOpPostNestBreakExprPLU3 burns U1 U3 then Constant-only term filter + Constant residual. **Integrity note:** Constant force overrides default !ConstAsCondition noConst for this residual stream (UP accepts Constant); natural Assign path desyncs. Seeds 2/4/6 held.
4. Next: e8035 free Expression nest — climbed.

**seed5 e7979→8027 climbed — invent residual free Statement Global U51 SelectDeref create:**
1. Capture: free Statement U100=30 (GO maps Return) → UP Global expand choose_ok_var U51 + SelectDeref empty create NewArray U99 + F0 F80 U6 multiphase; GO Return VS U100.
2. C++: Assign-shaped Lhs Global expand + select_deref_pointer empty create_and_initialize (visit_facts F0 retries).
3. GO: inventArrayOpPostNestBreakNextGlobalU51 residual on stmtReturn path (table maps 30→Return while stream is Assign Lhs); U51 + F80 create + F0 F80 U6 ladder. Seeds 2/4/6 held.
4. Next: e8027 forceNoFunc — climbed.

**seed5 e7957→7979 climbed — invent residual SelectLType U23 + multi-level Function-fail PL + Lhs:**
1. Capture: free Assign SelectLType F20=1 choose_random_pointer_type UP U23 vs GO U18 (same raw); then Function-fail EV for ** LType → GenerateNewParentLocal F50 F10×3 F20 F20 then Lhs F80 random_add…; GO under-noted derived_types + sole/F80-fail→VS.
2. C++: Type::choose_random_pointer_type(derived_types.size()); find_pointer_type deepen; ExpressionFuncall fail → ExpressionVariable empty choose_var → GenerateNewParentLocal SE-free READ qfer; Lhs select_deref empty create random_add.
3. GO: invent residual bare floor n=23 (no pad loop — noteDerivedPointer base-keyed no-ops hang); multi-level Function-fail PL create qfer levels=2 + skip address residual; LhsEmptyCreateN=11 F80 F10 F50 F20×2 F50 F20×2. Seeds 2/4/6 held.
4. Next: e7979 Global U51 — climbed.

**seed5 e7945→7957 climbed — invent residual Break free Assign Function-fail EV + Lhs:**
1. Capture: next free Assign Function useExisting miss at max_funcs → ExpressionVariable PL U1 sole + Lhs empty create F80 F10 F50 F20 F20 U2; GO F30 FuncCreateAttr residual then wrong Lhs.
2. C++: ExpressionFuncall failed → ExpressionVariable; Lhs select_deref_pointer empty create.
3. GO: inventArrayOpPostNestBreakStmtEra skips freeMultiIVPostEAFuncCreateAttrDone; Function-fail PL sole; LhsEmptyCreateN=10 single empty+U2. Seeds 2/4/6 held.
4. Next: e7957 SelectLType U23 — climbed.

**seed5 e7933→7945 climbed — invent residual Break Lhs multiphase phase1 + free Statements:**
1. Capture: after CreateArray Lhs, UP F80 random_add F10 F50 NewArray=0 address accept then Statement U100 Assign; GO accepted Lhs early then BlockSize U4 / Expression residual.
2. C++: Lhs do-while third empty create accepts; Block continues free Statements (not must_return).
3. GO: LhsEmptyCreateN phase 1 third create accept; NeedStmtN + skipNextBlockSize for more free Statements. Seeds 2/4/6 held.
4. Next: e7945 Function-fail EV — climbed.

**seed5 e7919→7933 climbed — invent residual Break Assign Lhs SelectDeref empty create:**
1. Capture: after RHS PL * create, Lhs SelectDeref F80 empty → random_add F10 F50 + NewArray=0 initNull F0 fail → second F80 create NewArray CreateArray vs GO F80=0→VS.
2. C++: select_deref_pointer empty (VariableSelector.cpp:1266–1315) random_add_qualifiers + create_and_initialize; Lhs do-while after visit fail.
3. GO: inventArrayOpPostNestBreakLhsEmptyCreate multiphase after RHS PL create. Seeds 2/4/6 held.
4. Next: e7933 Lhs phase1 — climbed.

**seed5 e7909→7919 climbed — invent residual Break Assign RHS Global visit-fail reselect:**
1. Capture: Assign RHS Expression Variable Global U100 sole/visit fail → UP reselects U100 ParentParam→PL * create (F50 F10×2 F20 F20 U3) vs GO empty Global F20.
2. C++: ExpressionVariable do-while after visit_facts fail; SelectParentParam miss → SelectParentLocal stack + GenerateNewParentLocal SE-free * qfer + make_init address choose_ok_var U3.
3. GO: inventArrayOpPostNestBreakExprGlobalReselect + single-level * (QferLvl1) + address U3. Seeds 2/4/6 held.
4. Next: e7919 Lhs empty create — climbed.

**seed5 e7901→7909 climbed — invent residual Break continues block + post-itemize U4:**
1. Capture: after S0 CreateArray itemize field_vars, UP U4 then Statement U100 Assign vs GO Statement U100 (lastStmtWasReturn stopped block).
2. C++: StatementBreak is not must_return — Block keeps free Statements; after create_array_and_itemize, choose_ok_var U4 among ~4 int-compatible ok_vars then next StatementProbability.
3. GO: burn U4 after invent Break S0 create; do not set lastStmtWasReturn/skipNextBlockSize. Seeds 2/4/6 held.
4. Next: e7909 Assign VS reselect — climbed.

**seed5 e7449→7901 climbed — invent residual Break CreateArray struct S0:**
1. Capture: after CreateArray sizes, UP F50 field Constants then U120 init_num vs GO initNum U120 first (int32 element).
2. C++: ArrayVariable::CreateArrayVariable — if aggregate, create_field_vars (per-field Constant::make_random) before pure_rnd_upto(total/2) init_num; alts Constant::make_random(type). Seed5 S0 = uint64_t + int8_t.
3. GO: invent residual Break create type struct S0 (not int32); formatAggregate make_init; burnCreateFieldVarsConstants before init_num via isAgg path. Seeds 2/4/6 held.
4. Next: e7901 post-itemize U4 — climbed.

**seed5 e7433→7449 climbed — invent residual Break ExpressionVariable create:**
1. Capture: after invent residual Break, UP U1 F20 Constant×2 CreateArray vs GO bare break + Statement.
2. C++: StatementBreak Expression forced eVariable → PL stack U1 + empty GenerateNewParentLocal NewArray create multiphase.
3. GO: inventArrayOpPostNestBreakEV parentStackPick U1 + create qferMode 0; stop block after Break. Seeds 2/4/6 held.
4. Next: e7449 CreateArray create_field_vars — climbed via S0 type.

**seed5 e7429→7433 climbed — invent free Expression Global expand ok_vars U51:**
1. Capture: e7429 UP choose_ok_var U51 vs GO U66/U49 under/over inventory.
2. C++: free Expression simple SelectLType → SelectGlobal expand_struct_union (bitfields) + eFlexible; GlobalList includes residual CreateArray orphans; dual-emitted PL tops not on GlobalList.
3. GO: invent residual pool = mergedGlobals + array/simple orphanGlobals expand, drop pointer matches, drop dual-emitted PL tops (keep fields), dedup expr → U51. Seeds 2/4/6 held.
4. Next: e7433 PL F50 U64 — climbed.

**seed5 e7411→7429 climbed — invent ArrayOp Lhs SelectDeref create residual + free Expression nest:**
1. Capture: e7423 UP PL stack U1 vs GO sticky freeMultiIVForLhsExprContinue U2; then e7425 empty-params VS filter; e7426 live GlobalNonvolatiles U12.
2. C++: after invent ArrayOp Lhs second-PP miss → SelectDeref create residual F80…U3 accept, free Expression nest (PL stack U1 sole, empty-params VariableSelectFilter → Global live U12) without PostNestLhs SelectDeref F80 ladder.
3. GO: SecondPPMiss residual arm; residual PL intercept uses `parentStackPick` (not bare pick(2)); invent residual sticky empty-params VS + live Nonvolatiles choose; free Expression loop without PostNestLhs. Seeds 2/4/6 held.
4. Next: e7429 UP Global eFlexible U51 — climbed.

**seed5 e6024→6046 climbed — post-NewArray PL choose U14 + PP multiphase U7/sole/U7:**
1. Capture: e6024 UP choose_ok_var U14 vs GO sticky multiphase U13 (post-Return PL pn≥2).
2. C++: after NewArray+address Lhs CreateArray residual, free Expression PL choose pool is U14 again (inventory grew). PP after residual: 0 U7, 1 sole, 2+ U7 live again (not sticky always-sole after first sole).
3. GO: after `LhsNewArrayVSDone`, PL pads U14; `PostNewArrayPPN` is 0→U7, 1→sole, 2+→U7. Seeds 2/3/4/6 held.
4. Next: e6046 UP U120 tries=0 (Assign→F80 Lhs) vs GO U120 tries=1 after F50.

**seed5 e5984→6024 climbed — SelectDeref NewArray address nested create + Lhs VS itemize + PP sole:**
1. Capture: e5984 UP F20 (nested address create under SelectDeref NewArray) vs GO U99 CreateArray.
2. C++: ExpressionAssign Lhs SelectDeref empty create NewArray=1 + make_init address → choose empty → nested `create_and_initialize` for pointer pointee (NewArray F20 + make_init address F20 + choose_ok_var U3) then outer CreateArray U99 U10 U4 itemize U8. SelectDeref continue F80=0 → VS ParentLocal stack U1 + multi-dim itemize U8 visit fail → SelectDeref live U8×2 F80=0 → NewValue F10→PL create F20 F20 U3 accept. Free Expression Function-fail/PP after: first live U7 then sole (not sticky multiphase always-U7). Sticky `ppPostPad≥14` bare CreateArray and ParamU7 GlobalU21 force U6 skip the nested residual.
3. GO: after `freeMultiIVNeedNoRhsPostEAReturnGlobalF0Done`, NewArray+address burns nested F20 F20 U3 before CreateArray; F80=0 VS residual stack U1 + itemize multiphase + NewValue→PL create; shared `PostNewArrayPPN` (0 U7, 1 sole, 2+ U7) across termVariable/Function-fail/pointer PP. Seeds 2/3/4/6 held.
4. Next: e6024 UP PL choose U14 vs GO sticky U13.

**seed5 e5956→5984 climbed — Function CREATE Global U3 + ptr-cmp * qfer + Assign RHS PP sole:**
1. Capture: e5956 UP U3 vs GO F50 createOnDemandGlobalFromERSEFree after Function CREATE residual (useExisting=0) → ExpressionVariable Global.
2. C++: sticky GlobalU21 force-create is wrong after post-Return F0 residual — SelectGlobal still has ~3 matching globals (Comma lhs const/vol struct). Later free Expression ptr-cmp nPtr floor U21 + idx=10 is * (WRITE qfer F50 F10 + self F50) not default ** levels-only; Assign RHS Function-fail ParentParam soles then Lhs SelectDeref F80 (not sticky free Expression PP U7).
3. GO: after `freeMultiIVNeedNoRhsPostEAReturnGlobalF0Done`, Function-fail Global pads choose_ok_var U3; post-Return non-PP ptr-cmp clamps stars=1; Assign RHS (`skipFuncRetQfer`) ParentParam sole-accept. Seeds 2/3/4/6 held.
4. Next: e5984 UP F20 (nested address create under SelectDeref NewArray) vs GO U99 CreateArray.

**seed5 e5953→5956 climbed — Expression const/vol struct filters Assign:**
1. Capture: e5953 UP U120=6 tries=1 vs GO U120=109 tries=0 (after Comma U120=119 + pickNonVoid U16=15).
2. C++ Expression.cpp:173–175: `is_const_struct_union || is_volatile_struct_union` filters eAssignment. Comma lhs type=nil → NonVoid picks const/vol aggregate → Assign rejected (tries=1) → Function accepted.
3. GO: `isConstOrVolatileStructUnionType` in term `disallowed` for termAssign. Seeds 2/3/4/6 held.
4. Next: e5956 UP Global choose U3 vs GO GenerateNewGlobal F50 (Function CREATE residual inventory).

**seed5 e5928→5953 climbed — post-Return For body StatementReturn ParentParam→PL stack U2:**
1. Capture: e5928 UP U2=0 vs GO U100=52 (StatementProbability after Return EV under-burned).
2. C++: For body BlockSize U4=2 then StatementReturn (U100=30) → ExpressionVariable VS ParentParam (U100=83) empty/choose miss → SelectParentLocal stack U2 (Function::stack={parent free body, For body}) sole accept → For ends; parent free Statement U100=6 IfElse + Expression U120…
3. GO: armed `freeMultiIVNeedNoRhsPostEAReturnForBody` on SelectLoopCtrl U26 floor; Return EV ParentParam burns stack U2 direct (beats sticky ifBody U6 / post-Return U1). Seeds 2/3/4/6 held.
4. Next: e5953 UP U120=6 tries=1 vs GO U120=109 tries=0 (Expression term filter after nested Function/pickNonVoid U16).

**seed5 e5882→5928 climbed — post-Return Lhs SelectDeref sole + Global U36 + Stmt need_no_rhs multiphase + SelectLoopCtrl U26:**
1. Capture: e5882 UP U120 tries=1 vs GO F20 (SelectDeref empty create residual after PP U7).
2. C++: free ExpressionAssign Lhs SelectDeref empty create F80 F20 F20=1 F0 fail then F80 F20 F20=0 address soles existing simple pointee (no nested create F20) → free Expression U120. Later free Expression Global eFlexible U36+itemize U4; Statement need_no_rhs Assign Lhs F80=0→VS Global U4+F0 → PL U2+U8 miss → SelectDeref empty F80 F80=0 → PL empty create U14+qfer+Constant accept + SafeOpFlags F50 U4 + extra U4; For SelectLoopCtrlVar ok_vars ≈26 (not sticky era U37).
3. GO: one-shot Lhs address sole under ReturnGlobalF0Done; Global pad multiphase U36+U4 itemize; Statement Lhs Global U4+F0 then PL multiphase U8 miss / empty create accept + SafeOp extra U4; post-Return SelectLoopCtrl floor U26 (not era U37). Seeds 2/3/4/6 held.
4. Next: e5928 UP U2 vs GO U100 (For body residual after SelectLoopCtrl U26 + SafeOp).

**seed5 e5843→5882 climbed — post-Return PL U13 + Global F0 + PP U7 multiphase:**
1. Capture: e5843 UP U13=4 tries=0 vs GO U14=11 (sticky post-Return PL over-count).
2. C++: free Expression Variable U100=57 is ParentLocal (scope 35–64), not Global. After Return, stack U1 + choose_ok_var expanded ok_vars U14 (e5755/e5781) then U13 (e5843 after e5808 residual). Later Function-fail Global visit F0 → VS PP→PL U1 U2 U7 + Constant; subsequent PP live U7 (e5874) not sticky U1+U2.
3. GO: multiphase PLN 0–1 U14 / 2+ U13; one-shot GlobalU21 Function-fail Global F0 residual; PP multiphase 0/2+ U7, 1 U1+U2. Seeds 2/3/4/6 held.
4. Next: e5882 UP U120 tries=1 vs GO F20 (SelectDeref empty create residual).

**seed5 e5809→5843 climbed — SelectGlobal empty simple retype U14 + OuterLhsSole:**
1. Capture: e5809 UP U14=11 tries=1 (random_type_from_type) vs GO F20 (sticky e5397 multiphase F20 F20 without retype).
2. C++: free ExpressionAssign Lhs F80=0→VS Global U100=13 empty → SelectGlobal `Type::random_type_from_type` choose_random_simple U14 + GenerateNewGlobal WRITE create_and_initialize (NewArray F20 + Constant pure_rnd F50 F50 U3) → visit fail → SelectDeref F80 F0 + empty create F20 F20 accept → next free Expression U120. Sticky e5397 residual re-fired F20 F20 multiphase (pointer-era pack) after PostEAGlobalU21 already set.
3. GO: one-shot e5397 multiphase under `!PostEAGlobalU21`; later simple Global empty retypes U14 + Constant create residual; arm OuterLhsSole so nested outer ExpressionAssign Lhs soles (UP free Expression U120, not outer SelectDeref F80). Seeds 2/3/4/6 held.
4. Next: e5843 UP U13 (PL choose_ok_var) vs GO U14 (sticky residual inventory +1).

**seed5 e5692→5730 climbed — PL choose_ok_var U14 (not retype) + Global U23 + live PL multiphase:**
1. Capture: e5692 UP U14=0 tries=0 (null Filter) vs GO U14 tries=1 (pickSimpleNonVoid SIMPLE_TYPES).
2. C++: free Expression PL after stack U5 is **choose_ok_var** among 14 live locals (VariableSelector::choose_ok_var — no Filter), not `random_type_from_type` / SIMPLE_TYPES_PROB_FILTER. Later GlobalList pad multiphase U31 (e5608 itemize) then U23 (e5697). Live PL multiphase: U14 choose → sole → *** mode-2 create (F50 F10×3 + F10 + F20 F20 U2 addr) → sole again.
3. GO: replace residual retype with unfiltered U14; Global pad U31 once then U23; live PL multiphase soles/create/qfer/address. Seeds 2/3/4/6 held.
4. Next: e5730 UP U4 (Lhs SelectDeref itemize/continue) vs GO U100 (Statement after F80 U5 accept).

**seed5 e5554→5692 climbed — post residual PL empty create multiphase + PP sole + live choose:**
1. Capture: e5554 UP F50 (PP→PL empty create) vs GO U120 (sole after stack U5).
2. C++: free Expression ParentParam miss → SelectParentLocal empty → GenerateNewParentLocal ** READ qfer + nested address residual (random_loose F50 + nested NewArray F20 F20 U3); later S2* Function-fail PP→PL address U4 among live pointees; Lhs F80 sole; GlobalList U31 + array itemize U3; Lhs SelectDeref null F0×2 then address U3; PL live U2+itemize / U3 fail reselect create F10 F20 F50 F50 U20; PP sole after one-shot empty create.
3. GO: one-shot PP empty force + ** create residual multiphase; skip sticky PLCreateN==2 sole under postEA; S2* address multiphase (first nested bitfields, later U4); Lhs F80 sole after 2nd S2 create; Global U21→U31 after PostEACreateN≥2 + U3 itemize; Lhs empty create address U3 after U31; Function PL force S2 create; Expression PL live multiphase (empty create / U2 itemize / U3 reselect create / PP sole). Seeds 2/3/4/6 held.
4. Next: e5692 UP U14=0 tries=0 vs GO U14 tries=1 (retype filter / LCG after stack U5).

**seed5 e5420→5554 climbed — StatementAssign outer Lhs sole + post residual derived_types/PL:**
1. Capture: e5420 UP U100=64 (next Statement) vs GO F80 (outer Statement Lhs SelectDeref after nested ExpressionAssign residual).
2. C++: ExpressionAssign is Statement Assign RHS (termAssign after SelectLType pointer). Nested Lhs residual ends → outer Statement Lhs select_must_use/sole (zero RNG) → next Statement U100 AssignOps/Expression multiphase. Later free Expression ptr-cmp derived_types U21 (not sticky ArrayOp U17); PL stack U5 (not sticky forceU6 after ptr-cmp); second+ free Expression PL empty create ** READ qfer F50 F10×3 + NewArray.
3. GO: arm `ppPostPadSkipStmtLhs` after residual; floor ptr-cmp derived_types U21 under `freeMultiIVNeedNoRhsPostEAGlobalU21`; PL stack U5 + multiphase sole then force empty create with levels floor 2. Seeds 2/3/4/6 held.
4. Next: e5554 UP F50 (PL create) vs GO U120 (sole again).

**seed5 e5398→5420 climbed — ExpressionAssign Lhs Global create + SelectDeref residual:**
1. Capture: e5398 UP F20 (GenerateNewGlobal after U100=13) vs GO F80 (early sole after U100 ends ExpressionAssign).
2. C++: Lhs F80=0→VS Global empty create F20 F20 then SelectDeref empty create F80 F10 F50 F20 F20 CreateArray… then more VS multiphase.
3. GO: bypass early sole under post-If era; burn Global F20 F20 + SelectDeref create residual inline (VS is outside SelectDeref for{}). Seeds 2/3/4/6 held.
4. Next: e5420 UP U100=64 vs GO F80 after residual.

**seed5 e5373→5398 climbed — post-If Expression PP→PL stack U6 + S2* empty create:**
1. Capture: e5373 UP U6=1 vs GO U5=3 (same raw) after ExpressionAssign RHS Function-fail → ExpressionVariable U100=93 ParentParam.
2. C++: Function::stack.size()=6; stack[1] empty → GenerateNewParentLocal struct S2* qferMode 0: NewArray F20 + make_init address F20 + pointee S2 NewArray F20 + bitfield Constants U11585… (g_282/g_283) then Lhs F80=0→VS Global.
3. GO: parentStackPick post-If U6 (era+LhsSelDone+!IfBody); Function-fail PP→PL force empty create S2* qfer 0; keep doAddrResidual under GlobalU21 when era S2*; formatAggregate S2 bitfields. Seeds 2/3/4/6 held.
4. Next: e5398 UP F20 (Global empty create after U100=13) vs GO F80 after Lhs F80=0→VS.

**seed5 e5324→5373 climbed — post-If need_no_rhs-era Lhs SelectDeref live + VS PL/PP stack U6:**
1. Capture: e5324 UP U5=2 (SelectDeref live) vs GO F10=0 (empty create) after parent free For Assign Lhs F80=1 (IfBody cleared).
2. C++: free multi-IV need_no_rhs-era parent free For after need_no_rhs If — select_deref_pointer eDereference pool ~5 (U5+U7 fail → F80 U5 accept); need_no_rhs Assign F80=0 → VS PL U6+U5 miss → SelectDeref U5/U4+U7; PP U100=73 → PL stack U6 + NewValue U14 create; later simple Assign SelectDeref live U5.
3. GO: freeMultiIVNeedNoRhsEraLhsSelN live multiphase under era+!IfBody+LhsSelDone; PostIfPL nStack=6 + U5 miss; PostIfSelDeref multiphase after PL miss; sticky PP→PL U2 overridden to U6 post-If; later simple Assign live U5 accept. Seeds 2/3/4/6 held.
4. Next: e5373 UP U6 vs GO U5 (same raw) after U100=93 PP.

**seed5 e5289→5324 climbed — StatementContinue first-stmt null + atMax Break EV + SelectLoopCtrl U37 era floor:**
1. Capture: e5289 UP U100=42 tries=1 (reject ArrayOp 59 atMax_compound blk_depth=5) vs GO U100=59 tries=0.
2. C++: nested For body (BLOCKPROB max=2) rejects first-stmt Continue (StatementContinue.cpp:64–66, no EV RNG) → retries Assign; third StatementProbability at max rejects ArrayOp then Break + EV Global U4+U4; parent free For SelectLoopCtrl ~37 then Assign Lhs SelectDeref.
3. GO: `blockStmtsEmitted` + scoped first-stmt Continue null under NestedFor; StatementFilter atMax; Break EV; SelectLoopCtrl floor 37 under freeMultiIVNeedNoRhsEra. Seeds 2/3/4/6 held.
4. Next: e5324 UP U5 (SelectDeref) vs GO F10 (empty create) after parent For Assign Lhs F80=1.

**seed5 e5163→5289 climbed — For body Continue→Assign (not postArrayFor U2) + PL U6 empty create:**
1. Capture: e5163 UP U120=7 (AssignOps) vs GO U2=1 (postArrayFor) after For body U100=38 Continue, U100=65 Assign.
2. C++: free multi-IV need_no_rhs If then-body free For body keeps Continue then StatementAssign → AssignOps U120=7 → SelectLType F50 F30 U3 → RHS Expression Function-fail Variable PL stack U6 (For frame) + Lhs F80=0 → VS PL U6 empty create NewArray F20 (match_exact qfer, no WRITE F50).
3. GO: seed2 e948 Assign→For remap fired on sticky loopIVPool>1+multiDim under arrayLoopDepth=1 even outside array-loop Continue; gate remap to array-loop frame and exclude freeMultiIVNeedNoRhsIfBody/Era. Nested For arms freeMultiIVNeedNoRhsIfNestedFor → parentStackPick/Lhs PL U6 + empty create qferMode 0 F20-first. Seeds 2/3/4/6 held.
4. Next: e5289 UP U100=42 tries=1 vs GO U100=59 tries=0 (StatementFilter).

**seed5 e5147→5163 climbed — SelectLoopCtrl U37 inventory (need_no_rhs If-body For):**
1. Capture: e5147 UP U37=27 (SelectLoopCtrlVar) vs GO U16=4 (same raw; live count under-materialised).
2. C++ SelectLoopCtrlVar (VariableSelector.cpp:1120–53): find_all_non_array_visible + choose_var(WRITE, int, eConvert, no_bitfield). At e5147 (func_36): ok_vars≈37 = ~7 globals (incl. volatile g_170; make_iteration rejects vol after choose), ~7 expanded params (p_37 + S0 p_38/p_39 fields + p_40/p_41), ~23 parent-chain locals. Volatiles stay in pool (effect_stm cleared). Free For uses make_random_loop_control (no rw_directive arrays).
3. GO: countVisibleIntLoopCtrl keeps volatiles; under freeMultiIVNeedNoRhsIfBody floor nCtrl to 37 when residual under-materialises locals/params (mirror e2943 U15 ArrayOp-body floor). Seeds 2/3/4/6 held.
4. Next: e5163 UP U120=7 vs GO U2=1 (same raw) after For body Statement U100=38 U100=65.

**seed5 e5120→5147 climbed — need_no_rhs Global accept + If sole + SelectDeref + For:**
1. Capture: e5120 UP U120=67 (AssignOps after If then Statement U100=98) vs GO F10 (NewValue create after forced Global visit-fail).
2. C++: free Expression Variable Global eFlexible U4+itemize U4 visit **accepts** (If condition). Then sole Assign; nested PL U5+U3; Lhs SelectDeref live U5/U4/U7; next Statement For U100=27 tries=0 (IN_LOOP, not filterCompound).
3. GO: accept U4+U4; need_no_rhs free If sole (no BlockSize/multiDim bonus); clear filterCompound + IN_LOOP; SelectDeref live residual; PL U5+U3.
4. Seed2/3/4/6 held. Next: e5147 SelectLoopCtrl U37 vs U16 (inventory).

**seed5 e4900→5120 climbed — PP→PL empty create NewArray CreateArray + PL U8:**
1. Capture: e4900 UP F50=1 (NewArray create) vs GO U7 choose after need_no_rhs PP U100=69 stack U4=3.
2. C++: SelectParentParam empty params → SelectParentLocal; nested `local_vars.empty()` → GenerateNewParentLocal WRITE F50 vol + create_and_initialize NewArray F20=1 → Constant primary hex + CreateArray U99 itemize; no_signed_overflow rejects → SelectDeref U7. Later PL choose grows U8 after NewArray; more PP/PL empty creates; Lhs accepts SafeOpFlags F50 U4 → free Statement.
3. GO: free multi-IV PP multiphase empty create with `burnSimpleConstant` + `burnCreateArrayVariable`; PL pool U8 after create era; further PP/PL empty-create residuals.
4. Seed2/3/4/6 held.

**seed5 e4560→4900 climbed — need_no_rhs Lhs Global eDerefExact + PP/PL multiphase:**
1. Capture: e4560 UP U3=1 (SelectGlobal eDerefExact pointer-preference) vs GO U15 residual (e3127) after need_no_rhs PreDecr F80=0 → VS Global.
2. C++: Lhs WRITE eDerefExact + pointer preference; dummy shrinks U3→U2→sole; F0 null; no_signed_overflow; SelectDeref U7 ladders; PP→PL stack U4 create/choose multiphase; PL pools U3/U6/U7/U8; empty create hex next31; long do-while.
3. GO: skip e3127 U15 under `lhsNoSignedOverflow`; free multi-IV Global/PL/PP multiphase + SelectDeref U7; maxVSTries=80; hex Constant residual.
4. Seed2/3/4/6 held.

**seed5 e4504→4560 climbed — NewValue Global/PL U14 + need_no_rhs Lhs ladder:**
1. Capture: e4504 UP U14=5 tries=1 vs GO F10 (createOnDemandGlobal without retype) after need_no_rhs PreDecr F80=0 → VS NewValue Global.
2. C++: `GenerateNewVariable` → `random_type_from_type` choose_random_simple U14 then GenerateNewGlobal WRITE (F50 vol, no const F10) + create_and_initialize. Signed retype fails `no_signed_overflow` (Lhs.cpp:110–112) → SelectDeref live pointer pool shrink U5→U2 then F80=0→VS. Later PP→PL stack U4 + choose U5 miss; NewValue→PL qferMode WRITE F50 only (not READ F50 F10); NewArray create residual then more SelectDeref U7; PL stack U4 + choose U7.
3. GO: chooseLValueEx NewValue→Global retype eSimple U14 + WRITE F50 + skipRandomQfer create; `lhsNoSignedOverflow` rejects signed creates; free multi-IV need_no_rhs SelectDeref ladders + PP/PL stack U4 choose; NewValue→PL always qferMode 3 WRITE + retype.
4. Seed2/3/4/6 held.

**seed5 e4473→4504 climbed — multi-level SelectLType *** + PL qfer + Lhs nested peel:**
1. Capture: e4473 UP F50=1 vs GO F20=0 after PL create F50 F10×2 (GO 1-star underburn).
2. C++: SelectLType PointerAsLType F50 + make_random_pointer_type F20 pick derived[5] ind=2 → find_pointer_type → *** ; Function-fail PL create random_qualifiers F50 F10×4 (3 levels+self) + NewArray F20 F20 address sole; Lhs SelectDeref empty create random_add F10 F50 + NewArray=0 address → random_loose F50 + nested create_and_initialize peels F20×6 + CreateArray U99….
3. GO: free multi-IV For body SelectLType F20 deepen+floor *** (list under-models mid indices); skip residual e1914 levels=1 floor under freeMultiIVForBodyU3; multi-level PL address sole (no sticky U4); Lhs addVol multi-level nested peel residual (not sticky e2980 U3 U3).
4. Seed2/3/4/6 held. Next: e4504 U14 vs F10 (NewValue→Global create after F80=0 VS).

**seed5 e4460→4473 climbed — VS multiphase PP accept (no extra U100) + StatementFilter + If stack U4:**
1. Capture: e4460 UP U100=92 tries=1 vs GO U100=37 (extra VS reselect after PP U7 U4).
2. C++: ExpressionVariable PL visit-fail → VS PP U7 + pointer-boost U4 accepts (no further VS U100). Free Expression multiphase ends → StatementProbability tries=1 rejects Continue 37 → Assign 92. Nested If then-body under free multi-IV For pushes Function::stack → PL create stack U4 (e4468).
3. GO: remove residual extra VS U100 after PP U7 U4; arm StmtNoLoop one-shot for StatementFilter Continue reject; clear MultiLvl forceU3; push blockStack on free multi-IV If then/else; freeMultiIVForBodyU3 uses live blockStack when >3.
4. Seed2/3/4/6 held. Next: e4473 F50 vs F20 (PL create multi-level random_qualifiers underburns vs UP F50 F10×4 then NewArray).

**seed5 e4368→4460 climbed — multi-level post-ladder PL stack U3 + live inventory:**
1. Capture: e4368 UP U3=0 vs GO U6=0 (same raw) after multi-level SelectDeref create ladder — PL stack size.
2. C++: free multi-IV For body `Function::stack.size()=3` throughout; residual forceU6 sole is wrong sticky. After ladder: PL multiphase (U3 stack + choose_ok_var U2+itemize U10 / U3 / sole / U9…); Global live U24→U23→U8; Lhs SelectDeref empty create address U2 then live choose fail ladder U2 F0 + U3 U4 U8 F0×3 then F80=0→VS; later empty create F20 F20 accept; PL visit-fail → VS PP U7 multiphase.
3. GO: arm `freeMultiIVPostEAMultiLvlPLStackU3` after multi-level VS create; bypass residual forceU6; PL/Global/Lhs multiphase under that flag. Next: e4460 U100 v=92 vs v=37 (extra VS reselect after PP U7 U4 fail).
4. Seed2/3/4/6 held.

**seed5 e4227→4368 climbed — SelectDeref multi-level create_and_initialize F20 ladder:**
1. Capture: e4227 UP F20=1 vs GO F50=1 after matching F80 + F20×3=0 (SelectDeref empty create).
2. C++: ExpressionAssign Lhs type is pointer (`**`); `find_pointer_type(t,true)` → `t*` (ind≥2). Outer `create_and_initialize` NewArray F20 + make_init address F20; nested pointee is still a pointer → nested NewArray F20 + make_init F20 (null Constant has no pure_rnd — not simple Constant F50 F50 U20). `match_exact_qualifiers` during ExpressionAssign Lhs skips `random_add`/`random_loose` F50/F10 (F80→F20). Parent free Expression Variable: VS multiphase PL U3 fail → PL U3 create `**` qfer F50 F10×2+self + NewArray address U2 CreateArray; alts F20=0 → choose U2 (+ multi-dim `*` itemize U8 U7 when v=0).
3. GO: default address residual assumed simple pointee Constant F50. Multi-level Lhs now peels nested make_init F20; clear sticky must_use sole; arm VS multiphase PL U3 fail/create; `burnCreateArrayMultiLvlAltU2` for ** alts. Next: e4368 U3 vs U6 (PL stack after ladder).
4. Seed2/3/4/6 held.

**seed5 e4221→4227 climbed — multi-dim addressable itemize after Global choose:**
1. Capture: e4221 UP U8=3 U7=0 vs GO F80 after matching e4220 U2 Global choose.
2. C++: ExpressionVariable eFlexible `choose_var` prefers addressable lower-ind when higher empty (VariableSelector.cpp:456–489). Addressable includes multi-dim `*` arrays → `choose_ok_var` → `ArrayVariable::itemize` all dims (e4221 U8 U7 for `[8][7]`). Then ExpressionAssign Lhs SelectDeref F80.
3. GO: multiDim pointer path is exact-level only (U(n)=2 by coincidence on exact `**`) and skips itemize on non-array picks. After multi-level GlobalU21 live choose, burn `itemizeArrayCandidate` for first multi-dim addressable `*` array (`pointerAddrOfMatch` / ptr_type). Next: e4227 F20 vs F50 (SelectDeref empty create).
4. Seed2/3/4/6 held.

**seed5 e4220→4221 climbed — GlobalU21 Function-fail SelectGlobal choose first:**
1. Capture: e4220 UP U2=0 vs GO F50 SE-free create after ptr-cmp ** Assign RHS Function-fail Variable Global.
2. C++: `SelectGlobal` always `choose_var` first; only `GenerateNewGlobal` when empty (VariableSelector.cpp:648–666).
3. GO: GlobalU21 Function-fail path always force-created. For multi-level want (`**+`), try `selectExprVariableFromER` live choose first (e4220 U2). Next: e4221 U8 U7 itemize (multi-dim array) vs Lhs F80.
4. Seed2/3/4/6 held.

**seed5 e4215→4220 climbed — ptr-cmp operand star-depth (GlobalU21 * clamp gate):**
1. Capture: e4215 UP F10=0 vs GO U120=82 after matching e4210 U16=9 (ptr-cmp choose) + ExpressionAssign WRITE qfer short one F10/F50 pair.
2. C++: `choose_random_pointer_type` → `derived_types[9]` is `eULongLong**` (ind=2). ExpressionAssign null-qfer WRITE burns F50 F10 ×2 levels + self F50 (no self const).
3. GO: sticky GlobalU21 `stars=1` clamp (e2274 era — early qfer-burning picks were ind=1) under-modeled idx=9 as `*`. Gate clamp on `derivedPtrTypes < 16` (pre-eULong* inventory); once n≥16, non-PP `idx>0 → **` yields ind=2 WRITE qfer.
4. Seed2/3/4/6 held.

**seed5 e4210→4215 climbed — eULong* derived inventory (pointerBaseKey):**
1. Capture: e4210 UP U16=9 vs GO U15=9 same raw (ptr-cmp `choose_random_pointer_type`).
2. C++: `Type::derived_types` is keyed by `Type*` identity — `eULong` and `eULongLong` are distinct even when both are 64-bit on LP64. SelectDeref empty create at e3434 called `find_pointer_type(eULong, true)` → ADD n=16 (ind=1). GO collapsed both to `uint64_t` so `noteDerivedPointer` was a no-op.
3. GO: `pointerBaseKey` distinguishes `uint64_t` by `HexDigits` (8 = eULong / GenerateRandomLongConstant; 16 = eULongLong). Do **not** split signed `int64_t` the same way — that over-counted seed4 SelectDeref live inventory (e2732 U14 vs U13). Next: e4215 qfer F10 (operand star-depth for derived idx=9).
4. Seed2/3/4/6 held.

**seed5 e4094→4210 climbed — Global pool settle + NewValue Lhs accept:**
1. Capture: e4094 UP Global choose U3 vs GO U2 (same raw) late in Lhs do-while; after fix e4111 U14 tries + NewValue create; e4119 Statement IfElse.
2. C++: choose_var pointer-preference then remaining eDerefExact ok_vars; late PL creates settle live Global pool so choose n stays U3 (e3932/e4018/e4093); NewValue→PL Type::random_type_from_type choose_random_simple (U14 + SIMPLE_TYPES_PROB_FILTER tries); need_no_rhs SafeOpFlags F50+U4; visit accept → next Statement. freeMultiIV Expression nest must stop after that Assign.
3. GO: globalPoolSettled freezes phase-B countdown after late create growth; NewValue pickSimpleNonVoid + create + SafeOpFlags + clear freeMultiIVForLhsExprContinue.
4. Seed2/3/4/6 held.

**seed5 e3647→4094 climbed — Lhs do-while U7 + Constant hex + VS inventory:**
1. Capture: e3647 UP F80=1 vs GO F80=0 after CreateArray U7 ladder + F50 F20 F50 create residual.
2. C++: Lhs.cpp do-while after CreateArray; Constant::make_random F50=0 burns RandomHexDigits(8) untraced; F80 continues SelectDeref itemize; VS scope table Global/PL/PP/NewValue; dummy.push_back shrinks choose pools; PL stack blocks differ (pre-existing locals / empty create / array itemize); NewArray create marks block with new sizes for later itemize.
3. GO: structural Lhs residual — burnSimpleConstant hex, Global phase-A U3→U2→sole then phase-B countdown, per-block PL inventory, NewArray block size tracking.
4. Seed2/3/4/6 held.

**seed5 e3571→3647 climbed — free multi-IV post-F30 AssignOps SelectLType + Lhs CreateArray U7:**
1. Capture: e3571 UP U120=90 vs GO U100=50 same raw — after useExisting=0 + F30 F0, GO ExpressionVariable fallthrough; UP continues AssignOps/SelectLType-shaped body (RHS Constant U120 then Lhs).
2. C++: F50 F30 F0 is Type::SelectLType (Pointer/Struct/FloatAsLType; float disabled → F0) after simple AssignOps, then RHS Expression Constant + Lhs::make_random SelectDeref multiphase; need_no_rhs AssignOps (U120=102) continues Lhs empty create + CreateArray; post-CreateArray SelectDeref itemizes collective array sizes U7 (ArrayVariable::itemize) until F80=0 → VS.
3. GO: free Expression term Function useExisting=0 residual burns SelectLType F30 F0 then RHS Expression + compact Lhs residual (not maxFuncs ExpressionVariable); AssignOps need_no_rhs + Lhs empty-create CreateArray + U7 itemize ladder phases (F0 then F50 F20 F50).
4. Seed2/3/4/6 held. Next was e3647 (climbed to 4094).

**seed5 e3535→3571 climbed — free multi-IV Global U16 itemize + PL create + Lhs + FuncAttr:**
1. Capture: e3535 UP U4=3 vs GO U120=55 same raw — after free multi-IV Global residual U16=6, C++ ExpressionVariable multiphase F50 rechoose U16=10 + ArrayVariable::itemize U4 then VS U100 ParentParam; GO accepted early and parent binary ShiftBy stole F50 U16.
2. C++: choose_ok_var itemizes collective arrays; Lhs after residual PL/Global creates is SelectDeref F80 U2 fallthrough VS U100 (same iteration); post-itemize free Expression Variable select_must_use soles (no U100); later Global live U4; Function useExisting=0 burns FuncAttr F30 F0 before ExpressionVariable.
3. GO: free multi-IV U16 pad itemize multiphase (v=0 U10; v=6 F50+U16+U4+U100; skip ShiftBy); must_use sole; PL stack U3 + empty create U14 + Lhs F80 U2 U100; PadDone stops sticky U16 pad; live Global U4 + Lhs; stop forceStdFuncSimple; FuncAttr F30 F0 residual.
4. Seed2/3/4/6 held. Next was e3571 (climbed to e3647).

**seed5 e3450→3535 climbed — free multi-IV GlobalList inventory + post-EA Lhs ladder:**
1. Capture: e3450 Global choose U23 (UP) vs U22 (GO) same raw — sticky residual GlobalU21 pad stuck at U22 after e2702 while C++ eFlexible simple grew by nested GenerateNewGlobal pointee on free multi-IV residual EA Lhs SelectDeref empty create (e3434 Constant residual).
2. C++ SelectDeref non-vol → GenerateNewParentLocal(ptr) + make_init address-of empty → GenerateNewGlobal(simple) on GlobalList. Later Global multiphase: e3470 filtered live U8, e3523+ U16 (not sticky U23). e3471 Lhs SelectDeref soles null pointer F0 then empty create NewArray CreateArray + U1 ladder; F80=0 → VS PP U100=70 stack U2 sole (no create) → parent U120. PL choose after e3440 NewValue create grows eFlexible U6→U7 (e3462).
3. GO: residual Global pad multiphase U23 / U8 / U16 under freeMultiIVEALhsF20x4Done; materialize nested simple Global on e3434 Constant residual; LivePL floor U6 then U7; one-shot Lhs sole F0 after Global U8 pad; free multi-IV VS PP/PL sole accept (not sticky e2083 F20 create).
4. Seed2/3/4/6 held. Next was e3535 (climbed to e3571).

**seed5 e3185→3406 climbed — free multi-IV post-nest SelectDeref empty create after Global fails=1:**
1. Capture: after post-nest SelectDeref U2 choose fail + VS Global fail (e3180–83 match), UP e3184 F80 sole/pure fail then e3185 F80 empty create F10 F50 F20 F20 F0 vs GO sticky `postAggLhsDerefChooseFails=1` pool U10.
2. C++ Lhs::make_random dummy invalid_vars: first SelectDeref chooses among 2 (U2), fails visit → dummy; F80=0 VS Global fails → dummy; next SelectDeref `choose_ok_var` soles remaining pointer (len==1, no RNG) then empty; next empty → `random_add_qualifiers` F10 F50 + `create_and_initialize` F20 NewArray + F20 init (null→validate F0 continue; address→choose U3) (VariableSelector.cpp:1240–89, Lhs.cpp:70–140). addVol GenerateNewGlobal grows GlobalList + `find_pointer_type` even on visit fail.
3. GO: arm `freeMultiIVForLhsExprPostNestLhsEmptyCreate` on Global fail (clear sticky fails pool); SelectDeref phase 0 pure/sole fail; phase 1+ empty create random_add F10 F50 + F20 F20 (null F0 retry + GlobalList pad; address U3 + VS U100 accept). Floor `derivedPtrTypes` to 15 after accept so ptr-cmp e3238 U15 matches (free multi-IV nest CreateArray residual also grew derived_types).
4. freeMultiIV forceStdFuncSimple skips `inPtrCmpExpr` (ExpressionFuncall.cpp:73–75 pointer → user F50; e3403 matched).
**seed5 e3406→3450 climbed — free multi-IV residual ExpressionAssign RHS Function-fail → Lhs SelectDeref empty create:**
1. Capture: after user Function F50=0 create-fail → ExpressionVariable PL U100 U2 (e3403–05 match), UP e3406 F80 Lhs SelectDeref empty F20×4 U3 U4 vs GO sticky force Residual/BodyU3 PL createOnDemand F50 qfer.
2. C++ StatementAssign order RHS then Lhs: maxFuncs Function-fail → ExpressionVariable SelectParentLocal soles existing local (no create RNG) then Lhs::make_random SelectDeref empty create (outer F20 F20 + nested pointee create_and_initialize F20 F20 + choose U3 U4). Later free Expression Variable PL empty create F50 F10×2 F20 F20 + address U2 U4; then live PL choose U6 eFlexible integers.
3. GO: stop sticky freeMultiIVForBodyU3 force-create under residual continue; Function-fail PL sole when no exact ptr; one-shot EA Lhs F20×4 U3 U4; arm freeMultiIVPostEALhsPLCreate then LivePL (U6 floor); residual PL address U2 U4 under continue; int32 hex width 8 (not 16) on address pointee Constant.
4. Seed2/3/4/6 held. Next was e3450 (climbed to e3516).


**seed5 e3176→3185 climbed — free multi-IV parent Expression U120 Variable + post-nest Lhs (not F80 end RHS early):**
1. Capture: after free multi-IV ExpressionAssign Lhs accept (e3175 F50 / nested Lhs U3), UP U120 Variable (parent shift binary ShiftBy F50 + RHS ExpressionVariable PL U2 U4) vs GO sticky `postAggSkipShiftByOnce` skip RHS → Statement F80 / outer Lhs SelectDeref.
2. C++: nested Assign is RHS of outer ExpressionAssign under FunctionInvocation binary shift (e3044 U18=17). After nested Lhs accept, outer Lhs soles; parent shift runs `ShiftByNonConstantProb` F50 then RHS Expression U120 Variable (PL stack U2 choose U4). Free multi-IV Comma nest ends on that Variable; parent ExpressionAssign Lhs::make_random continues SelectDeref (e3180 F80 U2 F80=0 → VS Global fail → more SelectDeref e3184+).
3. GO: clear sticky `postAggSkipShiftByOnce` + arm `ppPostPadOuterLhsSole` (allow under free multi-IV even if StackU6CreateDone); PL choose U4 once after null-alt CreateArray residual (not sticky U5); after free multi-IV expr nest, `lhsMakeRandomWrite` with U2 SelectDeref choose + first Global fail (not e4330 sole under `postAggDerefChooseU2AfterCreate`).
4. Seed2/3/4/6 held. Next was e3185 (climbed).

**seed5 e3076→3176 climbed — CreateArray null-alt re-itemize sizes + free multi-IV PL U2 (not residual U9 U8 U3 / ParamU7 U6):**
1. Capture: after ExpressionAssign Lhs SelectDeref NewArray CreateArray (e3060–75 match U99 dims [3][4][8] + alts + itemize F0), UP SelectDeref re-itemize U3 U4 U8 F0 ladder vs GO sticky e1215 residual U9 U8 U3; then F80=0 → VS PL U2 vs GO ParamU7 force U6 create.
2. C++ `create_array_and_itemize` leaves collective array in pool; null pointer alts → FactPointTo opportunistic_validate F0; Lhs do-while SelectDeref `choose_ok_var` soles array → `ArrayVariable::itemize` last sizes + F0 until F80=0. Free multi-IV `Function::stack.size()=2`; VS PL choose among 2; visit fail → more SelectDeref (U2 F0; sizes itemize F0; F80=0 → PP/PL U3 F50 accept).
3. GO: after CreateArray `hadNullPtrAlt`, keep `createdArrEA` + `createdArrEANullValidate` so retries burn `lastArraySizes`+F0 (clear sticky U9 U8 U3 residual). Free multi-IV ExpressionAssign F80=0 → PL stack U2 + choose U2 + SelectDeref ladder (not ParamU7 e1225 U6+create).
4. Seed2/3/4/6 held. Next was e3176 (climbed).

**seed5 e2989→3076 climbed — free multi-IV Lhs VS accept → Expression nest (not Statement U100):**
1. Capture: after Lhs VS multiphase PL accept (e2988), UP U120 Expression (Constant+Comma… stdfunc F5, PL create/choose, ExpressionAssign Lhs F80 CreateArray) vs GO next Statement U100.
2. C++ StatementAssign ends after Lhs, but residual free Expression stream (lhsAfterParamMiss family) continues before next StatementProbability; Comma LHS retypes to simple → atMax stdfunc F5; residual PL stack U2; first PL empty create, NewValue→PL U14 retype, later PL choose U5.
3. GO: arm freeMultiIVForLhsExprContinue after VS accept; two free Expressions; parentStackPick U2; termVariable residual PL multiphase (create once / NewValue retype / choose U5); force atMax stdfunc simple under residual; hoist freeMultiIV body stack U3 above sticky ParamU7 U6.
4. Seed2/3/4/6 held. Next was e3076 (climbed).

**seed5 e2966→2989 climbed — maxFuncs Function-fail pointer ExpressionVariable + free multi-IV Lhs residual:**
1. Capture: after free For SelectLoopCtrl, Assign RHS Function useExisting empty candidates — GO residual U2 pad vs UP ExpressionVariable U100.
2. C++ FunctionInvocation::make_random: empty choose_func at max_funcs → failed → ExpressionVariable (ExpressionFuncall.cpp:84–90). GlobalU21 U2 residual only for early multiphase (e2410/e2440).
3. GO: stop FuncUseExistingU2 residual after 2 fires; free multi-IV For body stack U3 + empty PL create + address U4; Lhs SelectDeref addVol residual F50 F20 F20 U3 U3 then VS multiphase U100 PL / PP U7 / PL accept (no F80 between).
4. Seed2/3/4/6 held. Next was e2989 (climbed).

**seed5 e2943→2966 climbed — free For SelectLoopCtrl inventory + loop_control:**
1. Capture: Statement For e2942, UP SelectLoopCtrl U15 + make_random_loop_control; GO sticky loopIVPool U2 + multi-dim array_control U9 U8.
2. C++ SelectLoopCtrlVar (VariableSelector.cpp:1120–53): find_all_non_array_visible + choose_var(WRITE, int, eConvert, no_bitfield); array_control only when rw_directive must_use arrays non-empty — free For uses loop_control.
3. GO: postArrayFor only when arrayLoopFresh or continue-remapped (not sticky loopIVPool alone); free multi-IV path uses countVisibleIntLoopCtrl (expand non-bitfield fields, dedupe) + floor U15 inside residual GlobalU21 ArrayOp body; loop_control residual.
4. Seed2/3/4/6 held. Next: e2966 UP U100=50 vs GO U2 (Funcall-ish).

**seed5 e2916→2943 climbed — SelectDeref create wildcard qfer + nested body continue:**
1. Capture: after VS multiphase e2908 + Statement Assign Constant RHS, UP SelectDeref create F50 F10 F50 (random_qualifiers) vs GO F10 F50 (random_add).
2. C++ select_deref_pointer (VariableSelector.cpp:1248–54): `qfer->wildcard` (Constant get_qualifiers / need_no_rhs) → `random_qualifiers(ptr, WRITE, no_volatile=true)` = per-level F50+F10 + self F50; else `random_add_qualifiers`.
3. GO: track top-level RHS term Constant → `assignRhsWildcardQfer`; create uses random_qualifiers when wildcard. After AddrCreateVS one-shot, address create runs choose_ok_var U4+itemize (not filterCompound sole). Nested body NeedStmt drain loop + PP→PL empty create (stack U2 U14) + Statement filter allows For at shallow residual depth.
4. Seed2/3/4/6 held. Next was e2943 (climbed).

**seed5 e2883→2916 climbed — Lhs !addVol address create validate-fail → VS multiphase:**
1. Capture: after F80 F10 F50 F20 F20 U2, UP U100 Global/PL/PP ladder… U16 create residual through e2908; GO accepted Lhs → Expression U120.
2. C++ Lhs::make_random do-while (Lhs.cpp:70–140): SelectDeref create may fail visit_facts / fall through to VariableSelector::select without second F80.
3. GO: one-shot residual phase after SE-free !addVol address U2 under GlobalU21 — scope-fail ladder (PL miss, PP U7×2, PL stack U3 U5, Global U16 + create residual) without F80 between reselects; arm +1 free Statement so nested func body continues U100 Assign (e2909).
4. Seed2/3/4/6 held. Next was e2916 (climbed).

**seed5 e2877→2883 climbed — RHS PP→PL residual sole + Lhs !addVol address U2:**
1. Assign RHS ExpressionVariable ParentParam (U100=92) miss on non-matching param → SelectParentLocal stack U5.
2. Residual GlobalU21 after first pointer PL create: free Expression PL is sole after stack (choose_ok_var n==1; VariableSelector.cpp:979–1001). ParentParam miss falls through to the same SelectParentLocal — GO dynLocs over-count exact `int32_t*` (U4) while UP soles then Lhs SelectDeref F80.
3. Lhs SelectDeref empty create !addVol → GenerateNewParentLocal (VariableSelector.cpp:1274–1289): make_init address-of choose_ok_var U2 before seed2 filterCompound first-create sole (which skipped U2).
4. Seed2/3/4/6 held. Next: e2883 UP VS U100 multiphase after address U2 vs GO Expression U120 (Lhs accept too early / missing post-create residual).

**seed5 e2862→2877 climbed — Lhs SelectDeref random_add VolatilePointers SE-free:**
1. `select_deref_pointer` empty create: `random_add_qualifiers(no_volatile=!SE-free)` → F10 ConstPointers then F50 VolatilePointers when SE-free (CVQualifiers.cpp:479–490; VariableSelector.cpp:1253–54).
2. StatementAssign simple Lhs uses statement-entry SE-free snapshot (Function RHS must not suppress F50 via markFuncEffect).
3. `addVol` → GenerateNewGlobal nested residual by Lhs star depth; multi-level (***): F50×2 + F20×2; 1-star: F50 + F20 F20 U5; simple: F50 + Constant.
4. Multiphase e2799 first create still short non-vol U2; seed2 late for-body early-accept preserved for stars < 2.
5. Seed2/3/4/6 held. Next was e2877 RHS PP→PL U4 (climbed).

**seed5 e2851→2862 climbed — create_and_initialize Constant HexDigits width:**
1. UP `derived_types[9]=eULongLong**` → `find_pointer_type` deepens to `***`; leaf `Constant` burns `RandomHexDigits(16)` (depth gap 16 after F50=0).
2. SelectLType multi-level pointer under residual GlobalU21 sets `HexDigits=16`/`Bits=64` on the Expression type (Name stays `int32_t***` for inventory).
3. Nested `create_and_initialize` peels preserve `Bits`/`HexDigits` so digits write into constant strings (not discarded).
4. Seed2/3/4/6 held. Next was e2862 VolatilePointers F50 (climbed).

**seed5 e2842→2851 climbed — nested address random_loose_qualifiers:**
1. Track outer random_qualifiers vol/const bits; `random_loose_qualifiers` only draws F50 when parent bits allow (CVQualifiers.cpp:422–457); LooserConstProb=50.
2. `remove_qualifiers(1)` pops self; nested `create_and_initialize` with non-wildcard qfer (no re-random_qualifiers).
3. Seed2/3/4/6 held. Next was e2851 Constant hex width (climbed).

**seed5 e2831→2842 climbed — Function-fail Global SE-free qfer:**
1. Residual GlobalU21 Function-fail empty Global: first create keeps skipRandomQfer (e2279 F20 NewArray); later uses `createOnDemandGlobalFromERSEFree` (CVQualifiers self F50 when SE-free).
2. SelectLType F20 ptr-to-ptr in GlobalU21 deepens stars (find_pointer_type(t,true)) and floors *** so 4×(F50 F10) levels+self match.
3. Nested multi-level address init peels one `*`. Seed2/3/4/6 held. Next: e2842 nested init residual.

**seed5 e2827→2831 climbed — derived_types permanent + composite pointers:**
1. `Type::derived_types` is process-global; do not restore on Expression snapshot rollback.
2. Residual GlobalU21 SelectLType F20 ptr-to-ptr: ensure `find_pointer_type` for each AllTypes struct/union before `rnd_upto(derived_types.size())` (UP U12 vs under-count U9).
3. Seed2/3/4/6 held. Next: e2831 Function-fail Global create qfer (F50 SE-free vs F20 NewArray).

**seed5 e2817→2827 climbed — Lhs WRITE must_use on must_write_vars:**
1. Array-loop tracks both `must_read` (access 0/2) and `must_write` (access 1/2).
2. Function-fail EV READ `select_must_use` accepts after itemize+fields+F75 (visit success).
3. StatementAssign Lhs WRITE `select_must_use` over `must_write_vars` (Lhs.cpp:74–75) — second U2+S0 Constants+F75 (e2817–22).
4. Seed2/3/4/6 held. Next: e2827 SelectDeref/pointer pool U12 vs U9.

**seed5 e2811→2817 climbed — Function-fail EV aggregate select_must_use (VariableSelector.cpp:1461–1506):**
1. Track real `must_read` array list when `make_random_array_loop` access 0/2; push/pop with array-loop frames.
2. Function-fail aggregates: `select_must_use_var` → itemize + create_field_vars Constants + F75.
3. dim>iv_bounds skip; inventory expand on type-match dim-fail. Gate on residual GlobalU21.

**seed5 e2808→2811 climbed — SelectLType ok_structs (Type.cpp:1591–1597):**
1. `StatementAssign::make_random` passes `no_volatile = !effect_context.is_side_effect_free()`.
2. Each Statement starts SE-free → `no_volatile=false` → **include** `is_volatile_struct_union` types (S2 with vol bitfields).
3. Go always filtered `s.isVolatile` → n=2; UP n=3 (S0+S1+S2). Now `ctx.effectSEFree` gates the filter; track `isConst` for `no_const=true`.

**seed6 full PASS (23/23) — structural:**
1. **Lhs WRITE `SelectParentLocal` empty** (`VariableSelector.cpp:979–989`): when `!multiDimArrays` and stack block `local_vars` empty, `random_type_from_type` → `choose_random_simple` (**eSimpleType** + `SIMPLE_TYPES_PROB_FILTER`) + `GenerateNewParentLocal` qferMode 3 WRITE (was F80 retry).
2. **`Block::make_random` max=0** (`Block.cpp:140–146`): no-aggregate pure function body → free stmt count `max+1=1` then **`append_return_stmt`** → `StatementReturn` → `ExpressionVariable` SelectGlobal (true `GlobalList`, no `fromParentLocal` pad) → empty `GenerateNewGlobal` retype U14 + skipRandomQfer NewArray/Constant.
3. Seed2/4 held; seed5 first_div restored **2808** (SelectLType non-vol struct pool U3 vs U2).

**e2133–e2229 climbed:**
1. Function-fail `struct S0` Global: sameWidth fix + SE-free GenerateNewGlobal + struct Constant (bitfield pow half-width).
2. `create_field_vars`: per-field `Constant::make_random` after aggregate CreateVariable (e2157–2177).
3. ExpressionComma LHS type=null → U14 AllTypes (struct) then Function-fail create; RHS continues int32.
4. Empty SelectParentLocal after postAggGlobalCreate → retype U14 + GenerateNewParentLocal (e2188–2196).

**e2317–e2370 climbed:**
1. Live GlobalList after one-shot U23 (e2317 U9).
2. PL idx=0 F0 fail → VS reselect ParentLocal + `choose_ok_var` + array itemize (e2337–42); local `isArray`/`arrayLen` metadata.
3. SelectDeref live pointer choose U13 (e2351); inventory pad when under-count.
4. postAgg if-then body `StatementFilter` atMax (e2356 tries=2).

**e2371–e2394 climbed:**
1. Multi-dim array itemize after PL choose (U9 U4 U7 / g_86; U9 U9 U3 / g_126) + F0.
2. SelectDeref live U13; fail-once then U12+itemize under postAgg if-body filter.
3. Lhs PL stack n=6; Lhs PL choose U5 + multi-dim itemize F0.
4. `arraySizes` on globals/locals for multi-dim itemize; StatementFilter if-body atMax.

**e2395–e2420 climbed:**
1. Lhs VS-miss → inner SelectDeref chain (U13 itemize F0; U13 F0; U12 accept).
2. if-body inherits `inLoop` (Continue U100=36 legal) + clear skipNextBlockSize.
3. After Continue: skip AssignOps/SelectLType; PL stack residual + create address U2 U4.

**e2421–e2581 climbed:**
1. postAgg PL idx=0: first U5+F0 reselect (e2337); later accept without F0 → parent U120 (e2530–31).
2. ExpressionAssign self-vol F50: pure `effectSEFree` + late GlobalPicks force; skip force when Comma follows Variable select (e2534 AssignOps without F50).
3. SelectDeref NewArray address: one-shot U3 U4 then CreateArray U99; later straight U99 (e2540).
4. postAgg CreateArray pointer alt inits: F20-only (sole/empty pointees, no U2).
5. Lhs PL after F80=0: choose U4 + multi-dim itemize U9 U4 U7 F0; reselect PP→stack U6 + retype U14 + create.

**e2582–e2674 climbed:**
1. Retype U14 type drives Constant hex width (eUShort hex4 vs Lhs t hex8).
2. postAgg Global live: U9 then U24 pad once; later exact non-array U2 (not inflate 17/24).
3. Assign self-F50 force only when !varSelectSticky (postAgg Variable select); pre-postAgg keeps force (e2084).
4. Address-of Expression residual one-shot (e2092); later pointees U6 U3 U7 U1 F80=0→VS.
5. postAgg PP→PL Lhs: choose U5 F0 F80 chain → VS Global (not F20×4 create).

**e2675–e2690 climbed:**
1. Nested ExpressionAssign RHS finish → arm `ppPostPadOuterLhsSole` so outer Lhs skips SelectDeref F80 (parent term U120).
2. postAgg Global eFlexible: exact n==2 (e2627); else U9 pool with array at index 6 for itemize U10 (e2687).

**e2691–e2760 climbed:**
1. e2691 empty PL create residual matched (F10 F20 F50 F50 path).
2. postAggLhsDerefFailOnce (e2707) empty SelectDeref; later first U13 accepts (e2732+).
3. postAgg if-then stmtCount +3 (e2355 U4=0 body) so long Assign does not open else early (e2733/e2751 Statement U100).
4. ptr-cmp derived_types floor U9 after fail-once (e2744).
5. StatementReturn ParentParam: pad choose U5 + visit_facts fail → VS retry U100 (e2758–60).

**e2761–e2852 climbed:**
1. Return ParentParam U5 accept (not EV retry); postAgg if-then continues after Return.
2. ArrayOp allowed late postAgg (`postAggLhsDerefFailOnce` + depth&lt;max); F5=0 aryno=0 path: U15 IV + loop_control + SafeOp×3 + body.
3. postAgg stmtCount +4; PL stack n=5 after ArrayOp; PL ladder (U4 / Global U24 / multi-dim itemize→NewValue create).
4. PL SelectDeref residual F80 U8 U4 → VS U100.

**e2853–e2934 climbed:**
1. After PL F80 U8 U4 U100: ExpressionAssign residual (AssignOps U120, RHS Variable, Lhs F80 chain) + sibling Expressions (sole Variable term + Function stdfunc on int32).
2. Global live after ArrayOp: U24 non-array pool (e2920); SelectDeref F20 F20 → Expression residual×2 then Lhs accept (e2924–34).

**e2935–e3005 climbed:**
1. Address Expression residual one-shot with exprDepth=0 + allowFuncOnce (Function tries=0).
2. Global U24 one-shot then U9; skip array itemize post-residual.
3. Stack n=4 after residual; empty PL create F10 F20 F50 F50 once; locals U5.
4. ExpressionAssign allow + skip self/GlobalPicks F50 force.
5. Second SelectDeref F20×2 → CreateArray residual F20 F50 F50 U20.

**e3006–e3129 climbed:**
1. Assign+Lhs via `postAggNeedLhsAfterRhs` + `lhsMakeRandomWrite` (e3023–66); Statement boundary e3067.
2. PL one-shot U5+itemize (e3104–09); later PL stack→VS reselect without U5 (e3115).
3. SelectDeref after Lhs-write: one-shot U7 (e3076); then U12 F0 / U11→VS (e3122–26).
4. Lhs Global choose U15 + residual U14 U13 (e3127–29).

**e3130–e3276 climbed:**
1. After Lhs Global U15: `make_random_loop_control` + SafeOpFlags×3 + body U4 (e3130–43).
2. Next Statement unfiltered tries=0 → IfElse U100=5 (e3144); condition Expression U120… (not force Assign).
3. ptr-cmp derived_types floor U10 after U15 era (e3175).
4. PL stack n=5 sole after U15 (e3178); 3rd PL → VS reselect PP sole (e3218); 4th → create U14 F20 F50 F50 U20 (e3269); ExpressionAssign self F50 restored (e3264).
5. ExpressionAssign Lhs SelectDeref choose U7→F80 U6 after U15 (e3190–92).

**e3277–e3392 climbed:**
1. ExpressionAssign Lhs PP→PL: stack sole U5 + F0 + F80 F20 F20 (not double U5) after U15 (e3275–80).
2. Global sole F0 (no U9) → PL U5 F0 → PP sole (e3314–20 empty-reselect path).
3. Later PL stack U5 + locals U4 (e3321–23); Continue no longer arms skip-AssignOps after U15 (e3340 AssignOps U120).
4. After Continue: PL stack n=6; create qferMode 1 F50 F10 F20; struct Constant field-order when U15 StackU6 (e3372–92).

**e3393–e3531 climbed:**
1. PL struct create: `burnCreateFieldVarsConstants` after init (e3393+ double field Constant residual).
2. ExpressionAssign signed AssignOps: VectorFilter exclude incr/decr (e3440 tries=1).
3. After StackU6 create: disable OuterLhsSole skip; Lhs F80 F20 F20 U2 accept (e3445–48).
4. Post-StackU6 PL stack sole (no U4 locals) → Expression U120 (e3521).

**e3532–e3587 climbed:**
1. Comma AllTypes after StackU6: NonVoid-style filter (accept volatile struct; e3532 tries=0).
2. StackU6 PL create always qferMode 1 (e3537 F50 F10 F20).
3. StackU6 PL n=0 sole (e3521); n≥1: U5 then VS reselect Global U43 (e3584–86).
4. `pickChooseRandom` for SelectLType pointer base (Type::choose_random).

**e3588–e3645 climbed:**
1. Comma !SE-free after StackU6: two-shot NonVoid (e3464+e3532) then NonVoidNonVolatile (e3588 tries=4).
2. StackU6 PL ladder: n=0 sole; n=1 U5→Global U43; n=2 sole+F0→PL U6 U5→PP; n≥3 U5+F0→Global U2.
3. StackU6-era Global eFlexible scale U14 (e3637; was sticky U9).

**e3646–e3734 climbed:**
1. `isPointerNullConstant`: pure `((type)(0))` only — not `Contains("(0)")` (nested exprs false-triggered forced Variable RHS on ptr-cmp; e3646).
2. StackU6 pointer Global empty exact → choose U2 pad (e3648).
3. StackU6 PL n≥4: choose_ok_var U4 accept (e3656; not n≥3 F0 residual).
4. StackU6 Lhs PP→PL: NewArray F20 + CreateArray simple-element alts before U15 F0 residual (e3660–68).

**e3735–e3772 climbed:**
1. StackU6 PL n==5: U4 + itemize [9][4][7] F0 → Global U43 (e3733–40).
2. n==6: VS reselect + stack + itemize [6][3][7][1] (e3743–49).
3. n≥7: sole accept (e3756–58).
4. Global eFlexible U14 idx=5: size-10 array itemize U10 + U18 residual (e3752–54).
5. StatementAssign Lhs SelectDeref: U13 fail (no F0) → F80 U12 → VS (e3769–72).

**e3773–e3830 climbed:**
1. After Lhs VS PP following U12: visit_facts fail residual Expression stream
   (Function useExisting F50 F0, nested Function binary/ptr-cmp, Constant tries,
   PL itemize, more SelectDeref) before Assign ends — was closing to Statement U100 too early.

**e3831–e3875 climbed:**
1. e3831: bare 4×upto(14) → real `pickChooseRandom` (AllTypes filter tries=3 v=13).
2. SelectLType pointer base → `Expression::make_random` on pointer type (create
   qfer F50×2 F10×2 + NewArray CreateArray via real createOnDemand).
3. StackU6 NewArray skip wrong U6 address residual; post-residual Lhs F80 continue;
   Global U2 after PP residual (drop sticky U14); SelectDeref U13→U12 countdown;
   ParentParam miss → PL U6+itemize[9][4][7] F0.

**e3876–e4035 climbed:**
1. e3876 root cause: after SelectDeref U12, UP **accepts Lhs** → Statement U100 Assign
   (U120 AssignOps / PointerAsLType); GO wrongly VS NewValue F10. Fixed accept.
2. Pointer Lhs SelectDeref U7 + itemize [9][9][3] F0×2 (e3883–95); stack n=5 after.
3. PL sole after stack U5; post-ptr SelectDeref U13…U9 F0 countdown; Global U2 (e3919).
4. ExpressionAssign Lhs U7 F0 U6 U7 F80=0 → VS PL/NewValue create residual (e3934–54).
5. ArrayOp U100=56 F5=0 aryno=0 → For residual header (e3955–74).
6. haltGen after f10 late residual exhaust (avoid silent hang on longer seed4 stream).

**e4085–e4237 climbed:**
1. e4085 root: ptr-cmp null-LHS → `randomPointerVariableExpr` (forced Variable), not
   main `termVariable`. NO_DANGLING empties choose → GenerateNewParentLocal qferMode 2
   (F50 F10 F10). Gate on post-ptr era (avoid e2236 regression).
2. Out-of-range derived_types idx (nPtr floor 12, list under-count) → `struct S0*`
   (not int32_t**); address residual nested Global + create_field_vars (U181 bitfield).
3. e4204: post-create PL choose itemizes multi-dim [9][9][3] before F0.

**e4237–e4250 climbed:**
1. e4237: second PL after ptr-cmp create — F50 + nested Expression (not U5 itemize).
2. Nested Variable → NewValue F10 → PL U5 retype U14 F10 F20 F50 (allow retype under StackU6).
3. NeedLhs → Lhs F80=0 → Global U100 U2 F50 accept (not sole-fail F80 loop).

**e4250–e4268 climbed:**
1. Skip residual ShiftBy after NeedLhs (NeedLhs cleared before outer shift resumes).
2. PL U4 choose (not sticky F50+nested); NeedLhs + ForceDerefCreate F20 F20 U5.
3. OuterLhsSole under postPtrCreate for free Variable streak.

**e4268–e4330 climbed:**
1. After ForceDerefCreate: free Constant int32 RandomHexDigits(8) (UP depth +8; not uint8 hex=2).
2. OuterLhsSole burns parent ShiftBy F50; arm NeedLhs on next Variable for Assign Lhs.
3. Empty SelectDeref create F20 F20 U99… (no inventory U5); then choose U2 fail-loop;
   PL U5 / U5 U5 F0; Param U5 U4; Global sole-accept (not U2+F50).

**e4330–e4335 climbed:**
1. Run Lhs after Variable in-Expression (`finishVar`) so parent Expression continues
   (not StatementAssign NeedLhs ending statement).
2. After Global Lhs sole: next Expression Variable VS sole-accept; unwind nested
   binaries (`postAggUnwindBinaryAfterExprVar`) + SkipParentExpr → Statement Lhs F80.

**e4335–e4408 climbed:** (prior)
Expression nest after Global create Lhs; U15 Global; PL F50; U5+F0 VS.

**e4408–e4481 climbed:**
1. After depth-block Variable (ParentParam): ForceNoFunc tries=1 → Variable U120=86.
2. Global U15; stop nest; SelectDeref F80 U12+[9][4][7]F0… F80=0 → PL create residual.
3. Unfiltered Statement U100=8 IfElse; clear binary unwind so RHS continues.
4. Nest PL U2 choose; stack U6 + U4+F0 + VS U100; block create → F80 Lhs era.

**e4481–e4545 climbed:**
1. After nest PL F0→VS sole: Expression Lhs SelectDeref U7+U4 (not empty F20 create).
2. Statement SelectDeref countdown U12+F0…U9 accept; round2 U12…U8 itemize + post-VS U7….
3. needNoRhs ++/-- Lhs also uses nest countdown (was gated out).

**e4545–e5308 climbed:**
1. Pool off-by-one after U6+[9][9][3]F0 realigned; nest SelectDeref countdown continues past U6.
2. F80=0 VS miss ladder (`postAggNestVSMisses`): U100-only → U6 → long create→U3/U2 →
   U5+U8 → U6+itemize947 → short create+8×next31 → U5/U6 itemize phases → NewValue
   F10+PL create (16×next31) → PL U6 U5 U4 U4 / U4 / U3+U10 residuals.
3. Multi-phase U2 itemize residual tables (993 vs 947) after each VS miss create/fail.
4. Cap VS misses / pool indices extended so countdown does not drop early.

**e5308–e6099 climbed:**
1. miss16: Global choose U3 (not sticky U6) → long U2 947-heavy phase.
2. miss17–36: create residuals (hex 8×next31 after F50=0), U2+U8/U10, U6+U5 U4 U4,
   U6+itemize 993/947, short create → multi-phase U2 via `nestU2ItemizeKind`.
3. miss37: NewValue U100=95 → F10 PL create Constant residual (partial accept).

**e6099–e6402 climbed:**
1. e6099: NewValue residual must not burn trailing F50+U4 (those are needNoRhs SafeOpFlags).
2. SelectLType derived_types U13 pad; stack U6 after nest VS; multi-level *** PL create.
3. Function-arg PP→PL force create; skip empty-pointee address residual; Global U2 not U44.
4. Re-arm nest SelectDeref countdown U12 after Lhs create; roundN≥2 short U10…U3 + U2 phases.
5. miss38–40: U2 / U5+993 / U4+U8 accept; VS miss cap 50.

**e6402–e6479 climbed:**
1. e6402: nest ExpressionAssign one-shot self F50 qfer (post-ptr seFree was skipping).
2. SelectLType/stack pads; EA Lhs F80→F50+Expression residual; Global U54; stack U5.
3. e6460: PP-scope EA Lhs F80=0 create residual F20×3 + Expression stream.

**e6479–e6595 climbed:**
1. e6479/e6507: Variable PL VS reselect U100 U5 U4 (alternate; no F0).
2. e6531: nest ptr-cmp derived_types floor U16 (was U12).
3. e6535–36: nest pointer ExpressionAssign levels-only qfer; RHS nested Assign keep skipQfer.
4. e6539–41: create qferMode 0 + NewArray address residual U2 before CreateArray.
5. e6549–89: nest Lhs empty create F20…; Global pointer CreateArray residual chain.
6. e6590–93: outer Lhs skip + PP→PL sole-accept + depth filter after residual Variable.

**e6595–e6608 climbed:**
1. e6595: sticky `ppPostPadDepthBlock` after nest Lhs Global residual (Statement resets exprDepth).
2. Nest Function/Assign/Comma must not bypass natural or sticky depthBlock.
3. e6597: GlobalList choose U17 (not sticky nest U54).
4. e6598: skip needNoRhs SafeOpFlags after nest Global Variable Expression.
5. e6605: PL stack U5 visit-fail → VS U100 reselect only (no U4 choose).

**e6608–e6635 climbed:**
1. e6608: nest Constant hex floor hn=16 (int8 under-width desynced LCG after F50=0).
2. depthBlock Assign must not re-open via allowAssignPad (e6612).
3. e6611: 2nd nest Global U17 F50 residual + next Expression noConst (Variable tries=14).
4. e6622: nest NewValue→PL create F50 U8 residual after Constant U20.

**e6635–e6716 climbed:**
1. e6635: after 3rd nest ParentParam sole Variable, skip open Function-binary
   ShiftBy F50 U32 (UP free Expression U120 tries=2). Count≥3 only — earlier
   soles must keep e6628 ShiftBy notConstant.
2. e6637: 3rd nest Global U17 attempt is visit_facts F0 (no U17) then VS PL;
   F50 residual only on exactly 2nd U17 choose (not 3rd).
3. e6638–41: after that F0, PL stack U5 visit fail → VS Global U17 accept
   (no locals choose U5).
4. e6646: arm NeedLhs + nest SelectDeref countdown U12… with e6646 residual
   table (U11+F0; U7/U5/U4 itemize 947; one 993; U4+F0) through F80=0.
5. e6712: NewValue→PL after F80=0 is stack U5 + U14 + F50 F20 F50 (not U4 choose).

**e6716–e6823 climbed:**
1. e6712–15: WRITE NewValue→PL retype U14 + qfer F50 + NewArray F20 + Constant
   hex next31 (formatSimpleConstant) so Statement U100 LCG matches.
2. e6716–33: Statement ArrayOp F5=0 array_loop aryno=1 select_array U13 +
   SelectLoopCtrlVar U39 + residual; sole return (no body).
3. e6734–49: second ArrayOp aryno=0 SelectLoopCtrlVar U38 + same residual sole.
4. e6821–22: after nest ArrayOp residual, PL stack U6 + choose U4 (not VS U100).

**e6823–e6895 climbed:**
1. e6823: clear sticky skipShiftBy/unwind after nest ArrayOp PL U4 so Function
   binary ShiftBy F50 runs; arm skip only on exact 3rd PP sole (not ≥3 re-arm).
2. e6859: ptr-cmp derived_types floor U17 after nest ArrayOp residual.
3. e6865: ptr-cmp PL create qferMode 1 (F50 F10 self) not mode 2.
4. e6878: GlobalList choose U55 after nest ArrayOp residual (not sticky U17).

**e6895–e7033 climbed (past 7000):**
1. e6895: free Constant hex natural type width after nest ArrayOp residual
   (sticky hn=16 desynced LCG); Assign self F50 restored; skip address U2;
   2nd+ pointer Global U2 pad; NewValue→PL qferMode1 no F50 U8.
2. e6963: one-shot after NewValue create — PL U5 + multi-dim itemize U9 U9 U3 F0;
   later inventory PL U4 sole (e6995); phase-1 stack-only VS reselect (e6998).
3. e6972+: Global pad ladder after residual — U55, U2, U54, U19…; e7008 7th
   multi-cand visit_facts F0 → PL U6 U5 F0 reselect + NeedLhs.
4. e7017–32: Lhs SelectDeref U12+F0 U11 VS + Expression residual U120 F50 F0
   F5 F10 U18 F50 F50 U4 (not sticky F20 create).

**e7033–e7259 climbed:**
1. e7033–46: Lhs residual Expression+ShiftBy then real F80=0 → VS WRITE U6 U4 U4
   Global sole (not Statement early).
2. e7047–53: post-Lhs Function residual (U120 tries=1 via uptoWithFilter) F5 F10
   U18 F50 F50 U4.
3. e7054–56: free Expression Variable + Global pad U55 re-arm after F0 era.
4. e7057–7110: Assign/Lhs create residual trail + hex next31.
5. e7111+: real Expression with Function allowed → Function residual stream.

**e7259–e7305 climbed:**
1. e7259: StackU6 empty pointer Global sole without U2 after nest ArrayOp residual
   (was sticky choose_ok_var U2 residual).
2. e7299: pointer ExpressionAssign qfer ≥2 levels F50 F10 + self F50 after residual.
3. e7305: Function-arg aggregate Global create NewArray F20 first (skip SEFree F50).

**e7305–e7443 climbed:**
1. e7305–72: Function-arg Global CreateArray residual + dim ladder; Lhs NewArray
   residual U2 U2 U5 → PL stack U4 sole F20 F20 (not sticky U6/F80).
2. e7372–82: PP→PL after residual — choose U2 visit miss → VS PP→PL stack +
   ** qfer F50 F10×3 F20 F20; skip address residual (UP Expression U120 next).
3. e7401: GlobalList multi-cand gn=10 U54; gn=11 e7439 visit_facts F0 reselect.
4. e7413: NewValue→PL simple qferMode 2 F10 (not sticky residual SE-free F50 F10).
5. e7421–35: inventory PL after NewValue — U4 sole, U5 choose, then stack-only
   VS Global U19 reselect (no sticky itemize re-arm).

**e7443–e7516 climbed (past 7500):**
1. e7443: keepExpr residual free Expression after F0→PP+Constant (depth-block
   Variable PL U5 + Lhs F80) — not emitStatements early.
2. e7448–97: Lhs SelectDeref residual after PL itemize — U12+993/947/F0, U11,
   VS PL 993, U10, VS PL U4 U3, U8, VS PL U4 U4 accept.
3. e7498: force next Statement U100 + Expression in same block (not BlockSize U4).
4. e7510: Global multi-cand gn=12 U55.

**e7516–e7736 climbed:**
1. e7516: SelectLType/ptr-cmp derived_types floor U18/U19 after keepExpr residual.
2. e7579: PL stack drops to U3 after keepExpr Lhs accept; empty create then
   inventory U5/U4/sole ladder (PLStackU3N).
3. e7634–95: ptr-cmp/PP→PL create qferMode 2 keep type; address residual
   U2 U3 U3 (**) or U4 (*).

**e7736–e7776 climbed:**
1. e7605/e7736: PLStackU3 AddrCreateN — first address residual U2 accept; later
   !NewArray&&!initNull burns pointee NewArray F20 + make_init F20 + ** U2 U3 U3
   + CreateArray U99 **inline** inside ppPostPad≥15 block (live-picks swallow
   fall-through).
2. e7748: CreateArray pointer alts under PLStackU3 burn U2 U3 U3 (not bare U2);
   skip post-alt U2 U5 itemize residual (e7754).
3. e7762: Global multi-cand gn=14 visit_facts F0 → PL U3+U4 accept (not sticky
   e7008 U5 F0 under PLStackU3).

**e7776–e7857 climbed:**
1. e7776: free Expression Global pointer empty create (SE-free **** qfer + nested
   make_init peel residual) — not residual sole → Statement Assign F80 early.
2. e7809: Statement Assign Lhs SelectDeref empty create F10 F50 + CreateArray.
3. e7823–40: unfiltered For SelectLoopCtrlVar U33…U30 + loop_control residual.
4. e7848–55: re-arm PLStackU4; PP→PL create without e7372 U2 + address U8.

**e7857–e8029 climbed (past 8000):**
1. e7857: Statement Assign Lhs SelectDeref live choose U6 (clear nest countdown).
2. e7861–75: short nest CD U12 / U11+[9][4][7]F0 → VS PL U3 U4 U9 U4 U7 F0.
3. e7876–906: multiphase SelectDeref ladder (CD2 U11…U6, CD3 U4/U3/U2 itemize,
   Global/PP create residuals) through 8000+.

**e8029–e8390 climbed:**
1. e8029: CD3 n≥400 explicit 947/993 itemize table + F0 residual multiphase leave.
2. e8271–73: free Expression PL stack U2 + locals U4 (clear sticky PLStackU4/itemize).
3. e8276+: post-CD3 free Expression Global multiphase U56→U19 (not residual sole/U2).
4. e8294+: ExpressionAssign skip residual qfer; ptr-cmp derived_types U21.
5. e8308–36: ExpressionAssign Lhs SelectDeref residual U11…U8 (not F20 create).
6. e8345+: post-CD3 PL stack U2 + phased locals (U4 / U4+947 / U5).
7. e8373–81: ShiftBy/Constant depthBlock for high-tries Variable filter.

**e8390–e8603 climbed (past 8500):**
1. e8390: Statement Assign RHS clear sticky depthBlock → Function (U120=1 F5…).
2. e8381: Constant U20 arms one-shot depthBlock (maxDB=1); not every Constant.
3. e8438: post-CD3 PL stack multiphase U2×4 then U3.
4. e8445/e8495: PL create address residual U2 then U5.
5. e8483: empty Global sole after first U2 pad.
6. e8516: GlobalList pad U56 again after U19.

**e8603–e8682 climbed:**
1. e8603: post-CD3 PP sole after first fallthrough (e8485 U3 create).
2. e8606: Global pad multiphase U56/U19/U56/U2.
3. e8610–77: U3-stack PL inventory table (U5/sole/U5+F0/U4/U5+993/U5).
4. e8669: NewValue→PL simple qfer mode1 then mode2.

**e8682–e8719 climbed:**
1. e8682: Statement Assign Lhs SelectDeref residual U12+947 / U12+F0 / U11+U4
   (clear sticky SkipStmtLhs after long post-CD3 RHS).
2. e8702: second Statement Assign Lhs residual U6→VS Global/PP→PL create.

**e8719–e8809 climbed:**
1. e8719–40: after StmtLhs2 residual, For/array residual (F5 aryno U4 +
   select_array U10 U3×3 + U32 U3 U2 F0 + SafeOpFlags).
2. e8747: post-CD3 pointer ExpressionAssign levels F50 F10 + self F50
   (e8294 first free Assign was non-pointer AssignOps only).
3. e8753: post-CD3 n==1 pointer Global multiphase (e8701 sole; e8753 pad U2).
4. e8765–88: third Statement Lhs SelectDeref residual U5/U4/U3 F0/U2 F0 + VS create.
5. e8796: fourth Statement Lhs F80 U5 accept; clear sticky e4332 ExprUnwind residual.

**e8809–e8855 climbed:**
1. e8809: U120=77 is Variable (not Function); post-CD3 For residual arms
   mustReadLive + select_must_use F75 accept (no U2).
2. e8822: second pointer ExpressionAssign **** qfer (F50 F10×4 + self).
3. e8831: Function-fail Global create F20×4 + CreateArray ladder residual.
4. e8848: ExpressionAssign Lhs F80 F20 F20 U2 after create residual.

**e8855–e8918 climbed:**
1. e8855: Statement Lhs F80=0 was nest VS-miss ladder (U5); force PP→PL U2
   create residual before ladder (intercept at SelectDerefPointerProb F80=0).
2. e8875: SelectDeref countdown U7 U6 F0 U5+993 U5 U4 U3 (not U12 start).
3. e8892+: F80=0 VS multiphase Global U7 / PP U3 U2+993 / PL create CreateArray.

**e8918–e9031 climbed (past 9000):**
1. e8919: CreateArray alt Constant hex width — suspend ppPLPad force-hn=8 so
   pickSimpleNonVoid eUChar (U14=6) burns RandomHexDigits(2) not 8.
2. e8927: after CreateArray residual, break lhsDerefLoop (needNoRhs F50 U4
   is SafeOpFlags unary, not residual); next Statement ArrayOp unfiltered.
3. e8927–50: ArrayOp F5=0 array_loop residual (aryno + U13 select + SafeOpFlags)
   then body For SelectLoopCtrlVar U30 + must_use itemize U4 U4 F75.
4. e8975–94: Statement Lhs SelectDeref U7 accept / U7 F0 U6 U5 chain.
5. e8995+: nested ArrayOp residual U29…U23 SelectLoopCtrlVar + body past 9000.

**e9031–e9267 climbed:**
1. e9031: ArrayOp2 body ExpressionAssign self F50 after Function !SE-free sticky.
2. e9060+: ArrayOp2 must_use multiphase itemize (U5 U3… / U4 / U5 U4) + F75.
3. e9092+: Global **** create F50 F10×4 F20×4; PL U6 create; second Global
   F20×4 + Lhs SelectDeref CreateArray re-itemize until F80=0 → VS U100.
4. e9150: ** ExpressionAssign qfer floor lv=2 (not 4).
5. e9222: need_no_rhs Lhs must_use WRITE + SafeOpFlags F50 U4.

**e9267–e9331 climbed:**
1. e9267: ptr-cmp null Constant → forced Variable must_use U5 U5 F75 (pointer
   arrays; trySelectMustUseVar rejects "*").
2. e9270–72: simple must_use miss window → VS U100; GlobalList pad U56.
3. e9276: AfterGlobalU56 must_use U5 U5 U2 F75; PL stack U6 sole.
4. e9288: Constant hex natural width (hn=16 only after unary size-3 once).
5. e9294–23: SelectDeref U7…U5 F0 + U4/993 residual; Global U7; outer Lhs sole.

**e9331–e9651 climbed:**
1. e9331: ArrayOp2 Function-fail ExpressionVariable(int*) must_use U3×4 F75
   (one-shot; pointer types previously pure-rejected).
2. e9336: Statement Lhs F80 U5 F0 F80=0 → PL U4 force create F20 F20 + residual.
3. e9369: ArrayOp2 For SelectLoopCtrl U29 U28 + loop_control/SafeOpFlags.
4. e9450: multiphase must_use n=8 U4 U4 U3 U2 F75; later pure miss.
5. e9463/e9616: EA Lhs empty create U7 then U2; **** PL qfer floor3; EA qfer *** .
6. e9603: ArrayOp2 EA qfer floor lv=3 after first ** floor2.

Next plateau: seed4 e17013 after PL F80 ladder F80=0 U100 → U3 U3 F0 residual (vs GO free U120). Toward 20000+.

**e16417–e17013 climbed:**
1. e16417: post-CreateArray Variable multiphase (PP U1 U2 F0; PL itemize NewValue).
2. e16558: late Comma → useEx PP create field catalog (hex widths from UP depth).
3. e16732+: post-array Global multiphase (U2 F0 create; CreateArray; U39; U3; NewValue).
4. e16773+: late PL U2 / F80 CreateArray; free PP U2 F50 U16; Assign afterAsg vs forceStdfunc.
5. seed2 37939 held.

**e15887–e16417 climbed:**
1. e15887: late PP F50 U32 threshold latePPN≥4 (was 6); Assign n≥8 F50+afterAsg.
2. e15940: late Comma F80 one-shot; must_use residual; CREATE head NonVoid; post-CREATE-head stdfunc.
3. e16088+: late Global U9 F0 / F0 reselect; late Constant F50+hex; residual j 250→500.
4. e16096 ** WRITE / e16244 **** WRITE; useEx=0 Global CreateArray + itemize ladder.
5. seed2 37939 held.

**e15685–e15887 climbed:**
1. e15685: PP multiphase U5 U4 (ppVS≥4).
2. e15687: Global U8 sole; e15700 PL create U14 F50 F20 + For residual.
3. e15731+: Assign n=6 F50 afterAsg; n≥7 bare; late Global U3/U39; Variable tries filters.
4. seed2 37939 held.

**e15387–e15685 climbed:**
1. e15387: post-CREATE Global ok_vars **U129** (was U102; CreateArray + field growth).
2. e15397–405: Variable PP U5 U3; PL U5 U5 U2 U5; no-Assign tries=1.
3. e15413–60: PL ***** create qfer; Constant SelectDeref F80 ladder; useExisting CREATE head.
4. e15483–685: Comma re-enters Lhs SelectDeref residual — Global U9/U8U10/U8U4; PP itemize; PL U5 U5/U3.
5. seed2 37939 held.

**e15185–e15387 climbed:**
1. e15185: post-CREATE Function useExisting residual — PL stack U6 multiphase (was U5 sole).
2. e15191: StatementFilter max blk_depth rejects compound (IfElse/For) tries=1 → Continue.
3. e15199–200: useExisting=0 after CREATE body → F0 marker (not stdfunc F5); end special Function era.
4. e15204/11 stdfunc F5; e15219 **** WRITE qfer; e15230 useExisting CreateArray F80 F20×; e15248 Comma no_func tries=1 + U14.
5. e15280 ** qfer; useEx=0 VS Global/PL; field Constant hex gaps; e15364 Constant tries=3 + F80; stdfunc resume.
6. seed2 37939 held.

**e13968–e15102 climbed:**
1. e13968: Global ok_vars multiphase U5 U8…U3 U8/U10/U4 catalog (G8–G23).
2. e14021: PP multiphase P6 itemize U9 U9 U3 F0; full P1–P14 catalog.
3. PL multiphase L1–L22 (itemize / create / U3 U4 U8 / U2 U5).
4. e14315: VS create NewArray F20 → burnCreateArrayVariable (U99 dims + init Constants + itemize).
5. e15083: NewValue N2 Constant small accept ends Lhs residual; SafeOpFlags F50 U4 + Statement U100; Function useExisting CREATE head F20 U14.
6. seed2 37939 held.

**e13471–e13968 climbed:**
1. e13471: after longlong Const hex, ShiftByNonConstant F50=0 → make_random_upto(64) for 8-byte shift RHS.
2. e13483: clear depth after NewValue; Comma → Statement Lhs SelectDeref F80 ladder (not U14).
3. e13484–13968: long Lhs SelectDeref loop — F80 itemize U9 U9 U3 F0 (×229), VS U100 multiphase Global/PL/PP/NewValue with create hex.
4. seed2 37939 held.

**e13379–e13471 climbed:**
1. e13379: CreateArray VS F20 NewArray=0 → Constant F50=0 hex×8 (unlogged digits).
2. Preserve lastHexN under re-armed depth (longlong hex×16 after NewValue U14=5).
3. Depth-era Global small ok_vars U3/U2 + ExpressionVariable retry; PL phases 5–7 sole/U7/struct create.
4. e13406: after CreateArray-era small Const → SelectDeref F80 residual; e13431 depth+no_const Variable-only.
5. e13432: PL struct create_field_vars hex widths 16/8/8 + U181 + field pack.
6. seed2 37939 held.

**e13009–e13379 climbed:**
1. e13009–55: Comma lhs U14 NonVoid + no_const; NewValue U4+SE-free F50 trails.
2. e13056: stop re-arming depthFilterExpr after itemize closed (new Function trees restart low depth).
3. e13067–70: post-itemize Assign F50 again; skip F50 once after afterAsg Variable; bare Function after Assign (no F5).
4. e13119+: Global ok_vars n=102 (no itemize pack); 2nd F50 / 3rd F50+U16 residual.
5. e13179: Assign RHS filter rejects Comma once; e13238 ptr-cmp no_func + Constant LHS→forced Variable RHS.
6. e13249+: PL multiphase stack U4 (choose/create/SelectDeref); re-arm depth after PL create; NewValue U14 NonVoid + eLongLong hex×16; CreateArray F80 U7 itemize.
7. seed2 37939 held.

**e12596–e13009 climbed:**
1. e12596: after Assign F50, Variable sole (skip U100) → Function binary.
2. e12633: one-shot useExisting F50 + aggregate Constant hex gaps + burnSC×11.
3. e12678: post-pack Constant extra F50; NewValue→PL; hex width from U14/SafeOpFlags.
4. e12841: ptr-cmp must_use U2×3 F75; depth-filter Expression tries; Global U36; CreateArray itemize residual past 13000.
5. seed2 37939 held.

**e9651–e10002 climbed:** need_no_rhs multiphase; EmptyCreateN; EA/PL qfer; Global U56→U2; Function-arg must_use. seed2 held.

**e10002–e10466 climbed:** PLAfterNeed; GlobalSoleN; Lhs ladders; Function CREATE. seed2 held.

**e10466–e11052 climbed:** CreateArray create_field_vars + itemize; PL U2; derived_types U23; EA ****. Past 11000. seed2 held.

**e11052–e11119 climbed:** EmptyCreateN after FuncArg+field-vars burns F20×2+U2 (not U2-only); double-F80 fail loop before second F20×4 create; Statement Lhs F80 U9 (pool under-count). seed2 37939 held.

**e11119–e12017 climbed:** Stmt Lhs F80 U9 → VS + Expression residual past 12000. seed2 held.

**e12017–e12596 climbed:** After VS Global U2, continue Lhs F80 U2/U7/U9 itemize residual; multiphase Global U2/U3; Variable tries filters; CreateArray alts/re-itemize; Assign qfer; long Expression residual. seed2 37939 held.
Seeds 5–21; COUNT=20.

**e716–e788 climbed:** `select_must_use_var` after multi-dim IV creates (U2+F75), max-funcs forces stdfunc without F80, ptr-comparison uses `derived_types` size + pointer operand types, parent stack n=5 after multi-dim nesting.

**CreateArray dimension ladder:** instrumented binary / seed2 traces use step **60** (1d 60% / 2d 30% / 3d ~9%), matching the comment in `ArrayVariable.cpp`. The checked-in tree source has `step = 100` (always dim=1 for `num∈[1,99]`), which cannot produce multi-dim events such as seed2 e565 (`U99=93` → sizes 4×4×9). Go follows the live instrumented stream, not the tree literal.

**Known RNG debt:**
- Pointer-element `CreateArrayVariable` alt inits under non-strict arrays should burn full `make_init_value` address-of residual (F20 null prefix only today).
- `parentStackPick` / `blockStack` still approximate (cap 3 pre-multi-dim; pin 5 post); true `Function::stack.size()` would remove the pin.
- Full `select_must_use_var` (itemize multi-dim, must_read membership, visit_facts accept) only partially modeled; gated on `multiDimArrays` + `inParamExpr`.
