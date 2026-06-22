# Fase 3 — Modelo de Domínio e Estado (w2pp-OpenWYD)

> **Objetivo:** descrever as entidades vivas em memória do TMSrv e como o estado global é
> organizado, para escolher o modelo de concorrência/dados da stack nova.
>
> **Reuso:** complementa `docs/agents/component-deep-analyzer/component-analysis-TMSrv-Core-*.md`,
> `-TMSrv-CUser-*.md`, `-TMSrv-CMob-*.md`, `-TMSrv-CItem-*.md`. Aqui consolida-se a topologia de
> estado e os pontos de concorrência.

---

## 1. A relação de índice central (conn ↔ player ↔ mob)

O sistema inteiro gira em torno de **um único índice inteiro** que liga sessão, jogador e entidade
de mundo.

```
conn  ∈ [0, MAX_USER)          MAX_USER = 1000   (Basedef.h:116)

pUser[conn]   -> CUser   : a SESSÃO (socket, conta, modo, trade, timers)
pMob[conn]    -> CMob    : a ENTIDADE DE MUNDO do jogador (posição, score, AI=N/A)
pMob[conn].MOB -> STRUCT_MOB : os dados persistíveis (nome, exp, equip, carry)

pMob[MAX_USER .. MAX_MOB)      MAX_MOB = 25000   (Basedef.h:167)
              -> CMob    : os MOBS/NPCs reais (índice >= 1000)
```

Evidência:
- `Basedef.h:116`: *"Max users on CUser pUser and starting index of npcs and mobs"* — confirma que
  os índices `[0,1000)` de `pMob[]` são reservados aos jogadores e `[1000,25000)` aos mobs/NPCs.
- Handlers indexam o mesmo `conn` em ambos os arrays: `pUser[conn].Mode` e `pMob[conn].MOB.*`
  (ex.: `_MSG_DeleteCharacter.cpp:36`, `_MSG_Challange.cpp:50`, `_MSG_AccountLogin.cpp:95`).
- O campo `HEADER.ID` (Fase 1) carrega justamente esse `conn` no fio — por isso o dispatcher valida
  `ID ∈ [0, MAX_USER)` antes de qualquer handler (`ProcessClientMessage.cpp:42`).

> **Implicação nº 1 (migração):** "player" e "mob" não são tipos diferentes — compartilham
> `STRUCT_MOB`/`CMob` e o **mesmo array** `pMob[]`, particionado por faixa de índice. Na stack nova
> isso pode virar um trait/interface comum (`Entity`) com duas implementações, mas a aritmética de
> índice (id < 1000 ⇒ player) é assumida em todo o código e nas mensagens de fio (`AttackerID`,
> `TargetID`, `OpponentID` são índices de `pMob`). Preservar a numeração.

---

## 2. Entidades em memória

### 2.1. `CUser` (sessão) — `pUser[MAX_USER]` (`Server.h:70,258`)

A camada de **conexão/sessão**. Campos-chave (`TMSrv/CUser.h:39-88`):

| Campo | Tipo | Papel |
|-------|------|-------|
| `AccountName[16]` | char | conta logada |
| `Slot` | int | qual dos 4 personagens está selecionado |
| `IP` | uint | IP do cliente |
| `Mode` | int | **máquina de estado da sessão** (ver §3.1) |
| `cSock` | `CPSock` | o socket + buffers de recv/send (Fase 1) |
| `Cargo[128]` | `STRUCT_ITEM` | cópia em memória do baú da conta |
| `Coin` | int | gold do cargo |
| `Trade` | `MSG_Trade` | estado da negociação em andamento |
| `AutoTrade` | `MSG_SendAutoTrade` | loja pessoal |
| `IsAutoTrading`, `TradeMode` | int | flags de trade |
| `LastAttack/LastMove/LastAction(+Tick)` | int | timestamps anti-flood / anti-speed |
| `CountMob1..3`, `QuestAtiva`, `LastQuestDay` | | progresso de quest diária |
| `BlockPass`, `IsBlocked` | | trava de char |
| `LastReceiveTime`, `NumError` | | liveness e "cra points" (anti-cheat) |

### 2.2. `CMob` (entidade de mundo) — `pMob[MAX_MOB]` (`Server.h:259`)

Posição, estado de combate/AI e os dados de `STRUCT_MOB`. Campos-chave (`TMSrv/CMob.h`):

| Campo | Papel |
|-------|-------|
| `Mode` | **máquina de estado da entidade** (ver §3.2) |
| `MOB` (`STRUCT_MOB`) | dados persistíveis (nome, classe, exp, equip[16], carry[64], score) |
| `extra` (`STRUCT_MOBEXTRA`) | cidadania, fama, ClassMaster, quests, soul |
| `TargetX/TargetY` | destino atual (movimento) |
| `affect[MAX_AFFECT]` | buffs/debuffs ativos |
| posição/grid, alvo de combate, timers de AI | |

