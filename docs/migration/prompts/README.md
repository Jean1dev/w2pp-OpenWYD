# Prompts de execução da migração

Prompts mestres para a **etapa de execução** da migração (a documentação de engenharia reversa em
`docs/migration/` já está pronta). Mesmo estilo do `../DOCUMENTATION-PROMPT.md`: contexto embutido +
bloco PROMPT copiável + notas de uso.

Ordem de uso:

1. **`../DOCUMENTATION-PROMPT.md`** — (já executado) gerou toda a pasta `docs/migration/`.
2. **`implement.md`** — planeja e executa a reescrita em Go, **fase a fase** (sequência do
   `migration-plan §4`), seguindo `development-guidelines/Go-development-guidelines.md`.
3. **`validate.md`** — audita o trabalho de cada fase (paridade comportamental + aderência às
   guidelines + Definition of Done do `migration-plan §6`) e emite relatório de conformidade.

Prompts auxiliares de frentes grandes:

- **`agent-prompt-all-skills-fix.md`** — cria a documentação por classe, matriz de gaps, plano de
  implementação e plano de validação para corrigir todas as skills de forma faseada. Deve esgotar as
  fontes locais antes de gerar perguntas para o agente Windows. Status: executado; resultados em
  `../skills/`, com respostas Windows consolidadas em `../skills/windows-agent-findings.md`.

`PROGRESS.md` (criado pelo `implement.md`) registra o status de cada fase e os itens UNVERIFIED.
