# Contratos — Variantes de Combine/Refino (lote 2)

> 9 handlers que são **quase-duplicatas** da engine de combine (o `_MSG_CombineItem` base está no
> lote 1). O relatório de arquitetura já apontou o risco de **drift** entre eles. Fonte:
> `TMSrv/_MSG_CombineItem*.cpp`. Rates/tabelas na Fase 4 §3.

---

## Contrato compartilhado (vale para todas as variantes "Item[]")
Aplica-se a: `_MSG_CombineItemEhre` (0x02D3), `_MSG_CombineItemTiny` (0x03C0),
`_MSG_CombineItemShany` (0x02C4), `_MSG_CombineItemAilyn` (0x03B5), `_MSG_CombineItemAgatha`
(0x03BA), `_MSG_CombineItemOdin` (0x02D2/0x02E2), `_MSG_CombineItemLindy` (0x02C3),
`_MSG_CombineItemAlquimia` (0x02E1) e o **base** `_MSG_CombineItem` (0x03A6, lote 1).

- **Gatilho/struct:** `MSG_CombineItem` (`Item[MAX_COMBINE]`, `InvenPos[MAX_COMBINE]`).
- **Validações (idênticas entre variantes):** para cada slot de combine não-vazio
  (`Item[i].sIndex != 0`): `InvenPos[i]` em `[0, pMob[conn].MaxCarry)`; fora do range →
  **`RemoveTrade(conn)`** e aborta. (As variantes herdam o mesmo esqueleto de loop/validação —
  confirmado por leitura comparada dos cabeçalhos.)
- **Núcleo (engine):** confere a **receita** (combinação de itens válida) → se inválida,
  `_NN_Wrong_Combination`. Se válida, faz o **roll de sorte** `rand() % 115` contra a taxa da receita
  (rate table, Fase 4 §3; influência de `Sanc`/anct) → **sucesso**: aplica o upgrade no item
  resultante + `SendItem` + `_NN_Processing_Complete`; **falha**: `_NN_CombineFailed` (pode consumir/
  degradar insumos conforme a receita). `ItemLog` em ambos.
- **Diferença entre variantes:** receita, taxa, custo, slots consumidos e efeito final variam. Ailyn
  preserva os slots 0/1 na falha e custa 50M; Tiny custa 100M e destrói o alvo na falha; Agatha
  preserva o slot 1; Shany entrega o item 633 via `PutItem`; Alquimia usa `Special[2]`; Lindy é um
  desbloqueio Arch determinístico e persistente; Ehre possui oito resultados diferentes.
- **Anti-cheat/risco:**
  - **Dup/anti-spam:** o cooldown anti-spam de refino está **comentado** no base
    (`_MSG_UseItem.cpp:209-221`, ver lote 1) — reintroduzir no novo servidor.
  - **Drift:** 9 cópias quase iguais → na migração, **unificar numa única engine** parametrizada por
    receita/tabela (resolve o risco apontado pela arquitetura).
  - **RNG:** `rand() % 115` é a fonte de não-determinismo — seed injetável para golden cases
    (testar **distribuição**, não valor exato; Fase 8 §4).
  - As sete variantes restantes foram portadas em `handler/combine_variants.go`, com matchers puros
    em `internal/combine/match.go`, a partir de `GetFunc.cpp:187-506,622-687` e dos respectivos
    `_MSG_CombineItem<Nome>.cpp`. Odin continua no handler dedicado `combine_odin.go`.

## Caso especial — `_MSG_CombineItemExtracao` (0x02D4)
- **Gatilho/struct:** **`MSG_STANDARDPARM2`** (não `MSG_CombineItem`): `Parm2 = ItemSlot`.
- **Validações:** `ItemSlot` em `[0, pMob[conn].MaxCarry)`; `item = Carry[ItemSlot].sIndex` em
  `(0, MAX_ITEMLIST)`.
- **Efeitos:** **extração** — separa/remove um componente de um item (ex.: extrair pedra/efeito),
  diferente do combine aditivo das outras variantes. (Detalhe da receita: UNVERIFIED, ler arquivo.)
- **Risco:** struct e semântica distintas das demais; não reaproveita o esqueleto `Item[]`. Tratar
  como handler próprio na engine unificada.