### 2.3. `CItem` — `pItem[MAX_ITEM]` (`Server.h:264`, `MAX_ITEM=5000`)

Itens **no chão** (drops): posição no grid, conteúdo (`STRUCT_ITEM`), dono temporário, tempo de
decaimento (`_MSG_DecayItem`). Itens em inventário/equip/cargo vivem **dentro** de `STRUCT_MOB`/
`CUser.Cargo`, não em `pItem[]`.

### 2.4. Guilda / Party / NPC

- **Guilda:** estado global em arrays — `g_pGuildWar[65536]`, `g_pGuildAlly[65536]`
  (`Server.h:57-58`), `GuildScore[MAX_GUILD=4096]` (`:76`), `GuildImpostoID[MAX_GUILDZONE]` (`:66`).
  `Guild` é um `ushort` em `STRUCT_MOB`; `GuildLevel` define membro(0)…líder(9).
- **Party:** `MAX_PARTY=12` (`Basedef.h:228`); membros referenciados por índice `conn`; líder via
  `Leaderconn`. Sem struct dedicada persistida — é estado de runtime.
- **NPC/Spawn:** `CNPCGenerator mNPCGen` + `CNPCSummon mSummon` (`Server.h:255-256`); guardas em
  `g_pGuard[MAX_NPC_GUARD_COUT=7]`.

---

## 3. Máquinas de estado

### 3.1. Sessão do jogador (`CUser.Mode`) — `CUser.h:26-37`

```
 USER_EMPTY(0) ──AcceptUser──► USER_ACCEPT(1) ──_MSG_AccountLogin──► USER_LOGIN(2)
                                                          │
                                          (DBSrv valida conta) 
                                                          ▼
                                                  USER_SELCHAR(11)  ◄─── tela de seleção
                                                          │ _MSG_CharacterLogin
                                                          ▼
                                                  USER_CHARWAIT(12) ─ aguarda DBSrv
                                                          │ _MSG_DBCNFCharacterLogin
                                                          ▼
                                                   USER_PLAY(22)  ◄── jogando
                                                          │ logout / quit
                                                          ▼
                                              USER_SAVING4QUIT(24) ─ salva no DBSrv
                                                          ▼
                                                   USER_EMPTY(0)

  USER_WAITDB(13) = aguardando resposta genérica do DBSrv (criação/confirmação)
```

Transições disparadas por handlers (`Mode = USER_*` em `_MSG_AccountLogin.cpp`,
`_MSG_CharacterLogin.cpp`, `ProcessDBMessage.cpp`). Estados `*WAIT*` são **pontos de espera
assíncrona** pela resposta do DBSrv — críticos para o modelo de concorrência da stack nova
(request/response com o DB).

### 3.2. Entidade de mundo (`CMob.Mode`) — `CMob.h:26-35`

```
 MOB_EMPTY(0)        slot vazio
 MOB_USERDOCK(1)     player "ancorado" (entrou, ainda não ativo no mundo)
 MOB_USER(2)         player ativo no mundo
 ── mobs (NPC AI) ──
 MOB_IDLE(3) ─► MOB_PEACE(4) ─► MOB_COMBAT(5) ─► MOB_RETURN(6)
                    ▲                │
                    └──── MOB_ROAM(8)│► MOB_FLEE(7)
 MOB_WAITDB(9)       mob aguardando persistência
```

A AI de mob (idle→roam→combat→flee/return) roda no timer de servidor (`ProcessSecMinTimer`,
`MobKilled.cpp`, `CMob.cpp`). Players usam apenas `MOB_USERDOCK`/`MOB_USER`.

### 3.3. Trade (negociação P2P)

Estado em `CUser.Trade`/`TradeMode` + `STRUCT_ITEM Item[MAX_TRADE=15]`. Fluxo:
`_MSG_TradingItem` (colocar item) → `_MSG_Trade` (confirmar, `MyCheck`) → ambos confirmam →
troca atômica → `_MSG_QuitTrade` (cancelar). Detalhe na Fase 5/6.

### 3.4. War / Castle (Zakum)

Estado global: `TowerCount`, `TowerStage`, `GuildTower` (`Server.h:73-75`), `g_pGuildWar/Ally`,
`GuildScore[]`, `BrState` (estado de "Batalha Real"?). Orquestrado por `CWarTower`/`CCastleZakum`
em timers. Detalhe na Fase 4/6.

---

## 4. Estado global mutável (inventário de variáveis compartilhadas)

Tudo é **estado global de processo** (sem encapsulamento de sessão). Principais (`Server.h`):

