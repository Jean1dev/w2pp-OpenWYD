# Component Deep Analysis Report: TMSrv-CUser

**Component**: TMSrv-CUser (Player Session / Connection Entity)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/TMSrv/CUser.cpp` (91 lines), `Source/Code/TMSrv/CUser.h` (140 lines), with cross-references into `Server.cpp`

---

## 1. Executive Summary

`CUser` in TMSrv represents one connected player slot: its socket, its session state-machine
position, the selected character's in-game data, personal storage, trade state, and a set of
anti-cheat / anti-flood counters. The game server keeps a fixed array `pUser[MAX_USER]` of these
objects; a connection index (`conn`) is the player's handle throughout the server.

The class itself is thin on behavior — its only methods are `AcceptUser` and `CloseUser`
(`CUser.cpp:51,81`). It is primarily a **data aggregate**: the authoritative character record is
the embedded `STRUCT_SELCHAR SelChar` (`CUser.h:92`), personal warehouse items live in
`Cargo[MAX_CARGO]` (`CUser.h:65`), and dozens of timing/anti-cheat fields (`LastAttackTick`,
`AttackTime`, `PotionTime`, `UseItemTime`, `LastMove`, `NumError`) gate client actions. The
character combat/stat formulas themselves are not in this file; they live in the mob/skill code
(`CMob.cpp`, `MobKilled.cpp`) and `Server.cpp`, which read and mutate `SelChar` on behalf of the
player. This component is therefore best understood as the session container plus its lifecycle
and anti-abuse rules.

A notable rule is the "cra point" (crack/cheat point) mechanism: protocol violations accumulate
into `NumError`, and once the accumulated value crosses a threshold the player is force-logged
out (`Server.cpp:1006-1018`).

---

## 2. Data Flow Analysis

```
1. Listen socket FD_ACCEPT -> CUser::AcceptUser(ListenSocket) (CUser.cpp:51)
   - accept(), WSAAsyncSelect(FD_READ|FD_CLOSE), Mode = USER_ACCEPT
2. Client login messages drive Mode: ACCEPT -> LOGIN -> SELCHAR -> CHARWAIT -> PLAY
   (DBSrv round-trips populate SelChar during SELCHAR/CHARWAIT)
3. While Mode == USER_PLAY, _MSG_* handlers read/write this CUser via pUser[conn]:
   - movement updates LastMove; attacks gated by AttackTime/LastAttackTick
   - item use gated by UseItemTime/PotionTime; trades use Trade/AutoTrade state
