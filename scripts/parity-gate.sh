#!/usr/bin/env bash
set -euo pipefail

# Opinionated parity gate: integrity (anti-gaming) then multi-seed compare.
COUNT="${COUNT:-20}"
SEED_START="${SEED_START:-2}"
TIMEOUT_SECS="${TIMEOUT_SECS:-5}"
# Set SKIP_INTEGRITY=1 only for emergency debug; gates default to strict.
SKIP_INTEGRITY="${SKIP_INTEGRITY:-0}"

UPSTREAM_GEN_CMD="${UPSTREAM_GEN_CMD:-/nix/store/hrf9nixgjz33q1563l9bxx155py477qv-csmith-2.3.0/bin/csmith}"
UPSTREAM_INCLUDE="${UPSTREAM_INCLUDE:-/nix/store/hrf9nixgjz33q1563l9bxx155py477qv-csmith-2.3.0/include/csmith-2.3.0}"
GO_GEN_CMD="${GO_GEN_CMD:-GOCACHE=/tmp/go-cache go run ./cmd/csmith}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ "$SKIP_INTEGRITY" != "1" ]]; then
  echo "=== parity-gate: integrity (anti-gaming) ==="
  if [[ -x ./.build/csmith-instrumented/src/csmith ]]; then
    SEED="$SEED_START" WORKDIR="${WORKDIR:-.build/parity}" \
      GO_CMD="$GO_GEN_CMD" UPSTREAM_CMD=./.build/csmith-instrumented/src/csmith \
      scripts/validate-integrity.sh
  else
    scripts/validate-integrity.sh --code-only
  fi
fi

UPSTREAM_GEN_CMD="$UPSTREAM_GEN_CMD" \
UPSTREAM_INCLUDE="$UPSTREAM_INCLUDE" \
GO_INCLUDE="$UPSTREAM_INCLUDE" \
GO_GEN_CMD="$GO_GEN_CMD" \
scripts/compare-upstream.sh \
  --count "$COUNT" \
  --seed-start "$SEED_START" \
  --timeout "$TIMEOUT_SECS"
