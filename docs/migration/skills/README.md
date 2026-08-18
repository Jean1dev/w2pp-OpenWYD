# Skills - auditoria de paridade

Esta pasta registra a auditoria inicial de todas as skills carregadas pelo servidor Go a partir de
`Release/Common/SkillData.csv`. O objetivo é separar o que ja tem fluxo Go verificavel do que ainda
precisa de implementacao, captura Windows ou golden cases no cliente real.

## Metodologia

- Fonte de catalogo: `Release/Common/SkillData.csv`, arquivo ISO-8859, lido com nomes decodificados em Latin-1.
- Loader auditado: `tmserver/internal/content/skilldata.go`, que usa o indice da coluna 0, ignora linhas fora de `[0,248)`, consome 20 inteiros e divide `AffectTime` por 4 (`affectTimeDivisor`), igual ao legado (`Basedef.cpp:6708`). O loader e **fiel**: nao ha tuning aqui.
- Duracao de buff (issue #229): o tuning vive em **um unico lugar**, `world.AffectDuration`, aplicado quando um **cast** instala o affect (`SetAffect`/`SetTick`). Defaults do tmServer: `ScalePct` 15, piso 60 s (so para affects **nao agressivos**), teto 10 min — flags `-affect-scale-pct` / `-affect-min-seconds` / `-affect-max-minutes`. Zero value = formula legada exata, que e o que os testes de paridade exercitam. A issue #92 tinha tentado o mesmo objetivo dobrando o divisor do loader para 8; isso mexia so na base (o multiplicador de mastery `(100+Special)/100`, ate 5x, e quem domina) e, por ser divisao inteira no load, truncava a cauda curta da tabela alem do pretendido (issue #236). Ver `ingame-bugs.md` B14.
- Banda alvo do tuning (issue #236, segunda metade): o `ScalePct` **nao e uniforme** — ele so se aplica fora da banda que a politica existe para cortar. `AffectDuration.withinTargetBand` isenta um affect **nao agressivo** cuja duracao **base** (`AffectTime+1`, sem mastery) ja cabe sob `MaxTicks`, entao a cauda curta do CSV (85 Escudo Dourado, 90 Toxina, 96 Poder Superior, 41 Teleporte, 216 Magia Misteriosa…) mantem a curva legada com o piso de 60 s por baixo, em vez de ser esmagada ate ele. Affects **agressivos** ficam de fora da isencao de proposito: alongar CC (Perseguicao, Nevasca, Lamina Congelante) seria mudanca de PvP que nenhuma issue pediu. Regra em uma frase: *buff amigo segue o legado ate o teto, com piso de 60 s; affect hostil e buff longo seguem no scale.* Ver `ingame-bugs.md` B15.
- A pertinencia a banda e julgada pela **base**, nunca pelo tick ja inflado pela mastery — assim ela e propriedade da linha do CSV e constante em `Special`, e a curva nao pode inverter. Julgando o valor inflado, o Teleporte (base 16 ticks) cruzava `MaxTicks` no alto da mastery e caia do ramo identidade para o de 15 %: 2m08 sem mastery, 6m24 em `Special` 200 e de volta a **1m36** em `Special` 400. Nao ha limiar delicado aqui: as bases do `SkillData.csv` sao 1-16 ticks e depois 151, sem nada no meio. Travado por `TestReleaseAffectDurationIsMonotonicInMastery`.
- Os testes de duracao rodam contra o **CSV real** (`TestReleaseAffectDuration*` em `handler/affect_duration_content_test.go`), nao contra fixtures de `content.Spell`. Esse era o seam por onde as tentativas anteriores passaram: a #92 mexeu no loader e a #229 na politica, cada uma testada isoladamente contra fixtures, e nenhuma foi rodada sobre a tabela inteira — por isso a cauda curta passou batido duas vezes.
- Divergencias deliberadas do legado (o port e fiel por padrao; estas tres nao sao, e cada uma tem
  justificativa escrita em `ingame-bugs.md`): o remap de elementos do affect 25 (**B13**, issue #233), a
  politica de duracao de buff (**B14/B15**, issues #92/#229/#236) e o efeito do affect 24 — Samaritano
  passa a dar CON/MaxHP em vez de AC (**B16**, issue #267).
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

> **A matriz sobre-declara.** A auditoria da issue #267 (`audit-affects.md`) achou efeito de conteudo
> sem tratamento no Go — tick types 3/12/46 (skills 226, 202, 225) sao no-ops silenciosos — e a
> `DoRemoveHide` da Huntress nunca foi portada. Leia `151/0/0/0` como "existe case em Go para o efeito
> principal", nao como cobertura. E leia a evidencia `SCORE` do mesmo jeito: ela diz que existe um case
> no `affect_score.go`, **nao** que alguem conferiu o numero. Linha com `SCORE` e sem `TEST` e linha que
> so se confirma lendo o codigo — foi assim que a #267 nasceu.

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
- `audit-affects.md`: auditoria do pipeline de affects (issue #267) — divida mapeada que ainda nao virou codigo.
- `progress.md`: tracker de execucao por fase; prevalece enquanto as tabelas por classe nao forem recontadas.
- `implementation-plan.md`: fases executaveis.
- `validation-plan.md`: testes automatizados e validacao manual.
- `windows-agent-findings.md`: respostas consolidadas do agente Windows.
- `windows-agent-questions.md`: status das perguntas Windows e pendencias de captura.
- `windows-agent-prompts.md`: prompts historicos copiaveis usados na rodada Windows.