4. Protocol violations call the cra-point accumulator -> NumError += val (Server.cpp:1006)
5. Quit/disconnect: Mode -> USER_SAVING4QUIT (save to DBSrv) -> CloseUser (CUser.cpp:81)
```

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| State machine | Session progresses EMPTY→ACCEPT→LOGIN→SELCHAR→CHARWAIT→PLAY→SAVING4QUIT | `CUser.h:26-37` |
| Lifecycle | New connection set to USER_ACCEPT on accept | `CUser.cpp:75` |
| Lifecycle | CloseUser resets slot to USER_EMPTY and clears AccountName | `CUser.cpp:87-89` |
| Anti-cheat | "cra point" accumulation; force logout at threshold | `Server.cpp:1006-1018` |
| Anti-cheat | NumError reset periodically (per-timer) | `Server.cpp:9645` |
| Action gating | Attack/move/item timestamps throttle client actions | `CUser.h:84-89,125-129` |
| Access control | `Admin` flag distinguishes GM sessions | `CUser.h:114` |
| Economy | Per-session `Coin` and `Donate` balances held on the session | `CUser.h:66,121` |
| Storage | Personal cargo/warehouse `Cargo[MAX_CARGO]` | `CUser.h:65` |
| Trade | Trade / AutoTrade sub-state with flags and timer | `CUser.h:46-48,74,80` |

### Detailed breakdown

---

### Business Rule: Session State Machine

**Overview**: Each connection advances through a defined set of `Mode` values that determine
which messages are valid at any time.

**Detailed description**: The modes are defined in `CUser.h:26-37`: `USER_EMPTY` (free slot),
`USER_ACCEPT` (socket accepted, pre-login), `USER_LOGIN` (account authenticated), `USER_SELCHAR`
(waiting for DBSrv to send the character-selection struct), `USER_CHARWAIT` / `USER_WAITDB`
(waiting DB confirmation of character entry), `USER_PLAY` (in world), and `USER_SAVING4QUIT`
(persisting before disconnect). The bulk of gameplay handlers guard on `Mode == USER_PLAY`
(68 references across TMSrv `.cpp` files), ensuring that world actions are rejected unless the
player is fully in-game. `AcceptUser` sets `USER_ACCEPT` (`CUser.cpp:75`) and `CloseUser` returns
the slot to `USER_EMPTY` (`CUser.cpp:87`). The DBSrv login round-trips (`_MSG_DBAccountLogin`,
`_MSG_DBCharacterLogin`) drive the SELCHAR/CHARWAIT transitions and populate `SelChar`.

**Rule workflow**: accept → authenticate → select character (DB) → confirm (DB) → play → save → close.

---

### Business Rule: "Cra Point" Anti-Cheat Accumulation

**Overview**: Malformed or illegal client behavior accrues penalty points; enough points
disconnect the player.

**Detailed description**: A helper in `Server.cpp` adds a violation weight `val` to
`pUser[conn].NumError` (`Server.cpp:1006`). Most violations are also logged with the account name
and IP (with a few benign types excluded at `Server.cpp:1000`). When `NumError` reaches the
threshold (`>= 2000000000`, `Server.cpp:1008`), the server sends the client the
`_NN_Bad_Network_Packets` message, calls `CharLogOut(conn)`, logs the triggering packet type, and
ends processing (`Server.cpp:1010-1018`). A periodic server timer resets `NumError` to 0
(`Server.cpp:9645`), so the mechanism measures violation *rate* rather than lifetime count. This
is the primary in-server defense against malformed-packet flooding and protocol abuse.

**Rule workflow**: illegal packet → add cra points to NumError → if over threshold → notify + logout → periodic reset.

---

### Business Rule: Action Throttling Timestamps

**Overview**: Several timestamp fields rate-limit combat, movement, and item use to curb speed
hacks and spam.

**Detailed description**: `CUser` stores `LastAttack`/`LastAttackTick` (`CUser.h:84-85`),
`LastMove` (`:86`), `LastAction`/`LastActionTick` (`:87-88`), and the millisecond timers
`UseItemTime`, `AttackTime`, `PotionTime`, `LastClientTick` (`:125-129`). Handlers compare the
incoming client tick / current time against these to decide whether an action is allowed yet
(enforcement lives in `Server.cpp` and the `_MSG_*` handlers that own each action). Because the
decision uses values the server controls, the timing model is server-authoritative, though it
relies on each handler actually consulting the relevant field.

**Rule workflow**: action arrives → compare now/clientTick to stored last-time → allow & update, or reject (and possibly add cra points).

---

### Business Rule: Trade and AutoTrade State

**Overview**: Player-to-player trading and the AFK auto-shop are modeled as session sub-states.

**Detailed description**: `TradeMode` (`CUser.h:61`), the embedded `MSG_Trade Trade` (`:74`),
and `MSG_SendAutoTrade AutoTrade` (`:80`) plus `IsAutoTrading`, `ISTradTimer`, `ISTradFlag`
(`:46-48`) capture an in-progress trade or personal store. The trade lifecycle (offer, confirm,
lock, commit) is driven by the trade `_MSG_*` handlers, which mutate this state and move items
between `SelChar` inventory and the counterpart. The session also tracks `LojinhaTimer`
(personal-shop timer, `:132`).

**Rule workflow**: open trade/shop → set TradeMode/flags → exchange items on confirm → clear state.

---

### Business Rule: Privilege, Blocking, and Identity

**Overview**: The session carries access-control and identity fields used for GM powers, bans,
and multi-account/hardware tracking.

**Detailed description**: `Admin` (`CUser.h:114`) marks a GM session and gates admin-only
handlers. `BlockPass` / `IsBlocked` (`:57-58`) represent an account block/secondary password.
`Mac[4]` (`:109`) stores a client hardware identifier used for ban/duplicate detection, and
`PKMode` (`:105`) tracks the player-kill stance. `IsBillConnect` (`:94`) and the billing-related
`Unk_2728`/`Unk_2732` fields tie the session to the (currently stubbed) billing path.

**Rule workflow**: on login populate identity → handlers check Admin/IsBlocked → PK/billing state consulted during play.

---

## 4. Component Structure

```
TMSrv/CUser.h   (class CUser)
├── Identity/session: AccountName, Slot, IP, Mode, Admin, Mac[4], BlockPass/IsBlocked
├── Transport:        cSock (CPSock)
├── Character:        SelChar (STRUCT_SELCHAR) — authoritative in-world character
├── Inventory/econ:   Cargo[MAX_CARGO], Coin, Donate
├── Trade:            TradeMode, Trade, AutoTrade, IsAutoTrading/ISTrad* , LojinhaTimer
├── Action timing:    LastAttack(Tick), LastMove, LastAction(Tick), AttackTime,
│                     UseItemTime, PotionTime, LastClientTick
├── Anti-cheat:       NumError, NumError reset, LastReceiveTime
├── Chat/UI:          Whisper, Guildchat, PartyChat, Chatting, MuteChat, KingChat, LastChat
└── Methods:          AcceptUser(), CloseUser()
TMSrv/CUser.cpp  (ctor zeroing, AcceptUser, CloseUser)
```

## 5. Dependency Analysis

```
Internal:
  CUser ──> Basedef (STRUCT_SELCHAR, STRUCT_ITEM, MSG_Trade, MSG_SendAutoTrade, MAX_CARGO)
  CUser ──> CPSock (cSock transport)
  Server.cpp / _MSG_* handlers ──> CUser (pUser[conn]) for nearly all gameplay
