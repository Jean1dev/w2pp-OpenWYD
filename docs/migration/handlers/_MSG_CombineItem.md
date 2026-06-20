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

## Efeitos colaterais
- **Consome os insumos:** zera cada `Carry[InvenPos[i]]` e envia `SendItem` (`:55-62`).
- Roll de sucesso (Fase 4 §3.1): `_rand = rand()%115; if >=100 -=15; success = _rand <= combine`
  (`:80-84`). `LOCALSERVER` força sucesso (debug).
- Em sucesso: item resultante `Carry[ipos].sIndex = joia + extra` (`extra = ItemList[idx].Extra`,
  `joia = Item[1].sIndex - 2441`, `0..3`); `BASE_SetItemSanc(item, 7, 0)` (`:86-118`).

## Saídas (S→C)
- `_MSG_CombineComplete` (0x03A7) com `parm`: `0`=combinação inválida, `1`=sucesso, `2`=falha.
- `SendItem` (0x0182) para atualizar os slots afetados.
- Mensagens: `_SS_CombineSucceed`, `_NN_Wrong_Combination`, `269` (falha).

## Anti-cheat / Riscos
- **Insumos consumidos ANTES do roll** — em falha o jogador perde os itens (comportamento
  intencional do WYD). Preservar a ordem.
- **Sem cooldown** (igual refino) — anti-macro a decidir.
- A distribuição `rand()%115` com achatamento é constante de economia — **reproduzir exatamente**.
- Joia base `2441` e sanc `7` são mágicos — preservar.
- **Consolidação recomendada:** as ~10 variantes diferem só na função `GetMatchCombine<X>` e no
  efeito aplicado — na stack nova, uma **engine de receitas data-driven** (tabela receita→taxa→
  resultado) substitui os 10 handlers. Validar cada variante por captura.
