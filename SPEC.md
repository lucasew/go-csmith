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
| Measurement scripts | `find-rng-divergence.sh`, `compare-upstream.sh`, `parity-gate.sh`, `validate-a.sh`, **`validate-integrity.sh`** |
| Integrity | **Event match alone is not acceptance.** `scripts/validate-integrity.sh` must pass (wired into `parity-gate.sh`). |

### 5.1 Technique — Csmith control flow (required)

**Implementers MUST mirror Csmith’s control flow**, not invent an independent generator that only matches LCG outputs.

1. Open the relevant C++ under `.build/csmith-src/src/` (e.g. `Expression.cpp`, `FunctionInvocation.cpp`, `VariableSelector.cpp`, `StatementAssign.cpp`, `Lhs.cpp`, `CVQualifiers.cpp`).
2. Trace the **same function/method sequence** for the failing event (SITE lines / callsites).
3. Patch Go so the **same RNG consumers** run in the **same order** (function call CREATE → signature → param exprs → body; Assign → AssignOps → RHS Expression → Lhs; etc.).
4. Prefer shared helpers that name the upstream API (`burnCreateArrayVariable` ≈ `CreateArrayVariable`) over ad-hoc coin sequences.
5. Multi-seed: fixes must be **structural** (options/effects/inventory), not `if seed == 4`.

### 5.2 Banned techniques (integrity fail)

These make `first_divergence_event` look good while **gaming** the gate. **`validate-integrity.sh` fails the run** if present:

| Ban | Why |
|-----|-----|
| Packed residual event tables (`f10_late_residual_data.go`, `f10LateResidualPacked`, offline `[]uint32` stream dumps) | Replays a single seed’s RNG offline instead of generating |
| `residualPlayer` / `burnF10LateExprResidual` as primary stream driver | RNG burns without Csmith Expression/Statement graph |
| `silenceTrace` / `rng.silent` to stop tracing | Fakes event-count match after residual exhausts |
| Seed-literal branches (`if seed == 2`) in generator paths | Non-portable hardcodes |
| Event match with **thin source** (Go C ≪ upstream C lines / globals) | Events advanced without materializing AST |

Allowed: temporary debug prints; timeouts; bounded retries that still call real create/select APIs; instrumented upstream for measurement.

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
| Seed 2 event match | **claimed 37939/37939** but **fails integrity** (`validate-integrity.sh`) — residual pack + `silenceTrace` gaming |
| Seed 2 source match | **FAIL integrity** — residual-driven; not Csmith-flow AST |
| 20-seed gate | **Blocked** until integrity passes and multi-seed event+source |

**Integrity (new):** `scripts/validate-integrity.sh` bans residual tables, `residualPlayer`, `silenceTrace`, seed hardcodes, thin source. Wired into `parity-gate.sh`. Event match **without** integrity is **not** a gate pass.

Next: remove residual pack; implement real Expression/Statement after F10#7 per Csmith C++; re-climb multi-seed with integrity green.

**e716–e788 climbed:** `select_must_use_var` after multi-dim IV creates (U2+F75), max-funcs forces stdfunc without F80, ptr-comparison uses `derived_types` size + pointer operand types, parent stack n=5 after multi-dim nesting.

**CreateArray dimension ladder:** instrumented binary / seed2 traces use step **60** (1d 60% / 2d 30% / 3d ~9%), matching the comment in `ArrayVariable.cpp`. The checked-in tree source has `step = 100` (always dim=1 for `num∈[1,99]`), which cannot produce multi-dim events such as seed2 e565 (`U99=93` → sizes 4×4×9). Go follows the live instrumented stream, not the tree literal.

**Known RNG debt:**
- Pointer-element `CreateArrayVariable` alt inits under non-strict arrays should burn full `make_init_value` address-of residual (F20 null prefix only today).
- `parentStackPick` / `blockStack` still approximate (cap 3 pre-multi-dim; pin 5 post); true `Function::stack.size()` would remove the pin.
- Full `select_must_use_var` (itemize multi-dim, must_read membership, visit_facts accept) only partially modeled; gated on `multiDimArrays` + `inParamExpr`.
