# Prompt Mestre — Auditoria e Correção Faseada de Todas as Skills

> **Status 2026-07-10:** executado em `docs/migration/skills/`. As respostas Windows
> `WIN-SKILL-001..006` foram incorporadas em
> `docs/migration/skills/windows-agent-findings.md`; as perguntas restantes sao somente capturas ao
> vivo nao bloqueantes para a implementacao server-side.
>
> **Como usar:** copie a seção "PROMPT" abaixo para um agente de codificação/documentação no
> repositório local. O objetivo desta rodada **não é sair alterando gameplay imediatamente**: primeiro
> produzir um mapa confiável das skills por classe, cruzar esse mapa com a implementação Go atual,
> registrar lacunas de paridade e entregar um plano faseado de implementação + validação.
>
> O agente deve esgotar as fontes locais antes de pedir ajuda ao agente Windows. Só gere perguntas para
> o agente Windows quando faltar evidência objetiva que dependa da fonte completa, dumper MSVC x86 ou
> captura do cliente real.

## Contexto

Este projeto é a reescrita em Go do servidor WYD, mirando o cliente original sem modificação. O
contexto operacional atual está em `docs/migration/SESSION-PRIMER.md`; leia-o antes de qualquer
conclusão. A frente de skills/buffs já tem implementação parcial em Go, mas ainda há pontos
`UNVERIFIED`, comportamentos incompletos por classe e riscos de regressão em combate.

Referências obrigatórias:

- `AGENTS.md` / instruções do repositório: single-owner game loop, sem locks no estado do mundo.
- `docs/migration/SESSION-PRIMER.md`: status atual, lições de debug e pontos `UNVERIFIED`.
- `docs/migration/prompts/agent-prompt-skills.md`: lacunas já conhecidas para eventual agente Windows.
- `docs/migration/handlers/_MSG_Attack.md` e `docs/migration/handlers/lote2-itens-char.md`.
- `docs/migration/game-rules.md`, `docs/migration/data-formats.md`, `docs/migration/protocol-spec.md`.
- Implementação atual em `tmserver/internal/handler`, `tmserver/internal/combat`,
  `tmserver/internal/content`, `tmserver/internal/world`, `tmserver/internal/protocol`.
- Fonte legada local em `Source/`, quando existir. Não assuma que ela está completa.

---

## PROMPT

````
Você é um engenheiro sênior trabalhando na correção faseada de TODAS as skills dos personagens no
w2pp-OpenWYD. Sua tarefa nesta execução é produzir documentação técnica e um plano de implementação
validável. Não implemente alterações de gameplay nesta primeira rodada, exceto se o usuário pedir
explicitamente depois.

O escopo cobre as 4 classes e skills compartilhadas:
- Transknight / TK: índices 0..23.
- Foema / FM: índices 24..47.
- BeastMaster / BM: índices 48..71.
- Huntress / HT: índices 72..95.
- Sephira/shared/extra-class: índices 96+.

Princípio central: evidência antes de paridade. Para cada regra, fórmula, constante, pacote ou efeito,
aponte uma fonte local concreta. Quando a fonte local não provar o comportamento, registre
`UNVERIFIED` e gere uma pergunta objetiva para o agente Windows. Não chute.

═══════════════════════════════════════════════════════════
FASE 0 — LEITURA E INVENTÁRIO LOCAL
═══════════════════════════════════════════════════════════

Leia obrigatoriamente:
- `docs/migration/SESSION-PRIMER.md`
- `docs/migration/prompts/agent-prompt-skills.md`
- `docs/migration/handlers/_MSG_Attack.md`
- `docs/migration/handlers/lote2-itens-char.md`
- `docs/migration/game-rules.md`
- `tmserver/internal/content/skilldata.go`
- `tmserver/internal/combat/skill.go`
- `tmserver/internal/handler/combat.go`
- `tmserver/internal/handler/skill.go`
- `tmserver/internal/handler/affect_score.go`
- `tmserver/internal/handler/affect_tick.go`
- `tmserver/internal/handler/transform.go`
- `tmserver/internal/handler/summon.go`
- `tmserver/internal/world/affect.go`
- testes relacionados a skills/combate/affects.

Use `rg` para localizar:
- `SkillData`, `STRUCT_SPELL`, `g_pSpell`, `BASE_GetSkillDamage`, `BASE_GetManaSpent`.
- `SetAffect`, `SetTick`, `GetParryRate`, `BASE_GetDoubleCritical`, `GetCurrentScore`.
- `InstanceType`, `AffectType`, `TickType`, `Passive`, `Aggressive`, `LearnedSkill`.
- `SkillIndex`, `Dam`, `ReqMp`, `CurrentMp`, `CurrentExp`, `MSG_UpdateScore`.

Também localize o arquivo real de conteúdo `SkillData.csv` usado pelo servidor e extraia dele o
catálogo completo de skills. Preserve nomes como aparecem no conteúdo, incluindo encoding legado.

