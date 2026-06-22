# Component Deep Analysis Report: TMSrv-Core

**Component**: TMSrv-Core (Game Server bootstrap, reactor, and message dispatch)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/TMSrv/Server.cpp` (10553), `Server.h`,
`ProcessClientMessage.cpp` (312), `ProcessDBMessage.*`, the `_MSG_*` handler family (58 files),
`SendFunc.*`, `GetFunc.*`, `CReadFiles.*`

---

## 1. Executive Summary

TMSrv-Core is the heart of the game server: it boots the process, connects to DBSrv, opens the
client listen socket, runs the single-threaded WinSock reactor, and routes every inbound message
to a handler. It is the largest unit in the project — `Server.cpp` alone is 10,553 lines — and it
is the trust boundary between untrusted game clients and authoritative game state.

The control structure is a classic WYD reactor. `WinMain` (`Server.cpp:3910`) registers a window
whose procedure `MainWndProc` (`Server.cpp:4173`) receives WinSock notification messages:
`WSA_ACCEPT` accepts new players (`CUser::AcceptUser`), `WSA_READ` drains a player's socket and
calls `ProcessClientMessage(User, Msg, FALSE)` per framed message (`Server.cpp:4430-4456`),
`WSA_READDB` services the DBSrv link and dispatches via `ProcessDBMessage` (`Server.cpp:4344-4369`),
and `WSA_READBILL` covers the (stubbed) billing link. Startup wires DBSrv at
`DBServerAddress:7514` (`Server.cpp:4017`) and the game listener at `GAME_PORT`
(`Server.cpp:4089`).

`ProcessClientMessage` (`ProcessClientMessage.cpp:38`) is a single `switch(std->Type)` over
roughly 60 message types, each delegating to an `Exec_MSG_*` handler. Before dispatch it enforces
several gate rules: the message `ID` must be a valid user slot, the server must not be shutting
down (`ServerDown`), pings are dropped, and an internal `SKIPCHECKTICK` marker short-circuits
self-originated traffic. The dispatcher is also a front line of anti-cheat: a client that tries
to push `_MSG_UpdateScore` is penalized with `AddCrackError` (`ProcessClientMessage.cpp:106-110`),
the "cra point" mechanism defined at `Server.cpp:998`.

---

## 2. Data Flow Analysis

```
Boot:   WinMain (Server.cpp:3910)
          -> register MainWndProc window class (Server.cpp:3849)
          -> CReadFiles loads static game data
          -> DBServerSocket.ConnectServer(DBServerAddress, 7514, WSA_READDB) (Server.cpp:4017)
          -> ListenSocket.StartListen(GAME_PORT, WSA_ACCEPT) (Server.cpp:4089)
Reactor: MainWndProc (Server.cpp:4173)
          case WSA_ACCEPT -> pUser[User].AcceptUser(ListenSocket.Sock) (Server.cpp:4475)
          case WSA_READ   -> cSock.Receive -> loop ReadMessage
                              -> ProcessClientMessage(User, Msg, FALSE) (Server.cpp:4456)
          case WSA_READDB -> DBServerSocket.Receive -> loop ReadMessage
                              -> ProcessDBMessage(Msg) (Server.cpp:4369)
          (reconnect to DBSrv on link error, Server.cpp:4282-4317)
Dispatch: ProcessClientMessage (ProcessClientMessage.cpp:38)
          -> validate ID, ServerDown, Ping, SKIPCHECKTICK
          -> switch(Type) -> Exec_MSG_<X>(conn, pMsg)
          -> handler mutates pUser[conn]/pMob[conn] and replies via SendFunc
