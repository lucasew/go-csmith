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

```bash
# Level B (testing.T)
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  go test ./pkg/csmith -run UpstreamBodyParityBattery -count=1

# Level C (testing.F)
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  go test ./pkg/csmith -run '^$' -fuzz=FuzzUpstreamBodyParityFuzzy -fuzztime=16x
```

Unset `CSMITH_UPSTREAM` → **warn + skip** (no `.build`/`PATH` search). Body mismatches use **go-cmp** (`-upstream +go`). Failing `-fuzz` inputs land in `pkg/csmith/testdata/fuzz/` and re-run until fixed or deleted. Details: SPEC §3.5a.
