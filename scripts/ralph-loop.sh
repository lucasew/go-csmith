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
RESET_ON_NO_IMPROVEMENT="${RESET_ON_NO_IMPROVEMENT:-1}"

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
  --reset-on-no-improvement
  --no-reset-on-no-improvement
  --no-checkpoint-commits
  --checkpoint-commits
  --dry-run
  -h|--help

Env overrides:
  SEED, MAX_ITERS, WORKDIR, CONTEXT, STRICT_RAW, PROMPT_FILE, CLAUDE_CMD,
  MEMORY_FILE, STALL_LIMIT, CHECKPOINT_COMMITS, DRY_RUN
  RESET_ON_NO_IMPROVEMENT

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
    --reset-on-no-improvement) RESET_ON_NO_IMPROVEMENT=1; shift ;;
    --no-reset-on-no-improvement) RESET_ON_NO_IMPROVEMENT=0; shift ;;
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

if git rev-parse --show-toplevel >/dev/null 2>&1; then
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cd "$REPO_ROOT"
fi

if [[ ! -f "$PROMPT_FILE" ]]; then
  echo "prompt file not found: $PROMPT_FILE" >&2
  exit 1
fi

if [[ ! -x scripts/find-rng-divergence.sh ]]; then
  chmod +x scripts/find-rng-divergence.sh
fi

mkdir -p "$WORKDIR"

if [[ -z "$MEMORY_FILE" ]]; then
  MEMORY_FILE="MEMORY.md"
fi

if [[ ! -f "$MEMORY_FILE" ]]; then
  cat > "$MEMORY_FILE" <<MEM
# Ralph Loop Memory

- seed: $SEED
- started_at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- prompt_file: $PROMPT_FILE
- claude_cmd: $CLAUDE_CMD
- stall_limit: $STALL_LIMIT
- reset_on_no_improvement: $RESET_ON_NO_IMPROVEMENT

## Iterations

| run | iter | ts_utc | mode | pre_score | post_score | improved | post_div | reason | action |
|---:|---:|---|---|---:|---:|---:|---:|---|---|
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

backup_memory_file() {
  local backup_file=""
  if [[ -f "$MEMORY_FILE" ]]; then
    backup_file="$(mktemp)"
    cp "$MEMORY_FILE" "$backup_file"
  fi
  echo "$backup_file"
}

restore_memory_file() {
  local backup_file="$1"
  if [[ -n "$backup_file" && -f "$backup_file" ]]; then
    mkdir -p "$(dirname "$MEMORY_FILE")"
    cp "$backup_file" "$MEMORY_FILE"
    rm -f "$backup_file"
  fi
}

reset_repo_except_memory() {
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    return 0
  fi
  local memory_backup=""
  memory_backup="$(backup_memory_file)"
  git reset HEAD -- . >/dev/null
  git checkout HEAD -- . >/dev/null
  restore_memory_file "$memory_backup"
}

commit_memory_only() {
  local iter="$1"
  local pre_score="$2"
  local post_score="$3"
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    return 0
  fi
  if [[ ! -f "$MEMORY_FILE" ]]; then
    echo "[loop] warning: memory file not found '$MEMORY_FILE'"
    return 0
  fi
  local add_err=""
  add_err="$(git add -A -- "$MEMORY_FILE" 2>&1)" || {
    echo "[loop] warning: could not git add memory file '$MEMORY_FILE' ($add_err)"
    return 0
  }
  if git diff --cached --quiet; then
    return 0
  fi
  local msg="memory: seed ${SEED} iter ${iter} score=${pre_score}->${post_score}"
  git commit -m "$msg" >/dev/null
  echo "[loop] memory commit created: $msg"
}

auto_fill_learned_block() {
  local iter="$1"
  local pre_score="$2"
  local post_score="$3"
  local post_div="$4"
  local post_reason="$5"
  local changed_files=""
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    changed_files="$(git diff --name-only | tr '\n' ',' | sed 's/,$//')"
  fi
  if [[ -z "$changed_files" ]]; then
    changed_files="none_detected"
  fi
  {
    echo ""
    echo "## Learned (iter $iter)"
    echo "- hypothesis: align first divergence path around event ${post_div}"
    echo "- cpp_reference: csmith/src/<to_be_filled_by_agent>"
    echo "- go_change: ${changed_files}"
    echo "- memory_reuse: table row from previous iteration"
    echo "- outcome_expected: move score from ${pre_score} to >${post_score}"
    echo "- handoff: inspect event ${post_div}, reason=${post_reason}, and align RNG call order"
  } >> "$MEMORY_FILE"
}

best_score="-1"
best_iter="0"
stall_count="0"
run_seq=0
last_iter_seen=999999

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
  memory_tail_file="$WORKDIR/seed_${SEED}.iter_${iter}.memory_tail.md"

  if [[ -f "$MEMORY_FILE" ]]; then
    tail -n 120 "$MEMORY_FILE" > "$memory_tail_file" || true
  else
    : > "$memory_tail_file"
  fi

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

