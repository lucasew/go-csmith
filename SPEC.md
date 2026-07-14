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

**Review process for implementers:** after a patch, a separate pass **reads the code** (diff + call graph around the fix) and confirms it maps to Csmith C++ methods — not only that `find-rng-divergence` score improved.

Allowed: temporary debug prints; timeouts; bounded retries that still call real create/select APIs; instrumented upstream for measurement only.

### 5.3 Technique constraints

Any remaining technique must:

1. Stay inside the repo’s blast radius
2. Have timeouts against non-termination
3. Converge by consulting C++ (not unmotivated RNG hacks)

Order of preference: fix local RNG/call-path alignment first; structural reshape only when the same divergence class blocks progress.

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
| Seed 2 event match | **PASS** — full **37939/37939** (held after seed4 climb →6099) |
| Seed 2 source match | **FAIL** — residual-driven path; not full Csmith-flow AST |
| 20-seed gate | **In progress** — seed3 **PASS** 64/64; seed4 first_div **6099** (5308→6099; nest VS miss16–37 + U2 phases; seed2 full held). |

**Integrity:** reviewers **read the implementer diff** (no integrity scripts). Reject residual packs, `silenceTrace`, seed hardcodes, event-only climbs. Require call flow aligned with Csmith C++.

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

Next plateau: seed4 e6099 GO extra F50 after NewValue residual vs UP Statement U100 tries=1.
Seeds 5–21; source; COUNT=20.

**e716–e788 climbed:** `select_must_use_var` after multi-dim IV creates (U2+F75), max-funcs forces stdfunc without F80, ptr-comparison uses `derived_types` size + pointer operand types, parent stack n=5 after multi-dim nesting.

**CreateArray dimension ladder:** instrumented binary / seed2 traces use step **60** (1d 60% / 2d 30% / 3d ~9%), matching the comment in `ArrayVariable.cpp`. The checked-in tree source has `step = 100` (always dim=1 for `num∈[1,99]`), which cannot produce multi-dim events such as seed2 e565 (`U99=93` → sizes 4×4×9). Go follows the live instrumented stream, not the tree literal.

**Known RNG debt:**
- Pointer-element `CreateArrayVariable` alt inits under non-strict arrays should burn full `make_init_value` address-of residual (F20 null prefix only today).
- `parentStackPick` / `blockStack` still approximate (cap 3 pre-multi-dim; pin 5 post); true `Function::stack.size()` would remove the pin.
- Full `select_must_use_var` (itemize multi-dim, must_read membership, visit_facts accept) only partially modeled; gated on `multiDimArrays` + `inParamExpr`.
