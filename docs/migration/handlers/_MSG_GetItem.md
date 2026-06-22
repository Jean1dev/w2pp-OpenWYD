# Contrato — `_MSG_GetItem`

- **Gatilho:** Type `0x0270` (624, `CLIENT2GAME`). Struct `MSG_GetItem` (`ItemID`, `DestType`,
  `DestPos`, ...).
- **Fonte:** `TMSrv/_MSG_GetItem.cpp:21-252`.

## Pré-condições e validações
1. Vivo e jogando: `Hp > 0 && Mode == USER_PLAY` — senão `AddCrackError(1,13)` + `SendHpMode`
   (`:25-30`).
2. Não em trade (igual ao DropItem) (`:32-43`).
3. `DestType == ITEM_PLACE_CARRY` (só pega para o inventário) (`:45-49`).
4. **Decodifica ID:** `itemID = m->ItemID - 10000` (offset de id de chão!) (`:51`). Bounds
   `0 < itemID < MAX_ITEM` (`:53-54`).
5. Item existe: `pItem[itemID].Mode != 0` — senão envia `_MSG_DecayItem` (some no cliente) (`:56-73`).
6. **Distância:** o jogador deve estar a ≤ **3 células** do item (`TargetX/Y` vs `pItem.PosX/Y ±3`)
   — senão log de falha e aborta (`:75-83`).
7. Restrição especial: `itemID==1727` exige `Level >= 1000` (`:85-86`).

## Efeitos colaterais
- Copia `pItem[itemID].ITEM` para um slot livre do `Carry[]` do jogador.
- Remove o item do chão (`pItem[itemID]` zerado) e do `pItemGrid`.
- `ItemLog` (auditoria).

## Saídas (S→C)
- `_MSG_CNFGetItem` (0x0171) ao dono (item adicionado ao inventário).
- `_MSG_DecayItem` (0x016F) se o item já não existe.
- `_MSG_RemoveMob`/remoção do item no chão (broadcast).

## Anti-cheat / Riscos
- Checagem de distância (anti-teleport-pickup) e de existência (anti-race).
- **`ItemID - 10000`**: o id de item no chão é deslocado em +10000 no fio — preservar exatamente.
- **Dup/race:** dois jogadores pegando o mesmo item — hoje serializado (thread única); na stack nova
  exige claim atômico por `pItem[]`.
- Inventário cheio: tratar (provavelmente recusa/mantém no chão) — confirmar no restante do handler.
