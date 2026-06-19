# Component Deep Analysis Report: Basedef

Component: **Basedef** (shared global data-structure, constants and base-logic layer)
Date: 2026-06-19
Files analyzed:
- `Source/Code/Basedef.h` (2920 lines)
- `Source/Code/Basedef.cpp` (7468 lines)

Note: `Basedef.cpp` is stored in Non-ISO extended-ASCII (Latin-1) encoding, with Portuguese-language comments. Line references below were obtained with byte-oriented tooling and should be treated as accurate to within encoding tolerance.

---

## 1. Executive Summary

### Purpose
`Basedef` is the canonical schema and constant repository for the entire WYD server suite. It defines:
- Game-wide tuning constants and limits (`#define` block, `Basedef.h:47`-`Basedef.h:486`).
- The Plain-Old-Data (POD) structs that represent every persistent and transient game entity: account, character/mob, item, score, affect, guild, quest, castle, camp (`Basedef.h:490`-`Basedef.h:1201`).
- The full wire protocol: every inter-process message struct (`MSG_*`) shared by Client, TMSrv, DBSrv and NPTool (`Basedef.h:1203`-`Basedef.h:2735`).
- Prototypes for the `BASE_*` helper library and `extern` declarations for global game tables (`Basedef.h:2737`-`Basedef.h:2918`).
- The implementation of the `BASE_*` helpers and the global table definitions (`Basedef.cpp`).

### Role in System
It is the lowest shared layer. All four executables (TMSrv, DBSrv, BISrv, NPTool) and the auxiliary tools (DropTool, ExpTool, EDITAPPMOB, EDITAPPSHOP, ClientPatch) `#include "Basedef.h"` and link `Basedef.cpp`. The structs defined here are written directly to disk (account files) and serialized directly to sockets (messages), so the byte layout of this header is the de-facto on-disk and on-wire format.

### Key Findings
- This is a single-header / single-translation-unit "god module": it mixes constants, data schema, network protocol, and behavioral logic. Coupling to it is total — every server depends on it.
- All serialization is fixed-size binary struct copy. There are no versioning, schema-evolution, or `static_assert` size guards (no `static_assert` or `sizeof` checks exist in either file). Layout correctness depends entirely on MSVC default packing plus a handful of localized `#pragma pack(push,1)` regions (`Basedef.h:1047`, `1545`, `1823`, `2451`).
- The account password is stored as a plaintext fixed-size char buffer `AccountPass[ACCOUNTPASS_LENGTH]` at `Basedef.h:1020`, and is embedded in the persisted `STRUCT_ACCOUNTFILE` and login messages.
- No test files of any kind exist in the repository (see Section 10).
- Numerous fields are explicitly labeled `Unk`, `UNK`, `Useless`, `EMPTY[]` — indicating reverse-engineered layout with padding/reserved regions whose semantics are not fully known (e.g. `STRUCT_BEASTBONUS` `Basedef.h:758`, `STRUCT_MOB::Merchant`/`SaveMana` `Basedef.h:560`/`588`).

---

## 2. Data Flow Analysis

`Basedef` is a shared schema; it has no runtime behavior of its own beyond the `BASE_*` helper utilities. Data flows through it as a set of nested POD structs reused across persistence and network layers.

### Item flow — `STRUCT_ITEM` (`Basedef.h:500`)
The atomic unit. 8 bytes: `short sIndex` plus a 3-element `stEffect[3]` union where each slot is either a `short sValue` or a `{cEffect, cValue}` byte pair. Convenience macros `EF1/EFV1..EF3/EFV3` (`Basedef.h:515`-`520`) alias the effect slots. Every container (equip, carry, cargo, trade, shop, prize) is an array of `STRUCT_ITEM`. The `BASE_GetItemAbility` family (`Basedef.cpp:1537`, `1687`, `1885`, `2034`) decodes these effect slots into gameplay attributes; `BASE_GetItemSanc` / `BASE_SetItemSanc` (`Basedef.cpp:2136`, `2280`) read/write refine level.

### Mob / character flow — `STRUCT_MOB` (`Basedef.h:556`)
The full character record: name, clan, guild, class, coin, exp, saved teleport position, `BaseScore` + `CurrentScore` (`STRUCT_SCORE`), `Equip[MAX_EQUIP]`, `Carry[MAX_CARRY]`, learned skills, bonus point pools, skill bar, regen, resists. Paired with `STRUCT_MOBEXTRA` (`Basedef.h:620`) for class-mastery, quest progress, celestial save slots, and donate info. `STRUCT_CAPSULE` (`Basedef.h:752`) bundles `MOB` + `extra`.

