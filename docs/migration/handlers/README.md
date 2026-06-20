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

**Lotes 2–N — PENDENTES (catálogo abaixo; reaproveitar o template).** São os 50 handlers
restantes. Cada um segue o mesmo padrão observado nos críticos: (1) cast da struct; (2) checagem de
`pUser[conn].Mode == USER_PLAY` e `Hp > 0`; (3) validação de bounds de slot/grid; (4) mutação de
`pMob[conn].MOB`/`pUser[conn]`; (5) resposta via `SendFunc`; (6) `AddCrackError` em violação.

## Catálogo dos handlers restantes (gatilho → arquivo-fonte)

| Handler (Type) | Fonte | Domínio | Notas para o contrato |
|----------------|-------|---------|-----------------------|
| `_MSG_AccountSecure` (0x0FDE) | `_MSG_AccountSecure.cpp` | conta | PIN numérico; valida `NumericToken` |
| `_MSG_DeleteCharacter` (0x0211) | `_MSG_DeleteCharacter.cpp` | conta | exige senha; → DBSrv |
| `_MSG_CharacterLogout` (0x0215) | `_MSG_CharacterLogout.cpp` | sessão | salva e volta à seleção |
| `_MSG_Restart` (0x0289) | `_MSG_Restart.cpp` | sessão | reviver/voltar cidade |
| `_MSG_Action` (0x036C/66/68) | `_MSG_Action.cpp` | movimento | rota/velocidade; anti-speedhack |
| `_MSG_Motion` (0x036A) | `_MSG_Motion.cpp` | movimento | emotes |
| `_MSG_ChangeCity` (0x0291) | `_MSG_ChangeCity.cpp` | movimento | troca cidade (Merchant bits) |
| `_MSG_ReqTeleport` (0x0290) | `_MSG_ReqTeleport.cpp` | movimento | teleporte |
| `_MSG_NoViewMob` (0x0369) | `_MSG_NoViewMob.cpp` | visão | culling de mobs |
| `_MSG_MessageChat` (0x0333) | `_MSG_MessageChat.cpp` | chat | público + comandos `/` (admin) |
| `_MSG_MessageWhisper` (0x0334) | `_MSG_MessageWhisper.cpp` | chat | sussurro |
| `_MSG_TradingItem` (0x0376) | `_MSG_TradingItem.cpp` | item | mover item inv↔equip↔trade↔cargo |
| `_MSG_Trade` (0x0383) | `_MSG_Trade.cpp` | trade | confirmar troca (atômica) |
| `_MSG_QuitTrade` (0x0384) | `_MSG_QuitTrade.cpp` | trade | cancelar |
| `_MSG_DeleteItem` (0x02E4) | `_MSG_DeleteItem.cpp` | item | destruir item |
| `_MSG_SplitItem` (0x02E5) | `_MSG_SplitItem.cpp` | item | dividir stack |
| `_MSG_UpdateItem` (0x0374) | `_MSG_UpdateItem.cpp` | item | abrir/fechar baú/estado |
| `_MSG_ApplyBonus` (0x0277) | `_MSG_ApplyBonus.cpp` | char | distribuir pontos (Str/Int/Dex/Con/Skill) |
| `_MSG_SetShortSkill` (0x0378) | `_MSG_SetShortSkill.cpp` | char | barra de skills |
| `_MSG_PKMode` (0x0399) | `_MSG_PKMode.cpp` | char | modo PK |
| `_MSG_Deprivate` (0x028C) | `_MSG_Deprivate.cpp` | char | (privar/expulsar?) |
| `_MSG_REQShopList` (0x027B) | `_MSG_REQShopList.cpp` | loja | abre loja NPC |
| `_MSG_Buy` (0x0379) | `_MSG_Buy.cpp` | loja | compra de NPC |
| `_MSG_Sell` (0x037A) | `_MSG_Sell.cpp` | loja | venda p/ NPC |
| `_MSG_ReqBuy` (0x0398) | `_MSG_ReqBuy.cpp` | auto-trade | compra de loja de jogador |
| `_MSG_SendAutoTrade` (0x0397) | `_MSG_SendAutoTrade.cpp` | auto-trade | abrir loja pessoal |
| `_MSG_ReqTradeList` (0x039A) | `_MSG_ReqTradeList.cpp` | auto-trade | listar |
| `_MSG_Deposit` (0x0388) | `_MSG_Deposit.cpp` | banco | depositar gold/cargo |
| `_MSG_Withdraw` (0x0387) | `_MSG_Withdraw.cpp` | banco | sacar |
| `_MSG_SendReqParty` (0x037F) | `_MSG_SendReqParty.cpp` | party | convidar |
| `_MSG_AcceptParty` (0x03AB) | `_MSG_AcceptParty.cpp` | party | aceitar (checa nível/região) |
| `_MSG_RemoveParty` (0x037E) | `_MSG_RemoveParty.cpp` | party | sair/expulsar |
| `_MSG_InviteGuild` (0x03D5) | `_MSG_InviteGuild.cpp` | guilda | convidar p/ guilda |
| `_MSG_GuildAlly` (0x0E12) | `_MSG_GuildAlly.cpp` | guilda | aliança (→DBSrv) |
| `_MSG_War` (0x0E0E) | `_MSG_War.cpp` | guerra | guild war |
| `_MSG_Challange` (0x028E) | `_MSG_Challange.cpp` | guerra | desafio/imposto de zona |
| `_MSG_ChallangeConfirm` (0x028F) | `_MSG_ChallangeConfirm.cpp` | guerra | confirmar |
| `_MSG_Quest` (0x028B) | `_MSG_Quest.cpp` | quest | progresso/entrega |
| `_MSG_ReqRanking` (0x039F) | `_MSG_ReqRanking.cpp` | ranking | consulta |
| `_MSG_CapsuleInfo` (0x02CD) | `_MSG_CapsuleInfo.cpp` | cash | cápsula (→DBSrv) |
| `_MSG_PutoutSeal` (0x03CC) | `_MSG_PutoutSeal.cpp` | item | retirar selo |
| `_MSG_CombineItemEhre/Tiny/Shany/Ailyn/Agatha/Odin/Lindy/Alquimia/Extracao` | `_MSG_CombineItem*.cpp` | refino | variantes da engine de combine (Fase 4 §3) |

> **Status da Fase 5: PARCIAL.** Lote 1 (8 críticos) com contrato completo. Lotes seguintes
> catalogados com gatilho/struct/domínio — gerar em lotes de 5–8, usando o template dos críticos e a
> fórmula correspondente da Fase 4.
