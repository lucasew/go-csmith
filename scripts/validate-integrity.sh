#!/usr/bin/env bash
# validate-integrity.sh — fail the parity pipeline if Go is gaming the RNG/source gates.
#
# Catches techniques that make event streams match without following Csmith's
# control flow or materializing equivalent AST (packed residual tables,
# silenceTrace, seed-specific hardcodes, etc.).
#
# Usage:
#   scripts/validate-integrity.sh [--seed N] [--workdir DIR] [--skip-gen]
#   scripts/validate-integrity.sh --code-only   # static checks only
#
# Env:
#   SEED, WORKDIR, UPSTREAM_CMD, GO_CMD  (same defaults as find-rng-divergence)
#   MAX_RESIDUAL_SITE_FRAC  (default 0.02) max fraction of GO trace sites from residual
#   MIN_SOURCE_LINE_RATIO   (default 0.80) go.c lines / up.c lines when events match
#   MIN_GLOBAL_RATIO        (default 0.70) go g_ count / up g_ count when events match
#   ALLOW_INTEGRITY_FAIL=1  print findings but exit 0 (debug only)
set -euo pipefail

SEED="${SEED:-2}"
WORKDIR="${WORKDIR:-/tmp/csmith-parity}"
UPSTREAM_CMD="${UPSTREAM_CMD:-./.build/csmith-instrumented/src/csmith}"
GO_CMD="${GO_CMD:-GOCACHE=/tmp/go-cache go run ./cmd/csmith}"
CODE_ONLY=0
SKIP_GEN=0
MAX_RESIDUAL_SITE_FRAC="${MAX_RESIDUAL_SITE_FRAC:-0.02}"
MIN_SOURCE_LINE_RATIO="${MIN_SOURCE_LINE_RATIO:-0.80}"
MIN_GLOBAL_RATIO="${MIN_GLOBAL_RATIO:-0.70}"
ALLOW_INTEGRITY_FAIL="${ALLOW_INTEGRITY_FAIL:-0}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

usage() {
  sed -n '2,25p' "$0" | sed 's/^# \?//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed) SEED="$2"; shift 2 ;;
    --workdir) WORKDIR="$2"; shift 2 ;;
    --code-only) CODE_ONLY=1; shift ;;
    --skip-gen) SKIP_GEN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

fail=0
findings=()

note() { findings+=("$1"); echo "  FAIL: $1" >&2; fail=1; }
ok()   { echo "  OK: $1"; }

echo "=== Integrity check (anti-gaming) ==="
echo "repo=$ROOT seed=$SEED"

# ---------------------------------------------------------------------------
# 1) Static: banned control-flow bypasses in pkg/csmith
# ---------------------------------------------------------------------------
echo "[1/4] Static source bans (pkg/csmith)..."

# Packed seed-specific residual event tables (replay upstream stream offline).
if [[ -f pkg/csmith/f10_late_residual_data.go ]]; then
  note "banned file pkg/csmith/f10_late_residual_data.go (seed-packed residual event table)"
fi
if rg -n --glob 'pkg/csmith/**/*.go' 'f10LateResidualPacked|residualEv packing|Code generated from f10_late' >/dev/null 2>&1; then
  note "banned packed residual table symbols (f10LateResidualPacked / generated residual data)"
fi

# Residual player that consumes RNG without Csmith call graph.
if rg -n --glob 'pkg/csmith/**/*.go' 'type residualPlayer|burnF10LateExprResidual|func \(p \*residualPlayer\)' >/dev/null 2>&1; then
  note "banned residualPlayer / burnF10LateExprResidual (RNG replay without Csmith control flow)"
fi

# Silencing trace to fake event-count match after residual exhaust.
if rg -n --glob 'pkg/csmith/**/*.go' 'silenceTrace\(|\.silent\s*=\s*true' >/dev/null 2>&1; then
  note "banned silenceTrace / rng.silent (hides post-residual RNG; fakes event-count match)"
fi

# Hardcoded seed branches that short-circuit generation.
if rg -n --glob 'pkg/csmith/**/*.go' 'if\s+seed\s*==\s*[0-9]+|seed\s*==\s*2\b|SEED\s*==\s*2' >/dev/null 2>&1; then
  note "banned seed-literal branches in generator (seed-specific hardcodes)"
