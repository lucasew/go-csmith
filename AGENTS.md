# Agent instructions (go-csmith)

**Source of truth:** [`SPEC.md`](SPEC.md). Component inventory: [`CHECKLIST.md`](CHECKLIST.md).

## Mandatory process (do not weaken)

| Topic | Where |
|-------|--------|
| Event / `first_div` climb is never the work item | SPEC §3.1a |
| Checklist 100% + body diverge = false marks | SPEC §3.1b |
| Exact program body gate; stats out | SPEC §3.5 |
| **Multi-seed parity plan (levels A/B/C, frozen battery, phases, anti-cheat)** | **SPEC §3.5a** |
| 1:1 unit contract + same-hunk cite | SPEC §3.4, §5.4 |
| Cheat catalog (reject on sight) | SPEC §7 |

Read **SPEC §3.5a** before multi-seed work. Comply without rewriting it into
stream-climbing, battery shrinkage, or seed-literal generation.

## North star

Default options + seed → **bit-identical C program body** as golden upstream
(emergent from fair C++-linked units). Seed 2 alone is not multi-seed done.

## Body parity vs upstream (integration)

Lives in **`./test/bodyparity`** (not `pkg/csmith`) so core unit tests stay fast.

```bash
# Level B battery
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  go test ./test/bodyparity -run TestBodyParityBattery -count=1

# Level C fuzz (usual day-to-day while closing residual seeds)
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  go test ./test/bodyparity -run '^$' -fuzz=FuzzBodyParity -fuzztime=30s
```

`CSMITH_UPSTREAM` required. Body mismatches use **go-cmp**. Crashers: `test/bodyparity/testdata/fuzz/`. Details: SPEC §3.5a.
