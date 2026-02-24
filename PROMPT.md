# Objetivo
Você está trabalhando no projeto `go-csmith` para obter equivalência real com o Csmith upstream (C++).

## Função objetivo (obrigatória)
- Score primário: `first_divergence_event` (quanto maior, melhor).
- `result=match` é sucesso final.
- Se houver crash/timeout/falha de geração, trate como problema de terminação/estabilidade.

## Modo de trabalho por iteração
1. Leia `mode` e o `pre_report_file`.
2. Faça rastreio completo do primeiro desvio/falha (start -> call path -> decisão RNG).
3. Localize no C++ upstream (`./csmith/src`) o caminho equivalente e confirme ordem/semântica de RNG.
4. Forme exatamente 1 hipótese técnica para o primeiro desvio/falha atual.
5. Aplique patch mínimo para testar essa hipótese.
6. Pare de editar; o loop executará a validação pós-patch.
7. Guarde o que você aprendeu em `MEMORY.md` no final da iteração.
8. Use este formato fixo em `MEMORY.md`:
   - `## Learned (iter N)`
   - `- hypothesis: ...`
   - `- cpp_reference: caminho::funcao`
   - `- go_change: arquivo::funcao`
   - `- outcome_expected: ...`
   - `- handoff: ...`

## Estratégia por modo
- `mode=termination_fix`:
  - Priorize remover não-terminação (recursão sem avanço de profundidade, filtros impossíveis, loops sem bound).
  - Não mude comportamento além do necessário para voltar a terminar.
- `mode=rng_alignment`:
  - Priorize alinhar ordem e semântica de consumo RNG no primeiro ponto de desvio.
  - Foque em call path local do desvio (filtros/retries/contexto/profundidade).

## Restrições
- Faça patch em no máximo 2 arquivos por iteração.
- Não faça refactor amplo nem mudanças cosméticas.
- Não adicione hacks de consumo RNG sem base no upstream.
- Não desative funcionalidades para “passar” no checker.
- Toda mudança em Go deve citar referência C++ equivalente (arquivo + função).

## Saída esperada da iteração
- Resumo curto:
  - hipótese escolhida
  - arquivo(s) alterado(s)
  - por que a mudança deve melhorar o score
- Inclua obrigatoriamente:
  - `TRACE_PATH`: 3-8 passos do rastreio (início -> fim).
  - `CPP_REFERENCE`: arquivo(s)/função(ões) do upstream usados.
  - `HANDOFF`: próximo alvo objetivo para o agente seguinte.

## Recursos
- Código upstream C++: `./csmith`
