# Contratos — Itens & Personagem (lote 2)

> Movimentação de itens (não-trade), atributos e flags do personagem. Fonte: `TMSrv/_MSG_*.cpp`.
> Slots e `STRUCT_ITEM` na Fase 2; `_MSG_UseItem`/`_MSG_DropItem`/`_MSG_GetItem` no lote 1.

---

## `_MSG_TradingItem` (0x0376) — mover item (inv↔equip↔cargo↔trade)
- **Gatilho/struct:** `MSG_TradingItem` (`DestPlace`,`DestSlot`,`SrcPlace`,`SrcSlot`,`WarpID`).
- **Validações:** `Hp!=0` e `Mode==USER_PLAY` (senão `SendHpMode`+`AddCrackError(1,19)`); se em trade
  → `RemoveTrade` ambos; se `TradeMode` → bloqueia ("CantWhenAutoTrade"). Acesso ao **cargo** exige
  estar perto de um NPC de cargo (`Merchant==2`, `MOB_PEACE`, dentro de `VIEWGRID`) **ou** ser FM
  (`Class==3`) com skill `0x2000` — senão `SendClientSignal(...,412)`. Bounds de slot por place
  (`MAX_CARRY-4`, `MAX_CARGO`, `MAX_EQUIP`).
- **Efeitos:** resolve ponteiros via `GetItemPointer` e faz o **swap** entre origem/destino;
  re-equipar recalcula score. `Equip[14]` (montaria) tratado à parte.
- **Anti-cheat/risco:** **handler central de dup de item** — preservar a validação de place/slot e a
  atomicidade do swap (Fase 8 §2.7). Item "no-trade"/montaria têm regras especiais. Distância do NPC
  de cargo é a checagem-chave para abrir o baú.

## `_MSG_DeleteItem` (0x02E4) — destruir item do inventário
- **Gatilho/struct:** `MSG_DeleteItem` (`Slot`, `sIndex`).
- **Validações:** `Slot` em `[0, MAX_CARRY-4)`; `sIndex` em `(0, MAX_ITEMLIST)`; `Mode==USER_PLAY`;
  se em trade → `RemoveTrade` ambos e aborta.
- **Efeitos:** loga o item (`BASE_GetItemCode` + `ItemLog`) e **zera** `Carry[Slot]`.
- **Risco:** não confere `sIndex` contra o item realmente no slot antes de apagar — confiar só no
  `Slot`. Na migração, validar que `Carry[Slot].sIndex == m->sIndex` antes de destruir.
