# Fase 6 — Fluxos Ponta-a-Ponta (w2pp-OpenWYD)

> Diagramas de sequência (ASCII) dos caminhos principais. Atores: **C** = cliente `WYD.exe`,
> **TM** = TMSrv (jogo), **DB** = DBSrv (persistência), **BI** = BISrv (billing). Setas com o
> `Type` (Fase 1) e o handler (Fase 5). Pré/pós-condições de `Mode` da sessão (Fase 3 §3.1).

---

## 1. Login (conta) — C → TM → DB → C

```
C                         TM                          DB
│  INITCODE (0x1F11F311)  │                            │   (handshake, Fase 1 §1.2)
│────────────────────────►│ Mode: EMPTY→ACCEPT (AcceptUser)
│                         │                            │
│  _MSG_AccountLogin       │                            │
│  (0x020D, user+pass+ver) │                            │
│────────────────────────►│ Exec_MSG_AccountLogin       │
│                         │  • valida ClientVersion=7640│
│                         │  • CheckFailAccount < 3      │
│                         │  • Mode ACCEPT→LOGIN         │
│                         │  _MSG_DBAccountLogin (0x0803)│
│                         │───────────────────────────►│ valida AccountPass (texto plano!)
│                         │                            │ lê STRUCT_ACCOUNTFILE de disco
│                         │   _MSG_DBCNFAccountLogin     │ (DBGetSelChar → STRUCT_SELCHAR)
│                         │◄───────────────────────────│ (0x0416)
│  _MSG_CNFAccountLogin     │ ProcessDBMessage:           │
│  (0x010A + lista chars)  │  • pUser.SelChar = SELCHAR  │
│◄────────────────────────│  • Mode LOGIN→SELCHAR        │
│  [tela de seleção]       │                            │
```

Falhas (DB→TM→C): `_MSG_DBAccountLoginFail_Account/_Pass/_Block/_Disable` →
`_MSG_NewAccountFail`/mensagem; `_MSG_DBAlreadyPlaying`→`_MSG_AlreadyPlaying`.

---

## 2. Criação / seleção de personagem

```
C                         TM                          DB
│ _MSG_CreateCharacter      │ (Mode==SELCHAR)            │
│ (0x020F, slot+name+class)│ BASE_CheckValidString       │
│────────────────────────►│ Mode SELCHAR→WAITDB          │
│                         │ _MSG_DBCreateCharacter(0x0802)│
│                         │───────────────────────────►│ grava char no STRUCT_ACCOUNTFILE
│                         │  _MSG_DBCNFNewCharacter(0x0418)│
│ _MSG_CNFNewCharacter      │◄───────────────────────────│
│ (0x0110, SELCHAR)         │ Mode WAITDB→SELCHAR          │
│◄────────────────────────│                            │
```

Seleção (entrar no mundo):
```
│ _MSG_CharacterLogin       │ (Mode==SELCHAR; billing gate)│
│ (0x0213, slot)           │ Mode SELCHAR→CHARWAIT         │
│────────────────────────►│ pMob[conn].Mode = MOB_USER    │
│                         │ _MSG_DBCharacterLogin(0x0804) │
│                         │───────────────────────────►│ carrega STRUCT_MOB do slot
│                         │  _MSG_DBCNFCharacterLogin     │
│ _MSG_CNFCharacterLogin    │◄───────────────────────────│ (0x0417)
│ (0x0114: STRUCT_MOB,      │ Mode CHARWAIT→PLAY            │
│  pos, weather, skillbar) │ injeta pMob[conn] no grid    │
│◄────────────────────────│ broadcast _MSG_CreateMob aos vizinhos
│ [no mundo]               │                            │
```

---

## 3. Movimentação

```
C                                   TM (vizinhos)
│ _MSG_Action (0x036C: PosX/Y, Route[24], TargetX/Y)
│──────────────────────────────────►│ Exec_MSG_Action
│                                    │ • valida velocidade/rota (anti-speed)
│                                    │ • atualiza pMob[conn].pos + pMobGrid
│   _MSG_Action (broadcast)           │
│◄──────────────────────────────────│ reenvia aos jogadores na visão
│ _MSG_CreateMob / _MSG_RemoveMob     │ (entra/sai da área de visão)
│◄──────────────────────────────────│
```

---

## 4. Ataque → morte → drop

```
C                         TM                                vizinhos
│ _MSG_Attack (0x0367)      │ Exec_MSG_Attack                  │
│ (SkillIndex, Target...)  │ • Mode==PLAY, Hp>0               │
│────────────────────────►│ • cadência ≥800ms, tick válido   │
│                         │ • calcula dano (server-auth)      │
│   _MSG_Attack (broadcast) │ • aplica dano ao(s) alvo(s)      │
│◄────────────────────────│──────────────────────────────────►│ (todos veem)
│   _MSG_SetHpDam / SetHpMp │                                  │
│◄────────────────────────│                                  │
│         (se alvo morre):  │ MobKilled():                     │
│                         │  • distribui EXP (Fase 4 §1)      │
│   _MSG_CNFMobKill         │  • drop gold/item (Fase 4 §2)    │
│◄────────────────────────│  • CreateItem no chão → pItem[]   │
│   _MSG_CreateItem (chão)  │──────────────────────────────────►│
│◄────────────────────────│   _MSG_RemoveMob (despawn alvo)   │
```

