# Architectural Analysis Report — w2pp-OpenWYD

Date: 2026-06-19
Scope: project root, source under `Source/Code`. Compiled binaries, archives (`*.rar`), `.git`, `.vs`, `project-analizer`, `enc_temp_folder`, and `serverlist bin` editor binaries were excluded from source analysis (runtime configuration text under `Release/` and `serverlist bin/` was inspected for the Infrastructure section).

---

## 1. Executive Summary

w2pp-OpenWYD is an open-source server emulator for the MMORPG *With Your Destiny* (WYD), a fork of the "Cavaleiros de Kersef" (WYD cdk) project. It is written in C++ targeting Windows and built with MSVC (Visual Studio, PlatformToolset `v143`, Windows SDK `10.0`, `CharacterSet MultiByte`). The system is a classic three-tier WYD server cluster composed of three networked Windows processes plus a set of standalone data-editor utility tools.

System architecture (as observed in source):

- **TMSrv** (`Source/Code/TMSrv/`, ~87 files) — the main game/world server. It hosts all in-world gameplay: characters, mobs, items, skills, trade, guild, war/castle, NPC generation, and the bulk of the client protocol handlers (58 `_MSG_*.cpp` files). It is by far the largest component (`Server.cpp` alone is ~10,553 lines; `Basedef.cpp` ~7,468 lines).
- **DBSrv** (`Source/Code/DBSrv/`, ~12 files) — the database/account server. It owns account authentication, account/character persistence (binary account files), ranking, and guild transfer logic. TMSrv delegates account login and persistence to DBSrv over an internal TCP link.
- **BISrv** (`Source/Code/BISrv/`, ~7 files) — the billing/item ("Bill") server. It processes a fixed-size 196-byte authorization packet (`_AUTH_GAME`, `g_cGame = sizeof(_AUTH_GAME)`) used for the billing/cash-shop path.
- **Shared base layer** — `Source/Code/Basedef.h` / `Basedef.cpp` (data structures, constants, game tables) and `Source/Code/CPSock.h` / `CPSock.cpp` (the packet/socket layer with the WYD packet header and keyword-table encoding). `Source/Code/ItemEffect.h` carries shared item-effect definitions.

Key technology findings:

- Networking uses **WinSock (`<WinSock.h>`) with `WSAAsyncSelect`** event-driven, Windows-message-pumped sockets (confirmed in `TMSrv/CUser.cpp:61` and `DBSrv/CUser.cpp:49`). No `CreateThread`/`std::thread`/`_beginthread` calls were found in TMSrv/DBSrv/BISrv, indicating a **single-threaded, message-loop-driven** I/O model per server.
- The packet layer (`CPSock.cpp`) implements a custom WYD packet **header** (`HEADER`: `Size`, `KeyWord`, `CheckSum`, `Type`, `ID`, `ClientTick`) and a **keyword-table byte transform** using a hardcoded 512-byte `pKeyWord` table ("7.xx keys"). The decode routine, on checksum mismatch, sets an error code **but still returns the packet** (`CPSock.cpp:458-464`).
- Persistence is **flat-file / binary-file based**, not a relational DBMS. Accounts are stored in `STRUCT_ACCOUNTFILE` binary records (`Basedef.h:1085`) containing a plaintext-field `AccountPass` (`STRUCT_ACCOUNTINFO.AccountPass`, `Basedef.h:1020`). Numerous `.txt`/`.csv`/`.dat`/`.bin` data files under `Release/` drive game content and configuration.
- The Visual Studio solution `Source/Cavaleiros de Kersef.sln` contains **only DBSrv, TMSrv, and ClientPatch_v7662** (plus solution/folder containers). **BISrv and all utility editor projects are NOT members of the main solution** and exist as standalone `.vcxproj` files.

Most architecturally significant risks are inherent to this server lineage: a single hardcoded packet key table shared client/server, checksum failures not rejecting packets, plaintext-stored account passwords in binary files, hardcoded IP addresses in runtime config and source-adjacent data, and a single-threaded per-process model that makes each server a single point of failure for its tier.

---

## 2. System Overview

### 2.1 Annotated structure tree (source)

