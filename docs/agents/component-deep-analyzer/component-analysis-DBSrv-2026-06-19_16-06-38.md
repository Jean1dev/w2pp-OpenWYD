# Component Deep Analysis Report: DBSrv

**Component**: DBSrv (Database / Account Server)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/DBSrv/` (analysis and reporting only)

---

## 1. Executive Summary

DBSrv is the persistence and account-authority server of the OpenWYD stack. It owns
all durable game state: player accounts, characters, guild metadata, the experience/grind
ranking, and item-day logs. It exposes no client-facing protocol; instead it speaks an
internal binary message protocol to TMSrv instances (and to an admin client), acting as
the single source of truth that TMSrv queries on login, character select, save, and quit.

Persistence is **file-based**, not an RDBMS. The class `CFileDB` (`Source/Code/DBSrv/CFileDB.cpp`,
2,712 lines) holds an in-memory array `pAccountList[MAX_DBACCOUNT]` of loaded accounts and
serializes `STRUCT_ACCOUNTFILE` records to/from disk via `DBReadAccount` / `DBWriteAccount`.
The server runs as a Win32 GUI process driven by `MainWndProc` (`Server.h:41`) and a
WinSock `WSAAsyncSelect` reactor; no worker threads were found, so it is single-threaded.

Key findings:
- **Authentication is plaintext.** Passwords are compared with `strcmp(file.Info.AccountPass, m->AccountPassword)` (`CFileDB.cpp:677`) and stored as a plain field in the account file.
- **The "encrypted password" feature is non-functional.** `GetEncPassword` returns eight random numbers and `FALSE` (`CFileDB.cpp:2696`); `SetEncPassword` has an empty body (`CFileDB.cpp:2710`).
- The protocol surface is a single `switch(std->Type)` (`CFileDB.cpp:240`) handling 28 message types.
- Concurrency safety relies entirely on the single-threaded reactor; there is no file locking or transactional write path observed.

---

## 2. Data Flow Analysis

```
1. TMSrv connects to DBSrv listen socket; CUser::AcceptUser accepts (CUser.cpp:39)
2. Inbound bytes assembled by CPSock; ProcessClientMessage(conn,msg) (Server.h:39)
3. Message routed to CFileDB::ProcessMessage(Msg, conn) (CFileDB.cpp:236)
4. switch(std->Type) dispatches to a per-message handler (CFileDB.cpp:240)
5. Account handlers call DBReadAccount/DBWriteAccount against on-disk account files
6. Reply built via SendDBSignal / SendDBSignalParm / SendDBMessage back to TMSrv
7. Periodic ProcessSecTimer / ProcessMinTimer drive logs and housekeeping (Server.h:48-49)
```

Login flow (`_MSG_DBAccountLogin`, `CFileDB.cpp:611`):
account name upper-cased → reserved device-name rejection (COM0-9 / LPT0-9) →
`DBReadAccount` → coin sanity clamp → account expiry check via `Info.Year`/`Info.YearDay` →
optional `TempKey` server-change path (memcmp) → plaintext password compare →
duplicate-session resolution (`_MSG_DBAlreadyPlaying` / `_MSG_DBStillPlaying` / `SendDBSavingQuit`).

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| Validation | Account name is upper-cased before lookup | `CFileDB.cpp:617` |
| Validation | Reserved device names (COM0-9, LPT0-9) rejected at login | `CFileDB.cpp:619-626` |
| Auth | Password compared in plaintext via `strcmp` | `CFileDB.cpp:677` |
| Auth | Delete-character requires matching plaintext password | `CFileDB.cpp:1327` |
| Business Logic | Account expiry by calendar Year/YearDay blocks login | `CFileDB.cpp:648-660` |
| Business Logic | Coin balance clamped to >= 0 on login | `CFileDB.cpp:646` |
| Business Logic | TempKey enables server-change without password | `CFileDB.cpp:663-674` |
| Concurrency | Duplicate active session forces save-and-quit of prior | `CFileDB.cpp:686-700+` |
| Security (stub) | `GetEncPassword` returns random ints, no real encryption | `CFileDB.cpp:2696` |
| Security (stub) | `SetEncPassword` is an empty no-op | `CFileDB.cpp:2710` |
| Data hygiene | Account password buffer last two bytes forced to 0 | `CFileDB.cpp:2547-2548` |

### Detailed breakdown

---

### Business Rule: Account Authentication (plaintext)

**Overview**: On `_MSG_DBAccountLogin`, DBSrv validates the supplied account name and
password against the on-disk account record before allowing TMSrv to proceed.

**Detailed description**: The handler upper-cases the inbound account name with `_strupr`
so lookups are case-insensitive, then rejects Windows reserved device names (`COM0`–`COM9`,
`LPT0`–`LPT9`) which could be abused as filenames in the file-based store. It loads the
record with `DBReadAccount`; a missing record yields `_MSG_DBAccountLoginFail_Account`.
The password itself is compared byte-for-byte with `strcmp(file.Info.AccountPass, m->AccountPassword)`
at `CFileDB.cpp:677`; a mismatch returns `_MSG_DBAccountLoginFail_Pass`. The stored field
`AccountPass` is a plain character buffer (`STRUCT_ACCOUNTFILE.Info.AccountPass`,
`ACCOUNTPASS_LENGTH`), i.e. credentials are persisted and transmitted without hashing.

The same plaintext check guards destructive operations: deleting a character
(`_MSG_DBDeleteCharacter`) requires `strncmp(m->Password, ...AccountPass, ACCOUNTPASS_LENGTH)`
to match (`CFileDB.cpp:1327`), otherwise the attempt is logged and rejected.

**Rule workflow**: receive login → upper-case name → reject reserved names → read account file
→ (optional TempKey server-change bypass) → `strcmp` password → success or `*Fail_Pass`.

---

### Business Rule: Account Expiry / Block by Date

**Overview**: Accounts carry an expiry expressed as `Info.Year` and `Info.YearDay`; logins
past that point are blocked.

**Detailed description**: When both `Info.Year` and `Info.YearDay` are non-zero, the handler
compares them against the current `localtime` `tm_year` / `tm_yday`. If the stored year is at
or beyond the current year (with the day-of-year tie-break), it returns
`_MSG_DBAccountLoginFail_Block` (`CFileDB.cpp:648-660`). This implements time-limited or
banned accounts. The comparison expression mixes `>=` and `&&`/`||` without parentheses, so
the effective predicate is `Year >= current_year`; this is documented here as an observation,
not a correctness claim.

**Rule workflow**: login → if expiry set → compare to now → block or continue.

---

### Business Rule: Server-Change via TempKey

**Overview**: A player moving between server channels can be re-authenticated by a one-time
key instead of a password.

**Detailed description**: If the account record holds a non-zero `TempKey` and the request
carries a matching `Zero` field, `memcmp` validates it (`CFileDB.cpp:663-674`); on match the
key is cleared and login jumps to `lb_sucess`, bypassing the password check entirely. On
mismatch the key is cleared and persisted, forcing a normal re-login. This is a
ticket-style hand-off between TMSrv channels.

**Rule workflow**: login with TempKey present → memcmp → clear key → success (bypass password) OR clear+persist+reject.

---

### Business Rule: Duplicate Session Resolution

**Overview**: Only one active session per account is permitted across TMSrv instances.

**Detailed description**: After authentication, DBSrv looks up whether the account is already
indexed as playing (`GetIndex`). If a different connection holds it, the server either replies
`_MSG_DBAlreadyPlaying` (when no save is requested) or `_MSG_DBStillPlaying` and triggers
`SendDBSavingQuit(IdxName, 0)` to force the prior session to save and disconnect
(`CFileDB.cpp:686-700+`). This prevents item duplication via parallel logins of one account.

**Rule workflow**: post-auth → check existing index → same conn ok → else force-save-quit prior.

---

### Business Rule: Non-functional Password Encryption (documented stub)

**Overview**: The codebase contains an "encrypted password" hand-off used during server
change, but the implementation does not encrypt anything.

**Detailed description**: `GetEncPassword(idx, Enc)` fills eight `Enc[]` slots with
`rand() % 900 + 100` and returns `FALSE` (`CFileDB.cpp:2696-2708`); `SetEncPassword` has an
empty body (`CFileDB.cpp:2710-2713`). The `SecurePass` flag on each account-list entry is set
and reset around these calls but the underlying transform is absent. This is recorded as a
factual observation about the current state of the feature, not a recommendation.

**Rule workflow**: server-change requests enc password → random numbers returned → no real cryptographic state stored.

---

## 4. Component Structure

```
Source/Code/DBSrv/
├── Server.cpp        # 2341 lines: Win32 host, MainWndProc reactor, admin protocol,
│                     #   ProcessClientMessage routing, config/log, timers
├── Server.h          # extern globals, function decls, MAX_SERVER user arrays
├── CFileDB.cpp       # 2712 lines: ProcessMessage switch (28 handlers), account CRUD,
│                     #   DBReadAccount/DBWriteAccount, guild/capsule, enc-password stubs
├── CFileDB.h         # CFileDB class + STRUCT_ACCOUNTLIST
├── CReadFiles.cpp    # 1313 lines: loads static game data files at startup
├── CRanking.cpp      # 392 lines: RankingSystem / GrindRanking implementation
├── CRanking.h        # ranking classes (grind/exp ranking, MAX_RANK_INDEX)
├── CUser.cpp         #   64 lines: connection entity, AcceptUser (WSAAsyncSelect)
├── CUser.h           # CUser connection-state class
├── resource.h        # Win32 resource ids
└── stdafx.h          # precompiled header
```

## 5. Dependency Analysis

```
Internal:
  CFileDB ──> Basedef structs (STRUCT_ACCOUNTFILE, STRUCT_ITEM, STRUCT_GUILDINFO, ...)
  CFileDB ──> CPSock (SendDBSignal/SendDBMessage write through socket layer)
  Server  ──> CFileDB (global cFileDB), CUser pUser[MAX_SERVER], CRanking rankingSystem
  CUser   ──> CPSock (cSock member), WinSock accept/WSAAsyncSelect