═══════════════════════════════════════════════════════════
FASE 1 — DOCUMENTAÇÃO POR CLASSE
═══════════════════════════════════════════════════════════

Crie a pasta `docs/migration/skills/` e produza estes arquivos:

- `README.md`: índice, metodologia, legenda dos status e visão geral dos gaps.
- `transknight.md`: skills 0..23.
- `foema.md`: skills 24..47.
- `beastmaster.md`: skills 48..71, incluindo transformação e summons.
- `huntress.md`: skills 72..95.
- `sephira-shared.md`: skills 96+ e regras compartilhadas.

Cada arquivo por classe deve ter:
- Resumo da identidade mecânica da classe.
- Tabela de todas as skills da classe.
- Seção de regras comuns da classe.
- Seção de lacunas e riscos.
- Seção de testes mínimos esperados.

Use esta tabela base para cada skill:

| Índice | Nome | Árvore | Tipo | Target | Mana | SkillPoint | Instance | Affect/Tick | Fórmula/Efeito | Status | Evidência |
|-------:|------|--------|------|--------|-----:|-----------:|----------|-------------|----------------|--------|-----------|

Regras para preencher:
- `Árvore`: `index%24/8 + 1`, com observação quando a fonte usar outra divisão.
- `Tipo`: ativo, passivo, dano, heal, buff, debuff, transformação, summon, utilitário ou desconhecido.
- `Instance`: `InstanceType/InstanceValue` do `SkillData.csv`.
- `Affect/Tick`: `AffectType/AffectValue/AffectTime` e `TickType/TickValue`.
- `Fórmula/Efeito`: descreva o comportamento esperado e cite a função/fonte.
- `Status`: um de `IMPLEMENTED`, `PARTIAL`, `MISSING`, `UNVERIFIED`.
- `Evidência`: arquivo local + função/seção, ou `WINDOWS_REQUIRED:<id>`.

Critérios de status:
- `IMPLEMENTED`: há evidência local do comportamento e código Go cobrindo o fluxo principal.
- `PARTIAL`: existe parte do comportamento, mas falta fórmula, pacote, visual, edge case, persistência
  ou teste.
- `MISSING`: a regra está documentada ou aparece no conteúdo, mas não existe implementação Go.
- `UNVERIFIED`: não há evidência local suficiente para afirmar paridade.

═══════════════════════════════════════════════════════════
FASE 2 — MATRIZ DE IMPLEMENTAÇÃO ATUAL
═══════════════════════════════════════════════════════════

Crie `docs/migration/skills/implementation-plan.md`.

O plano deve cruzar a documentação com a implementação Go atual por subsistema, não só por arquivo:
- Catálogo e parser de `SkillData.csv`.
- Aprendizado de skill (`MSG_ApplyBonus`, requisitos, pontos, 8ª skill exclusiva, livros Sephira).
- Validação de cast (`_MSG_Attack`, classe, learned-mask, passiva, mana, ouro especial).
- Fórmulas de dano/heal e resistência.
- Buffs/debuffs (`SetAffect`, `SetTick`, refresh de score e ícones).
- Transformação BM.
- Summons BM.
- Hotbar/short skill.
- Persistência de learned skill, special, skill bar, short skill e affects.
- Pacotes S→C que fazem o cliente atualizar HP/MP/EXP/score/equip/affect.

Organize a implementação em fases pequenas e executáveis:

1. **Core comum de skills**
   - Corrigir lacunas compartilhadas de cast, mana, parry/crit se verificados, score refresh e pacotes.
   - Manter a regra de regressão: melee nunca pode ser bloqueado por `SkillIndex`/`Dam` não verificados.

2. **Transknight**
   - Implementar/corrigir skills 0..23 por árvore.
   - Destacar skills que dependem de weapon damage, mastery TK, mitigação ou status defensivo.

3. **Foema**
   - Implementar/corrigir skills 24..47.
   - Destacar heal, buffs, resistência, fórmulas de magia e interação com learned bit 19 quando aplicável.

4. **BeastMaster**
   - Implementar/corrigir skills 48..71.
   - Separar transformação, buffs de forma, summons, mesh/visual e efeitos no score.

5. **Huntress**
   - Implementar/corrigir skills 72..95.
   - Destacar skills com weapon damage, distância, dano físico/mágico e buffs/debuffs próprios.

6. **Sephira/shared**
   - Implementar/corrigir skills 96+.
   - Mapear books, learned bits, level scaling, resurrection e utilitários.

7. **Fechamento de paridade**
   - Resolver itens `UNVERIFIED` com agente Windows/captura real.
   - Consolidar golden cases e validação manual no cliente.

Para cada fase, liste:
- Objetivo.
- Mudanças esperadas.
- Arquivos prováveis.
- Riscos de paridade.
- Testes obrigatórios.
- Dependências de perguntas Windows, se houver.

