# Component Deep Analysis Report: TMSrv-CastleWar

**Component**: TMSrv-CastleWar (Siege / Guild-War + Castle Quest subsystem)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/TMSrv/CWarTower.cpp` (319), `CWarTower.h`,
`Source/Code/TMSrv/CCastleZakum.cpp` (554), `CCastleZakum.h`

---

## 1. Executive Summary

This component implements OpenWYD's large-group event content. It comprises two related but
distinct systems:

- **CWarTower** — the guild "War of Territory" tower. A capturable tower mob is owned by the
  guild that holds it (`GTorreGuild`); killing the tower transfers ownership to the killer's
  guild, the holding guild periodically accrues `Fame`, and the held state is synchronized to
  DBSrv via `MSG_GuildInfo`. It is a guild-versus-guild PvP territory mechanic driven on a
  schedule (`GuildProcess(tm*)`).
- **CCastleZakum** — a timed, party-based PvE castle quest (the Zakum castle). It is configured
  from `../../Common/Settings/CastleQuest.txt`, gated by keys (`OpenCastleGate`), tracks a quest
  level, a countdown timer, and a registered party with a leader, spawns waves of mobs and
  bosses, and on boss kill distributes item, EXP, and coin prizes to the party leader.

Both hook into the global `MobKilled` flow (each exposes its own `MobKilled(target, conn, ...)`)
and into the per-second/timed processing loop. Guild and actor state is read from the shared
`pMob[]`/`STRUCT_MOB` and `GuildInfo[]` structures.

---

## 2. Data Flow Analysis

War Tower:
```
1. Scheduled tick -> CWarTower::GuildProcess(timeinfo) (CWarTower.cpp:42)
2. If a guild holds the tower (GTorreGuild != 0): GuildInfo[guild].Fame += 100 (CWarTower.cpp:90)
   -> build MSG_GuildInfo -> DBServerSocket.SendOneMessage to DBSrv (CWarTower.cpp:94)
