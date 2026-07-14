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

**Implementers MUST mirror Csmith’s control flow**, not invent an independent generator that only matches LCG outputs.

1. Open the relevant C++ under `.build/csmith-src/src/` (e.g. `Expression.cpp`, `FunctionInvocation.cpp`, `VariableSelector.cpp`, `StatementAssign.cpp`, `Lhs.cpp`, `CVQualifiers.cpp`).
2. Trace the **same function/method sequence** for the failing event (SITE lines / callsites in traces).
3. Patch Go so the **same RNG consumers** run in the **same order** (e.g. FunctionInvocation CREATE → `make_random_signature` → param exprs → body; Assign → AssignOps → RHS Expression → Lhs).
4. Prefer shared helpers that name the upstream API (`burnCreateArrayVariable` ≈ `CreateArrayVariable`) over ad-hoc coin sequences.
5. Multi-seed: fixes must be **structural** (options/effects/inventory), not `if seed == 4`.

### 5.2 Integrity review (read the code — no integrity scripts)

**Do not add scripts that grep for bans as a gate.** Humans and agent **reviewers** open the implementer’s diff and reject work that games metrics.

When reviewing a climb/commit, **read** `pkg/csmith/*.go` (and related) and fail the review if you find:

| Reject if present | Why |
|-------------------|-----|
| Packed residual event tables (`f10_late_residual_data.go`, `f10LateResidualPacked`, offline `[]uint32` stream dumps of one seed) | Replays a seed offline instead of generating |
| `residualPlayer` / `burnF10LateExprResidual` as primary stream driver | RNG burns without Csmith Expression/Statement graph |
| `silenceTrace` / `rng.silent` to stop tracing | Fakes event-count match after residual exhausts |
| Seed-literal branches (`if seed == 2`) in generation paths | Non-portable hardcodes |
| Event match with **thin/wrong source** (Go C ≪ upstream structure) while claiming “done” | Events advanced without materializing real AST |
| Coin sequences with no corresponding C++ call path | Unmotivated hacks |
| **Discarding entropy** (see below) | Advances LCG without using the draw |

#### No discarding entropy (unless upstream does too)

**Default:** every `upto` / `flipcoin` / `next31` (and filtered variants) must **use** its result for the same purpose Csmith does: choose a term, accept/reject a filter, build a constant digit, pick a pool index, etc.

**Exception — only when upstream discards too:** a draw may be ignored in Go **if and only if** the corresponding Csmith C++ call path also consumes RNG without using that value for generation (same call site / same reason). Cite the C++ function and show the upstream discard (e.g. failed candidate still advanced `rand_depth_`, short-circuit after genrand, DFS rollback that re-draws). Matching an untraced LCG step that Csmith performs for a real API (e.g. `RandomHexDigits` digits written into the constant) is **not** discard — the value is used.

| Reject (Go-only discard) | Why |
|--------------------------|-----|
| `_ = r.upto(n)` / `_ = r.flipcoin(p)` / `_ = r.next31()` solely to pad the trace with **no** matching C++ draw | Discarded entropy to fake stream alignment |
| Loops of untraced `next31` “hex gaps” that never append digits **and** are not what `RandomHexDigits` / sibling C++ does | Fake LCG sync |
| Residual multiphase tables that burn catalogued U/F sequences then throw away values **without** a C++ path that burns the same sequence | Event pack without generation |
| Drawing a value then ignoring it to force a different path when C++ uses the value | Divergent control flow + discard |

**Allowed:**

- Csmith uses the draw (including untraced `RandomHexDigits` → digit in constant text).
- Csmith discards the draw at that site — Go may discard **only** by implementing that same C++ path (not a free-standing pad).
- `uptoWithFilter` / VectorFilter retries where **tries** match upstream and the accepted index drives selection.
- Real create/select that fails visit_facts and retries (upstream does this).

**Review heuristic:** if removing the RNG call would not change generated C (only the event stream) **and** there is no C++ call at that point that performs the same genrand, it is illegal discard. Prefer implementing the real C++ path over inventing pads.

**Review process for implementers:** after a patch, a separate pass **reads the code** (diff + call graph around the fix) and confirms it maps to Csmith C++ methods — not only that `find-rng-divergence` score improved. Explicitly check for Go-only discarded draws (`_ = r.…` padding, hex-gap loops, residual catalogs) unless the review cites matching upstream discard.

Allowed: temporary debug prints; timeouts; bounded retries that still call real create/select APIs; instrumented upstream for measurement only.

### 5.3 Technique constraints

Any remaining technique must:

1. Stay inside the repo’s blast radius
2. Have timeouts against non-termination
3. Converge by consulting C++ (not unmotivated RNG hacks)
4. **No discarding entropy unless upstream does too** — every RNG consumer either feeds a real decision/materialised value or mirrors a documented C++ discard at the same site (§5.2)

Order of preference: fix local RNG/call-path alignment first; structural reshape only when the same divergence class blocks progress. Residual multiphase catalogs that only burn stream without a C++ counterpart are **out**; reimplement the C++ path that produces those draws.

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


## 9. Progress (living)

| Gate | Status |
|------|--------|
| Instrumented upstream build | `scripts/build-instrumented-upstream.sh` → `.build/csmith-instrumented/` |
| Seed 2 re-baseline | Running vs golden `0cdc710` / csmith 2.4.0 |
| Seed 2 event match | **PASS** — full **37939/37939** (held after seed4 climb →19171) |
| Seed 2 source match | **FAIL** — residual-driven path; not full Csmith-flow AST |
| 20-seed gate | **In progress** — seed3 **PASS** 64/64; seed4 first_div **19171** (18927→19171; ptr-cmp LHS Const/Assign qfer + forced Variable RHS PL create, late free Global/PP/PL multiphase, Const F80 create; seed2 full held). Toward 20000+. |

**Integrity:** reviewers **read the implementer diff** (no integrity scripts). Reject residual packs, `silenceTrace`, seed hardcodes, event-only climbs, and **discarded entropy** (`_ = r.upto/flipcoin/next31` padding, unused hex gaps, residual catalogs that only advance LCG). Require call flow aligned with Csmith C++ where every draw is used.

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
