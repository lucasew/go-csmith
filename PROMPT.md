# Objetivo
Você está trabalhando no projeto `go-csmith` para obter equivalência real com o Csmith upstream (C++).

## Função objetivo (obrigatória)
- Score primário: `first_divergence_event` (quanto maior, melhor).
- `result=match` é sucesso final.
- Se houver crash/timeout/falha de geração, trate como problema de terminação/estabilidade.

## Modo de trabalho por iteração
1. Leia `mode` e o `pre_report_file`.
2. Forme exatamente 1 hipótese técnica para o primeiro desvio/falha atual.
3. Aplique patch mínimo para testar essa hipótese.
4. Pare de editar; o loop executará a validação pós-patch.
5. Guarde o que você aprendeu e outras informações relevantes em MEMORY.md

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

## Saída esperada da iteração
- Resumo curto:
  - hipótese escolhida
  - arquivo(s) alterado(s)
  - por que a mudança deve melhorar o score

## Recursos
- Código upstream C++: `./csmith`
