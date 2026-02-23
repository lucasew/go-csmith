#!/usr/bin/env bash
set -euo pipefail

SEED="${SEED:-2}"
MAX_ITERS="${MAX_ITERS:-20}"
WORKDIR="${WORKDIR:-/tmp/csmith-parity}"
CONTEXT="${CONTEXT:-8}"
STRICT_RAW="${STRICT_RAW:-0}"
PROMPT_FILE="${PROMPT_FILE:-PROMPT.md}"
CLAUDE_CMD="${CLAUDE_CMD:-claude}"
MEMORY_FILE="${MEMORY_FILE:-}"
CHECKPOINT_COMMITS="${CHECKPOINT_COMMITS:-1}"
DRY_RUN="${DRY_RUN:-0}"

usage() {
  cat <<'USAGE'
Usage: scripts/ralph-loop.sh [options]

Simple Claude loop:
  1) run RNG divergence checker
  2) if mismatch, call Claude with generated prompt
  3) repeat until match or max iterations

Options:
  --seed N
  --max-iters N
  --workdir DIR
  --context N
  --strict-raw
  --prompt-file PATH
  --claude-cmd "CMD"
  --memory-file PATH
  --no-checkpoint-commits
  --checkpoint-commits
  --dry-run
  -h|--help

Env overrides:
  SEED, MAX_ITERS, WORKDIR, CONTEXT, STRICT_RAW, PROMPT_FILE, CLAUDE_CMD,
  MEMORY_FILE, CHECKPOINT_COMMITS, DRY_RUN

Example:
  scripts/ralph-loop.sh --seed 2 --max-iters 30 --claude-cmd "claude"
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed) SEED="$2"; shift 2 ;;
    --max-iters) MAX_ITERS="$2"; shift 2 ;;
    --workdir) WORKDIR="$2"; shift 2 ;;
    --context) CONTEXT="$2"; shift 2 ;;
    --strict-raw) STRICT_RAW=1; shift ;;
    --prompt-file) PROMPT_FILE="$2"; shift 2 ;;
    --claude-cmd) CLAUDE_CMD="$2"; shift 2 ;;
    --memory-file) MEMORY_FILE="$2"; shift 2 ;;
    --checkpoint-commits) CHECKPOINT_COMMITS=1; shift ;;
    --no-checkpoint-commits) CHECKPOINT_COMMITS=0; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ ! -f "$PROMPT_FILE" ]]; then
  echo "prompt file not found: $PROMPT_FILE" >&2
  exit 1
fi

if [[ ! -x scripts/find-rng-divergence.sh ]]; then
  chmod +x scripts/find-rng-divergence.sh
fi

mkdir -p "$WORKDIR"

if [[ -z "$MEMORY_FILE" ]]; then
  MEMORY_FILE="$WORKDIR/seed_${SEED}.memory.md"
fi

if [[ ! -f "$MEMORY_FILE" ]]; then
  cat > "$MEMORY_FILE" <<MEM
# Ralph Loop Memory

- seed: $SEED
- started_at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- prompt_file: $PROMPT_FILE
- claude_cmd: $CLAUDE_CMD

## Iterations
MEM
fi

for ((iter=1; iter<=MAX_ITERS; iter++)); do
  echo ""
  echo "========== ITERATION $iter/$MAX_ITERS =========="

  report_file="$WORKDIR/seed_${SEED}.iter_${iter}.report.txt"
  scripts/find-rng-divergence.sh --seed "$SEED" --workdir "$WORKDIR" --context "$CONTEXT" $([[ "$STRICT_RAW" == "1" ]] && echo "--strict-raw") | tee "$report_file"

  if grep -q "^result=match" "$report_file"; then
    echo "[loop] parity achieved at iteration $iter"
    exit 0
  fi

  mismatch_event="$(awk -F= '/^first_divergence_event=/{print $2}' "$report_file" | tail -n1)"
  up_event="$(awk -F': ' '/^upstream_event:/{print $2}' "$report_file" | tail -n1)"
  go_event="$(awk -F': ' '/^go_event:/{print $2}' "$report_file" | tail -n1)"
  reason="$(awk -F= '/^reason=/{print $2}' "$report_file" | tail -n1)"

  iter_prompt="$WORKDIR/seed_${SEED}.iter_${iter}.prompt.md"
  iter_log="$WORKDIR/seed_${SEED}.iter_${iter}.agent.log"

  {
    cat "$PROMPT_FILE"
    cat <<P

---
## Iteração Atual
- seed: $SEED
- iteration: $iter/$MAX_ITERS
- first_divergence_event: ${mismatch_event:-unknown}
- reason: ${reason:-unknown}
- upstream_event: ${up_event:-<none>}
- go_event: ${go_event:-<none>}
- report_file: $report_file

Instrução:
- Faça as mudanças para empurrar o primeiro desvio para frente.
- Finalize para o loop rodar a próxima comparação.
P
  } > "$iter_prompt"

  {
    echo ""
    echo "### Iteration $iter"
    echo "- timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    echo "- mismatch_event: ${mismatch_event:-unknown}"
    echo "- reason: ${reason:-unknown}"
    echo "- upstream_event: ${up_event:-<none>}"
    echo "- go_event: ${go_event:-<none>}"
    echo "- report_file: $report_file"
    echo "- prompt_file: $iter_prompt"
    echo "- agent_log: $iter_log"
  } >> "$MEMORY_FILE"

  cmd="$CLAUDE_CMD -p \"\$(cat '$iter_prompt')\""
  echo "[loop] claude cmd: $cmd"

  if [[ "$DRY_RUN" == "1" ]]; then
    : > "$iter_log"
  else
    # Direto no terminal, sem redirecionamento mágico
    bash -lc "$cmd"
    : > "$iter_log"
  fi

  if [[ "$CHECKPOINT_COMMITS" == "1" && "$DRY_RUN" != "1" ]]; then
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      if ! git diff --quiet || ! git diff --cached --quiet; then
        git add -A
        if ! git diff --cached --quiet; then
          commit_msg="checkpoint: seed ${SEED} iter ${iter} div=${mismatch_event:-na}"
          git commit -m "$commit_msg" >/dev/null
          echo "[loop] checkpoint commit created: $commit_msg"
          echo "- checkpoint_commit: $commit_msg" >> "$MEMORY_FILE"
        fi
      fi
    fi
  fi
done

echo "[loop] max iterations reached without parity match"
exit 1