```
Source/
├── Cavaleiros de Kersef.sln          # Solution: contains DBSrv, TMSrv, ClientPatch_v7662 only
└── Code/
    ├── Basedef.h / Basedef.cpp        # SHARED: game data structures, constants, tables (~7.5k LOC cpp)
    ├── CPSock.h / CPSock.cpp          # SHARED: packet header + socket/encoding layer (keyword table)
    ├── ItemEffect.h                   # SHARED: item-effect definitions
    │
    ├── TMSrv/                          # MAIN GAME SERVER (~87 files)
    │   ├── Server.cpp / Server.h       #   core server state + globals (~10.5k LOC)
    │   ├── imple.cpp                    #   GM/admin command implementation (ProcessImple)
    │   ├── CUser.cpp / CUser.h          #   per-connection user object; socket accept (WSAAsyncSelect)
    │   ├── CItem.cpp / CItem.h          #   item subsystem
    │   ├── CMob.cpp / CMob.h            #   mob/NPC entity subsystem
    │   ├── CNPCGene.cpp / CNPCGene.h    #   NPC generation
    │   ├── CCastleZakum.* / CWarTower.* #   castle/war/tower (GvG, RvR) subsystems
    │   ├── MobKilled.cpp                #   mob-death / drop handling
    │   ├── CReadFiles.cpp / .h          #   game data-file loaders
    │   ├── GetFunc.* / SendFunc.*       #   read accessors / client send helpers
    │   ├── ProcessClientMessage.* (.h)  #   client packet dispatch (included by 51 files)
    │   ├── ProcessDBMessage.* (.h)      #   DBSrv->TMSrv message handling
    │   ├── ProcessSecMinTimer.cpp       #   periodic (sec/min) timer logic
    │   ├── Language.h                   #   localized strings
    │   └── _MSG_*.cpp (58 files)        #   one handler per client protocol message
    │
    ├── DBSrv/                          # DATABASE / ACCOUNT SERVER (~12 files)
    │   ├── Server.cpp / Server.h        #   server core, admin link, guild transfer
    │   ├── CFileDB.cpp / CFileDB.h      #   account file DB, password enc accessors (~2.7k LOC)
    │   ├── CUser.cpp / CUser.h          #   per-connection object (WSAAsyncSelect accept)
    │   ├── CRanking.cpp / CRanking.h    #   ranking generation (reads STRUCT_ACCOUNTFILE)
    │   ├── CReadFiles.cpp / CReadFiles.h #   data-file loaders
    │   └── stdafx.h
    │
    ├── BISrv/                          # BILLING / ITEM SERVER (~7 files)
    │   ├── Main.cpp / Main.h            #   dialog-based app entry
    │   ├── ProcessMessage.cpp / .h      #   billing message dispatch (HEADER-based)
    │   └── CUser.cpp / CUser.h          #   per-connection object
    │
    ├── ClientPatch_v7662/              # client-side patch DLL (PE hook into game client)
    │   ├── main.cpp / Main.h, Hook.cpp, Functions.cpp, PE_Hook.h
    │
    └── (standalone utility editors — NOT in main .sln)
        ├── NPTool/        (CUser, ProcessMessage, Main)   # NP/cash tool (server-talking)
        ├── DropTool/      (main)                          # drop-table editor
        ├── ExpTool/       (main)                          # exp-table editor
        ├── EDITAPPMOB/    (File, Main)                     # mob data editor
        ├── EDITAPPSHOP/   (File, Main)                     # shop data editor
        ├── AttributeMap_Editor/ (main)                     # attribute map editor
        ├── ZerarSkill/    (main)                          # skill-reset tool
        └── SearchPass/    (main)                          # account password search tool
```

### 2.2 Architectural patterns identified

- **Multi-process server cluster (tiered):** Game logic (TMSrv), account/persistence (DBSrv), and billing (BISrv) are separate executables connected by internal TCP links. This is the canonical WYD topology.
- **Event-driven reactor over Windows messages:** Each server uses WinSock `WSAAsyncSelect` to deliver socket events (`FD_READ | FD_CLOSE`) to a window procedure (custom `WSA_*` messages defined in `CPSock.h`, e.g. `WSA_READ`, `WSA_READDB`, `WSA_ACCEPT`, `WSA_READBILL`). A single message loop processes them — effectively a single-threaded reactor.
- **Message-handler dispatch table:** Client packets are routed by `Type` from `HEADER` into per-message handlers. TMSrv splits these into 58 separate `_MSG_*.cpp` files, each defining one `Exec_MSG_*` function; `ProcessClientMessage.h` is the shared dispatch/contract header (included by 51 files).
- **Shared "fat header" base layer:** `Basedef.h` is a very large central header defining nearly all game structs/constants; it is included by 16 source files across all three servers, creating high inbound coupling at the base.
- **Connection-object pattern:** Each tier wraps a connection in a `CUser` class that owns a `CPSock`. The `CPSock` class encapsulates buffered send/recv plus the keyword-table encode/decode.
- **File-as-database persistence:** No SQL engine; binary account records and text/CSV content files act as the data store (`Release/.../*.txt`, `*.csv`, `*.bin`, `*.dat`).