3. Player attacks tower -> CWarTower::TowerAttack(conn, idx) validates guild (CWarTower.cpp:147)
4. Tower mob killed -> CWarTower::MobKilled -> GTorreGuild = killer's guild (CWarTower.cpp:129)
5. Tower respawns -> GGenerateMob stamps the holding guild onto the new mob (CWarTower.cpp:138)
```

Castle (Zakum) Quest:
```
1. Config loaded from CastleQuest.txt (CCastleZakum.cpp:44)
2. Party leader opens gate -> OpenCastleGate validates key vs gatekey/quest (CCastleZakum.cpp:72)
3. Quest starts: CastleQuestLevel/Time set, party registered, StartTime sent (CCastleZakum.cpp:155-183)
4. Wave mobs spawned for [MOB_INITIAL..MOB_END] excluding bosses (CCastleZakum.cpp:147-153)
5. Boss killed -> CCastleZakum::MobKilled -> CastleQuestClear; prizes to leader (CCastleZakum.cpp:210-242)
```

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| Ownership | Tower captured by the killer's guild | `CWarTower.cpp:129` |
| Reward | Holding guild gains Fame +100 per process tick | `CWarTower.cpp:90` |
| Validation | Cannot attack tower of own guild (or with no guild) | `CWarTower.cpp:152` |
| Sync | Guild state pushed to DBSrv via MSG_GuildInfo | `CWarTower.cpp:94` |
| Respawn | Regenerated tower keeps holding guild ownership | `CWarTower.cpp:138-143` |
| Config | Castle quest defined in CastleQuest.txt | `CCastleZakum.cpp:44` |
| Gate | Gate opens only with correct key and quest level | `CCastleZakum.cpp:91-97` |
| Party | Quest tracks party array + leader, timed | `CCastleZakum.cpp:155-183` |
| Waves | Spawns mob id range excluding boss ids | `CCastleZakum.cpp:147-153` |
| Reward | Boss kill grants item/EXP/coin prizes to leader | `CCastleZakum.cpp:231-242` |

### Detailed breakdown

---

### Business Rule: Guild Tower Capture and Ownership

**Overview**: The war tower belongs to whichever guild last destroyed it; ownership confers
periodic Fame and is the win signal of the territory event.

**Detailed description**: A module-level `GTorreGuild` holds the current owning guild id. When
the tower mob dies, `CWarTower::MobKilled` reads the killer's guild from `pMob[conn].MOB.Guild`
and assigns it to `GTorreGuild` (`CWarTower.cpp:116-129`), resolving the guild group as
`Guild / MAX_GUILD` and fetching the name via `BASE_GetGuildName`. When the tower respawns,
`GGenerateMob` stamps `pMob[tmob].MOB.Guild = GTorreGuild` onto the new tower so the visual/owner
state persists (`CWarTower.cpp:138-143`). The capture event is logged ("war_tower1 ...").

**Rule workflow**: kill tower → set GTorreGuild to killer's guild → respawn tower under that guild.

---

### Business Rule: Tower Attack Eligibility

**Overview**: Only members of a rival guild may damage the tower.

**Detailed description**: `CWarTower::TowerAttack(conn, idx)` returns FALSE when the attacker has
no guild or shares the tower's guild: `if (pMob[conn].MOB.Guild == 0 || pMob[conn].MOB.Guild ==
pMob[idx].MOB.Guild) return FALSE` (`CWarTower.cpp:152`). This prevents guildless players and the
owning guild from attacking, so only challengers can contest the tower.

**Rule workflow**: attack tower → check attacker guild != 0 and != tower guild → allow or reject.

---

### Business Rule: Holding-Guild Fame Accrual and DB Sync

**Overview**: While a guild holds the tower, it earns Fame on each scheduled process, persisted
through DBSrv.

**Detailed description**: `GuildProcess(timeinfo)` iterates actors and, for the holding guild,
increments `GuildInfo[usGuild].Fame += 100` (`CWarTower.cpp:90`), then constructs an
`MSG_GuildInfo` message (`Type = _MSG_GuildInfo`, the full `GuildInfo[usGuild]` payload) and sends
it to DBSrv via `DBServerSocket.SendOneMessage` (`CWarTower.cpp:83-94`). This makes Fame the
scored currency of holding the tower and keeps the authoritative guild record on DBSrv updated.

**Rule workflow**: scheduled tick → if guild holds tower → Fame += 100 → send MSG_GuildInfo to DBSrv.

---

### Business Rule: Castle (Zakum) Quest Start, Gate, and Timer

**Overview**: A party leader starts a timed instanced castle quest by opening a gate with the
correct key; the quest tracks level, time, and party membership.

**Detailed description**: Quest definitions load from `CastleQuest.txt`
(`CCastleZakum.cpp:44`) into `CastleQuest[]`. `OpenCastleGate(conn, gateid, m)` validates the
supplied key against `gatekey` and the active `CastleQuestLevel`, with a special-case for
`gatekey == 10` (`CCastleZakum.cpp:91-97`). On a valid start, the code spawns the wave mob id
range `[CastleQuest[Quest].MOB_INITIAL .. MOB_END]` excluding boss ids
(`CCastleZakum.cpp:147-153`), sets `CastleQuestLevel = Quest` and
`CastleQuestTime = QuestTime - 1` (`:155-156`), records `CastleLeader`, broadcasts `_MSG_StartTime`
to the leader and party (`:165-172`), and registers the party roster into `CastleQuestParty[]`
from `pMob[Leader].PartyList[]` (`:179-183`). The quest is single-active (`CastleQuestTime != -1`
guards re-entry, `:111`).

**Rule workflow**: leader opens gate → validate key/level → spawn waves → set timer & party → broadcast start.

---

### Business Rule: Castle Quest Completion and Rewards

**Overview**: Killing the designated boss clears the quest and pays out fixed prizes to the
party leader.

**Detailed description**: `CCastleZakum::MobKilled` checks whether the killed mob's
`GenerateID` matches a quest boss (`CastleQuest[k].BOSS[0/1]`, `CCastleZakum.cpp:216`); if so it
sets `CastleQuestClear = 1`, announces the clear (`_SN_CastleQuest_Killed`), and grants the party
leader each configured item prize via `PutItem` (`:231-232`), an EXP prize indexed by the
leader's `ClassMaster` (`pMob[partyleader].MOB.Exp += CastleQuest[k].ExpPrize[...]`, `:236`), and
a coin prize (`CastleQuest[k].CoinPrize`, `:242`). Rewards therefore go to the leader rather than
being split per member.

**Rule workflow**: boss killed → match quest boss → mark clear → grant item/EXP/coin prizes to leader.

---

## 4. Component Structure

```
TMSrv/CWarTower.h/.cpp   CWarTower
  ├── GuildProcess(tm*)        # scheduled Fame accrual + DBSrv sync
  ├── MobKilled(...)           # tower capture -> GTorreGuild
  ├── GGenerateMob(...)        # respawn tower under holding guild
  ├── TowerAttack(conn, idx)   # attack eligibility
  └── module state: GTorreGuild, GTORRE id
