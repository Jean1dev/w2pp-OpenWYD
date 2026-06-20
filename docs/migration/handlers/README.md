# Fase 5 — Contratos de Comportamento por Handler

> **Objetivo:** um contrato por handler `_MSG_*.cpp` (58 arquivos) para reimplementar e testar 1:1.
> Feito em **lotes** (o prompt pede para começar pelos críticos). Cada contrato traz: gatilho +
> struct de entrada, pré-condições/validações, efeitos colaterais, saídas S→C, anti-cheat e riscos.
>
> **Como ler junto:** os valores de `Type`/struct estão na Fase 1 (`protocol-spec.md`); as fórmulas
> referenciadas (exp, drop, refino) na Fase 4 (`game-rules.md`); os fluxos ponta-a-ponta na Fase 6.

## Status por lote

**Lote 1 — CRÍTICOS (COMPLETO, arquivo dedicado):**
| Handler | Arquivo | Type | Resumo |
|---------|---------|------|--------|
| `_MSG_AccountLogin` | [_MSG_AccountLogin.md](_MSG_AccountLogin.md) | 0x020D | login de conta → DBSrv |
| `_MSG_CreateCharacter` | [_MSG_CreateCharacter.md](_MSG_CreateCharacter.md) | 0x020F | criação de personagem |
| `_MSG_CharacterLogin` | [_MSG_CharacterLogin.md](_MSG_CharacterLogin.md) | 0x0213 | entrar no mundo |
| `_MSG_Attack` | [_MSG_Attack.md](_MSG_Attack.md) | 0x0367 | ataque/skill (+anti-speed) |
| `_MSG_DropItem` | [_MSG_DropItem.md](_MSG_DropItem.md) | 0x0272 | dropar item no chão |
| `_MSG_GetItem` | [_MSG_GetItem.md](_MSG_GetItem.md) | 0x0270 | pegar item do chão |
| `_MSG_UseItem` | [_MSG_UseItem.md](_MSG_UseItem.md) | 0x0373 | usar/equipar/refinar |
| `_MSG_CombineItem` | [_MSG_CombineItem.md](_MSG_CombineItem.md) | 0x03A6 | combine/Anct (engine) |

**Lote 2 — RESTANTES (COMPLETO, agrupados por domínio).** Os 50 handlers restantes foram
documentados em **arquivos por domínio** (handlers relacionados compartilham estado/fluxos; é mais
coeso que 1 arquivo por handler). Cada entrada traz gatilho+struct, validações, efeitos, saídas e
riscos, no mesmo padrão do lote 1.

| Arquivo (domínio) | Handlers cobertos |
|-------------------|-------------------|
| [lote2-sessao-conta.md](lote2-sessao-conta.md) | `AccountSecure`, `DeleteCharacter`, `CharacterLogout`, `Restart`, `Deprivate` |
| [lote2-movimento.md](lote2-movimento.md) | `Action` (+Action2/3), `Motion`, `ChangeCity`, `ReqTeleport`, `NoViewMob` |
| [lote2-chat.md](lote2-chat.md) | `MessageChat`, `MessageWhisper` |
| [lote2-itens-char.md](lote2-itens-char.md) | `TradingItem`, `DeleteItem`, `SplitItem`, `UpdateItem`, `PutoutSeal`, `SetShortSkill`, `ApplyBonus`, `PKMode` |
| [lote2-trade-autotrade.md](lote2-trade-autotrade.md) | `Trade`, `QuitTrade`, `SendAutoTrade`, `ReqTradeList`, `ReqBuy` |
| [lote2-loja-banco.md](lote2-loja-banco.md) | `REQShopList`, `Buy`, `Sell`, `Deposit`, `Withdraw` |
| [lote2-party-guilda-guerra.md](lote2-party-guilda-guerra.md) | `SendReqParty`, `AcceptParty`, `RemoveParty`, `InviteGuild`, `GuildAlly`, `War`, `Challange`, `ChallangeConfirm` |
| [lote2-quest-ranking-cash.md](lote2-quest-ranking-cash.md) | `Quest`, `ReqRanking`, `CapsuleInfo` |
| [lote2-combine-variantes.md](lote2-combine-variantes.md) | `CombineItem{Ehre,Tiny,Shany,Ailyn,Agatha,Odin,Lindy,Alquimia}` + `CombineItemExtracao` |

**Cobertura:** 8 (lote 1) + 50 (lote 2) = **58/58 handlers `_MSG_*.cpp`**.

> **Status da Fase 5: COMPLETO.** Todos os 58 handlers têm contrato.
> **`_MSG_MessageWhisper`** agora tem a enumeração completa dos 55 comandos em
> [_MSG_MessageWhisper-comandos.md](_MSG_MessageWhisper-comandos.md). Resta **1** com UNVERIFIED
> interno por ser subsistema extenso: **`_MSG_Quest`** (2753 linhas — catalogar todos os
> `npcMode`/etapas/recompensas). As receitas/taxas exatas das variantes de combine ficam na Fase 4 §3.