---

## 3. Critical Components Analysis

**Coupling definitions and method.** *Afferent coupling (Ca, incoming)* counts how many other components depend on a component (here: how many source files `#include` its header or reference its globals/types). *Efferent coupling (Ce, outgoing)* counts how many other components a component itself depends on (its own `#include`s and the external symbols it uses). All numbers below are **best-effort estimates** derived from counting `#include` directives and cross-file symbol references via `grep` over `.cpp`/`.h` files in `TMSrv/`, `DBSrv/`, `BISrv/`, and the shared `Source/Code/` root. They indicate relative magnitude, not exact compile-time edges. Measured anchors used: `Basedef.h` is included by 16 files; `CPSock.h` by 6; `ItemEffect.h` by 3; `ProcessClientMessage.h` by 51 files within TMSrv; there are 58 `_MSG_*.cpp` handlers.

| Component | Type | Location (relative) | Afferent Coupling (est.) | Efferent Coupling (est.) | Architectural Role |
|---|---|---|---|---|---|
| Basedef | Shared base library (header + impl) | `Source/Code/Basedef.h`, `Source/Code/Basedef.cpp` | Very High (~16+ files include it; nearly all structs/constants originate here) | Medium (Windows API, WinSock, STL) | Central data-model and constants for the whole cluster; defines `HEADER` consumers, `STRUCT_ACCOUNTFILE`, mob/item/quest structs, game tables |
| CPSock (packet/socket layer) | Shared networking library | `Source/Code/CPSock.h`, `Source/Code/CPSock.cpp` | High (~6 files include header; every server's `CUser` owns a `CPSock`) | Low–Medium (WinSock, `Basedef.h`) | Defines wire `HEADER`, keyword-table encode/decode, checksum, buffered send/recv; foundation of all inter-process and client traffic |
| ItemEffect | Shared definitions header | `Source/Code/ItemEffect.h` | Low–Medium (~3 files) | Low | Item-effect constants shared between item handling code |
| TMSrv Core (Server) | Game-server core module | `Source/Code/TMSrv/Server.cpp`, `Server.h` | Very High (hub for all TMSrv subsystems and `_MSG_*` handlers; declares global game state) | Very High (includes Basedef, CPSock, ItemEffect, CItem, CMob, CUser, CNPCGene, GetFunc, SendFunc, ProcessClientMessage, ProcessDBMessage, CReadFiles, CWarTower) | Owns global world state, server loop, timers, and inter-server sockets (`DBServerSocket`, `BillServerSocket`) |
| TMSrv User (CUser) | Connection/session object | `Source/Code/TMSrv/CUser.cpp`, `CUser.h` | High (referenced across handlers and Server) | Medium (CPSock, Basedef, Server globals) | Per-client session; accepts sockets via `WSAAsyncSelect`; holds account/IP/MAC and play state |
| TMSrv Item subsystem (CItem) | Game subsystem | `Source/Code/TMSrv/CItem.cpp`, `CItem.h` | High (~5+ includers; used by item/trade/combine handlers) | Medium | Item creation, validation, manipulation |
| TMSrv Mob subsystem (CMob) | Game subsystem | `Source/Code/TMSrv/CMob.cpp`, `CMob.h` | High (~4+ includers) | Medium | Mob/NPC entities, used by attack/spawn/kill flows |
| TMSrv NPC generation (CNPCGene) | Game subsystem | `Source/Code/TMSrv/CNPCGene.cpp`, `CNPCGene.h` | Medium | Medium | NPC/mob spawn generation |
| TMSrv Mob death/drops (MobKilled) | Game subsystem | `Source/Code/TMSrv/MobKilled.cpp` | Medium | High (CItem, CMob, drop tables) | Mob-kill rewards, EXP, item drops |
| TMSrv Castle/War (CCastleZakum, CWarTower) | Game subsystem | `Source/Code/TMSrv/CCastleZakum.*`, `CWarTower.*` | Medium | High | Castle siege / war-tower (GvG/RvR) event logic |
| TMSrv Client dispatch (ProcessClientMessage) | Protocol dispatch | `Source/Code/TMSrv/ProcessClientMessage.cpp`, `.h` | Very High (header included by 51 files) | High | Routes inbound client packets to `_MSG_*` handlers |
| TMSrv DB dispatch (ProcessDBMessage) | Protocol dispatch | `Source/Code/TMSrv/ProcessDBMessage.cpp`, `.h` | High | High | Handles DBSrv->TMSrv responses (login result, char data, persistence) |
| TMSrv Message handlers (`_MSG_*`) | Protocol handler set | `Source/Code/TMSrv/_MSG_*.cpp` (58 files) | Low individually | High (each depends on ProcessClientMessage.h + subsystems) | Implement individual client operations: login, trade, combine, guild, war, shop, party, item, attack, etc. |
| TMSrv GM/admin (imple) | Admin/command module | `Source/Code/TMSrv/imple.cpp` | Medium | High (Server, CItem, CCastleZakum, SendFunc) | `ProcessImple` GM command interpreter and `SaveAll` |
| TMSrv Send/Get helpers (SendFunc, GetFunc) | Utility modules | `Source/Code/TMSrv/SendFunc.*`, `GetFunc.*` | High (used by most handlers) | Medium | Outbound client message builders / state accessors |
| TMSrv File loaders (CReadFiles) | Data-loading module | `Source/Code/TMSrv/CReadFiles.cpp`, `.h` | Medium | High (reads `Release/.../*.csv/.txt/.dat`) | Loads item/mob/config content from data files at startup |
| DBSrv Core (Server) | Account-server core | `Source/Code/DBSrv/Server.cpp`, `Server.h` | High (hub for DBSrv) | High (CFileDB, CRanking, CReadFiles, CUser, Basedef) | Account-server loop, admin link, guild transfer, config R/W |
| DBSrv File DB (CFileDB) | Persistence module | `Source/Code/DBSrv/CFileDB.cpp`, `CFileDB.h` | High | High (Basedef structs, file I/O) | Account list management, encoded-password accessors (`GetEncPassword`/`SetEncPassword`), account/char load-save |
| DBSrv Ranking (CRanking) | Reporting module | `Source/Code/DBSrv/CRanking.cpp`, `CRanking.h` | Medium | High (reads `STRUCT_ACCOUNTFILE` via `fopen`/`fread`) | Builds rankings by scanning account files |
| DBSrv User (CUser) | Connection object | `Source/Code/DBSrv/CUser.cpp`, `CUser.h` | Medium | Medium (CPSock, Basedef) | Per-connection session for DBSrv (accept via `WSAAsyncSelect`) |
| BISrv Core (Main) | Billing-server entry | `Source/Code/BISrv/Main.cpp`, `Main.h` | Medium | Medium | Dialog-based billing app; window/UI + message pump |
| BISrv Message processing (ProcessMessage) | Protocol dispatch | `Source/Code/BISrv/ProcessMessage.cpp`, `.h` | Medium | Medium (HEADER from CPSock, Basedef) | Parses billing packets (`HEADER`-prefixed; 196-byte `_AUTH_GAME`) |
| BISrv User (CUser) | Connection object | `Source/Code/BISrv/CUser.cpp`, `CUser.h` | Medium | Medium | Per-connection billing session |
| ClientPatch_v7662 | Client-side patch DLL | `Source/Code/ClientPatch_v7662/` | N/A (client-side) | Medium (PE hooking, Windows API) | Hooks/patches the game client executable (`PE_Hook.h`, `Hook.cpp`) |

> Note: For `_MSG_*.cpp`, afferent coupling is low per-file (each is a leaf invoked by the dispatcher), but collectively they form the largest surface of TMSrv. The exact symbol-reference counts could not be fully enumerated by `grep` alone; values are estimates.

---

## 4. Dependency Mapping

### 4.1 Server / shared-layer dependency and data flow (ASCII)

```
                         +-------------------------------+
                         |        Shared base layer       |
                         |  Basedef.h/.cpp  CPSock.h/.cpp  |
                         |        ItemEffect.h            |
                         +---------------+---------------+
                                         ^  (compiled into each server: structs, HEADER,
                                         |   keyword-table encode/decode, checksum)
            +----------------------------+----------------------------+
            |                            |                            |
   +--------+--------+          +--------+--------+          +--------+--------+
   |     TMSrv       |   TCP    |     DBSrv       |          |     BISrv       |
   | (game world)    | <======> | (account/files) |          | (billing/item)  |
   | ProcessClient   |  DB link | CFileDB account |          | 196B _AUTH_GAME |
   | ProcessDBMessage|          | ranking, guild  |          | ProcessMessage  |
   +--------+--------+          +--------+--------+          +--------+--------+
            ^                            |                            ^
            | TCP (game protocol,        | reads/writes               | TCP (biserver.txt:
            |  keyword-table encoded,    | STRUCT_ACCOUNTFILE         |  54.207.102.145:3000)
            |  HEADER+CheckSum)          | binary files, *.txt/.csv   |
            |                            v                            |
   +--------+--------+          +-----------------+                   |
   | Game Client     |          | File store      |                   |
   | (patched by     |          | Release/.../    |                   |
   |  ClientPatch_   |          |  *.bin .dat     |                   |
   |  v7662 DLL)     |          |  *.txt .csv     |                   |
   +-----------------+          +-----------------+                   |
            |                                                         |
            +---------------------------------------------------------+
              (billing/auth path; observed TMSrv holds BillServerSocket)
```

### 4.2 Client <-> server message flow (login example)

```
Client --(MSG_AccountLogin, encoded)--> TMSrv
   TMSrv/_MSG_AccountLogin.cpp:
     - validates ClientVersion (APP_VERSION) and Size
     - CheckFailAccount(); if >=3 fails -> reject
     - rewrites Type to _MSG_DBAccountLogin, sets ID=conn
     - DBServerSocket.SendOneMessage(...) ---------------> DBSrv
                                                            DBSrv verifies account
                                                            (CFileDB: account file + password)
   DBSrv --(DB response)--> TMSrv (ProcessDBMessage) --(result)--> Client
```

- The internal TCP endpoints in TMSrv are the `CPSock` globals `DBServerSocket` and `BillServerSocket` (declared `extern` in `Source/Code/TMSrv/Server.h:241,253`). Socket acceptance uses `WSAAsyncSelect` (`Source/Code/TMSrv/CUser.cpp:61`, `Source/Code/DBSrv/CUser.cpp:49`).

---

## 5. Integration Points

| Integration | Type | Location | Purpose | Risk Level |
|---|---|---|---|---|
| TMSrv → DBSrv link | Internal TCP (CPSock) | `Source/Code/TMSrv/Server.h:253` (`DBServerSocket`), used in `Source/Code/TMSrv/_MSG_AccountLogin.cpp` | Account login + character/account persistence delegation | High (no encryption/auth observed on the link; trust-based) |
| TMSrv → BISrv link | Internal TCP (CPSock) | `Source/Code/TMSrv/Server.h:241` (`BillServerSocket`); target `Release/TMsrv/run/biserver.txt` (`54.207.102.145 3000`) | Billing / cash-item authorization (196-byte `_AUTH_GAME`) | High (hardcoded external IP in config; plaintext link) |
| Game client ↔ TMSrv | Custom binary protocol | `Source/Code/CPSock.h` (`HEADER`), `Source/Code/CPSock.cpp` (keyword-table encode/decode + checksum), TMSrv `_MSG_*.cpp` | All gameplay messaging | High (shared static key table; checksum mismatch does not reject — `CPSock.cpp:458-464`) |
| Account/character persistence | Flat binary files | `STRUCT_ACCOUNTFILE` (`Source/Code/Basedef.h:1085`); read in `Source/Code/DBSrv/CRanking.cpp:165,180`; managed by `Source/Code/DBSrv/CFileDB.cpp` | Store accounts, characters, cargo, coin, quests | High (plaintext password field; no DBMS, no transactions) |
| Game-content data files | Text/CSV/DAT/BIN | `Release/Common/*.csv,*.txt`; `Release/TMsrv/run/*.csv,*.txt,*.dat,*.bin`; loaded by `CReadFiles` | Items, mobs, drops, rates, regions, language, NPC gen | Medium (file-driven; tampering/parse risk) |
| Server/cluster config | Text config | `Release/TMsrv/run/gameconfig.txt`, `localip.txt`, `serverlist.txt`, `biserver.txt`, `admin.txt`; `Release/DBsrv/run/config.txt`, `Server.txt`, `settings.txt`, `admin.txt` | Server identity, IPs, admin list, rates, billing target | High (hardcoded/local IPs; admin IP allowlist in plaintext) |
| Client patch DLL | PE hook into client | `Source/Code/ClientPatch_v7662/` (`Hook.cpp`, `PE_Hook.h`) | Patch the game client (version 7662) at load | Medium (client-side trust; bypassable by definition) |
| Utility editors ↔ data/server | Standalone tools | `NPTool/`, `DropTool/`, `ExpTool/`, `EDITAPPMOB/`, `EDITAPPSHOP/`, `AttributeMap_Editor/`, `ZerarSkill/`, `SearchPass/` | Out-of-band editing of game data / accounts | Medium (e.g., `SearchPass` operates on account passwords) |

---

## 6. Architectural Risks & Single Points of Failure

| Risk Level | Component | Issue | Impact | Details |
|---|---|---|---|---|
| High | DBSrv (`CFileDB`, account files) | Single account/persistence process backed by flat binary files | Loss/corruption of DBSrv or its files takes down login and all persistence for the cluster | No DBMS/transactions; `STRUCT_ACCOUNTFILE` binary records (`Basedef.h:1085`), scanned/loaded by `CFileDB.cpp` and `CRanking.cpp` |
| High | CPSock packet layer | Checksum mismatch returns packet instead of dropping it (`CPSock.cpp:458-464`); single static `pKeyWord` table shared by all clients | Malformed/forged packets can still reach handlers; one leaked key compromises all traffic | Keyword-table transform with `pKeyWord[512]` ("7.xx keys"); `Sum != CheckSum` only sets `*ErrorCode` |
| High | Each server process | Single-threaded `WSAAsyncSelect` message-loop model (no worker threads found) | Any blocking operation (e.g., large file scan during ranking) stalls all connections on that tier; one crash drops the whole tier | No `CreateThread`/`std::thread`/`_beginthread` in TMSrv/DBSrv/BISrv |
| High | Account passwords | Password stored in plaintext `AccountPass` field within account file struct | Credential exposure if files are read; `SearchPass` tool can enumerate | `STRUCT_ACCOUNTINFO.AccountPass` (`Basedef.h:1020`); `SearchPass/` utility exists |
| High | TMSrv ↔ DBSrv / BISrv links | Internal TCP links appear unencrypted and identity-trusted | A reachable network position can impersonate/inject on the cluster bus | `DBServerSocket`, `BillServerSocket` (`TMSrv/Server.h`); billing target hardcoded in `biserver.txt` |
| Medium | TMSrv Server core | Massive central module + huge `Basedef.h` shared header | High coupling; a change to base structs ripples cluster-wide; large blast radius | `Server.cpp` ~10.5k LOC; `Basedef.cpp` ~7.5k LOC; `Basedef.h` included by 16 files |
| Medium | Build configuration | Main solution excludes BISrv and all utility tools | BISrv (a live network service) is not built/maintained with the core; drift risk | `Cavaleiros de Kersef.sln` lists only DBSrv, TMSrv, ClientPatch_v7662 |
| Medium | Client-side trust | `ClientPatch_v7662` and client-driven versioning (`APP_VERSION` check) | Client checks are bypassable; server must not rely on client integrity | Version check in `TMSrv/_MSG_AccountLogin.cpp`; patch DLL hooks client |
| Medium | Config/data files | Game balance, rates, IPs, and admin allowlist all in editable plaintext | Misconfiguration or tampering changes server behavior/security posture | `Release/.../*.txt`, `*.csv`; `admin.txt` IP allowlist |
| Low–Medium | Stray/duplicate sources | `Source/enc_temp_folder/.../_MSG_MessageWhisper.cpp` duplicate exists | Risk of editing/building a stale copy | Out-of-tree duplicate of a TMSrv handler |

---

## 7. Technology Stack Assessment

- **Language:** C++ (mixed C-style and C++; `Basedef.h` includes STL — `<vector>`, `<map>`, `<unordered_map>`, `<memory>`, `<functional>`, `<tuple>`, `<array>`, `<fstream>`, `<sstream>`). No explicit `LanguageStandard` value was found in `TMSrv.vcxproj` (defaults to the toolset default for `v143`).
- **Compiler/Build:** MSVC, `PlatformToolset v143` (Visual Studio 2022 era), `WindowsTargetPlatformVersion 10.0`, `CharacterSet MultiByte`, `RuntimeLibrary` `MultiThreadedDebug`/`MultiThreadedDebugDLL` (Debug configs observed). Build system is **Visual Studio solution/projects** (`Source/Cavaleiros de Kersef.sln`, per-project `.vcxproj`). Runtime launch via `.bat` loop scripts (`Release/TMsrv/run/TMsrv.bat`, `Release/DBsrv/run/DBsrv.bat`).
- **Networking:** WinSock (`<WinSock.h>`), event-driven via `WSAAsyncSelect` bound to a Windows window/message loop. Custom application protocol with `HEADER` struct and keyword-table byte encoding (`CPSock`).
- **Platform/OS APIs:** Win32 (`<Windows.h>`, `<windowsx.h>`), RPC (`<Rpc.h>` in `Basedef.h`, likely for UUID/GUID). UI via Win32 dialogs/windows and resource scripts (`*.rc`, `resource.h`). BISrv links the Common-Controls v6 manifest (`BISrv/Main.h`).
- **Third-party libraries:** None detected in source; the stack relies on the Windows SDK, WinSock, and the C/C++ runtime. Runtime redistributables ship in `Release/` (`msvcr120.dll`, `msvcp120.dll`, `ucrtbase.dll`, plus debug CRTs `msvcr100d.dll`, `msvcr120d.dll`).
- **Persistence:** Flat-file (binary account records + text/CSV/DAT/BIN content); no SQL/ORM/DBMS.
- **Architectural patterns:** Multi-process tiered cluster; single-threaded reactor (message-pump) per process; per-connection object (`CUser` owning `CPSock`); message-type dispatch to per-message handler files; shared fat base header; file-as-database.
- **Client tooling:** A client patcher DLL (`ClientPatch_v7662`) and multiple standalone Win32 data editors.

---

## 8. Security Architecture and Risks

The trust boundary that matters most is **client ↔ TMSrv**: the client is untrusted (and explicitly patched by `ClientPatch_v7662`), while server-side validation is the only real control. Evidence-backed concerns:

- **Static, shared packet key table.** `Source/Code/CPSock.cpp` embeds a 512-byte `pKeyWord` table ("7.xx keys") used to transform every packet body. The transform is a fixed per-byte add/subtract keyed by position (`CPSock.cpp:430-452`). This is obfuscation, not cryptography: the same table is shared by all clients and the server, so capture of the binary or table yields full decode capability.
- **Checksum failures do not reject packets.** In `ReadMessage`, when `Sum != CheckSum` the function sets `*ErrorCode = 1` and `*ErrorType = Size` but **still returns the (decoded) packet** (`Source/Code/CPSock.cpp:458-466`). Whether the caller honors the error code determines actual rejection; the layer itself does not drop malformed input.
- **Fixed-size buffer copies from network data.** Handlers cast raw buffers to structs and `memcpy`/`strncpy` into fixed fields, e.g. `Source/Code/TMSrv/_MSG_AccountLogin.cpp` copies `m->AdapterName` into `pUser[conn].Mac` and uses `sscanf(m->AccountName, "%s", ...)` plus `strncpy(..., NAME_LENGTH)`. Size is validated against `sizeof(MSG_AccountLogin)` before use, but the broad use of `sscanf("%s")`/`sprintf`/`strncpy` across the 58 `_MSG_*` handlers is a buffer-handling risk surface that would need per-handler review to fully characterize.
- **Plaintext account passwords at rest.** `STRUCT_ACCOUNTINFO.AccountPass` (`Source/Code/Basedef.h:1020`) is a plain char field inside the binary `STRUCT_ACCOUNTFILE` (`Basedef.h:1085`). `DBSrv/CFileDB.h` exposes `GetEncPassword`/`SetEncPassword` (an "Enc" accessor pair), but the stored account struct field itself is a plaintext password slot, and a dedicated `SearchPass/` tool exists to search passwords.
- **Authentication is server-internal and trust-based.** Login validation happens in DBSrv over the internal `DBServerSocket` link with no observed transport security or mutual authentication between TMSrv and DBSrv (`Source/Code/TMSrv/_MSG_AccountLogin.cpp` forwards to DBSrv; link declared in `Source/Code/TMSrv/Server.h:253`). A failed-attempt counter exists (`CheckFailAccount` >= 3 blocks), which is a basic brute-force guard.
- **Hardcoded IP addresses / endpoints.**
  - Billing target hardcoded in `Release/TMsrv/run/biserver.txt`: `54.207.102.145 3000` (an external public IP).
  - Local/cluster IPs in `Release/TMsrv/run/localip.txt` (`192.168.18.12`), `Release/TMsrv/run/serverlist.txt`, `Release/DBsrv/run/localip.txt`/`serverlist.txt`.
  - Admin allowlist by IP in plaintext: `Release/TMsrv/run/admin.txt` and `Release/DBsrv/run/admin.txt` (`0 192.168.18.12`). Admin authorization therefore rests on source-IP matching.
- **GM/admin command path.** `Source/Code/TMSrv/imple.cpp` implements `ProcessImple`, a command interpreter (e.g., `SaveAll`). Its authorization depends on the admin/IP gating above; the interpreter parses many `char[128]` command tokens.
- **Client-side enforcement is inherently bypassable.** Version gating (`m->ClientVersion != ClientVersion` using `APP_VERSION`) and the `ClientPatch_v7662` DLL operate on the client and cannot be relied on as security controls.

(No claim is made here about exploitability of any specific copy; these are architectural observations from the cited files.)

---

## 9. Infrastructure Analysis

Deployment is **directory-based on Windows**, driven by batch launchers and plaintext config — there is no container/orchestration or IaC.

- **Launchers (auto-restart loop):** `Release/TMsrv/run/TMsrv.bat` and `Release/DBsrv/run/DBsrv.bat` run the server `.exe` inside a `:Loop` that re-launches on exit (crash-restart pattern). `Release/deletar Logs.bat` clears logs.
- **TMSrv runtime layout (`Release/TMsrv/run/`):** `TMSrv.exe`, CRT DLLs (`msvcr120.dll`, `msvcp120.dll`, `ucrtbase.dll`, plus debug CRTs), and data/config: `gameconfig.txt` (drop/event/billing/treasure/etc. settings), `Rates.txt`, `Language.txt`, `localip.txt`, `serverlist.txt`/`serverlist.bin`, `biserver.txt`, `admin.txt`, `NPCGener.txt`/`NPCGener.new.txt`, `Guard.txt`, `Chall.txt`, `Regions.txt`, `QuestDiaria.txt`, `LevelItem.txt`, `InitItem.csv`, `extraitem.csv`/`extraitem.bin`, `ItemDropList.txt`, `HeightMap.dat`, `AttributeMap.dat`, and bundled editor `.exe`s (`EDITAPPMOB.exe`, `EDITAPPSHOP.exe`, `NPTool.exe`).
- **DBSrv runtime layout (`Release/DBsrv/run/`):** `DBSrv` build artifacts, `config.txt` (`Sapphire`, `LastCapsule`), `Server.txt`, `settings.txt` (client-update URLs / UI strings), `localip.txt`, `serverlist.txt`, `admin.txt`, `Mac.txt`, `redirect.sample.txt`, and `BaseMob/` class templates.
- **Common shared data (`Release/Common/`):** `ItemList.csv`, `SkillData.csv`, `data00.csv`, `serverlist.txt`/`serverlist.bin`, `extraitem.bin`, `Ranking.txt`, `Guilds.txt`/`GuildInfo`, `serv00.htm`, `Settings/` (`QuestsRate.txt`, `CompRate.txt`, `SancRate.txt`, `CastleQuest.txt`, `MobMerc.txt`), and `ImportUser/` for account import.
- **Server-list distribution (`serverlist bin/`):** per-IP directories (`192.168.18.12`, `192.168.2.103`, `192.168.2.105`, `192.168.2.107`) plus `serverlist editor.exe` — a tool for producing the client-facing server list per environment.
- **Observations/limitations:** Infrastructure is local/LAN-oriented (RFC1918 addresses) with one hardcoded public billing IP. There is no detected CI, no environment abstraction (IPs are baked into files per deployment), and no secrets management (admin authorization and billing endpoints are plaintext files). Several runtime data files duplicate source-tree files (e.g., `ItemEffect.h`, `ItemCSum.h` appear under `Release/`), indicating a copy-based release process rather than a packaged build pipeline.

---

*End of report. Coupling figures are estimates from `#include`/symbol counting and are labeled as such; where the source did not yield a definitive answer (e.g., the precise location of each server's `WinMain`/message pump, which was not found by name in the searched `.cpp` files), the uncertainty is stated rather than guessed.*
