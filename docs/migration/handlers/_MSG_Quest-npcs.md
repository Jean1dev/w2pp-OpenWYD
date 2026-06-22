# `_MSG_Quest` (0x028B) — enumeração dos NPCs de quest

> Detalha o handler de interação com NPC de quest (resumo em
> [lote2-quest-ranking-cash.md](lote2-quest-ranking-cash.md)). Fonte:
> `Source/Code/TMSrv/_MSG_Quest.cpp` (2753 linhas). Linhas conferidas via `#pragma region`/`case`.

## Despacho (NPC → `npcMode`)
- **Entrada:** `MSG_STANDARDPARM2` — `Parm1 = npcIndex` (o NPC), `Parm2 = confirm` (0 = "perguntar",
  ≠0 = "confirmar/executar").
- **Pré:** `npcIndex` em `[MAX_USER, MAX_MOB)`; cancela trade/autotrade.
- **Resolução do tipo (`:50-160`):** o "tipo de NPC" vem de **`MOB.Merchant`** e, quando
  `Merchant==100`, do **grade** do item equipado no NPC
  (`BASE_GetItemAbilityNosanc(Equip[0], EF_GRADE0)`). Se `npcMode` continua `-1` → `return`.
  Depois entra num `switch(npcMode)`.

| Merchant | Grade | `npcMode` | Linha case | Propósito |
|---------:|------:|-----------|-----------:|-----------|
| 100 | 0 | `QUEST_COVEIRO` | 293 | Quest Arch 256 — etapa 1 (lvl 39–115) |
| 100 | 1 | `QUEST_JARDINEIRO` | 338 | Quest Arch 256 — etapa 2 (lvl 115–190) |
| 100 | 2 | `QUEST_KAIZEN` | 382 | Quest Arch 256 — etapa 3 (lvl 190–265) |
| 100 | 3 | `QUEST_HIDRA` | 426 | Quest Arch 256 — etapa 4 (lvl 265–320) |
| 100 | 4 | `QUEST_ELFOS` | 470 | Quest Arch 256 — etapa 5 |
| 100 | 5 | `LIDER_APRENDIZ` | 2625 | líder aprendiz (sub-quest/promoção) |
| 100 | 7/8/9 | `PERZEN` | 2398 | NPC Perzen |
| 100 | 13 | `QUEST_CAPAREAL` | 2342 | quest capa real |
| 100 | 14 | `CAPAVERDE_TELEPORT` | 2167 | teleporte (capa verde) |
| 100 | 15 | `MOLARGARGULA` | 2236 | quest "molar gárgula" (flag Mortal) |
| 100 | 16 | `TREINADORNEWBIE4` | 2046 | tutorial novato 4 |
| 100 | 22 | `SOBREVIVENTE` | 2597 | NPC sobrevivente |
| 100 | 30 | `GUARDA_REAL_EVT1` | 2664 | guarda real (evento) |
| 72 | — | `UXMAL` | 1313 | NPC Uxmal |
| 36 | — | `TREINADORNEWBIE1` | 1895 | tutorial novato 1 |
| 40 | — | `TREINADORNEWBIE2` | 1938 | tutorial novato 2 |
| 41 | — | `TREINADORNEWBIE3` | 1992 | tutorial novato 3 |
| 8 | — | `CAPAVERDE_TRADE` | 2188 | trade (capa verde) |
| 78 | — | `BLACKORACLE` | 2257 | oráculo negro |
| 120 | — | `CARBUNCLE_WIND` | 2364 | carbúnculo (vento) |
| 4 | — | `GOLD_DRAGON` | 1471 | dragão dourado |
| 10 | — | `AMU_MISTICO` | 253 | quest "Terra Mística" (flag Mortal; exige party) |
| 11 | — | `EXPLOIT_LEADER` | 1626 | líder (exploit/evento) |
| 12 | — | `JEFFI` | 514 | NPC Jeffi |
| 13 | — | `SHAMA` | 612 | NPC Shama |
| 14 / 15 | — | `KING` | 693 | NPC rei |
| 19 | — | `COMP_SEPHI` | 2102 | composição Sephi |
| 26 | — | `KINGDOM` | 1127 | kingdom broker |
| 30 | — | `ZAKUM` | 238 | info da quest Zakum (ocupação da área) |
| 31 | — | `MESTREHAB` | 1745 | mestre de habilidade |
| 68 | — | `GODGOVERNMENT` | 2565 | governo (god government) |
| 58 | — | `MOUNT_MASTER` | 170 | curar/ressuscitar montaria (`Equip[14]`) |
| 62 | — | `ARZAN_DRAGON` | 1397 | dragão de Arzan |
| 76 | — | `URNAMMU` | 1247 | NPC Urnammu |
| 74 | — | `KIBITA` | 2430 | NPC Kibita |
| 200 | — | `CURANDEIRO` | 2717 | curandeiro |

> **Total:** 38 tipos de NPC → 36 blocos `case`/`#pragma region` (KING cobre Merchant 14 e 15).

