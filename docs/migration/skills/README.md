# Skills - auditoria de paridade

Esta pasta registra a auditoria inicial de todas as skills carregadas pelo servidor Go a partir de
`Release/Common/SkillData.csv`. O objetivo é separar o que ja tem fluxo Go verificavel do que ainda
precisa de implementacao, captura Windows ou golden cases no cliente real.

## Metodologia

- Fonte de catalogo: `Release/Common/SkillData.csv`, arquivo ISO-8859, lido com nomes decodificados em Latin-1.
- Loader auditado: `tmserver/internal/content/skilldata.go`, que usa o indice da coluna 0, ignora linhas fora de `[0,248)`, consome 20 inteiros e divide `AffectTime` por 4 (`affectTimeDivisor`), igual ao legado (`Basedef.cpp:6708`). O loader e **fiel**: nao ha tuning aqui.
- Duracao de buff (issue #229): o tuning vive em **um unico lugar**, `world.AffectDuration`, aplicado quando um **cast** instala o affect (`SetAffect`/`SetTick`). Defaults do tmServer: `ScalePct` 15, piso 60 s (so para affects **nao agressivos**), teto 10 min — flags `-affect-scale-pct` / `-affect-min-seconds` / `-affect-max-minutes`. Zero value = formula legada exata, que e o que os testes de paridade exercitam. A issue #92 tinha tentado o mesmo objetivo dobrando o divisor do loader para 8; isso mexia so na base (o multiplicador de mastery `(100+Special)/100`, ate 5x, e quem domina) e, por ser divisao inteira no load, truncava a cauda curta da tabela alem do pretendido (issue #236). Ver `ingame-bugs.md` B14.
- Divisao por classe: TK `0..23`, FM `24..47`, BM `48..71`, HT `72..95`, Sephira/shared `96+`.
- Arvore: `(index % 24) / 8 + 1`.
- Status e evidencias foram cruzados com `tmserver/internal/combat`, `tmserver/internal/handler`, `tmserver/internal/world`, `tmserver/internal/protocol`, `Source/Code`, `Source/Buff Loop.txt` e os testes existentes.

## Legenda de status

| Status | Significado |
|--------|-------------|
| IMPLEMENTED | O fluxo principal existe em Go e ha fonte local para a formula ou efeito principal. |
| PARTIAL | Existe uma parte relevante, mas falta uma regra especial, efeito secundario, pacote, visual, target/range, persistencia especifica ou golden case. |
| MISSING | O comportamento aparece no conteudo ou fonte legada, mas nao existe implementacao Go do efeito principal. |
| UNVERIFIED | A arvore local nao prova a regra; precisa de captura real, layout MSVC x86 ou fonte completa Windows. |

## Contagem atual

| Grupo | IMPLEMENTED | PARTIAL | MISSING | UNVERIFIED | Total |
|-------|------------:|--------:|--------:|-----------:|------:|
| Transknight | 24 | 0 | 0 | 0 | 24 |
| Foema | 24 | 0 | 0 | 0 | 24 |
| BeastMaster | 24 | 0 | 0 | 0 | 24 |
| Huntress | 24 | 0 | 0 | 0 | 24 |
| Sephira/shared | 55 | 0 | 0 | 0 | 55 |
| **Total** | **151** | **0** | **0** | **0** | **151** |

`UNVERIFIED` ficou em zero na matriz por skill porque as lacunas locais desta rodada sao principalmente codigo ausente ou parcial. As perguntas Windows foram respondidas e agora `windows-agent-questions.md` rastreia apenas capturas ao vivo pendentes que documentam UI/bytes reais, sem bloquear a implementacao server-side provada por fonte.

## Codigos de evidencia

| Codigo | Fonte local |
|--------|-------------|
| CSV | `Release/Common/SkillData.csv`; parser em `tmserver/internal/content/skilldata.go:79-132`; legado em `Source/Code/Basedef.cpp:6657-6695`. |
| CAST | Validacao e gasto de skill em `tmserver/internal/handler/combat.go:28-303`; legado em `Source/Code/TMSrv/_MSG_Attack.cpp:21-270`. |
| DMG | Formulas em `tmserver/internal/combat/skill.go:29-136` e `tmserver/internal/handler/combat.go:373-415`; legado em `Source/Code/Basedef.cpp:1486-1515,6071-6077,6998-7096` e `_MSG_Attack.cpp:552-610`. |
| AFF | Aplicacao de `SetAffect`/`SetTick` em `tmserver/internal/handler/combat.go:305-370` e `tmserver/internal/world/affect.go:98-156`; legado em `Source/Code/TMSrv/Server.cpp:9209-9290`. |
| SCORE | Efeitos de score em `tmserver/internal/handler/affect_score.go:7-117`, `tmserver/internal/handler/item.go:687-830`, `tmserver/internal/handler/score_derive.go:82-185`; icones em `tmserver/internal/protocol/score.go:35-87`. |
| TRANSFORM | BM transform em `tmserver/internal/handler/transform.go:7-131`; tabela local em `Source/Code/Basedef.cpp:759-767`; legado de refresh visual em `_MSG_Attack.cpp:1242-1248`. |
| SUMMON | BM summon em `tmserver/internal/handler/summon.go:12-177`; tabela local em `Source/Code/Basedef.cpp:745-756`; legado em `_MSG_Attack.cpp:809-837`. |
| LEGACY | Fonte legada documenta regra ou efeito especial, normalmente em `_MSG_Attack.cpp:715-1170` ou `Source/Buff Loop.txt`. |
| WIN | Respostas Windows consolidadas em `windows-agent-findings.md`: dumper MSVC x86, fonte completa e dados cliente/servidor. |
| MISSING_GO | Busca local nos pacotes Go de handler/combat/world/protocol nao encontrou implementacao do efeito principal. |

## Principais gaps

- Sephira/shared foi fechado com as respostas `WIN-SKILL-007/008`: `SecLearnedSkill` e campo morto/reservado, `200..247` usam `LearnedSkill & (1 << (skillnum % 24))`, e affects `40/41/43/44/45/46/47/48` sao icon-only/no-op.
- As regras server-side de `_MSG_Attack`, `ReqHp/ReqMp`, delay, affect timer, layouts MSVC x86, `SecLearnedSkill` e effects 40+ ja foram provadas pelo agente Windows; a captura ao vivo do cliente 12000 continua pendente apenas para documentar bytes/visual reais.
- `GetParryRate`, `BASE_GetDoubleCritical`, `pSummonBonus` e `pTransBonus` existem na fonte local; captura real agora serve principalmente para validar visual/bytes de cliente.
- A regra de regressao B12 continua obrigatoria: melee e `Dam=-2`, skill e `Dam=-1`, slot vazio e `Dam=0`; nunca confiar em dano calculado vindo do cliente.

## Arquivos

- `transknight.md`: skills `0..23`.
- `foema.md`: skills `24..47`.
- `beastmaster.md`: skills `48..71`, transformacao e summons.
- `huntress.md`: skills `72..95`.
- `sephira-shared.md`: skills `96+`.
- `progress.md`: tracker de execucao por fase; prevalece enquanto as tabelas por classe nao forem recontadas.
- `implementation-plan.md`: fases executaveis.
- `validation-plan.md`: testes automatizados e validacao manual.
- `windows-agent-findings.md`: respostas consolidadas do agente Windows.
- `windows-agent-questions.md`: status das perguntas Windows e pendencias de captura.
- `windows-agent-prompts.md`: prompts historicos copiaveis usados na rodada Windows.