| Variável | Tipo/Tamanho | Domínio |
|----------|--------------|---------|
| `pUser[1000]` | `CUser` | sessões |
| `pMob[25000]` | `CMob` | players + mobs |
| `pItem[5000]` | `CItem` | itens no chão |
| `pHeightGrid[4096][4096]` | char | colisão (read-only após load) |
| `g_pAttribute[1024][1024]` | uchar | atributos de área (read-only) |
| `pMobGrid[4096][4096]` | ushort | **índice do mob por célula** (espacial) |
| `pItemGrid[4096][4096]` | ushort | índice do item por célula |
| `g_pGuildWar[65536]`, `g_pGuildAlly[65536]` | ushort | relações de guilda |
| `GuildScore[4096]` | int | placar de guilda |
| `g_pItemList[MAX_ITEMLIST]` | `STRUCT_ITEMLIST` | catálogo (read-only) |
| `BannedUser[1000]`, `pMac[200]` | | bans / bloqueio de MAC |
| `CurrentTime`, `LastSendTime`, `SecCounter` | uint | relógio do servidor |
| `ServerDown`, `BrState`, `CurrentWeather`, `Sapphire`, `ServerIndex` | int | flags globais |
| `DBServerSocket`, `BillServerSocket` | `CPSock` | links de saída |

`pMobGrid`/`pItemGrid` são índices espaciais: dado `(x,y)`, retornam o índice da entidade/item
naquela célula — usados para "quem está na minha visão" e colisão. São **estado derivado** que
precisa ser mantido em sincronia com `pMob[].pos` a cada movimento.

---

## 5. Concorrência: como o estado é acessado hoje

**Modelo atual: thread única, reactor WinSock.** Todo o `Server.cpp` roda no `MainWndProc`
(`Server.cpp:4173`), que processa eventos `WSA_*` sequencialmente (Fase 6). Não há locks porque não
há paralelismo: cada mensagem é processada do início ao fim antes da próxima.

Consequências e pontos de atrito para a stack nova:

| Hotspot | Por que dificulta concorrência |
|---------|-------------------------------|
| `pMob[]`/`pUser[]` globais mutáveis | qualquer handler lê/escreve qualquer slot por índice (ex.: ataque toca `pMob[atacante]` e `pMob[alvo]`); paralelizar exige locking por entidade ou actor-model |
| `pMobGrid`/`pItemGrid` | estado espacial compartilhado atualizado em todo movimento; ponto de contenção clássico |
| Estado de guilda/war global | `g_pGuildWar`, `GuildScore` lidos/escritos por múltiplos jogadores |
| Espera assíncrona do DBSrv | `USER_*WAIT*` bloqueia a sessão num estado intermediário; hoje resolvido por callbacks no reactor — na stack nova vira `async/await` ou mensagens entre atores |
| Trade/party P2P | mutam **dois** `CUser`/`CMob` ao mesmo tempo (origem+alvo) — operação que cruza fronteiras de entidade, candidata a deadlock se houver locks por entidade |
| RNG global (`rand()`) | usado em drop/refino/keyword; estado global de RNG — relevante para determinismo (Fase 8) |

> **Implicação nº 2 (modelo de concorrência recomendado):** dado o acoplamento por índice e as
> operações que cruzam entidades (ataque, trade, party, war), o caminho de menor risco é **manter o
> processamento de gameplay single-threaded** (um "game loop"/reactor autoritativo), isolando I/O de
> rede e persistência em tarefas assíncronas. Isso preserva a semântica atual sem reintroduzir
> bugs de corrida. Sharding por mapa/região é possível depois, mas a v1 do big-bang deve replicar o
> modelo de thread única para garantir paridade. Ver Fase 9.

---

## 6. Limites e constantes de capacidade (`Basedef.h`)

| Constante | Valor | Significado |
|-----------|------:|-------------|
| `MAX_USER` | 1000 | jogadores simultâneos / fronteira player↔mob |
| `MAX_MOB` | 25000 | players + mobs no mundo |
| `MAX_ITEM` | 5000 | itens no chão |
| `MAX_EQUIP` / `MAX_CARRY` / `MAX_CARGO` | 16 / 64 / 128 | slots de item por entidade/conta |
| `MAX_GRIDX` / `MAX_GRIDY` | 4096 | dimensão do mundo |
| `MAX_GUILD` / `MAX_GUILDZONE` | 4096 / 5 | guildas / zonas de guerra |
| `MAX_PARTY` | 12 | membros de party |
| `MAX_TRADE` | 15 | itens por trade |
| `MAX_TARGET` | 13 | alvos por ataque (AoE) |
| `MAX_AFFECT` | 32 | buffs por entidade |
| `MAX_MAC` | 200 | MACs bloqueados |
| `MOB_PER_ACCOUNT` | 4 | personagens por conta |
| `MAX_SERVER` | 10 | canais por DBSrv |

> **Status da Fase 3: COMPLETO** para a topologia de estado, entidades e máquinas de estado de
> sessão/mob. Detalhes finos da AI de mob e das máquinas de war/castle são aprofundados na Fase 4
> (regras) e Fase 6 (fluxos).