fi

# Giant opaque uint32 residual packs only (not normal generator.go).
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  base=$(basename "$f")
  # residual* files of any size with packed tables
  if [[ "$base" == *residual* ]]; then
    sz=$(wc -c <"$f" | tr -d ' ')
    if [[ "$sz" -gt 4096 ]] && rg -q '0x[0-9a-fA-F]{8}|\[\]uint32' "$f"; then
      note "oversized residual table in $f (${sz} bytes) — offline RNG stream dump"
    fi
  fi
done < <(find pkg/csmith -name '*.go' -type f)

# Documented anti-pattern: continueAfterF10Constant that only burns residual then silences.
if rg -n --glob 'pkg/csmith/**/*.go' 'continueAfterF10Constant' -A8 | rg -q 'burnF10Late|silenceTrace'; then
  note "continueAfterF10Constant wires residual burn + silenceTrace (event match without AST)"
fi

if [[ $fail -eq 0 ]]; then
  ok "no banned residual/silenceTrace/seed-table patterns in pkg/csmith"
fi

# ---------------------------------------------------------------------------
# 2) Static: require structural hooks that mirror Csmith (presence, not completeness)
# ---------------------------------------------------------------------------
echo "[2/4] Structural Csmith-flow hooks..."

require_sym() {
  local pat="$1" msg="$2"
  if ! rg -n --glob 'pkg/csmith/**/*.go' "$pat" >/dev/null 2>&1; then
    note "missing expected Csmith-flow hook: $msg (pattern: $pat)"
  fi
}

# These should exist as real functions/paths, not only residual.
require_sym 'func buildFunctionCallExpr|FunctionInvocation' "function invocation / CREATE path"
require_sym 'SelectParentLocal|parentStackPick|createOnDemandFromParentLocal' "ParentLocal selection/create"
require_sym 'make_random|SelectLType|emitLValueAssignment|StatementAssign' "statement assign / Lhs path"
require_sym 'CreateArray|burnCreateArrayVariable|NewArrayVariable' "array create path"

if [[ $fail -eq 0 ]]; then
  ok "core Csmith-like entrypoints present"
fi

if [[ "$CODE_ONLY" == "1" ]]; then
  echo
  if [[ $fail -eq 0 ]]; then
    echo "INTEGRITY_PASS (code-only)"
    exit 0
  fi
  echo "INTEGRITY_FAIL (code-only) findings=${#findings[@]}"
  if [[ "$ALLOW_INTEGRITY_FAIL" == "1" ]]; then exit 0; fi
  exit 1
fi

# ---------------------------------------------------------------------------
# 3) Dynamic: generate seed and check residual sites + source bulk
# ---------------------------------------------------------------------------
echo "[3/4] Dynamic generate seed=$SEED..."
mkdir -p "$WORKDIR"
BASE="${WORKDIR}/integrity_seed_${SEED}"
UP_C="${BASE}.up.c"
GO_C="${BASE}.go.c"
GO_RNG="${BASE}.go.rng"

if [[ "$SKIP_GEN" != "1" ]]; then
  if [[ ! -x "$UPSTREAM_CMD" ]]; then
    note "instrumented upstream missing: $UPSTREAM_CMD (run scripts/build-instrumented-upstream.sh)"
  else
    timeout 90s bash -lc "$UPSTREAM_CMD --seed $SEED > '$UP_C' 2>/dev/null" || note "upstream generate failed"
  fi
  if ! CSMITH_TRACE_RNG=1 CSMITH_TRACE_RNG_SITE=1 CSMITH_TRACE_RNG_FILE="$GO_RNG" \
      timeout 90s bash -lc "$GO_CMD --seed $SEED > '$GO_C'"; then
    note "go generate/trace failed"
  fi
else
  # Reuse find-rng-divergence artifacts if present
  [[ -f "${WORKDIR}/seed_${SEED}.up.c" ]] && UP_C="${WORKDIR}/seed_${SEED}.up.c"
  [[ -f "${WORKDIR}/seed_${SEED}.go.c" ]] && GO_C="${WORKDIR}/seed_${SEED}.go.c"
  [[ -f "${WORKDIR}/seed_${SEED}.go.rng" ]] && GO_RNG="${WORKDIR}/seed_${SEED}.go.rng"
fi