## Padrão comum de quest (vale para a maioria dos `case`)
1. **Pré-condições:** classe (`extra.ClassMaster` MORTAL/ARCH/CELESTIAL…), **faixa de nível**
   (min/max), flags de quest em `extra.QuestInfo` (Mortal/Arch/Celestial) e/ou itens no inventário.
2. **`confirm == 0`** → `SendSay(npcIndex, ...)`: o NPC **pergunta/explica** (preço, item exigido,
   etc.). Sem efeito de estado.
3. **`confirm != 0`** → **executa**: consome item/coin, concede recompensa (item/exp/teleporte),
   e **grava a flag de progresso** (`extra.QuestInfo.*` ou `pMob[conn].QuestFlag`). Loga.

## Detalhe — cadeia "Quest 256" (Arch) — `COVEIRO → JARDINEIRO → KAIZEN → HIDRA → ELFOS`
Etapas sequenciais de progressão de nível (rumo ao cap 256/Arch). Cada NPC:
- exige `ClassMaster ∈ {MORTAL, ARCH}` (senão `_NN_Level_Limit2`);
- exige **faixa de nível** específica (senão `_NN_Level_limit`);
- consome um **ticket** (item por etapa) do inventário (senão "traga o item X");
- seta `pMob[conn].QuestFlag` e **teleporta** o jogador para a área da prova (jitter `rand()%5-3`).

| Etapa | NPC (grade) | Faixa de nível | Ticket (`sIndex`) | `QuestFlag` | Teleporte (aprox.) |
|------:|-------------|----------------|------------------:|------------:|--------------------|
| 1 | COVEIRO (0) | 39 – 115 | 4038 | 1 | (2398, 2105) |
| 2 | JARDINEIRO (1) | 115 – 190 | 4039 | 2 | (2234, 1714) |
| 3 | KAIZEN (2) | 190 – 265 | 4040 | 3 | (464, 3902) |
| 4 | HIDRA (3) | 265 – 320 | 4041 | 4 | (668, 3756) |
| 5 | ELFOS (4) | (ver `:470`) | (ver fonte) | 5 | (ver fonte) |

## Exemplos de NPCs não-cadeia (amostra detalhada)
- **`MOUNT_MASTER` (Merchant 58, `:170`):** cura/ressuscita a montaria (`Equip[14]`, `sIndex`
  2330–2390). `confirm==0` mostra preço (`g_pItemList[mount].Price`); `confirm!=0` cobra gold e, por
  RNG (`vit -= rand()%3`), restaura a vitalidade do mount **ou** o destrói (`memset`). Recalcula score.
- **`ZAKUM` (Merchant 30, `:238`):** apenas **informa** quem está na área da quest Zakum
  (`GetUserInArea(2180,1160,2296,1270)`); sem efeito.
- **`AMU_MISTICO` (Merchant 10, `:253`):** quest "Terra Mística" — exige `ClassMaster==MORTAL`, flag
  `QuestInfo.Mortal.TerraMistica==0` e estar **em party**; ao confirmar seta
  `QuestInfo.Mortal.TerraMistica = 1`.
- **`TREINADORNEWBIE1..4`, `CAPAVERDE_*`, `KING`, `KINGDOM`, `CURANDEIRO`, traders/info:** NPCs de
  tutorial/teleporte/troca/serviço — mesmo padrão (perguntar/confirmar), efeitos pontuais
  (teleporte, item inicial, cura, troca). Passo-a-passo fino: abrir a linha do `case` indicada na
  tabela de despacho.

## Notas de migração
- **Estado de quest** mora em `STRUCT_MOBEXTRA.QuestInfo` (Mortal/Arch/Celestial, Fase 2 §1.5) e em
  `pMob[conn].QuestFlag` (estado volátil da etapa atual) — mapear cada flag no schema novo.
- **Tudo hardcoded:** faixas de nível, `sIndex` de tickets/recompensas, coordenadas de teleporte e o
  mapeamento Merchant/grade→quest. Forte candidato a **conteúdo data-driven** (ex.: tabela
  `quest`/`quest_step` ou scripts) — alinha com o gap apontado na comparação de projetos
  (open-wyd-scripts) e em game-rules (Fase 4).
- **Identificação do NPC por `Merchant` + `EF_GRADE0` do item equipado** é um quirk a preservar na
  leitura dos dados de NPC (Fase 2 — `NPCGener`/BaseMob) durante a migração.
- **UNVERIFIED (passo-a-passo fino):** recompensas/etapas internas exatas dos blocos longos
  (`JEFFI`, `SHAMA`, `KING`, `KINGDOM`, `URNAMMU`, `ARZAN_DRAGON`, `GOLD_DRAGON`, `EXPLOIT_LEADER`,
  `MESTREHAB`, `KIBITA`, `GODGOVERNMENT`) — o **despacho e o propósito** estão mapeados; abrir a linha
  do `case` ao implementar cada um.

> **Status:** todos os **38 tipos de NPC / 36 `case`** mapeados (gatilho Merchant/grade → modo →
> propósito → linha) + a cadeia Arch detalhada. Enumeração da Fase 5 concluída.
