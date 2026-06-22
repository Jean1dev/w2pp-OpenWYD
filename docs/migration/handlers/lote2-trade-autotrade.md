# Contratos — Trade entre jogadores & Auto-Trade (lote 2)

> Troca direta P2P e lojas pessoais (auto-trade). Caminho crítico de **dup de item** — testar com
> golden cases atômicos (Fase 8 §2.7). Fonte: `TMSrv/_MSG_*.cpp`.

---

## `_MSG_Trade` (0x0383) — confirmar troca P2P (atômica)
- **Gatilho/struct:** `MSG_Trade` (`Item[15]`, `InvenPos[15]`, `TradeMoney`, `MyCheck`, `OpponentID`).
- **Validações (todas levam a `RemoveTrade` em ambos se falham):**
  - `Hp!=0` & `USER_PLAY` (`AddCrackError(5,18)`);
  - `OpponentID` em `(0, MAX_USER)` e `pUser[OpponentID].Mode==USER_PLAY`;
  - `0 <= TradeMoney <= MOB.Coin`;
  - para cada item ofertado: `InvenPos` em `[0,MAX_CARRY-4)` e **`memcmp` do item ofertado contra o
    item real no slot** (de ambos os lados) — qualquer divergência aborta (anti-troca de item
    durante a confirmação).
- **Efeitos:** quando ambos confirmam (`MyCheck`) e o último oponente confere, executa a **troca
  atômica** (itens + gold trocam de dono). Ver continuação do arquivo p/ a transferência.
- **Anti-cheat/risco:** **o handler mais sensível a dup.** A migração deve fazer a troca como uma
  **transação única** (validar-tudo-depois-aplicar-tudo), preservando os `memcmp` de consistência.
  Manter no game-loop single-thread (sem concorrência).

## `_MSG_QuitTrade` (0x0384) — cancelar troca
- **Gatilho/struct:** `MSG_STANDARD`.
- **Validações:** `Hp>0` & `USER_PLAY` (`AddCrackError(10,17)`).
- **Efeitos:** `RemoveTrade` do oponente e de si; recalcula estado PvP (igual `PKMode`) e multicast
  `_MSG_PKInfo`.
- **Risco:** baixo; só cancela. Compartilha o cálculo de "state PvP" com `PKMode` (consolidar).

## `_MSG_SendAutoTrade` (0x0397) — abrir loja pessoal
- **Gatilho/struct:** `MSG_SendAutoTrade` (`Item[MAX_AUTOTRADE]`, `Coin[]`, `CarryPos[]`, `Title`,
  `Tax`).
- **Validações:** `Hp>0` & `USER_PLAY` (`AddCrackError(10,88)`); sem trade ativo; não já em
  `TradeMode`; **só em servidor de evento newbie** (`NewbieEventServer==0` → bloqueia); **só em
  vila** (`BASE_GetVillage` 0..4, e fora de uma área proibida). Por item: `Coin` em
  `[0,1999999999]`, item↔coin coerentes, **blacklist de `sIndex`** (508,3993,747,509,522,526–531,446),
  `CarryPos` em `[0,MAX_CARGO)`, não pode ser `EF_NOTRADE`, e **`memcmp` contra o item real no
  cargo**.
- **Efeitos:** seta `Tax = CityTax` da vila; copia o pacote para `pUser[conn].AutoTrade`; `TradeMode=1`;
  `SendAutoTrade(conn,conn)` e **multicast** `_MSG_CreateMobTrade` (mostra a loja no mundo).
- **Risco:** muitos índices/áreas hardcoded; o item vendido fica referenciado no **cargo** (não sai do
  inventário até a compra). Preservar a blacklist e o `memcmp`.

## `_MSG_ReqTradeList` (0x039A) — abrir a loja de outro jogador
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = autoID`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,87)`); `autoID` em `(0,MAX_USER)`; alvo em
  `TradeMode`; alvo dentro da **VIEWGRID** (senão loga "too far").
- **Efeitos:** `SendAutoTrade(conn, autoID)` (envia o conteúdo da loja do alvo).
- **Risco:** baixo; só leitura.

## `_MSG_ReqBuy` (0x0398) — comprar de uma loja pessoal
- **Gatilho/struct:** `MSG_ReqBuy` (`TargetID`, `Price`, `Tax`, `Pos`, `item`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,86)`, `RemoveTrade`); não em `TradeMode`/
  trade; `TargetID` em `(0,MAX_USER)`; alvo dentro da VIEWGRID; `Pos` em `[0,MAX_AUTOTRADE)`;
  `StorageSlot` (=`AutoTrade.CarryPos[Pos]`) em `[0,MAX_CARGO)`; **`Tax`/`Price`/`item` precisam bater
  exatamente** com a oferta (`AutoTrade`) **e** com o item real no cargo do vendedor (dois `memcmp`).
- **Efeitos:** se tudo confere, transfere item↔gold entre comprador e vendedor aplicando o imposto da
  cidade (continuação do arquivo).
- **Anti-cheat/risco:** par crítico de dup/economia com `SendAutoTrade`. Preservar os dois `memcmp`
  (oferta vs cargo) e o cálculo de imposto. Transação única no novo código.