### Score flow — `STRUCT_SCORE` (`Basedef.h:524`)
Combat stats block (level, AC, damage, HP/MP, STR/INT/DEX/CON, `Special[4]`). Embedded twice in `STRUCT_MOB` (base vs current) and propagated to the client through `MSG_UpdateScore` (`Basedef.h:1825`), `MSG_CreateMob` (`Basedef.h:1918`), `MSG_SelChar`/`STRUCT_SELCHAR` (`Basedef.h:1002`). `BASE_GetCurrentScore` (`Basedef.cpp:3014`) is the large derivation routine that folds base score + equipment + affects + extra into the effective current score.

### Account flow — `STRUCT_ACCOUNTINFO` (`Basedef.h:1017`) and `STRUCT_ACCOUNTFILE` (`Basedef.h:1085`)
`STRUCT_ACCOUNTFILE` is the on-disk account record. Its layout (commented byte offsets at `Basedef.h:1087`-`1091`) is: `Info` (account credentials), `Char[MOB_PER_ACCOUNT]`, `Cargo[MAX_CARGO]`, `Coin`, short-skill bars, per-character affects and mobExtra, donate, temp key, daily quest, block-pass. DBSrv reads/writes this struct directly; NPTool receives it whole via `MSG_NPAccountInfo` (`Basedef.h:2667`).

### Network flow
Every message begins with the `_MSG` macro header (`Basedef.h:1205`): `Size, KeyWord, CheckSum, Type, ID, ClientTick`. Routing direction is encoded by OR-ing `FLAG_*` constants (`Basedef.h:1212`-`1221`) into the message `Type`, e.g. `FLAG_CLIENT2GAME`, `FLAG_GAME2DB`, `FLAG_DB2NP`. The same struct (e.g. `MSG_DBSaveMob` `Basedef.h:1299`, `MSG_SavingQuit` `Basedef.h:1492`) carries `STRUCT_MOB`, `STRUCT_ITEM[]`, `STRUCT_MOBEXTRA`, `STRUCT_AFFECT[]` across the TMSrv↔DBSrv boundary by raw byte copy.

---

## 3. Business Rules & Logic

### Overview Table

| Rule Type | Rule Description | Location (file:line) |
|---|---|---|
| Capacity | Max concurrent users per game server = 1000; also start index of NPC/mob block | `Basedef.h:116` |
| Capacity | Max game servers attaching to one DBSrv = 10 | `Basedef.h:108` |
| Capacity | Max accounts per DBSrv = `MAX_USER * MAX_SERVER` | `Basedef.h:120` |
| Capacity | Characters per account = 4 | `Basedef.h:131` |
| Inventory | Equip slots = 16, Carry slots = 64, Cargo slots = 128 | `Basedef.h:135`-`137` |
| Inventory | Trade = 15, AutoTrade = 12, ShopList = 27 | `Basedef.h:139`-`142` |
| Progression | Max level = 399; max celestial level = 199 | `Basedef.h:177`-`178` |
| Progression | Skills per char = 24; combine slots = 8; classes = 4 | `Basedef.h:171`-`175` |
| Progression | Affect slots = 32 | `Basedef.h:182` |
| Item structure | `STRUCT_ITEM` = sIndex + 3 effect slots | `Basedef.h:500` |
| Item DB | Max distinct items in list = 6500; init items = 256 | `Basedef.h:199`, `201` |
| Sanity limit | Max HP/MP/damage = 1,000,000,000 | `Basedef.h:263`-`266` |
| Sanity limit | Stat caps STR/INT/DEX/CON = 32000 | `Basedef.h:272`-`275` |
| Sanity limit | Max attack velocity = 32000; max critical = 186000 | `Basedef.h:269`-`270` |
| World | Grid max X/Y = 4096; view grid 33x33 | `Basedef.h:160`-`161`, `155`-`156` |
| Entities | Max mobs = 25000; max NPC generators = 8192; max items on ground = 5000 | `Basedef.h:167`, `169`, `163` |
| Guild | Guild name length = 12; max guilds = 4096; guild zones = 5 | `Basedef.h:206`-`208` |
| Class model | Mortal=2, Arch=1, Celestial=3, CelestialCS=4, SuperCelestial=5 | `Basedef.h:238`-`242` |
| Realm | Red realm = 8, Blue realm = 7 | `Basedef.h:235`-`236` |
| Refine | Refine grade table REF_10..REF_15 = 10/12/15/18/22/27 | `Basedef.h:256`-`261` |
| Class base stats | Per-class base STR/INT/DEX/CON/HP/MP (4x6 table) | `Basedef.cpp:43` (`BaseSIDCHM`) |
| Reward | Per-hour reward item pool = {3210,3211,3212} | `Basedef.cpp:42` (`g_pRewardBonus`) |
| Default spawn | Cleared mob teleport position defaults to (2112,2112) | `Basedef.cpp:2988`-`2989` |
| Protocol | Message header fixed 12-byte layout via `_MSG` macro | `Basedef.h:1205` |
| Protocol | Routing flags OR-ed into message Type | `Basedef.h:1212`-`1221` |