═══════════════════════════════════════════════════════════
FASE 3 — PLANO DE VALIDAÇÃO
═══════════════════════════════════════════════════════════

Crie `docs/migration/skills/validation-plan.md`.

Inclua validação automatizada:
- Unit tests para fórmulas puras em `tmserver/internal/combat`.
- Handler tests para cast com mana suficiente/insuficiente, skill não aprendida, classe errada,
  passiva, custo em ouro, alvo morto, alvo NPC merchant e múltiplos alvos.
- Tests de affects para aplicação, reuso de slot, expiração, tick periódico, score derivado e
  persistência.
- Tests por classe cobrindo pelo menos uma skill de cada árvore e todas as skills especiais.
- Regressões obrigatórias:
  - melee sempre causa dano contra mob válido mesmo com encoding de `SkillIndex`/`Dam` não verificado.
  - `MSG_Attack` autoritativo usa `HEADER.ID = ESCENE_FIELD` quando necessário para HP/MP/EXP.
  - buffs não vazam entre personagens na mesma conexão.
  - score persistido não fica contaminado por buff derivado.

Inclua validação manual com cliente real:
- Aprender skill no NPC mestre e confirmar hotbar/learned mask.
- Castar buff curto, observar ícone e tempo real de expiração.
- Castar skill de dano em mob e player, observando HP/MP/EXP.
- Transformação BM com mudança visual para si e para outro player.
- Summon BM, ataque do summon e desaparecimento/limite.
- Resurrection/heal quando aplicável.

Para cada cenário manual, especifique:
- classe/nível/skill;
- preparação de personagem;
- ação no cliente;
- pacote/log esperado;
- resultado visual esperado;
- critério PASS/FAIL.

═══════════════════════════════════════════════════════════
FASE 4 — PERGUNTAS PARA O AGENTE WINDOWS
═══════════════════════════════════════════════════════════

Crie `docs/migration/skills/windows-agent-questions.md`.

Só inclua perguntas que NÃO puderam ser respondidas por fontes locais. Cada pergunta deve ter:
- ID estável, ex.: `WIN-SKILL-001`.
- Por que precisamos disso.
- Arquivos/funções locais já checados.
- O que o agente Windows deve extrair.
- Formato esperado da resposta.

Use o agente Windows para:
- Funções ausentes na fonte local, como `GetParryRate`, `BASE_GetDoubleCritical`, partes exatas de
  `BASE_GetCurrentScore`, `SetReqHp/SetReqMp`, `CurrentWeather` e tabelas hardcoded não presentes.
- Layouts que exigem `sizeof`/`offsetof` pelo MSVC x86.
- Comportamento real do cliente 12000/7662 em `SkillIndex`, `Dam`, `ReqMp`, tempo de buff, delay de
  skill e pacotes visuais.
- Tabelas completas como `pTransBonus[]` ou campos de `STRUCT_MOBEXTRA`, se forem necessários para
  uma skill.

Não peça ao agente Windows "corrigir skills" genericamente. Faça perguntas pequenas, verificáveis e
salvas em arquivos `captura-wyd-skills-<assunto>.md`.

═══════════════════════════════════════════════════════════
REGRAS DE ENGENHARIA
═══════════════════════════════════════════════════════════

- Preserve o invariant do mundo: todo estado de jogo é mutado somente dentro do loop de `World.Run`.
- Não introduza locks para estado de mundo.
- Não confie em campos do cliente como autoridade de dano, mana, HP, skill aprendida ou alvo válido.
- Não quebre o caminho melee por validações de skill ainda não verificadas.
- Fórmulas de jogo devem ser funções puras testáveis quando possível.
- Layouts de pacote/save devem continuar por offsets explícitos.
- Todo item sem evidência deve ficar marcado como `UNVERIFIED`, com pergunta Windows ou teste pendente.

═══════════════════════════════════════════════════════════
ENTREGÁVEL FINAL
═══════════════════════════════════════════════════════════

Ao final, entregue um resumo curto com:
- arquivos criados em `docs/migration/skills/`;
- contagem de skills por status;
- fases de implementação propostas;
- perguntas Windows geradas;
- testes prioritários.

Não implemente código de gameplay nesta execução. O próximo passo será escolher uma fase pequena do
`implementation-plan.md` e aplicar patches com testes.
````

---

## Notas de uso

- Este prompt substitui o impulso de "corrigir todas as skills" por uma auditoria rastreável e
  faseada. Isso evita regressões como bloquear melee por campos ainda não verificados do cliente.
- Rode este prompt antes de abrir uma fase de código. Depois use o documento gerado
  `implementation-plan.md` como backlog técnico.
- Se `windows-agent-questions.md` sair vazio, prossiga sem agente Windows. Se sair com perguntas,
  resolva primeiro as que bloqueiam a fase escolhida.
