# Agent instructions (go-csmith)

**Source of truth:** [`SPEC.md`](SPEC.md). Component inventory: [`CHECKLIST.md`](CHECKLIST.md).

## Mandatory process (do not weaken)

| Topic | Where |
|-------|--------|
| Event / `first_div` climb is never the work item | SPEC §3.1a |
| Checklist 100% + body diverge = false marks | SPEC §3.1b |
| Exact program body gate; stats out | SPEC §3.5 |
| **Multi-seed parity plan (levels A/B/C/D, frozen battery, phases, anti-cheat)** | **SPEC §3.5a / §3.5b** |
| 1:1 unit contract + same-hunk cite | SPEC §3.4, §5.4 |
| Cheat catalog (reject on sight) | SPEC §7 |

Read **SPEC §3.5a** before multi-seed work. Comply without rewriting it into
stream-climbing, battery shrinkage, or seed-literal generation.

## North star

**Drop-in:** defaults + golden-CLI flags + seed → **bit-identical program body** vs
golden `csmith` (levels A–D). **Library:** full `Options` + `Generate` for in-process
use; non-CLI CGOptions knobs are library-only (not drop-in gates).

## Body parity vs upstream (integration)

Lives in **`./test/bodyparity`** (not `pkg/csmith`) so core unit tests stay fast.

```bash
# Level B battery (defaults + frozen seeds)
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  go test ./test/bodyparity -run TestBodyParityBattery -count=1

# Level C sequential defaults (long)
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  BODYPARITY_LEVELC=10m go test ./test/bodyparity -run TestBodyParityLevelC -count=1 -timeout 15m

# Level D drop-in flags+seed fuzz
CSMITH_UPSTREAM=.build/csmith-instrumented/src/csmith \
  go test ./test/bodyparity -run '^$' -fuzz=FuzzBodyParity -fuzztime=30s
```

`CSMITH_UPSTREAM` required. `Options` + `Generate` = library; `CLIArgs` / `ForDropInParity` /
`OptionsFromFuzzBlob` = drop-in surface only. Body mismatches: **go-cmp**. Crashers:
`test/bodyparity/testdata/fuzz/`. Details: SPEC §3.5a / §3.5b.
