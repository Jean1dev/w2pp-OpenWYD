# Contratos — Quest, Ranking & Cash (lote 2)

> NPCs de quest, duelo/ranking e cápsula (cash). Fonte: `TMSrv/_MSG_*.cpp`.

---

## `_MSG_Quest` (0x028B) — interação com NPC de quest (o maior handler de regras)
- **Gatilho/struct:** `MSG_STANDARDPARM2` (`Parm1 = npcIndex`, `Parm2 = confirm`).
- **Validações:** `npcIndex` em `[MAX_USER, MAX_MOB)` (precisa ser NPC); cancela trade/autotrade.
- **Despacho por NPC (`npcMode`):** o "tipo de quest" vem de `Merchant==100` + o **grade** do item
  equipado no NPC (`BASE_GetItemAbilityNosanc(Equip[0], EF_GRADE0)`):
  `0=COVEIRO, 1=JARDINEIRO, 2=KAIZEN, 3=HIDRA, 4=ELFOS, 5=LIDER_APRENDIZ, 7/8/9=PERZEN, 13=CAPAREAL`,
  … (lista grande; arquivo de **2753 linhas**).
- **Efeitos:** por `npcMode`/`confirm`, executa as etapas de quest — checa pré-requisitos (nível,
  itens, `mobExtra.QuestInfo`), consome itens, entrega recompensas (exp/gold/item), avança flags em
  `mobExtra` (Mortal/Arch/Celestial) e `STRUCT_QUEST` (quest diária). Diálogos via `SendSay`.
- **Enumeração completa:** os 38 tipos de NPC de quest (despacho Merchant/grade → modo → propósito) e
  a cadeia Arch ("Quest 256") estão em **[_MSG_Quest-npcs.md](_MSG_Quest-npcs.md)**.
- **Anti-cheat/risco:** **maior superfície de regras de progressão** do jogo. Estado de quest mora em
  `STRUCT_MOBEXTRA.QuestInfo` (Fase 2 §1.5) + `pMob[conn].QuestFlag` — mapear cada flag. Faixas de
  nível, tickets, recompensas e coordenadas são **hardcoded** → fortes candidatos a conteúdo
  data-driven (tabela de quests). Passo-a-passo fino de alguns NPCs longos fica como UNVERIFIED no
  doc dedicado.

## `_MSG_ReqRanking` (0x039F) — duelo / ranking PvP
- **Gatilho/struct:** `MSG_STANDARDPARM2` (`Parm1 = tDuel`, `Parm2 = DuelParm` 0..4).
- **Validações:** `DuelParm` em `[0,4]`; `tDuel` em `(0,MAX_USER)`; alvo não pode ter `Whisper`
  bloqueado ("Deny whisper").
- **Efeitos:**
  - `DuelParm != 4` (pedido): registra `RankingTarget`/`RankingType` e **encaminha o convite** ao alvo
    (`pUser[tDuel].cSock.AddMessage`). Tipos 1/2 exigem ambos com guilda.
  - `DuelParm == 4` (aceite): valida reciprocidade (`pUser[tDuel].RankingTarget==conn`), bloqueia se
    `RankingProgress`, e inicia `DoRanking(type, conn, tDuel)`. `SendEtc` em ambos. Loga.
- **Risco:** estado de duelo em `pUser` (RankingTarget/Type/Progress); preservar a máquina de
  pedido→aceite. `DoRanking` (em `Server.cpp`) tem a lógica de combate ranqueado.

## `_MSG_CapsuleInfo` (0x02CD) — info de cápsula (cash)
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = Index`).
- **Efeitos:** seta `ID=conn`, reescreve `Type=_MSG_DBCapsuleInfo` e **encaminha ao DBSrv** (consulta
  de cápsula/cash resolvida lá).
- **Risco:** relay puro → vira RPC no `dbServer`. Sem validação local de `Index` (validar no novo).