Pickup (continuação):
```
│ _MSG_GetItem (0x0270, ItemID=id+10000)
│────────────────────────►│ Exec_MSG_GetItem
│                         │ • dist ≤3, pItem.Mode!=0
│   _MSG_CNFGetItem         │ • move p/ Carry[], limpa chão
│◄────────────────────────│ broadcast remoção do item
```

---

## 5. Trade entre jogadores

```
A                    TM                    B
│ _MSG_TradingItem    │                     │   (A coloca item no trade)
│───────────────────►│ Exec_MSG_TradingItem │
│   _MSG_Trade (estado)│────────────────────►│ B vê a oferta
│◄───────────────────│◄────────────────────│ B coloca item / confirma
│ _MSG_Trade (MyCheck) │                     │
│───────────────────►│ ambos MyCheck==1?    │
│                    │  → troca ATÔMICA      │
│                    │  (swap Item[]+gold)   │
│   _MSG_Trade (done) │◄────────────────────►│
│◄───────────────────│                     │
│ (ou _MSG_QuitTrade  │  cancela: RemoveTrade(ambos))
```

> Pontos críticos de dup (Fase 5: DropItem/GetItem): a troca toca **dois** `CUser`/`CMob`. Manter
> atômico (thread única hoje; na stack nova, transação por par).

---

## 6. Refino / Combine

```
C                         TM
│ _MSG_CombineItem (0x03A6) │ Exec_MSG_CombineItem
│ (Item[], InvenPos[])     │ • GetMatchCombine → taxa (0=inválido)
│────────────────────────►│ • consome insumos (zera Carry[])
│                         │ • roll: rand()%115 (Fase 4 §3.1)
│   _MSG_CombineComplete    │ • sucesso→ item+sanc7 ; falha→ perde
│◄────────────────────────│   parm: 0=inválido 1=ok 2=falha
│   _MSG_SendItem (slots)   │
│◄────────────────────────│
```

---

## 7. Guild War / Torre (GTorre)

```
TM (timer ProcessSecMinTimer / CWarTower)
│  GTorreHour chega → inicia janela de guerra
│  estado global: TowerStage, TowerCount, GuildTower, g_pGuildWar[]
│
C  ─_MSG_War (0x0E0E)─► TM ─(→DB para persistir resultado)─► DB
│  durante a guerra: ataques contam pontos → GuildScore[guild]
│  _MSG_SendWarInfo / _MSG_SendCastleState (broadcast placar)
│  fim da janela → guilda vencedora assume a torre/zona
│  _MSG_Challange (imposto de zona): líder cobra Exp/Coin (Fase 5)
```

> Detalhe completo em `CWarTower.*` (deep-dive `component-analysis-TMSrv-CastleWar-*.md`). Estados
> em `Server.h:73-76`. **UNVERIFIED** a sequência exata de mensagens da guerra — capturar.

---

## 8. Castle (Zakum)

```
TM (CCastleZakum + timer)
│  evento de castelo agendado (Settings/CastleQuest.txt)
│  spawn de boss/mobs do castelo (MOB_INITIAL..MOB_END, BOSS[2])
│  na morte do boss: CCastleZakum::KeyDrop (MobKilled.cpp:2870)
│   → dropa a "chave"/prêmio (STRUCT_CASTLEQUEST.Prize[])
│  _MSG_SendCastleState / _MSG_SendCastleState2 (broadcast)
│  guilda que cumprir a quest assume o castelo
```

---

## 9. Billing (TMSrv → BISrv)

```
TM                                   BI
│  SendBilling(conn, account, ...)    │   (durante CharacterLogin / cash)
│  pacote _AUTH_GAME (196 bytes CRU,  │   sem HEADER/ofuscação, Fase 1 §1.6)
│───────────────────────────────────►│ valida conta/saldo/expiração
│   _AUTH_GAME (196 bytes resposta)    │
│◄───────────────────────────────────│ resultado → libera/bloqueia login
│  (estados Unk_1816/Unk_2728 no       │
│   CharacterLogin refletem o billing) │
```

> Layout interno de `_AUTH_GAME` é **UNVERIFIED** (placeholder `char Unk[196]`, Fase 1 §4.3).
> Capturar o tráfego real TM↔BI (Fase 8) antes de reimplementar. O BISrv também fala com NPServer
> (`ProcessMessage.cpp`) para contas/cash externos (`_MSG_NP*`, Fase 1 §3.4).

---

## Notas de reator (todos os fluxos)

Tudo roda no **reactor single-thread** do TMSrv (`MainWndProc`, `Server.cpp:4173`): `WSA_ACCEPT`
(novas conexões), `WSA_READ` (drena socket → `ReadMessage` → `ProcessClientMessage`), `WSA_READDB`
(respostas do DBSrv → `ProcessDBMessage`). As esperas `USER_*WAIT*` são pausas assíncronas até a
resposta do DBSrv chegar num tick posterior do reator — na stack nova viram `async`/atores
(Fase 3 §5, Fase 9).

> **Status da Fase 6: COMPLETO** para login, char, movimento, ataque/morte/drop, trade, combine.
> PARCIAL/UNVERIFIED nas sequências exatas de war/castle/billing (capturar).