External:
  - WinSock (accept, WSAAsyncSelect) via AcceptUser
```

## 6. Afferent and Efferent Coupling

Afferent = units depending on CUser; efferent = what CUser depends on. CUser has extremely high
afferent coupling because essentially every gameplay handler reads/writes `pUser[conn]`; its own
dependencies are limited to Basedef structs and CPSock.

| Unit | Afferent (est.) | Efferent (est.) | Critical |
|------|-----------------|-----------------|----------|
| CUser (class) | Very High (Server + 58 _MSG_ handlers) | Basedef, CPSock, WinSock | High |
| SelChar (embedded) | Very High (combat/skill/item code) | Basedef | High |
| Cargo storage | Medium (trade/warehouse handlers) | Basedef | Medium |

## 7. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| Game client | TCP | Player commands ↔ session | CPSock binary | message structs | cra-point accumulation, force logout |
| DBSrv | Internal | Account/char load & save tied to session | CPSock binary | STRUCT_ACCOUNTFILE/SELCHAR | Mode WAITDB gating |
| Billing | Internal (stub) | IsBillConnect / donate flags | CPSock binary | n/a (BISrv stubbed) | none functional |

## 8. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| State machine | `Mode` (USER_*) | `CUser.h:26-37` | Gate valid actions by session phase |
| Object pool (fixed array) | `pUser[MAX_USER]` | `Server.cpp` | Pre-allocated player slots |
| Data aggregate / "fat session" | CUser holds character, trade, timers | `CUser.h` | Single handle per connection |

## 9. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| High | Cohesion | CUser is a large multi-responsibility aggregate (session, character, trade, anti-cheat) | Change-coupling; any handler can mutate any field |
| Medium | Memory layout | Many `Unk*` reverse-engineered offset fields and fixed buffers | Fragile binary-compatibility assumptions |
| Medium | Anti-cheat | Throttling depends on each handler consulting timers consistently | Gaps where a handler skips checks |
| Low | Identity | Mac[] and BlockPass are client-supplied | Spoofable identity signals |

## 10. Test Coverage Analysis

No automated tests exist for TMSrv-CUser or any module in the repository (no test project,
framework, or `*test*` sources). Session-state and anti-cheat logic is validated only at runtime.
Recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| CUser | 0 | 0 | None | No tests present in repo |
