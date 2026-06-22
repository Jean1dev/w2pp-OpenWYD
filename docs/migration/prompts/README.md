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

`PROGRESS.md` (criado pelo `implement.md`) registra o status de cada fase e os itens UNVERIFIED.
