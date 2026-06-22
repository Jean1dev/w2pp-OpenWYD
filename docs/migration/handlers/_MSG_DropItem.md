# Contrato — `_MSG_DropItem`

- **Gatilho:** Type `0x0272` (626, `CLIENT2GAME`). Struct `MSG_DropItem` (`SourType`, `SourPos`,
  `Rotate`, `GridX`, `GridY`).
- **Fonte:** `TMSrv/_MSG_DropItem.cpp:21-146`.

## Pré-condições e validações
1. Vivo e jogando: `Hp > 0 && Mode == USER_PLAY` — senão `AddCrackError(1,14)` + `SendHpMode`
   (`:25-30`).
2. **Não em trade:** se `Trade.OpponentID` → cancela trade de ambos e aborta (`:32-37`); se
   `TradeMode` (auto-trade) → `_NN_CantWhenAutoTrade` (`:39-43`).
3. Grid válido: `GridX < MAX_GRIDX && GridY < MAX_GRIDY` (`:45-50`).
4. Flag global `isDropItem != 0` (drop habilitado) (`:52-53`).
5. Célula livre: `GetEmptyItemGrid` ajusta `(gridx,gridy)` p/ posição vazia; se `titem==0` →
   `_NN_Cant_Drop_Here` (`:58-67`).
6. Origem: `SourType != ITEM_PLACE_EQUIP` (não dropa equipado direto) (`:69-73`); bounds por tipo:
   `CARRY` → `SourPos < MaxCarry`; `CARGO` → `SourPos < MAX_CARGO` (`:75-97`).
7. Item válido: `1 <= sIndex < MAX_ITEMLIST` (`:107-108`).
8. **Blacklist de itens não-dropáveis:** sIndex ∈ {508,509,522,526..537,446,747,3993,3994} são
   bloqueados (quest/atados) (`:110-111`).

## Efeitos colaterais
- `CreateItem(GridX, GridY, SrcItem, Rotate, 1)` → cria entidade em `pItem[]` (`:113`).
- `memset(SrcItem, 0)` — remove do inventário/cargo (`:126`).
- `ItemLog` do código do item (auditoria).

## Saídas (S→C)
- `_MSG_CNFDropItem` (0x0175) ao dono: confirma slot esvaziado + posição (`:128+`).
- `_MSG_CreateItem`/broadcast do item no chão aos jogadores na visão.
- Erros: `_NN_Cant_Drop_Here`, "Can't create object(item)".

## Anti-cheat / Riscos
- `AddCrackError` em drop com personagem morto.
- **Dup de item:** o fluxo cria-no-chão→limpa-origem deve ser atômico; se a stack nova introduzir
  concorrência, é ponto clássico de duplicação. Validar com golden case de drop+get concorrente.
- Blacklist hardcoded de sIndex — preservar a lista exata.