Internal: server-originated events call ProcessClientMessage(idx, &sm, TRUE) (e.g. Server.cpp:5077)
```

---

## 3. Business Rules & Logic

The handler layer enforces the protocol; the rules below are those owned by the dispatch/core
itself. Per-domain rules (items, mobs, etc.) are documented in their own component reports.

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| Validation | Message `ID` must be within `[0, MAX_USER)`, else logged & dropped | `ProcessClientMessage.cpp:42-51` |
| Availability | If `ServerDown >= 120`, inbound messages are dropped | `ProcessClientMessage.cpp:53-54` |
| Liveness | Each message updates `pUser[conn].LastReceiveTime` | `ProcessClientMessage.cpp:56-57` |
| Protocol | `_MSG_Ping` is ignored | `ProcessClientMessage.cpp:59-60` |
| Internal bypass | `SKIPCHECKTICK` from non-server source short-circuits | `ProcessClientMessage.cpp:63-64` |
| Anti-cheat | Client-sent `_MSG_UpdateScore` triggers AddCrackError | `ProcessClientMessage.cpp:106-110` |
| Anti-cheat | `AddCrackError(conn,val,Type)` accumulates and force-logs-out | `Server.cpp:998-1019` |
| Routing | ~60 message types routed to `Exec_MSG_*` handlers | `ProcessClientMessage.cpp:66-311` |
| DB link | DBSrv at port 7514; auto-reconnect on error | `Server.cpp:4017,4282-4317` |

### Detailed breakdown

---

### Business Rule: Inbound Message Gating

**Overview**: Before any handler runs, the dispatcher applies universal validity and liveness
checks.

**Detailed description**: `ProcessClientMessage` first casts the buffer to `MSG_STANDARD` and
verifies `std->ID` is a legal user slot `[0, MAX_USER)`; out-of-range ids are logged with their
type/size/keyword and discarded (`ProcessClientMessage.cpp:42-51`), which guards every handler
against indexing `pUser[]`/`pMob[]` out of bounds. It then drops all traffic when the server is
shutting down (`ServerDown >= 120`, `:53`), stamps `LastReceiveTime` for idle/timeout tracking
(`:56-57`), ignores `_MSG_Ping` (`:59`), and honors an internal `SKIPCHECKTICK` marker that
prevents re-processing of self-injected packets when `isServer == FALSE` (`:63-64`). Only after
these checks does it enter the dispatch switch.

**Rule workflow**: receive → validate ID → check ServerDown → update liveness → drop ping/self-marker → dispatch.

---

### Business Rule: Anti-Cheat at the Dispatch Boundary ("Cra Points")

**Overview**: The dispatcher rejects messages a legitimate client should never send and assigns
penalty points that can disconnect abusers.

**Detailed description**: Score is server-authoritative, so a client emitting `_MSG_UpdateScore`
is by definition cheating; the dispatcher logs it and calls `AddCrackError(conn, 2, 91)`
(`ProcessClientMessage.cpp:106-110`). `AddCrackError(int conn, int val, int Type)`
(`Server.cpp:998`) logs the event (except for a few benign types), adds `val` to
`pUser[conn].NumError`, and when the accumulated total reaches the threshold (`>= 2000000000`,
`Server.cpp:1008`) it sends `_NN_Bad_Network_Packets`, calls `CharLogOut(conn)`, and stops
processing (`Server.cpp:1010-1018`). A periodic timer resets `NumError`, so the rule limits the
*rate* of violations. This is the principal server-side defense against malformed-packet abuse.

**Rule workflow**: illegal/malformed message → AddCrackError(val) → NumError += val → over threshold → notify + logout.

---

### Business Rule: Message Routing (Protocol Surface)

**Overview**: Each message type maps to exactly one handler; related variants share a handler.

**Detailed description**: The `switch(std->Type)` (`ProcessClientMessage.cpp:66`) routes ~60
types. Several action/attack variants fold into one handler (`_MSG_Action/Action2/Action3` →
`Exec_MSG_Action`, `:96-100`; `_MSG_Attack/AttackOne/AttackTwo` → `Exec_MSG_Attack`, `:192-196`;
`_MSG_CombineItemOdin/Odin2` → one handler, `:266-269`). The themes are: session
(login/logout/create/delete/secure), movement (`Motion`, `Action`), combat (`Attack`,
`Challange`), chat (`MessageChat`, `MessageWhisper`), trade (`Trade`, `TradingItem`,
`ReqTradeList`, `QuitTrade`, `SendAutoTrade`), item (`UseItem`, `GetItem`, `DropItem`,
`UpdateItem`, `SplitItem`, `DeleteItem`, `ApplyBonus`, `CombineItem*` x10), shop/storage
(`REQShopList`, `ReqBuy`, `Buy`, `Sell`, `Deposit`, `Withdraw`, `CapsuleInfo`), party
(`SendReqParty`, `AcceptParty`, `RemoveParty`), guild/war (`InviteGuild`, `GuildAlly`, `War`),
and world (`ReqTeleport`, `ChangeCity`, `Quest`, `ReqRanking`, `PKMode`, `SetShortSkill`,
`PutoutSeal`). The server-side DB protocol is handled separately by `ProcessDBMessage`.

**Rule workflow**: type → switch case → Exec_MSG_* handler.

---

### Business Rule: DBSrv Link Management

**Overview**: TMSrv maintains a persistent connection to DBSrv and recovers it on failure.

**Detailed description**: At boot `DBServerSocket.ConnectServer(DBServerAddress, 7514, ...,
WSA_READDB)` establishes the DB link (`Server.cpp:4017`). In `MainWndProc`'s `WSA_READDB` case the
server drains DB replies and dispatches them via `ProcessDBMessage` (`Server.cpp:4344-4369`); on a
socket error it re-issues `ConnectServer` to reconnect (`Server.cpp:4282-4317`). Because the game
server cannot authenticate logins or persist characters without DBSrv, this link is a hard
dependency.

**Rule workflow**: boot → connect DB:7514 → on WSA_READDB read & dispatch → on error reconnect.

---

## 4. Component Structure

```
TMSrv/Server.cpp (10553)
  ├── globals/state (ServerDown, counters, pUser[], pMob[], pItem[], sockets)
  ├── WinMain (3910)            # boot: data load, DB connect, listen, message loop
  ├── MainWndProc (4173)        # reactor: WSA_ACCEPT/READ/READDB/READBILL
  ├── AddCrackError (998)       # anti-cheat penalty accumulation
  ├── timers (ProcessSecTimer/MinTimer), score/score-send, world processing
