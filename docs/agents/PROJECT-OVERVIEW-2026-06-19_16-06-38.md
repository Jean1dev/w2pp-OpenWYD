# w2pp-OpenWYD - Project Overview

**Generated on**: 2026-06-19 16:06:38

## Summary

w2pp-OpenWYD is an open-source server emulator for the MMORPG "With Your Destiny" (WYD),
written in C++ for Windows and built with Visual Studio / MSVC. It is a fork of the
"Cavaleiros de Kersef" (WYD cdk) sources. The runtime is a small cluster of cooperating
server processes plus a set of standalone editor/utility tools. Persistence is file-based
rather than backed by a relational database, and the entire third-party surface reduces to
the Windows SDK (WinSock, Win32, Winmm); there is no package manager and no external service
integration beyond the inter-server TCP links.

The system is organized around three servers. TMSrv is the game server and by far the largest
component, hosting world simulation, the client protocol, and the bulk of game rules across
`Server.cpp` (10.5k lines) and 58 per-message handler files. DBSrv is the account/persistence
authority that TMSrv queries for login, character data, ranking, and guild state. BISrv is
nominally the billing server but is, in its current state, a non-functional skeleton. All three
share a common base layer: `Basedef` (data structures and constants) and `CPSock` (the WinSock
packet/socket layer with table-based obfuscation and a checksum). Each server is a single-threaded
WinSock reactor driven by a Win32 window message pump.

This analysis produced a dependency audit, a full architectural analysis, and nine component
deep-dives (Basedef, CPSock, TMSrv-Core, TMSrv-CUser, TMSrv-CItem, TMSrv-CMob, TMSrv-CastleWar,
DBSrv, BISrv). The findings below aggregate those reports and are descriptive only.

## Architecture Overview

- Pattern: multi-process client-server with a single-threaded `WSAAsyncSelect` reactor per
  process (Win32 `MainWndProc` message pump). No worker threads were found.
- Topology: Client → TMSrv (game) → DBSrv (persistence, TCP port 7514); TMSrv → BISrv (billing,
  stubbed). DBSrv is the source of truth for accounts, characters, ranking, and guilds.
- Shared layers: `Basedef` defines the binary structs/constants used by all servers; `CPSock`
  frames, obfuscates, and checksums every message.
- Entity model: a single `STRUCT_MOB` array (`pMob[]`) represents both players and monsters;
  player session state lives in a parallel `CUser`/`pUser[]` array indexed by the same connection.
- Content systems: item refinement, mob spawning/drops/EXP, guild war towers, and a timed
  castle (Zakum) quest are implemented inside TMSrv.
- Only 3 of 12 Visual Studio projects (DBSrv, TMSrv, ClientPatch) are wired into the main
  solution; BISrv and the editor tools are standalone.

## Dependencies Health

- Toolchain: MSVC / Visual Studio (project toolsets v141/v142/v143 observed). Dependency surface
  is the Windows SDK only (WinSock `ws2_32`, Winmm, Win32/Common Controls).
- Persistence is flat files; no SQL/ODBC client is actually used despite default template
  references.
- One opaque committed third-party DLL (`3da_extra9.dll`) and shipped MSVC 2010/2013 + UCRT
  runtime DLLs that do not match the declared project toolsets.
- The dependency audit lists 6 UNVERIFIED items (versions/origins that could not be confirmed
  from the repository alone).

## Components Analyzed

- **Basedef** — Shared binary data structures and game constants used by all servers; includes the
  plaintext account-password field and fixed-size record layouts.
- **CPSock** — WinSock transport: framing against a 12-byte header, a static committed
  obfuscation table, and an additive checksum that does not reject mismatched packets.
- **TMSrv-Core** — Game server bootstrap, single-threaded reactor, and the ~60-type client
  dispatch switch routing to 58 handlers; also the client trust boundary and anti-cheat funnel.
- **TMSrv-CUser** — Player session entity (state machine, inventory/cargo, trade state, action
  timers, and the "cra point" anti-cheat counter).
- **TMSrv-CItem** — Item subsystem: type-tagged effect model, equip/use, probabilistic refinement
  with a per-level rate table, trade and drop handling.
- **TMSrv-CMob** — Unified actor subsystem: mob AI state machine, aggro, spawning from config,
  and the large `MobKilled` flow computing EXP (party split + hardcoded level curve) and drops.
- **TMSrv-CastleWar** — Guild war tower capture/Fame mechanic plus a timed party PvE castle
  (Zakum) quest with gate keys and item/EXP/coin prizes.
- **DBSrv** — File-based account/character persistence, plaintext-password authentication, a
  28-type message protocol, ranking, and guild/capsule storage.
- **BISrv** — Billing server skeleton; `ProcessMessage` is an empty stub, so no billing logic is
  implemented in this component.

## Critical Findings

### Security Risks

- Account passwords are stored and compared in plaintext (`DBSrv/CFileDB.cpp:677`); the
  "encrypted password" path is a non-functional stub returning random numbers
  (`DBSrv/CFileDB.cpp:2696`).
- The packet checksum does not reject mismatches — `CPSock::ReadMessage` returns the packet even
  when the checksum fails (`CPSock.cpp:464`); integrity depends on each caller honoring an error
  code, which is inconsistent.
- Protocol "encryption" is a static obfuscation table committed in source (`CPSock.cpp:29`), so
  it is fully reversible by anyone with the repository; the `INITCODE` handshake is a fixed public
  constant, not authentication.
- TMSrv trusts client-supplied slot/position fields after a single ID bounds check, placing the
  burden of validation on each of the 58 handlers (item-dup and out-of-bounds exposure).
- A public billing IP is hardcoded in a shipped config file (`Release/TMsrv/run/biserver.txt`).

### Technical Debt

- `Server.cpp` (10.5k lines) and `MobKilled.cpp` (3.5k-line single function) concentrate logic
  in very large units with global mutable state.
- Game balance (EXP curve) is hardcoded in level-band branches rather than configuration
  (`TMSrv/MobKilled.cpp:483-504`).
- Ten near-duplicate `_MSG_CombineItem*` handlers risk rule drift; a refine anti-spam cooldown is
  commented out (`TMSrv/_MSG_UseItem.cpp:209-221`).
- Numerous reverse-engineered `Unk*` offset fields and fixed-size buffers create fragile binary
  layout assumptions.
- No automated tests exist anywhere in the repository (no test project, framework, or test
  sources).

### Single Points of Failure

- Each server is single-threaded; any blocking operation stalls all connected players on that
  shard.
- DBSrv is a hard dependency for login and persistence; its `pAccountList`/file store has no
  observed locking or transactional write path, creating corruption risk on crash.
- CPSock and Basedef are depended upon by every server; a defect there is system-wide.

## Reports Index

See [MANIFEST.md](./MANIFEST.md) for the complete list of all generated reports.
