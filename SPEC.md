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
| Plateau | Stop blind guessing; read **C++** under `pkgs.csmith.src` for the failing path; align Go. |
| Stop | Goal gates met, or human stops the work. **No** “max iterations then abandon.” |
| Ralph loop | **Removed.** No `ralph-loop.sh`, no agent `PROMPT.md` / `MEMORY.md` ritual. |
| Measurement scripts | Keep/fix: `find-rng-divergence.sh`, `compare-upstream.sh`, `parity-gate.sh`, `validate-a.sh` |

### 5.1 Technique

Any technique is allowed if it:

1. Does not mess up things outside the repo’s control
2. Has timeouts against non-termination
3. Can still converge (on plateau: consult C++; do not invent unmotivated RNG hacks)

Order of preference in practice: fix local RNG/call-path alignment first; structural reshape only when the same divergence class blocks progress.

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
| Seed 2 event match | **In progress** — `first_divergence_event` advanced **92 → 302** (defaults) |
| Seed 2 source match | Blocked on event match |
| 20-seed gate | Not started |

Next plateau focus: `buildFunctionCallExpr` after event 104 (`F50` then expected another `F50` vs Go `F0`); address-of constant hex width still forced to 16 pending accurate type tracking.