TMSrv/ProcessClientMessage.cpp  # client dispatch switch -> Exec_MSG_*
TMSrv/ProcessDBMessage.cpp      # DBSrv reply dispatch
TMSrv/_MSG_*.cpp (58 files)     # one Exec_MSG_* handler per protocol message
TMSrv/SendFunc.* / GetFunc.*    # outbound message builders / inbound accessors
TMSrv/CReadFiles.*              # static game-data file loaders (startup)
```

## 5. Dependency Analysis

```
Internal:
  ProcessClientMessage ──> Exec_MSG_* handlers ──> CUser/pUser[], CMob/pMob[], CItem/pItem[]
  Core ──> CPSock (client + DB + bill sockets), SendFunc/GetFunc, CReadFiles, Basedef
  Core ──> ProcessDBMessage (DBSrv replies)
External:
  - WinSock (ws2_32): listen/accept/connect/recv/send, WSAAsyncSelect notifications
  - Windows API: window/message pump (MainWndProc), GDI for the server GUI
  - C runtime / filesystem: data files, logs
```

## 6. Afferent and Efferent Coupling

Afferent = units depending on the core; efferent = what the core depends on. TMSrv-Core has very
high efferent coupling (it touches every subsystem) and the dispatcher is the single funnel all
client traffic passes through.

| Unit | Afferent (est.) | Efferent (est.) | Critical |
|------|-----------------|-----------------|----------|
| ProcessClientMessage (dispatch) | Very High (every client packet) | 58 Exec_MSG_* handlers | High |
| MainWndProc (reactor) | Entry (WinSock events) | CPSock, ProcessClient/DBMessage, AcceptUser | High |
| Server.cpp globals/state | Very High (all handlers read pUser/pMob/pItem) | Basedef, CPSock | High |
| AddCrackError | High (handlers report violations) | CUser, logging | Medium |

## 7. Endpoints (client protocol surface)

Dispatched in `ProcessClientMessage` `switch(std->Type)` (`ProcessClientMessage.cpp:66-311`).
Direction is client→server unless noted; server-originated events reuse the same path with
`isServer = TRUE`.

| Theme | Message types | Handler |
|-------|---------------|---------|
| Session | AccountLogin, CharacterLogin, CharacterLogout, CreateCharacter, DeleteCharacter, AccountSecure | Exec_MSG_* (login/char) |
| Movement | Motion, Action/Action2/Action3 | Exec_MSG_Motion / Exec_MSG_Action |
| Combat | Attack/AttackOne/AttackTwo, Challange, ChallangeConfirm | Exec_MSG_Attack / Challange |
| Chat | MessageChat, MessageWhisper | Exec_MSG_MessageChat / Whisper |
| Item | UseItem, GetItem, DropItem, UpdateItem, SplitItem, DeleteItem, ApplyBonus, CombineItem(+Ehre/Tiny/Shany/Ailyn/Agatha/Odin/Odin2/Lindy/Alquimia/Extracao) | Exec_MSG_* (item/combine) |
| Trade | Trade, TradingItem, ReqTradeList, QuitTrade, SendAutoTrade | Exec_MSG_* (trade) |
| Shop/Storage | REQShopList, ReqBuy, Buy, Sell, Deposit, Withdraw, CapsuleInfo, PutoutSeal | Exec_MSG_* (shop/storage) |
| Party | SendReqParty, AcceptParty, RemoveParty | Exec_MSG_* (party) |
| Guild/War | InviteGuild, GuildAlly, War | Exec_MSG_* (guild/war) |
| World/Misc | ReqTeleport, ChangeCity, Quest, ReqRanking, PKMode, SetShortSkill, NoViewMob, Restart, Deprivate, Ping | Exec_MSG_* / dropped |
| Anti-cheat | UpdateScore (client-sent) | AddCrackError penalty (not a handler) |

DB-side protocol (TMSrv↔DBSrv) is routed by `ProcessDBMessage` and mirrors the DBSrv handlers
documented in the DBSrv component report.

## 8. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| Game client | TCP listen (GAME_PORT) | Player commands | CPSock binary | message structs | ID/ServerDown gates, AddCrackError |
| DBSrv | TCP connect (:7514) | Auth, persistence, ranking, guild | CPSock binary | MSG_* structs | auto-reconnect on error |
| Billing | TCP (WSA_READBILL) | Cash-shop (BISrv stubbed) | CPSock binary | n/a | none functional |
| Data files | Filesystem | Static world/item/mob/config | File IO | binary/text | CReadFiles at startup |

## 9. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| Reactor (single-thread) | MainWndProc + WSAAsyncSelect | `Server.cpp:4173` | Non-blocking multiplexed IO |
| Command dispatch (big switch) | `switch(std->Type)` -> Exec_MSG_* | `ProcessClientMessage.cpp:66` | Route messages to handlers |
| Handler-per-message | 58 `_MSG_*.cpp` files | TMSrv/ | One Exec function per protocol op |
| Object pools | pUser[]/pMob[]/pItem[] | `Server.cpp` | Pre-allocated entity arrays |
| Self-message reinjection | `ProcessClientMessage(idx, &sm, TRUE)` | `Server.cpp:5077,5517,...` | Server-originated events reuse client path |

## 10. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| Critical | SPOF | Single-threaded reactor handles all players | Any blocking op stalls the whole shard |
| High | Trust boundary | All handlers trust client-supplied slot/pos fields after a single ID check | Exploit surface depends on each handler validating |
| High | Size/complexity | `Server.cpp` is 10.5k lines with global mutable state | Hard to reason about; change-risk |
| Medium | Anti-cheat coverage | Penalties only where handlers call AddCrackError | Gaps where a handler omits checks |
| Medium | Availability | Hard dependency on DBSrv link for core flows | DBSrv outage blocks login/save |

## 11. Test Coverage Analysis

No automated tests exist for TMSrv-Core or any module in the repository (no test project,
framework, or `*test*` sources). The dispatcher, reactor, and 58 handlers — the project's entire
client trust boundary — are validated only at runtime. Recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| ProcessClientMessage / Server / handlers | 0 | 0 | None | No tests present in repo |