### Detailed Breakdown

- Server topology constants (`Basedef.h:108`-`120`): `MAX_USER` (1000) doubles as both the concurrent-player cap and the starting array index for NPC/mob slots, an overloaded-constant invariant: any change cascades into mob indexing. `MAX_SERVERNUMBER = MAX_SERVER + 1` reserves index 0 for the DB/listing slot (`Basedef.h:110`).

- Account dimension constants (`Basedef.h:125`-`131`): `ACCOUNTNAME_LENGTH=16`, `ACCOUNTPASS_LENGTH=12`, `REALNAME_LENGTH=24`, `EMAIL_LENGTH=48`, `ADDRESS_LENGTH=78`, `TELEPHONE_LENGTH=16`, `MOB_PER_ACCOUNT=4`. These fix the byte layout of `STRUCT_ACCOUNTINFO` and therefore the on-disk account file; they are reused as buffer sizes in many message structs (e.g. `char AccountName[ACCOUNTNAME_LENGTH]`).

- Inventory model (`Basedef.h:135`-`148`): `ITEM_PLACE_EQUIP/CARRY/CARGO` (0/1/2) form the discriminator used by `GetItemPointer` (`Basedef.cpp:2381`) and the `*Type/*Pos` fields throughout item messages. Grid dimensions (`CARGOGRIDX=9 x CARGOGRIDY=14 = 126` ≈ but not equal to `MAX_CARGO=128`; `CARRYGRIDX=9 x CARRYGRIDY=7 = 63` vs `MAX_CARRY=64`) — note the grid area and slot-count constants are off by a small margin; this is stated as an observed inconsistency, not interpreted.

- Progression caps (`Basedef.h:177`-`182`): `MAX_LEVEL=399`, `MAX_CLEVEL=199`, `MAX_AFFECT=32`, `MAX_SKILL=24`. The level constants drive the experience tables `g_pNextLevel[MAX_LEVEL+202]` and `g_pNextLevel_2[MAX_CLEVEL+202]` (`Basedef.h:2864`-`2865`); the `+202` padding is an unexplained slack allocation.

- Hard sanity limits (`Basedef.h:263`-`277`): HP/MP/damage/magic-damage clamps and stat limits (`LIMT_STR/INT/DEX/CON = 32000`, `LIMT_DAM/DAM_MG = 1e9`). These are the numeric ceilings the derivation routines clamp against. `MAX_CRITICAL=186000`, `MAX_VELOATK=32000`, `AFFECT_1D=10800`, `AFFECT_1H=450` define time/critical scaling.

- Soul / reservation bit flags (`Basedef.h:244`-`304`): `RSV_*` (0x01..0x80) are bitmask reservation flags on `STRUCT_MOB::Rsv`; `SOUL_*` (0..17) enumerate soul-stone combinations. These are pure invariants consumed by combat logic in the servers.

- Class base-stat matrix `BaseSIDCHM[4][6]` (`Basedef.cpp:43`): defines, per class (TK/FM/BM/HT), the base STR, INT, DEX, CON, HP, MP. This is the canonical starting-stat invariant for character creation.

- Guild-zone seed table `g_pGuildZone` (`Basedef.cpp:53`): hard-codes the 5 castle/guild war zones (Armia, Azran, Erion, Nippleheim, Noatum) with spawn, city-limit, war-area, and tax coordinates. This is data-as-code: the game world geography for guild war is embedded in the compiled binary.

- Reset/clear invariants: `BASE_ClearMob` (`Basedef.cpp:2984`) zeroes a mob then sets default spawn to (2112,2112) and clears all 16 equip + 64 carry slots; `BASE_ClearMobExtra` (`Basedef.cpp:3006`) defaults `ClassMaster` to `MORTAL`. These define the canonical "new character" state.