TMSrv/CCastleZakum.h/.cpp  CCastleZakum
  ├── CASTLE_QUEST_PATH = ../../Common/Settings/CastleQuest.txt
  ├── OpenCastleGate(...)      # gate key/level validation + quest start
  ├── MobKilled(...)           # boss-clear detection + prize payout
  └── module state: CastleQuest[], CastleQuestLevel/Time/Clear, CastleQuestParty[], CastleLeader
```

## 5. Dependency Analysis

```
Internal:
  CWarTower ──> pMob[]/STRUCT_MOB, GuildInfo[], BASE_GetGuildName, DBServerSocket (CPSock), MSG_GuildInfo
  CCastleZakum ──> pMob[]/STRUCT_MOB, CastleQuest[] config, PutItem, g_pMessageStringTable,
                   SendClientSignalParm, party list
External:
  - Filesystem: CastleQuest.txt
  - <time.h> tm for scheduling
```

## 6. Afferent and Efferent Coupling

Afferent = units depending on this component; efferent = its dependencies. These event systems
are invoked from the global MobKilled flow and the timed loop, and they depend broadly on guild,
actor, item, and DB facilities.

| Unit | Afferent (est.) | Efferent (est.) | Critical |
|------|-----------------|-----------------|----------|
| CWarTower | Medium (MobKilled, scheduler) | pMob, GuildInfo, CPSock(DB), Basedef | Medium |
| CCastleZakum | Medium (MobKilled, gate handler) | pMob, CastleQuest config, item/party/score | Medium |

## 7. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| DBSrv | Internal | Persist guild Fame/ownership | CPSock binary | MSG_GuildInfo | fire-and-forward |
| CastleQuest.txt | Filesystem | Castle quest definitions | File IO | text | depends on loader |
| Game client | TCP (indirect) | StartTime, announcements, prizes | CPSock binary | signal/score/item msgs | client messages |

## 8. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| Scheduled event tick | `GuildProcess(tm*)` | `CWarTower.cpp:42` | Time-driven war scoring |
| Shared-state event module | module-level `GTorreGuild`, `CastleQuest*` globals | both files | Single active event tracking |
| Config-driven content | `CastleQuest.txt` -> `CastleQuest[]` | `CCastleZakum.cpp:44` | Designer-editable quest data |
| Hook into kill pipeline | per-system `MobKilled` | both files | React to actor death |

## 9. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| Medium | Global state | Single `GTorreGuild` / `CastleQuest*` module globals | One concurrent event instance; not re-entrant |
| Medium | Reward fairness | Castle prizes paid only to the party leader (`:231-242`) | Reward routing depends on leader integrity |
| Medium | Validation | Gate key check mixes `!=`/`&&`/`==` without parentheses (`CCastleZakum.cpp:91`) | Predicate may not match intent |
| Low | Coupling | War scoring writes directly to GuildInfo and DBSrv | Tight coupling across modules |

## 10. Test Coverage Analysis

No automated tests exist for the castle/war subsystem or any module in the repository (no test
project, framework, or `*test*` sources). Event scoring, ownership transfer, and prize payout are
validated only at runtime. Recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| CWarTower / CCastleZakum | 0 | 0 | None | No tests present in repo |
