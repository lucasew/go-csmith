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
STALL_LIMIT="${STALL_LIMIT:-3}"
ROLLBACK_ON_REGRESSION="${ROLLBACK_ON_REGRESSION:-1}"

usage() {
  cat <<'USAGE'
Usage: scripts/ralph-loop.sh [options]

Score-gated Claude loop:
  1) run RNG divergence checker (pre)
  2) call Claude with a mode-specific prompt
  3) run checker again (post) and keep only improved iterations
  4) repeat until match, stall limit, or max iterations

Options:
  --seed N
  --max-iters N
  --workdir DIR
  --context N
  --strict-raw
  --prompt-file PATH
  --claude-cmd "CMD"
  --memory-file PATH
  --stall-limit N
  --rollback-on-regression
  --no-rollback-on-regression
  --no-checkpoint-commits
  --checkpoint-commits
  --dry-run
  -h|--help

Env overrides:
  SEED, MAX_ITERS, WORKDIR, CONTEXT, STRICT_RAW, PROMPT_FILE, CLAUDE_CMD,
  MEMORY_FILE, STALL_LIMIT, CHECKPOINT_COMMITS, DRY_RUN
  ROLLBACK_ON_REGRESSION

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
    --stall-limit) STALL_LIMIT="$2"; shift 2 ;;
    --rollback-on-regression) ROLLBACK_ON_REGRESSION=1; shift ;;
    --no-rollback-on-regression) ROLLBACK_ON_REGRESSION=0; shift ;;
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
- stall_limit: $STALL_LIMIT
- rollback_on_regression: $ROLLBACK_ON_REGRESSION

## Iterations

| iter | ts_utc | mode | pre_result | pre_reason | pre_score | pre_div | post_result | post_reason | post_score | post_div | improved | checkpoint |
|---|---|---|---|---|---:|---:|---|---|---:|---:|---:|---|
MEM
fi

run_divergence_check() {
  local report_file="$1"
  set +e
  scripts/find-rng-divergence.sh --seed "$SEED" --workdir "$WORKDIR" --context "$CONTEXT" $([[ "$STRICT_RAW" == "1" ]] && echo "--strict-raw") | tee "$report_file"
  local rc=${PIPESTATUS[0]}
  set -e
  return "$rc"
}

parse_report() {
  local report_file="$1"
  local out_prefix="$2"

  local result="failure"
  local reason="checker_failed"
  local score="-1"
  local mismatch_event=""
  local up_event="<none>"
  local go_event="<none>"
  local mode="rng_alignment"
  local fail_text=""

  if grep -q "^result=match" "$report_file"; then
    result="match"
    reason="match"
    score="999999999"
    mode="done"
  elif grep -q "^result=mismatch" "$report_file"; then
    result="mismatch"
    reason="$(awk -F= '/^reason=/{print $2}' "$report_file" | tail -n1)"
    mismatch_event="$(awk -F= '/^first_divergence_event=/{print $2}' "$report_file" | tail -n1)"
    if [[ "$mismatch_event" =~ ^[0-9]+$ ]]; then
      score="$mismatch_event"
    else
      score="0"
      mismatch_event="0"
    fi
    up_event="$(awk -F': ' '/^upstream_event:/{print $2}' "$report_file" | tail -n1)"
    go_event="$(awk -F': ' '/^go_event:/{print $2}' "$report_file" | tail -n1)"
    mode="rng_alignment"
  else
    fail_text="$(tail -n 5 "$report_file" | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g')"
    if grep -q "go generation/trace failed" "$report_file"; then
      reason="go_generation_failed"
      mode="termination_fix"
    elif grep -q "upstream generation/trace failed" "$report_file"; then
      reason="upstream_generation_failed"
      mode="infra_fix"
    else
      reason="checker_failed"
      mode="infra_fix"
    fi
  fi

  eval "${out_prefix}_result=\"\$result\""
  eval "${out_prefix}_reason=\"\$reason\""
  eval "${out_prefix}_score=\"\$score\""
  eval "${out_prefix}_mismatch_event=\"\$mismatch_event\""
  eval "${out_prefix}_up_event=\"\$up_event\""
  eval "${out_prefix}_go_event=\"\$go_event\""
  eval "${out_prefix}_mode=\"\$mode\""
  eval "${out_prefix}_fail_text=\"\$fail_text\""
}