- Protocol invariants: the `_MSG` macro (`Basedef.h:1205`) mandates that every message begins with `Size, KeyWord, CheckSum, Type, ID, ClientTick`. `Size` is set to `sizeof(struct)` by message constructors (e.g. `MSG_SendExpRanking` `Basedef.h:2601`), making struct size load-bearing for framing. Routing is by flag OR (`Basedef.h:1212`); a single numeric message id can carry multiple direction flags (e.g. `_MSG_AccountSecure` ORs four flags, `Basedef.h:1586`).

---

## 4. Component Structure (annotated outline of `Basedef.h`)

1. Includes + early enums/defines (`Basedef.h:24`-`80`): Windows + STL + WinSock headers; `LogType`, `Banned`, potion/speel enums.
2. `#pragma region Defines` (`Basedef.h:82`-`486`):
   - Window control IDs, ports, server topology (`82`-`121`).
   - Account-related lengths (`123`-`133`).
   - Inventory, grid, world, entity, progression, guild, string caps (`135`-`233`).
   - Realm/class/soul/refine/sanity-limit constants (`235`-`304`).
   - Quest / NPC / NPC-generator / dungeon ID constants (`306`-`484`).
3. `#pragma region Structures` (`Basedef.h:488`-`1081`): core game entities — `AccountBanned`, `STRUCT_ITEM`, `STRUCT_SCORE`, `STRUCT_RECYCLE`, `STRUCT_MOB`, `STRUCT_MOBEXTRA`, `STRUCT_AFFECT`, `STRUCT_CAPSULE`, `STRUCT_BEASTBONUS`, `STRUCT_TREASURE`, quest/castle/camp/guild structs, `STRUCT_SELCHAR`, `STRUCT_ACCOUNTINFO`, `STRUCT_RANKING` (packed).
4. `#pragma region File Structures` (`Basedef.h:1083`-`1201`): `STRUCT_ACCOUNTFILE`, `STRUCT_SPELL`, `STRUCT_GUARD`, `STRUCT_ITEMLIST`, `STRUCT_INITITEM`, `STRUCT_BLOCKMAC` — disk-format records.
5. `#pragma region Messages defines and structures` (`Basedef.h:1203`-`2735`): the `_MSG` macro, `FLAG_*` routing constants, and the full catalogue of `MSG_*` structs grouped by lane (DB<>Game, TM>DB, Client<>TM, NP).
6. `#pragma region Basedef functions prototypes` (`Basedef.h:2737`-`2846`): ~90 `BASE_*` prototypes (item ability decode, score derivation, equip/carry/cargo validation, file I/O, language tables, routing/distance).
7. `#pragma region Basedef Externs` (`Basedef.h:2848`-`2918`): global table declarations (guild zone, exp tables, item list, spell table, attribute map, sanc/bonus rate tables, server list).

---

## 5. Dependency Analysis

### External (system / library)
- Win32 / WinSock: `<Windows.h>`, `<WinSock.h>`, `<Rpc.h>` (`Basedef.h:25`, `42`, `43`) — ties the component to Windows/MSVC. `INT16/INT32` Win32 typedefs are used in structs (e.g. `STRUCT_RVRWAR` `Basedef.h:942`).
- C runtime / STL: `<time.h>`, `<cstdint>`, `<cassert>`, `<vector>`, `<map>`, `<unordered_map>`, `<string>`, `<fstream>`, `<mbstring.h>` (`Basedef.h:24`-`41`). `<mbstring.h>` indicates multi-byte/Hangul string handling (`BASE_CheckHangul` `Basedef.cpp:2594`).
- `Basedef.cpp` additionally includes `ItemEffect.h` (`Basedef.cpp:34`) — its only intra-project dependency.

### Internal
- `Basedef.h` has no project-internal includes; it is a leaf header that everything else depends on.
- `Basedef.cpp` depends only on `Basedef.h` and `ItemEffect.h`.

This makes Basedef a sink in the dependency graph: high inbound, near-zero outbound (outbound = OS + one project header).

---

## 6. Afferent and Efferent Coupling