- **Status Go (issue #120): ✅** `handler.deleteItem` (`item.go`), rota em `dispatch.go`. Guarda de
  trade abre mão do delete (cancela o trade, anti-dup); envia um `MsgSendItem` limpando o slot
  (o legado não envia nada) e persiste no save periódico. Loja NPC é stateless (sem guarda).

## `_MSG_SplitItem` (0x02E5) — dividir stack
- **Gatilho/struct:** `MSG_SplitItem` (`Slot`, `Num`).
- **Validações:** `Slot` em `[0, MAX_CARRY-4)`; `Num` em `(0,120)`; `Mode==USER_PLAY`; sem trade.
  **Whitelist de itens empilháveis** por `sIndex` (413,412,419,420,416,414 e faixa 2390–2419). Exige
  ≥1 slot livre; `amount > Num` e `amount > 1`.
- **Efeitos:** reduz o stack original em `Num` (`BASE_SetItemAmount`) e cria um novo item com `Num`
  unidades (`PutItem`); `SendItem` do slot. Loga.
- **Risco:** quantidade num campo de efeito do item (`BASE_GetItemAmount`); preservar a whitelist e os
  limites (anti-dup de stack).
- **Status Go (issue #120): ✅** `handler.splitItem` + `setItemAmount`/`isSplittable` (`item.go`).
  Whitelist {412,413,414,416,419,420, 2390–2419}; `EF_AMOUNT` (61) guarda a quantidade; coloca o novo
  stack via `AddToCarry` e envia dois `MsgSendItem`. Sem slot livre → mantém o stack inteiro.
  **Divergência deliberada (issue #268):** a whitelist inclui também **1774** (Pedra do Sábio), que a
  loja de NPC do portal vende em pack (`npcconfig.go` grava `EF_AMOUNT` no item vendido) — sem isso o
  Shift+clique no pack não fazia nada. O merge (`sameStackClass`) usa a mesma lista, então as duas
  pontas ficam consistentes.

## `_MSG_UpdateItem` (0x0374) — abrir portões/baús/estado de item no mundo
- **Gatilho/struct:** `MSG_UpdateItem` (`ItemID`, `State`).
- **Validações:** `Hp!=0` & `USER_PLAY`; `State` em `[0,5]`; `ItemID` em `[10000, 10000+MAX_ITEM)` →
  `gateid = ItemID-10000` em `[0,MAX_ITEM)`. Violações → `AddCrackError(50,50)`.
- **Efeitos:** delega a `CCastleZakum::OpenCastleGate` (portão de castelo); se o item-mundo exige
  **chave** (`EF_KEYID`), procura no inventário, **consome a chave** (zera slot + `SendItem`) e abre.
  `UpdateItem(gateid, STATE_OPEN)` + `GridMulticast`. Loga "opengate".
- **Risco:** lógica de chave/quest acoplada a `EF_KEYID`/`EF_QUEST`; portões de castelo dependem de
  estado de evento (Fase 6). Item-índice especial `773` (sem msg de "no key").
- **Status Go (issue #120): ✅ parcial.** `handler.updateItem` (`gate.go`), rota em `dispatch.go`.
  Portões vêm de `TMsrv/run/InitItem.csv` (`content.LoadInitItems` → `World.SeedWorldItem`, id em
  ordem de arquivo = ordem legada de `CreateItem`); são `GroundItem` com `State`/`Static` (não
  pegáveis). Fluxo: chave por `EF_KEYID` (39), consome + `MsgSendItem`, `State=STATE_OPEN`, eco
  `MsgUpdateItem` para quem está na view (`ForEachInViewAt`). Como no legado, um portão semeado nasce
  aberto e sem chave — o caminho de chave só é alcançável quando um sistema o tranca.
  **Adiado (documentado, não implementado):** portões de castelo (`CCastleZakum::OpenCastleGate`,
  precisa de estado de castelo/RvR) e o re-lock periódico (`ProcessSecMinTimer`) / comandos GM de
  travar (`imple.cpp`). A correspondência wire↔cliente do id de portão é **UNVERIFIED** (precisa de
  captura); ids seguem a ordem de `InitItem.csv` para casar com a sequência de `CreateItem`.

## `_MSG_PutoutSeal` (0x03CC) — retirar selo / "out capsule"
- **Gatilho/struct:** `MSG_PutoutSeal` (`SourType/SourPos`, `DestType/DestPos`, `GridX/Y`, `WarpID`,
  `MobName[16]`).
- **Validações:** `Mode==USER_PLAY`; sem trade; `Hp!=0` (senão devolve o item via `SendItem`);
  `GridX/Y < MAX_GRID`. Item de origem deve ser `sIndex==3443` com efeito ≠0; `MobName` válido
  (`BASE_CheckValidString`).
- **Efeitos:** `CharLogOut(conn)` e envia `_MSG_DBOutCapsule` ao **DBSrv** com os campos do pedido +
  `Slot` da conta (operação concluída no DBSrv — cria/retira personagem de cápsula). Loga.
- **Risco:** desloga o personagem para concluir no DBSrv (operação cross-serviço). Índice `3443`
  hardcoded. Na migração vira chamada gRPC ao `dbServer`.

## `_MSG_SetShortSkill` (0x0378) — barra de skills
- **Gatilho/struct:** `MSG_SetShortSkill` (`Skill1[4]`, `Skill2[16]`).
- **Efeitos:** `memcpy` direto: `Skill1`→`MOB.SkillBar[4]`; `Skill2`→`pUser[conn].CharShortSkill[16]`.
- **Risco:** **sem validação** de `Mode`/conteúdo — copia bytes do cliente direto na struct. Na
  migração, validar índices de skill (cada slot deve referenciar skill conhecida).

## `_MSG_ApplyBonus` (0x0277) — distribuir pontos de atributo
- **Gatilho/struct:** `MSG_ApplyBonus` (`BonusType` 0=Score 1=Special 2=Skill, `Detail`, `TargetID`).
- **Validações:** `Hp>0` & `USER_PLAY` (senão `AddCrackError(10,20)`).
- **Efeitos por tipo:**
  - `BonusType==0` (Score): consome `ScoreBonus` (1, ou **100 de cada vez** se `ScoreBonus>=300`) e
    soma em `BaseScore.Str/Int/Dex/Con` (Int→+2 MaxMp; Con→+2 MaxHp). `Detail` em `[0,3]`.
  - `BonusType==1` (Special): consome `SpecialBonus`, sobe `BaseScore.Special[Detail]` até teto
    `max_special` (200, ou 255 com skill correspondente aprendida) e meio-nível
    `3*(Level+1)` (Celestial: +3*400). 
  - `BonusType==2` (Skill): consome `SkillBonus` (continua adiante no arquivo).
  - Sempre: `GetCurrentScore` + `SendScore`/`SendEtc`; loga.
- **Anti-cheat/risco:** preservar os **tetos** e o passo de 100 (anti-exploit de pontos). Fórmulas de
  teto dependem de classe/skill — extrair para Fase 4. Caminho crítico de paridade de personagem.

## `_MSG_PKMode` (0x0399) — alternar modo PK
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm`).
- **Efeitos:** `pUser[conn].PKMode = Parm!=0`. Se em trade ativo → cancela ("Cant_trade_pkmode").
  Recalcula o estado de "culpado/zona PvP" (guilty, RvR/Castle/GTorre por região) e **multicast**
  `_MSG_PKInfo` aos próximos. Loga.
- **Risco:** o cálculo de "state" PvP por região/evento é hardcoded e replicado em `QuitTrade`/`PKMode`
  — consolidar numa função única na migração (Fase 4/6). Coordenadas de zona hardcoded.
