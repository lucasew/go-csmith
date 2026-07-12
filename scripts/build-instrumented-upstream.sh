#!/usr/bin/env bash
# Build RNG-instrumented upstream csmith from pkgs.csmith.src + in-repo patch.
# Output: .build/csmith-instrumented/src/csmith (gitignored)
#
# Note: ArrayVariable::CreateArrayVariable in the source tree uses step=100
# (always 1-d for num in 1..99), but seed2 RNG traces from this binary show
# multi-dim arrays consistent with step=60 (see SPEC.md §9 and burnCreateArrayVariable).
# Prefer live traces when aligning Go; do not "fix" Go back to step=100 without re-measuring.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if type loadDotfilesEnv &>/dev/null; then
  loadDotfilesEnv
elif [[ -n "${BASE_NIX_PATH:-}" ]]; then
  export NIX_PATH="$BASE_NIX_PATH"
fi

PATCH="$ROOT/third_party/csmith-rng-trace.patch"
BUILD_ROOT="$ROOT/.build"
SRC_DIR="$BUILD_ROOT/csmith-src"
BUILD_DIR="$BUILD_ROOT/csmith-instrumented"
BIN="$BUILD_DIR/src/csmith"

if [[ ! -f "$PATCH" ]]; then
  echo "missing patch: $PATCH" >&2
  exit 1
fi

echo "[1/4] Resolving pkgs.csmith.src..."
SRC_STORE="$(nix-build '<nixpkgs>' -A csmith.src --no-out-link)"
echo "  $SRC_STORE"

echo "[2/4] Preparing writable tree + applying patch..."
rm -rf "$SRC_DIR"
mkdir -p "$BUILD_ROOT"
cp -a "$SRC_STORE" "$SRC_DIR"
chmod -R u+w "$SRC_DIR"
# patch may already be applied if re-run on same tree; we always fresh-copy
(cd "$SRC_DIR" && patch -p1 < "$PATCH")

echo "[3/4] Building instrumented csmith (nix-shell + cmake)..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Use nixpkgs build inputs matching the csmith package.
nix-shell -p cmake m4 libbsd gcc --run "
  set -euo pipefail
  cd '$BUILD_DIR'
  cmake '$SRC_DIR' -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_FLAGS='-std=c++98'
  cmake --build . -j\"\${NIX_BUILD_CORES:-$(nproc)}\"
"

if [[ ! -x "$BIN" ]]; then
  # some cmake layouts put binary in build root
  if [[ -x "$BUILD_DIR/csmith" ]]; then
    BIN="$BUILD_DIR/csmith"
  else
    echo "instrumented binary not found under $BUILD_DIR" >&2
    find "$BUILD_DIR" -name csmith -type f 2>/dev/null | head
    exit 1
  fi
fi

echo "[4/4] Smoke-testing CSMITH_TRACE_RNG..."
lines="$(CSMITH_TRACE_RNG=1 timeout 30s "$BIN" --seed 2 2>&1 >/dev/null | rg -c '^[UF] depth=' || true)"
echo "  trace events on seed 2: ${lines:-0}"
if [[ -z "${lines:-}" || "$lines" -lt 1 ]]; then
  echo "instrumented build produced no RNG trace lines" >&2
  exit 1
fi

echo "OK: $BIN"
# record path for scripts
echo "$BIN" > "$BUILD_ROOT/upstream-instrumented.path"
echo "$SRC_STORE" > "$BUILD_ROOT/upstream-src.path"