Inbound (afferent) coupling is measured by `#include "Basedef.h"` across non-ignored sources. Confirmed includers (25 translation-unit headers/sources):
- TMSrv: 10 files (`CCastleZakum.h`, `CItem.h`, `CMob.h`, `CReadFiles.h`, `CWarTower.h`, `GetFunc.h`, `ProcessClientMessage.cpp/.h`, `ProcessDBMessage.h`, `Server.h`).
- DBSrv: 4 (`CFileDB.h`, `CRanking.h`, `CReadFiles.h`, `CUser.h`).
- BISrv: 2 (`CUser.h`, `ProcessMessage.h`).
- NPTool: 2 (`CUser.h`, `ProcessMessage.h`).
- Tools: ClientPatch, DropTool, ExpTool, EDITAPPMOB (2), EDITAPPSHOP (2).

Per-symbol-group coupling (all values are estimates inferred from `#include` reach and symbol naming; not from a compiled call graph):

| Struct / Symbol group | Afferent (Ca) — consumers | Efferent (Ce) — depends on | Notes |
|---|---|---|---|
| `STRUCT_ITEM` + `EF*` macros | Very high (estimate): all servers + all item tools | None (leaf POD) | Most reused type; embedded in dozens of structs/messages |
| `STRUCT_MOB` / `STRUCT_MOBEXTRA` | High (estimate): TMSrv, DBSrv, NPTool, EDITAPPMOB | `STRUCT_SCORE`, `STRUCT_ITEM` | Persisted + networked character record |
| `STRUCT_SCORE` | High (estimate) | None | Embedded in MOB, SELCHAR, many messages |
| `STRUCT_ACCOUNTINFO` / `STRUCT_ACCOUNTFILE` | Medium-high (estimate): DBSrv, NPTool | `STRUCT_MOB`, `STRUCT_ITEM`, `STRUCT_AFFECT`, `STRUCT_QUEST` | On-disk account format |
| `MSG_*` protocol structs | High (estimate): every networked module | `_MSG`, the entity structs | Wire contract between processes |
| `BASE_*` functions | High (estimate): mainly TMSrv combat/item, some DBSrv | All entity structs | Behavioral helpers |
| Constant `#define` block | Total (estimate): every file transitively | None | Array sizing + tuning across whole codebase |
| Global tables (`g_pItemList`, `g_pSpell`, `g_pNextLevel`, etc.) | Medium (estimate): TMSrv + tools | The structs they instantiate | Defined in `Basedef.cpp`, declared `extern` in header |

Efferent coupling of the component as a whole: low (OS headers + `ItemEffect.h`). Afferent coupling: maximal within this codebase. This is a classic high-fan-in stable-dependency module — but it is also volatile (mixes tuning constants and protocol), which is the source of its risk.

---

## 7. Integration Points

- Disk persistence: `STRUCT_ACCOUNTFILE` (`Basedef.h:1085`) is the binary account-file contract consumed by DBSrv file I/O (`CFileDB.h`) and tools.
- Process IPC / network: the `MSG_*` family plus `FLAG_*` routing (`Basedef.h:1212`) is the integration contract between Client, TMSrv, DBSrv, BISrv and NPTool. The `_MSG` header (`Basedef.h:1205`) defines framing.
- Data tables: binary table files are read/written through `BASE_ReadItemList`/`BASE_WriteItemList` (`Basedef.cpp:2869`/`2775`), `BASE_ReadSkillBin`/`BASE_WriteSkillBin` (`Basedef.cpp:2833`/`2737`), and message/mobname initializers — integrating external data files into the `g_p*` globals.
- No HTTP/REST endpoints are exposed by this component (one `BASE_GetHttpRequest` prototype at `Basedef.h:2832` is a client of an external HTTP source, not a server endpoint).

---

## 8. Design Patterns & Architecture

- POD / fixed-size binary serialization: every struct is a flat C struct designed for `memcpy`/`memset` and direct read/write to socket and file. Serialization = raw struct copy; no marshaling layer.
- Selective `#pragma pack(1)`: applied only to specific structs (`STRUCT_RANKING` `Basedef.h:1047`, `MSG_AccountLogin*` `Basedef.h:1545`, `MSG_UpdateScore` `Basedef.h:1823`, `MSG_AttackOne` `Basedef.h:2451`); the rest rely on default MSVC alignment. This is a deliberate per-struct ABI control pattern.
- Macro-as-struct-prefix: `_MSG` (`Basedef.h:1205`) injects a common header field block into every message struct — a textual mixin standing in for inheritance, chosen to preserve exact binary layout.
- Union + field-alias macros: `STRUCT_ITEM`'s effect slots use an anonymous union with `EF*`/`EFV*` `#define` accessors (`Basedef.h:503`-`520`) — compact binary encoding with named-field ergonomics.
- Flag-OR message routing: directionality encoded in bit flags OR-ed into the message id (`Basedef.h:1212`-`1221`) rather than separate routing metadata.
- Data-as-code tables: world geometry, drop rates, sanc rates, and class base stats are compiled-in arrays (`Basedef.cpp`), not external config.
- Anti-pattern observed (stated factually): god-module / header-of-everything — constants, schema, protocol, and behavior co-located in one header+impl pair.