## Memória Recente (obrigatório usar)
\`\`\`md
$(cat "$memory_tail_file")
\`\`\`

Instrução:
- Faça apenas uma hipótese por vez.
- Limite patch a no máximo 2 arquivos e evite refactor.
- Se mode=termination_fix: priorize remover loop/recursão não-terminante.
- Se mode=rng_alignment: priorize ordem exata de consumo RNG no primeiro desvio.
- Compare explicitamente com o upstream C++ em ./csmith/src e cite arquivo+função equivalentes.
- REGRA DURA: antes de mexer no código, VOCÊ DEVE preencher o bloco ## Learned (iter $iter) em MEMORY.md com a hipótese.
- Se não preencher a hipótese no MEMORY.md, NÃO faça patch de código.
- Atualize MEMORY.md com o bloco:
  - ## Learned (iter $iter)
  - - hypothesis: ...
  - - cpp_reference: caminho::funcao
  - - go_change: arquivo::funcao
  - - memory_reuse: qual item anterior você reutilizou
  - - outcome_expected: ...
  - - handoff: ...
- Ao final, pare de editar para o loop rodar a próxima comparação.
P
  } > "$iter_prompt"

  cmd="$CLAUDE_CMD --model opus --dangerously-skip-permissions --output-format stream-json --verbose -p \"\$(cat '$iter_prompt')\""
  echo "[loop] claude cmd: $cmd"

  if [[ "$DRY_RUN" == "1" ]]; then
    : > "$iter_log"
  else
    bash -lc "$cmd" | tee "$iter_log"
  fi

  if [[ "$DRY_RUN" != "1" ]]; then
    if ! grep -q "^## Learned (iter $iter)" "$MEMORY_FILE"; then
      auto_fill_learned_block "$iter" "$pre_score" "$post_score" "${post_mismatch_event:-0}" "$post_reason"
      echo "[loop] warning: agente não escreveu bloco Learned (iter $iter); bloco auto-preenchido adicionado"
    fi
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
  else
    stall_count=$((stall_count + 1))
    echo "[loop] no improvement: score ${pre_score} -> ${post_score} (stall=${stall_count}/${STALL_LIMIT})"
  fi

  if [[ "$post_score" =~ ^-?[0-9]+$ ]] && (( post_score > best_score )); then
    best_score="$post_score"
    best_iter="$iter"
  fi

  checkpoint_msg="-"

  if [[ "$post_result" == "match" ]]; then
    action="match"
    if [[ "$DRY_RUN" != "1" ]]; then
      if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        git add -A
        if ! git diff --cached --quiet; then
          commit_msg="checkpoint: seed ${SEED} iter ${iter} score=${pre_score}->${post_score} (match)"
          git commit -m "$commit_msg" >/dev/null
          checkpoint_msg="$commit_msg"
          action="checkpoint_match"
          echo "[loop] checkpoint commit created: $commit_msg"
        fi
      fi
    fi
    echo "| ${run_seq} | ${iter} | $(date -u +"%Y-%m-%dT%H:%M:%SZ") | ${pre_mode} | ${pre_score} | ${post_score} | ${improved} | ${post_mismatch_event:-0} | ${post_reason} | ${action} |" >> "$MEMORY_FILE"
    echo "[loop] parity achieved at iteration $iter"
    exit 0
  fi

  if [[ "$DRY_RUN" != "1" && "$improved" == "1" ]]; then
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      if ! git diff --quiet || ! git diff --cached --quiet; then
        git add -A
        if ! git diff --cached --quiet; then
          commit_msg="checkpoint: seed ${SEED} iter ${iter} score=${pre_score}->${post_score}"
          git commit -m "$commit_msg" >/dev/null
          checkpoint_msg="$commit_msg"
          echo "[loop] checkpoint commit created: $commit_msg"
        fi
      fi
    fi
  fi

  action="memory_only"
  if [[ "$improved" == "1" ]]; then
    action="checkpoint"
  fi
  echo "| ${run_seq} | ${iter} | $(date -u +"%Y-%m-%dT%H:%M:%SZ") | ${pre_mode} | ${pre_score} | ${post_score} | ${improved} | ${post_mismatch_event:-0} | ${post_reason} | ${action} |" >> "$MEMORY_FILE"

  if [[ "$DRY_RUN" != "1" && "$improved" != "1" ]]; then
    commit_memory_only "$iter" "$pre_score" "$post_score"
    if [[ "$RESET_ON_NO_IMPROVEMENT" == "1" ]]; then
      reset_repo_except_memory
      echo "[loop] reset applied after memory commit: git checkout HEAD em tudo"
    fi
  fi

  if (( stall_count >= STALL_LIMIT )); then
    echo "[loop] stalled for ${STALL_LIMIT} consecutive iterations; stopping early"
    exit 1
  fi
done

echo "[loop] max iterations reached without parity match"
exit 1
  if (( iter <= last_iter_seen )); then
    run_seq=$((run_seq + 1))
  fi
  last_iter_seen="$iter"