External:
  - WinSock (ws2_32) — TCP sockets, WSAAsyncSelect message-driven IO
  - Windows API (User32/GDI) — MainWndProc GUI host, fonts, HDC
  - C runtime / filesystem — account file read/write, logs
  No SQL/ODBC client is actually used; persistence is flat files.
```

## 6. Afferent and Efferent Coupling

Afferent coupling counts symbols/files that depend on a unit; efferent counts what the unit
depends on. Values below are estimates from `#include` graph and cross-file symbol usage
within `Source/Code/DBSrv/`.

| Component | Afferent (est.) | Efferent (est.) | Critical |
|-----------|-----------------|-----------------|----------|
| CFileDB | High (Server, all handlers route here) | Basedef, CPSock, filesystem | High |
| Server (host/reactor) | Entry point | CFileDB, CUser, CRanking | High |
| CUser (connection) | Server, CFileDB | CPSock, WinSock | Medium |
| RankingSystem | Server, CFileDB | Basedef, filesystem | Low |
| CReadFiles | Server (startup) | Basedef, filesystem | Low |

## 7. Endpoints (internal message protocol, TMSrv <-> DBSrv)

Dispatched in `CFileDB::ProcessMessage` `switch(std->Type)` (`CFileDB.cpp:240`):

| Handler (Type) | Purpose |
|----------------|---------|
| `_MSG_ReqTransper` | Transfer/transaction request |
| `_MSG_GuildZoneReport` | Guild-zone status report |
| `_MSG_War` | Guild war state |
| `_MSG_GuildAlly` | Guild alliance update |
| `_MSG_GuildInfo` | Guild metadata query/update |
| `_MSG_DBUpdateSapphire` | Sapphire (cash/event currency) update |
| `_MSG_DBNewAccount` | Create account |
| `_MSG_MessageDBRecord` | DB record message |
| `_MSG_NPAppeal` | NP/appeal handling |
| `_MSG_MessageDBImple` | GM/imple message |
| `_MSG_DBAccountLogin` | Account authentication |
| `_MSG_DBCreateCharacter` | Create character |
| `_MSG_DBCharacterLogin` | Character select/enter |
| `_MSG_DBNoNeedSave` | Mark session not needing save |
| `_MSG_DBSaveMob` | Persist mob/pet data |
| `_MSG_SavingQuit` | Save and disconnect |
| `_MSG_DBDeleteCharacter` | Delete character (password-gated) |
| `_MSG_AccountSecure` | Secure-account state |
| `_MSG_DBCreateArchCharacter` | Create special/arch character |
| `_MSG_MagicTrumpet` | Server-wide announce item |
| `_MSG_DBNotice` | Notice broadcast |
| `_MSG_DBCapsuleInfo` | Capsule (storage) info |
| `_MSG_DBPutInCapsule` | Deposit to capsule storage |
| `_MSG_DBOutCapsule` | Withdraw from capsule storage |
| `_MSG_DBServerChange` | Channel/server change (TempKey) |
| `_MSG_UpdateExpRanking` | Update experience ranking |
| `_MSG_DBItemDayLog` | Daily item logging |
| `_MSG_DBActivatePinCode` | Activate account PIN |
| `_MSG_DBPrimaryAccount` | Primary-account handling |

