# Bugs in-game (cliente real ↔ servidor Go) — mapeamento

> Estado: cliente "Cavaleiros de Kersef" 7662 conecta → loga → cria char → entra no mundo → anda.
> Estes são os bugs observados depois disso, com causa-raiz provável e o que falta para corrigir.
> Prioridade: P0 (quebra multiplayer/jogo) → P3 (cosmético).

| # | Bug | Prioridade | Causa-raiz provável | O que falta | Precisa do agente Windows? |
|---|-----|:---:|---|---|:---:|
| B1 | **Player duplica a cada passo** (visto de outra conta) | ~~P0~~ **NÃO REPRODUZIDO / OK no cliente real** | Fluxo básico e cruzamento de visão com 2 usuários testados no cliente real sem duplicação/bug visual. O código já envia `CreateMob` na entrada, `RemoveMob` no logout e revela NPCs/mobs ao andar; não há falha observável hoje. | Manter em observação; não mexer sem caso reproduzível. | — |
| B2 | **Gold inicial negativo** (-858993664) | ~~P1~~ **RESOLVIDO** | O valor `-858993664` = `0xCCCCCC00` = bytes `00 CC CC CC` no **offset 24** do `STRUCT_MOB`. O template BaseMob é um dump de memória com **padding 0xCC não-inicializado**; o cliente lê o gold no offset 24. Zerado em `EncodeCNFCharacterLoginRaw` (offsets 24-31). | — | — |
| B3 | **Sem NPCs/monstros no mapa** | P1 → **PARCIAL** | Spawn nunca ligado. **Feito:** parser `NPCGener.txt` (6103 geradores), loader dos templates `npc/<nome>` (816B), `SpawnMob` (10.496 mobs spawnados, cap 20k), e `CreateMob` dos mobs na visão na entrada **e ao andar** (`revealMobsInView` + grid). **IA iteração 1 feita** (`world/tick.go` + `handler/mobai.go`): monstro agro por proximidade/retaliação, persegue (1 tile/tick) e ataca o player corpo-a-corpo. **Falta:** hostilidade por clan, ranged, pathfinding real, roaming/segmentos, morte/ressurreição do player, respawn de mob; grupos completos (cap MinGroup≤6); ~1400 templates de Leader sem arquivo. | (refinos da IA — ver SESSION-PRIMER §7); reveal de players ao cruzar a visão andando | — |
| B4 | **Preview de classe na seleção = sempre TK** | ~~P2~~ **RESOLVIDO** | `STRUCT_SELCHAR` enviada **sem equipamento** → cliente desenha modelo padrão. | `sel.Equip[slot]` agora vem do `MobEquip` do template BaseMob da classe (`protocol.MobEquip` + `d.selCharsFrom`). | — |
| B5 | **Level mostra +1 na seleção** | ~~P3~~ **RESOLVIDO** | O cliente renderiza `SELCHAR.Score.Level` como one-based na tela de seleção. **Feito:** o preview envia `level-1` só no `SELCHAR`; o level autoritativo em jogo/login/persistência continua inalterado. | — | — |
| B6 | **Posição exata não persiste** | **COMPORTAMENTO ESPERADO** | Confirmado no cliente real/design atual: ao relogar, o personagem nasce no centro/spawn da última cidade visitada, não na coordenada exata onde deslogou. | Não implementar persistência de coordenada exata salvo se a regra de jogo mudar. | — |

## Observações
- **B1** foi mantido apenas como histórico: cliente real com 2 usuários não reproduziu duplicação/bug de visão.
- **B6** não é bug: spawn por última cidade visitada é o comportamento desejado.
- Setup: contas `test`/`test123` e `test2`/`test123`. Stack via `docker compose up`.

| B7 | **NPCs sem funcionalidade** (nome só no hover; loja/troca não abrem) | ~~P1~~ **PARCIAL/RESOLVIDO para loja+banco** | **Feito:** `CreateMob.Score.Merchant`, lojas NPC (`REQShopList`/`Buy`/`Sell`), banco/cargo e compra de item com `Price==0`. Ainda faltam NPCs específicos de quest/combinação/refino. | Implementar demais NPCs data-driven conforme roadmap. | depende do NPC |

## (adicione aqui os bugs que você já viu)
- …
