# Contratos — Loja NPC & Banco/Cargo (lote 2)

> Compra/venda com NPC mercador e depósito/saque de gold no cargo. Fonte: `TMSrv/_MSG_*.cpp`.

---

## `_MSG_REQShopList` (0x027B) — abrir loja de NPC
- **Gatilho/struct:** `MSG_REQShopList` (`Target`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,22)`); `Target` em `[MAX_USER, MAX_MOB)`
  (precisa ser um **mob/NPC**, não player); `Merchant!=0` (senão "He is not merchant"); em visão
  (`GetInView`).
- **Efeitos:** `Merchant==1` → `SendShopList(...,1)` (loja normal); `Merchant==19` →
  `SendShopList(...,3)` (loja especial).
- **Risco:** baixo; só envia catálogo. `Merchant` codifica o tipo de NPC.

## `_MSG_Buy` (0x0379) — comprar de NPC
- **Gatilho/struct:** `MSG_Buy` (`TargetID`, `TargetInvenPos`, `MyInvenPos`, `Coin`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,21)`); não em `TradeMode`/trade; `TargetID`
  em `[MAX_USER,MAX_MOB)`; posições em `[0,MAX_CARRY)`; `Merchant==1`; em visão (senão `_MSG_CloseShop`);
  `itemIndex` em `(0,MAX_ITEMLIST)`.
- **Efeitos:** dois caminhos de preço:
  - **Donate** (`EF_DONATE`>0): exige `Donate <= pUser.Donate`, slot de destino livre → entrega item e
    debita `Donate`.
  - **Gold:** debita `MOB.Coin` pelo preço da `ItemList` e entrega o item (continuação do arquivo).
- **Anti-cheat/risco:** preço vem da **ItemList do servidor**, não do cliente (bom). Preservar a
  checagem de visão e o caminho donate/cash (Fase 2 `EF_DONATE`). Caminho de economia → golden cases.

## `_MSG_Sell` (0x037A) — vender para NPC
- **Gatilho/struct:** `MSG_Sell` (`TargetID`, `MyType` 0=Equip/1=Carry/2=Cargo, `MyPos`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,23)`); se em trade → cancela; cargo
  bloqueado em `TradeMode`. Bounds por tipo: Carry `[0,MAX_CARRY-4)`, Cargo `[0,MAX_CARGO)`, Equip
  `(0,MAX_EQUIP)` (`MyPos<8` marca item equipado). `GetItemPointer` ≠ NULL.
- **Efeitos:** credita gold pelo preço de venda do item e remove-o do slot (continuação do arquivo).
- **Risco:** vender item equipado (`isEquip`) exige cuidado de recalcular score; bounds por place são
  o ponto crítico (anti-dup). Preço de venda derivado da ItemList.

## `_MSG_Deposit` (0x0388) — depositar gold no cargo
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = coin`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,1)`); `0 <= coin <= 2e9` e `coin <=
  MOB.Coin`; soma resultante `<= 2e9`.
- **Efeitos:** `MOB.Coin -= coin`; `pUser.Coin += coin`; ecoa a msg (`ID=ESCENE_FIELD`) e
  `SendCargoCoin`. Loga. Estouro → "Cant get more than 2G".
- **Risco:** dois cofres de gold (personagem `MOB.Coin` vs conta `pUser.Coin`); preservar os tetos de
  2 bilhões (overflow `int`, Fase 2).

## `_MSG_Withdraw` (0x0387) — sacar gold do cargo
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = coin`).
- **Validações:** `Hp!=0` & `USER_PLAY` (`AddCrackError(10,2)`); `0 <= coin <= 2e9` e `coin <=
  pUser.Coin`; soma em `MOB.Coin` `<= 2e9`.
- **Efeitos:** `MOB.Coin += coin`; `pUser.Coin -= coin`; ecoa msg (`ID=30000`) e `SendCargoCoin`. Loga.
- **Risco:** espelho do Deposit; mesmos tetos. `ID=30000`/`ESCENE_FIELD` são marcadores de cena para o
  cliente — preservar.
