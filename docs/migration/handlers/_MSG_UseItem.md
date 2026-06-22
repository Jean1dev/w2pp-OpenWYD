# Contrato — `_MSG_UseItem`

- **Gatilho:** Type `0x0373` (883). Struct `MSG_UseItem` (`SourType`, `SourPos`, `DestType`,
  `DestPos`, `GridX/Y`, `WarpID`).
- **Fonte:** `TMSrv/_MSG_UseItem.cpp` (handler grande; cobre equipar, consumir, e **refinar selado**).

## Pré-condições e validações
1. Estado de jogo (`USER_PLAY`, vivo) e bounds de `SourPos/DestPos` (`CARRY`/`EQUIP`/`CARGO`).
2. Para **refino de item selado** (`itemtype == 5`, `:200-225`):
   - Itens "selados" identificados por faixas de sIndex (`1234-1237,1369-1372,1519-1522,1669-1672,
     1901-1910,1714`) (`:200-201`).
   - `sanc >= 6 && Vol == 4` → `_NN_Cant_Refine_More` (`:203-208`).
   - `sanc >= 9` → `_NN_Cant_Refine_More` (`:227-229`).
3. **Cooldown de 1 s está DESATIVADO** (bloco comentado `:209-221`) — não há rate-limit de refino
   hoje (Fase 4 §3.4).

## Efeitos colaterais
- Equipar: move item `CARRY → EQUIP[slot]`, recalcula score (`BASE_GetCurrentScore`), valida
  requisitos (level/str/int/dex/con) do `STRUCT_ITEMLIST`.
- Consumir (poção/scroll): aplica efeito/affect, decrementa stack.
- Refinar: ajusta `sanc`/efeitos do item (ver tabelas `SancRate.txt`, Fase 4 §3.3).
- Teleporte por scroll: usa `WarpID`/`GridX/Y`.

## Saídas (S→C)
- `_MSG_UseItem` (eco/resultado), `_MSG_SendItem`/`_MSG_UpdateEquip` (atualiza slot/equip),
  `_MSG_SetHpMp`/`_MSG_UpdateScore`, mensagens `_NN_Cant_Refine_More`.

## Anti-cheat / Riscos
- Sem cooldown de refino server-side (anti-macro) — **divergência a decidir** (reativar vs paridade).
- Requisitos de equip validados no servidor — confirmar que nenhum bypass via `DestType` inválido.
- Faixas de sIndex de "selado" e limites de sanc são constantes mágicas — preservar.
- **UNVERIFIED:** o mapeamento completo de ação por tipo de item (este handler ramifica muito) —
  detalhar por captura de cada categoria de item.