best_score="-1"
best_iter="0"
stall_count="0"

for ((iter=1; iter<=MAX_ITERS; iter++)); do
  echo ""
  echo "========== ITERATION $iter/$MAX_ITERS =========="

  pre_report="$WORKDIR/seed_${SEED}.iter_${iter}.pre.report.txt"
  run_divergence_check "$pre_report" || true
  parse_report "$pre_report" "pre"

  if [[ "$pre_result" == "match" ]]; then
    echo "[loop] parity achieved before agent call at iteration $iter"
    exit 0
  fi

  iter_prompt="$WORKDIR/seed_${SEED}.iter_${iter}.prompt.md"
  iter_log="$WORKDIR/seed_${SEED}.iter_${iter}.agent.log"
  post_report="$WORKDIR/seed_${SEED}.iter_${iter}.post.report.txt"

  {
    cat "$PROMPT_FILE"
    cat <<P

---
## Iteração Atual
- seed: $SEED
- iteration: $iter/$MAX_ITERS
- mode: ${pre_mode}
- best_score_so_far: $best_score (iter $best_iter)
- current_score: ${pre_score}
- current_result: ${pre_result}
- current_reason: ${pre_reason}
- first_divergence_event: ${pre_mismatch_event:-unknown}
- upstream_event: ${pre_up_event:-<none>}
- go_event: ${pre_go_event:-<none>}
- pre_report_file: $pre_report
- stall_count: $stall_count/$STALL_LIMIT

Instrução:
- Faça apenas uma hipótese por vez.
- Limite patch a no máximo 2 arquivos e evite refactor.
- Se mode=termination_fix: priorize remover loop/recursão não-terminante.
- Se mode=rng_alignment: priorize ordem exata de consumo RNG no primeiro desvio.
- Compare explicitamente com o upstream C++ em ./csmith/src e cite arquivo+função equivalentes.
- Ao final, pare de editar para o loop rodar a próxima comparação.
P
  } > "$iter_prompt"

  {
    echo ""
    echo "### Iteration $iter"
    echo ""
    echo "#### Context"
    echo "- ts_utc: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    echo "- mode: ${pre_mode}"
    echo "- pre_report_file: $pre_report"
    echo "- prompt_file: $iter_prompt"
    echo "- agent_log: $iter_log"
    echo ""
    echo "#### Pre"
    echo "- result: ${pre_result}"
    echo "- reason: ${pre_reason}"
    echo "- score: ${pre_score}"
    echo "- first_divergence_event: ${pre_mismatch_event:-unknown}"
    echo "- upstream_event: ${pre_up_event:-<none>}"
    echo "- go_event: ${pre_go_event:-<none>}"
    if [[ "$pre_result" == "failure" ]]; then
      echo "- failure_tail: ${pre_fail_text:-<none>}"
    fi
  } >> "$MEMORY_FILE"

  cmd="$CLAUDE_CMD --model opus --dangerously-skip-permissions --output-format stream-json --verbose -p \"\$(cat '$iter_prompt')\""
  echo "[loop] claude cmd: $cmd"

  pre_stash_ref=""
  rollback_enabled_this_iter=0
  if [[ "$ROLLBACK_ON_REGRESSION" == "1" && "$DRY_RUN" != "1" ]]; then
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      # Save pre-agent worktree/index/untracked and re-apply it to keep a rollback point.
      git stash push -u -m "ralph-loop pre iter=${iter} score=${pre_score}" >/dev/null
      pre_stash_ref="stash@{0}"
      if git stash apply --index "$pre_stash_ref" >/dev/null 2>&1; then
        rollback_enabled_this_iter=1
      else
        echo "[loop] warning: failed to re-apply pre-iteration stash; rollback disabled for this iteration"
      fi
    fi
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    : > "$iter_log"
  else
    # Direto no terminal, sem redirecionamento mágico
    bash -lc "$cmd"
    : > "$iter_log"
  fi

  run_divergence_check "$post_report" || true
  parse_report "$post_report" "post"

  improved=0
  if [[ "$post_score" =~ ^-?[0-9]+$ && "$pre_score" =~ ^-?[0-9]+$ ]]; then
    if (( post_score > pre_score )); then
      improved=1
    fi
  fi

  if (( improved == 1 )); then
    stall_count=0
    echo "[loop] improved: score ${pre_score} -> ${post_score}"
    if [[ "$rollback_enabled_this_iter" == "1" ]]; then
      git stash drop "$pre_stash_ref" >/dev/null || true
      echo "[loop] dropped pre-iteration rollback stash"
    fi
  else
    stall_count=$((stall_count + 1))
    echo "[loop] no improvement: score ${pre_score} -> ${post_score} (stall=${stall_count}/${STALL_LIMIT})"
    if [[ "$rollback_enabled_this_iter" == "1" ]]; then
      git stash push -u -m "ralph-loop regressed iter=${iter} score=${pre_score}->${post_score}" >/dev/null || true
      if git stash apply --index "stash@{1}" >/dev/null 2>&1; then
        echo "[loop] rolled back regressed iteration using stash snapshot; regressed state kept at stash@{0}"
      else
        echo "[loop] warning: rollback apply failed; manual recovery may be needed"
      fi
      git stash drop "stash@{1}" >/dev/null || true
    fi
  fi

  if [[ "$post_score" =~ ^-?[0-9]+$ ]] && (( post_score > best_score )); then
    best_score="$post_score"
    best_iter="$iter"
  fi

  checkpoint_msg="-"

  if [[ "$post_result" == "match" ]]; then
    {
      echo ""
      echo "#### Post"
      echo "- result: ${post_result}"
      echo "- reason: ${post_reason}"
      echo "- score: ${post_score}"
      echo "- first_divergence_event: ${post_mismatch_event:-unknown}"
      echo "- upstream_event: ${post_up_event:-<none>}"
      echo "- go_event: ${post_go_event:-<none>}"
      echo "- post_report_file: $post_report"
      echo "- improved: $improved"
      echo ""
      echo "| $iter | $(date -u +"%Y-%m-%dT%H:%M:%SZ") | ${pre_mode} | ${pre_result} | ${pre_reason} | ${pre_score} | ${pre_mismatch_event:-0} | ${post_result} | ${post_reason} | ${post_score} | ${post_mismatch_event:-0} | $improved | $checkpoint_msg |"
    } >> "$MEMORY_FILE"
    echo "[loop] parity achieved at iteration $iter"
    exit 0
  fi

  if [[ "$CHECKPOINT_COMMITS" == "1" && "$DRY_RUN" != "1" && "$improved" == "1" ]]; then
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      if ! git diff --quiet || ! git diff --cached --quiet; then
        git add -A
        if ! git diff --cached --quiet; then
          commit_msg="checkpoint: seed ${SEED} iter ${iter} score=${pre_score}->${post_score}"
          git commit -m "$commit_msg" >/dev/null
          checkpoint_msg="$commit_msg"
          echo "[loop] checkpoint commit created: $commit_msg"
          echo "- checkpoint_commit: $commit_msg" >> "$MEMORY_FILE"
        fi
      fi
    fi
  fi

  {
    echo ""
    echo "#### Post"
    echo "- result: ${post_result}"
    echo "- reason: ${post_reason}"
    echo "- score: ${post_score}"
    echo "- first_divergence_event: ${post_mismatch_event:-unknown}"
    echo "- upstream_event: ${post_up_event:-<none>}"
    echo "- go_event: ${post_go_event:-<none>}"
    if [[ "$post_result" == "failure" ]]; then
      echo "- failure_tail: ${post_fail_text:-<none>}"
    fi
    echo "- post_report_file: $post_report"
    echo "- improved: $improved"
    echo "- checkpoint: $checkpoint_msg"
    echo ""
    echo "| $iter | $(date -u +"%Y-%m-%dT%H:%M:%SZ") | ${pre_mode} | ${pre_result} | ${pre_reason} | ${pre_score} | ${pre_mismatch_event:-0} | ${post_result} | ${post_reason} | ${post_score} | ${post_mismatch_event:-0} | $improved | $checkpoint_msg |"
  } >> "$MEMORY_FILE"

  if (( stall_count >= STALL_LIMIT )); then
    echo "[loop] stalled for ${STALL_LIMIT} consecutive iterations; stopping early"
    exit 1
  fi
done

echo "[loop] max iterations reached without parity match"
exit 1
