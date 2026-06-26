# Bugs in-game (cliente real ↔ servidor Go) — mapeamento

> Estado: cliente "Cavaleiros de Kersef" 7662 conecta → loga → cria char → entra no mundo → anda.
> Estes são os bugs observados depois disso, com causa-raiz provável e o que falta para corrigir.
> Prioridade: P0 (quebra multiplayer/jogo) → P3 (cosmético).

| # | Bug | Prioridade | Causa-raiz provável | O que falta | Precisa do agente Windows? |
|---|-----|:---:|---|---|:---:|
| B1 | **Player duplica a cada passo** (visto de outra conta) | **P0** → **PARCIAL** | Nenhum `_MSG_CreateMob` (0x0364) era enviado ao entrar na visão; o cliente criava um avatar novo a cada `_MSG_Action`. **Feito:** `EncodeCreateMobBody` (232B) + `EncodeRemoveMobBody` (16B) + sequência de entrada (`enterWorldView`: broadcast do novato p/ a visão + cada um da visão p/ o novato) + `RemoveMob` no logout. **Falta:** (a) `CreateMob`/`RemoveMob` quando se **cruza a visão andando** (não só na entrada); (b) **equip visual** (`BASE_VisualItemCode`) — players aparecem **sem equipamento** (Equip=0). | (a) update de visão no movimento (grid); (b) visual codes do equip | — |
| B2 | **Gold inicial negativo** (-858993664) | ~~P1~~ **RESOLVIDO** | O valor `-858993664` = `0xCCCCCC00` = bytes `00 CC CC CC` no **offset 24** do `STRUCT_MOB`. O template BaseMob é um dump de memória com **padding 0xCC não-inicializado**; o cliente lê o gold no offset 24. Zerado em `EncodeCNFCharacterLoginRaw` (offsets 24-31). | — | — |
| B3 | **Sem NPCs/monstros no mapa** | P1 → **PARCIAL** | Spawn nunca ligado. **Feito:** parser `NPCGener.txt` (6103 geradores), loader dos templates `npc/<nome>` (816B), `SpawnMob` (10.496 mobs spawnados, cap 20k), e `CreateMob` dos mobs na visão na entrada **e ao andar** (`revealMobsInView` + grid). **IA iteração 1 feita** (`world/tick.go` + `handler/mobai.go`): monstro agro por proximidade/retaliação, persegue (1 tile/tick) e ataca o player corpo-a-corpo. **Falta:** hostilidade por clan, ranged, pathfinding real, roaming/segmentos, morte/ressurreição do player, respawn de mob; grupos completos (cap MinGroup≤6); ~1400 templates de Leader sem arquivo. | (refinos da IA — ver SESSION-PRIMER §7); reveal de players ao cruzar a visão andando | — |
| B4 | **Preview de classe na seleção = sempre TK** | ~~P2~~ **RESOLVIDO** | `STRUCT_SELCHAR` enviada **sem equipamento** → cliente desenha modelo padrão. | `sel.Equip[slot]` agora vem do `MobEquip` do template BaseMob da classe (`protocol.MobEquip` + `d.selCharsFrom`). | — |
| B5 | **Level mostra +1 na seleção** | P3 | Display: SELCHAR `Score.Level` lido da `CharSummary` (DB level 1) mas exibe 2. Offset confere no teste; pode ser quirk do cliente (1-indexado) ou outro campo. | Confirmar o campo/offset do level exibido. | talvez |
| B6 | **Posição salva se perde** (anda, desloga, volta no spawn) | P2 | O proto gRPC `CharacterState` **não tem campos de posição** → `LoadCharacter` retorna 0,0; usamos o spawn do template. Mover não persiste. | Adicionar pos ao proto + dbServer salvar/carregar `SaveX/SaveY` (regenerar proto). | **NÃO** |

## Observações
- **B1 e B3 compartilham** o `_MSG_CreateMob` — fechar o layout dele destrava **ver outros players E ver NPCs**. É o maior desbloqueio. → próximo prompt p/ o agente Windows.
- **B4** dá pra fazer agora sem o agente (o equip está no template BaseMob, offset 140).
- Setup: contas `test`/`test123` e `test2`/`test123`. Stack via `docker compose up`.

| B7 | **NPCs sem funcionalidade** (nome só no hover; loja/troca não abrem) | P1 | (a) Nome: o `CreateMob.Score.Merchant` (tipo de NPC) não era enviado → cliente tratava como monstro. **Feito:** envio do `Merchant` do template (Galford=1 loja, Guarda_Carga=2 banco, Gate_Keeper=128). (b) **Loja/troca:** os handlers `_MSG_REQShopList`/`_MSG_ReqBuy`/`_MSG_Buy`/`_MSG_Sell` não existem no Go; falta a lista de itens por NPC + o fluxo de clicar→abrir loja. | (b) subsistema de loja/banco/troca (precisa do agente: packets + de onde vem a lista de itens de cada NPC) | **SIM** (loja/troca) |

## (adicione aqui os bugs que você já viu)
- …