## 8. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| TMSrv link | Internal server | Account/char persistence, guild, ranking | TCP (WSAAsyncSelect) | CPSock binary messages | `_MSG_*Fail_*` reply signals |
| Admin client | Internal | GM/admin operations | TCP | CPSock binary | `ProcessAdminMessage` |
| Account files | Filesystem | Durable account/character store | File IO | `STRUCT_ACCOUNTFILE` binary | return-code checks (`DBReadAccount` ret) |
| Static data files | Filesystem | Base mob set, server list, configs | File IO | binary/text | loaded at startup via CReadFiles |
| Logs | Filesystem | Daily/exp/item logs | File IO | text | append-only |

## 9. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| Reactor (event loop) | `WSAAsyncSelect` + `MainWndProc` | `CUser.cpp:49`, `Server.h:41` | Single-thread async socket IO |
| Command dispatch | `switch(std->Type)` | `CFileDB.cpp:240` | Route binary messages to handlers |
| File-as-database | `DBReadAccount`/`DBWriteAccount`, `pAccountList[]` | `CFileDB.cpp` | Flat-file persistence with in-memory cache |
| Singleton-ish globals | `cFileDB`, `rankingSystem`, `pUser[]` | `Server.h:104-113` | Process-wide shared state |

## 10. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| Critical | Authentication | Passwords stored and compared in plaintext (`CFileDB.cpp:677`) | Credential disclosure on file or wire capture |
| High | Crypto stubs | `GetEncPassword`/`SetEncPassword` non-functional | "Secure password" provides no protection |
| High | Persistence | No file locking / transactional write observed | Risk of corruption or partial writes on crash |
| High | Concurrency | Single-threaded reactor is a system-wide SPOF | Any blocking file op stalls all servers |
| Medium | Input handling | Fixed-size account structs copied via memcpy/strncpy | Bounds depend on caller-supplied lengths |
| Medium | Logic clarity | Mixed `>=`/`&&`/`||` without parentheses in expiry check (`CFileDB.cpp:654`) | Predicate may not match intent |

## 11. Test Coverage Analysis

No automated test files (unit, integration, or fixtures) were located anywhere in the
repository for DBSrv or any other module — there is no test project in the solution, no
test framework dependency, and no `*test*` source files. All validation of DBSrv behavior
is manual/runtime. This absence of any test harness is recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| CFileDB | 0 | 0 | None | No tests present in repo |
| RankingSystem | 0 | 0 | None | No tests present in repo |
| CUser / Server | 0 | 0 | None | No tests present in repo |