echo "[4/4] Trace-site + source bulk ratios..."

if [[ -f "$GO_RNG" ]]; then
  total_sites=$(rg -c '@' "$GO_RNG" 2>/dev/null || echo 0)
  residual_sites=$(rg -c 'residualPlayer|burnF10Late|f10_late|continueAfterF10' "$GO_RNG" 2>/dev/null || echo 0)
  total_sites=${total_sites//$'\n'/}
  residual_sites=${residual_sites//$'\n'/}
  if [[ "${total_sites:-0}" -gt 0 ]]; then
    # awk float compare
    frac=$(awk -v r="$residual_sites" -v t="$total_sites" 'BEGIN{printf "%.6f", r/t}')
    echo "  residual_site_frac=$frac ($residual_sites / $total_sites)"
    awk -v f="$frac" -v max="$MAX_RESIDUAL_SITE_FRAC" 'BEGIN{exit !(f+0 > max+0)}' && \
      note "GO RNG sites dominated by residual path (frac=$frac > max=$MAX_RESIDUAL_SITE_FRAC)"
  else
    note "GO RNG trace has no @callsite lines — cannot verify real control flow"
  fi
else
  note "missing GO RNG trace for site analysis: $GO_RNG"
fi

if [[ -f "$UP_C" && -f "$GO_C" ]]; then
  up_lines=$(wc -l <"$UP_C" | tr -d ' ')
  go_lines=$(wc -l <"$GO_C" | tr -d ' ')
  up_g=$(rg -c '^[sg]tatic .* g_[0-9]+' "$UP_C" 2>/dev/null || rg -c '\bg_[0-9]+\b' "$UP_C" | head -1 || echo 0)
  go_g=$(rg -c '^[sg]tatic .* g_[0-9]+' "$GO_C" 2>/dev/null || rg -c '\bg_[0-9]+\b' "$GO_C" | head -1 || echo 0)
  # Prefer declaration-ish counts
  up_g=$(rg -c '\bg_[0-9]+' "$UP_C" | head -1 || echo 0)
  go_g=$(rg -c '\bg_[0-9]+' "$GO_C" | head -1 || echo 0)
  # Unique globals
  up_ug=$(rg -o '\bg_[0-9]+\b' "$UP_C" | sort -u | wc -l | tr -d ' ')
  go_ug=$(rg -o '\bg_[0-9]+\b' "$GO_C" | sort -u | wc -l | tr -d ' ')
  echo "  source_lines up=$up_lines go=$go_lines"
  echo "  unique_globals up=$up_ug go=$go_ug"
  line_ratio=$(awk -v g="$go_lines" -v u="$up_lines" 'BEGIN{if(u<1)u=1; printf "%.4f", g/u}')
  g_ratio=$(awk -v g="$go_ug" -v u="$up_ug" 'BEGIN{if(u<1)u=1; printf "%.4f", g/u}')
  echo "  line_ratio=$line_ratio global_ratio=$g_ratio"
  awk -v f="$line_ratio" -v min="$MIN_SOURCE_LINE_RATIO" 'BEGIN{exit !(f+0 < min+0)}' && \
    note "source too thin vs upstream (line_ratio=$line_ratio < min=$MIN_SOURCE_LINE_RATIO) — likely residual without AST"
  awk -v f="$g_ratio" -v min="$MIN_GLOBAL_RATIO" 'BEGIN{exit !(f+0 < min+0)}' && \
    note "too few unique globals vs upstream (global_ratio=$g_ratio < min=$MIN_GLOBAL_RATIO)"
else
  note "missing UP/GO C sources for bulk comparison ($UP_C / $GO_C)"
fi

echo
if [[ $fail -eq 0 ]]; then
  echo "INTEGRITY_PASS"
  exit 0
fi

echo "INTEGRITY_FAIL (${#findings[@]} finding(s))"
echo "Remediation: delete residual stream dumps; remove silenceTrace; implement real"
echo "  Function/Expression/Statement/VariableSelector paths mirroring Csmith C++"
echo "  (.build/csmith-src). Event match alone is not acceptance if integrity fails."
if [[ "$ALLOW_INTEGRITY_FAIL" == "1" ]]; then
  echo "(ALLOW_INTEGRITY_FAIL=1 — not failing process)"
  exit 0
fi
exit 1