---

## 9. Technical Debt & Risks (observations only)

| Risk Level | Area | Issue | Impact |
|---|---|---|---|
| High | Security / data-at-rest | Plaintext password field `AccountPass[ACCOUNTPASS_LENGTH]` (`Basedef.h:1020`), embedded in `STRUCT_ACCOUNTFILE` and login/save messages | Credentials stored and transmitted in cleartext within the struct layout |
| High | Serialization safety | No `static_assert` / size or version guards anywhere in `Basedef.h`/`.cpp`; on-wire & on-disk format == raw struct layout | Any field/`#define` change silently breaks file and protocol compatibility |
| High | Packing fragility | Mixed default-aligned and `#pragma pack(1)` structs (`Basedef.h:1047`,`1545`,`1823`,`2451`); most messages default-aligned | Layout depends on MSVC; padding mismatches across builds/compilers corrupt parsing |
| Medium | Fixed-size buffers | Pervasive fixed `char[]` buffers populated by `strncpy`/`memcpy` (e.g. `STRUCT_RANKING::Name[32]` filled via `strncpy(...sizeof(Name))` `Basedef.cpp:1063`; many `MobName[NAME_LENGTH]`) | `strncpy` to full size may leave non-terminated strings; overflow risk if upstream sizes mismatch |
| Medium | Maintainability | God-module: constants + schema + protocol + logic in one 2920-line header + 7468-line impl | Total inbound coupling; any edit forces full-suite recompile and risk |
| Medium | Unknown layout regions | Many `Unk`/`UNK`/`Useless`/`EMPTY[]` fields and reserved padding (e.g. `STRUCT_BEASTBONUS` `Basedef.h:758`, `MSG_CNFCharacterLogin::Unk2[765]` `Basedef.h:1700`) | Reverse-engineered layout; semantics unverified, easy to misuse |
| Medium | Overloaded constants | `MAX_USER` is both player cap and NPC/mob start index (`Basedef.h:116`) | Single change cascades unexpectedly into entity indexing |
| Low | Constant inconsistency | Grid area vs slot count mismatch: Carry 9x7=63 vs `MAX_CARRY=64`; Cargo 9x14=126 vs `MAX_CARGO=128` (`Basedef.h:136`-`153`) | Off-by-margin; intent unclear (stated, not interpreted) |
| Low | Encoding | `Basedef.cpp` is Latin-1 with Portuguese comments; mixed-language identifiers | Tooling/encoding hazards; reduced readability for non-PT maintainers |
| Low | Magic numbers | Hard-coded IDs/coords throughout (NPC/quest/dungeon defines `Basedef.h:306`-`484`; guild-zone coords `Basedef.cpp:53`) | Game data baked into binary; changes require recompile |

---

## 10. Test Coverage Analysis

A repository-wide search for test artifacts was performed:
- File-name search for `*test*`, `*spec*`, `*gtest*`, `*catch*` (case-insensitive), excluding `.git`, `.vs`, `Release`, `project-analizer`, `enc_temp_folder`: **no matches**.
- Directory search for any `*test*` directory: **no matches**.
- No test framework includes (`gtest`, `catch`, `boost/test`) appear in the source tree.

Finding: there is **no automated test coverage** for `Basedef` or any other component in this repository. Given that `Basedef` defines the binary on-disk account format and the inter-process wire protocol with no size/version guards (Section 9), the complete absence of tests is itself a significant risk: layout regressions (field additions, packing changes, constant edits) cannot be detected automatically and would manifest only as runtime corruption or protocol desync. This is recorded as a finding, not a recommendation.

---

## Ambiguities / Stated Uncertainties
- Coupling magnitudes in Section 6 are estimates from `#include` reach and symbol naming, not from a compiled call/usage graph.
- Numerous struct fields are labeled `Unk`/`UNK`/`EMPTY`; their true semantics are not determinable from this component alone.
- The grid-vs-slot count discrepancies are reported as observed; the original design intent is unknown.
- `Basedef.cpp` line numbers were obtained against a Latin-1-encoded file and may shift by encoding interpretation.
