# Contrato — `_MSG_CombineItem` (engine de combine/Anct)

- **Gatilho:** Type `0x03A6` (934). Struct de combine: array `Item[MAX_COMBINE]` + `InvenPos[]`.
  As variantes (`Ehre/Tiny/Shany/Ailyn/Agatha/Odin/Lindy/Alquimia/Extracao`) têm Types próprios
  (Fase 1 §3.1) e handlers `_MSG_CombineItem*.cpp` análogos.
- **Fonte:** `TMSrv/_MSG_CombineItem.cpp:21-150`. Fórmula detalhada na Fase 4 §3.

## Pré-condições e validações
1. Itens da receita ainda presentes/inalterados no inventário (revalida; senão
   `ItemLog("item remove or changed")` + `_MSG_CombineComplete parm=0`) (`:36-43`).
2. `combine = GetMatchCombine(Item)` (taxa da receita). `combine == 0` → `_NN_Wrong_Combination` +
   `_MSG_CombineComplete parm=0` (`:46-53`).

### `GetMatchCombine` — a receita Anct (`TMSrv/GetFunc.cpp:76-147`)

`Item[0]` é o alvo, `Item[1]` a joia, `Item[2..7]` os sacrifícios. Taxa base `1`; cada
sacrifício soma `g_pAnctChance[sanc-7]` (`Compositor Item_+7/+8/+9` = `2/4/10` em
`Common/Settings/CompRate.txt`). Qualquer gate reprovado ⇒ `0`:

- item `747` em qualquer slot;
- `ItemList[alvo].nUnique` fora de `[41,49]` ou `Extra <= 0`;
- `EF_MOBTYPE(alvo) == 3`;
- por sacrifício: `EF_POS == 0`, `EF_ITEMLEVEL(alvo) > EF_ITEMLEVEL(sacrifício)`, ou sanc
  fora de `{7,8,9}`.

> **Armadilha de port (issue #270):** `EF_POS` **não é um `stEffect`**. `BASE_GetItemAbility`
> soma `g_pItemList[idx].nPos` **antes** de varrer os efeitos (`Basedef.cpp:1579-1580`, e as
> cópias idênticas em `:1741`/`:1912`) — e `Common/ItemList.csv` não tem uma única linha
> `EF_POS`: `nPos` é a **coluna 6**. Resolver `EF_POS` só pelos pares de efeito devolve sempre
> `0`, o que reprova todo sacrifício e torna a composição Anct impossível. O mesmo vale para
> `EF_REQ_*` (`ReqStr/ReqInt/ReqDex/ReqCon`), que também vêm de colunas próprias.

## Efeitos colaterais
- **Consome os insumos:** zera cada `Carry[InvenPos[i]]` e envia `SendItem` (`:55-62`).
- Roll de sucesso (Fase 4 §3.1): `_rand = rand()%115; if >=100 -=15; success = _rand <= combine`
  (`:80-84`). `LOCALSERVER` força sucesso (debug).
- Em sucesso: item resultante `Carry[ipos].sIndex = joia + extra` (`extra = ItemList[idx].Extra`,
  `joia = Item[1].sIndex - 2441`, `0..3`); `BASE_SetItemSanc(item, 7, 0)` (`:86-118`).

## Saídas (S→C)
- `_MSG_CombineComplete` (0x03A7) com `parm`: `0`=combinação inválida, `1`=sucesso, `2`=falha.
  Corpo é `MSG_STANDARDPARM` — `int Parm` de **4 bytes** (`Basedef.h:1254-1258`) — e
  `SendClientSignalParm` manda com `HEADER.ID = ESCENE_FIELD (30000)`, não o `conn`
  (`SendFunc.cpp:300-310`).
- `SendItem` (0x0182) para atualizar os slots afetados: `{short invType; short Slot;
  STRUCT_ITEM item}` = corpo de **12 bytes** (`Basedef.h:2037-2046`). Um slot consumido vai
  como item zerado, não como índice de slot cru.
- Mensagens: `_SS_CombineSucceed`, `_NN_Wrong_Combination`, `269` (falha).

## Anti-cheat / Riscos
- **Insumos consumidos ANTES do roll** — em falha o jogador perde os itens (comportamento
  intencional do WYD). Preservar a ordem.
- No sucesso o `_MSG_CombineComplete(1)` vai **antes** do `SendItem` do resultado
  (`:109,116`).
- **Sem cooldown** (igual refino) — anti-macro a decidir.
- A distribuição `rand()%115` com achatamento é constante de economia — **reproduzir exatamente**.
- Joia base `2441` e sanc `7` são mágicos — preservar.
- **Consolidação recomendada:** as ~10 variantes diferem só na função `GetMatchCombine<X>` e no
  efeito aplicado — na stack nova, uma **engine de receitas data-driven** (tabela receita→taxa→
  resultado) substitui os 10 handlers. Validar cada variante por captura.
