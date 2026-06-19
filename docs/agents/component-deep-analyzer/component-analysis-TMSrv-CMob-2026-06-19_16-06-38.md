# Component Deep Analysis Report: TMSrv-CMob

**Component**: TMSrv-CMob (Mob / NPC Subsystem — AI, spawning, death, drops, EXP)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/TMSrv/CMob.cpp` (1358), `CMob.h` (143), `CNPCGene.cpp` (976),
`MobKilled.cpp` (3506)

---

## 1. Executive Summary

CMob is the actor subsystem of the game server. A central design fact frames everything else:
the array `pMob[MAX_MOB]` (`CMob.h:143`) holds **both monsters and players** — `STRUCT_MOB` is
the unified stat block for any in-world actor. A player's live combat stats live in
`pMob[conn].MOB` (the same `conn` index used for `pUser[conn]`), while monsters occupy the higher
slots. This is why item refinement recomputes a player's score through `pMob[conn].GetCurrentScore`
(seen in the item subsystem) and why the same `CMob` AI/score code serves characters and NPCs.

The subsystem has three responsibilities:
- **AI / behavior** (`CMob.cpp`): a mode-based state machine (`MOB_IDLE`, `MOB_COMBAT`,
  `MOB_ROAM`, `MOB_FLEE`, `MOB_RETURN`, …), aggro management via `EnemyList[MAX_ENEMY=13]`,
  pathing/segments, and `BattleProcessor` / `StandingByProcessor` ticks driven by
  `ProcessorSecTimer`.
- **Spawning** (`CNPCGene.cpp`): `CNPCGenerator` parses `NPCGener.txt` to populate regions with
  mobs, attaches drop lists and level lists, and `CNPCSummon` handles summoned creatures.
- **Death, EXP, and drops** (`MobKilled.cpp`): the 3,506-line `MobKilled` function computes
  experience (with party splitting and a hardcoded level-banded curve), rolls item/rune/gold
  drops, triggers boss-chain spawns, and pushes EXP ranking updates to DBSrv.

EXP and drops are RNG-driven (`rand()`), and the EXP curve is heavily customized per level band
(`MobKilled.cpp:483-504`), which is characteristic of a private-server rebalance.

---

## 2. Data Flow Analysis

```
Spawn:    CNPCGenerator::ReadNPCGenerator parses NPCGener.txt (CNPCGene.cpp:45)
            -> ReadRegion / DropList / LevelList attach spawn + drop config
            -> GenerateMob places a CMob in pMob[]; GenerateIndex tracks its generator
AI tick:  ProcessorSecTimer -> StandingByProcessor (idle/roam) or BattleProcessor (combat)
            -> GetEnemyFromView / AddEnemyList / SelectTargetFromEnemyList for aggro
            -> GetNextPos / SetSegment for movement
Combat:   player or mob attack -> damage applied to target pMob[].MOB.CurrentScore.Hp
            -> on lethal hit -> MobKilled(target, conn, PosX, PosY) (MobKilled.cpp:41)
Death:    MobKilled -> compute EXP (GetExpApply) -> split to party -> apply bonus/cap
            -> roll drops (items/runes/gold via rand()) -> CreateItem on ground
            -> boss-chain GenerateMob for quest bosses -> MSG_UpdateExpRanking -> DBSrv
Respawn:  WaitSec / RateRegen timers schedule regeneration of the generator's mob
```

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| AI state | Mode machine: IDLE/PEACE/COMBAT/RETURN/FLEE/ROAM/WAITDB | `CMob.h:26-35` |
| Aggro | Enemy list capped at MAX_ENEMY (13); target selected from list | `CMob.h:24,72`, `CMob.cpp:330,405` |
| Movement | Segment/route pathing; WaitSec gates step timing | `CMob.cpp:174-205,494` |
| Combat range | Attack range from `EF_RANGE`; bosses overridden (KEFRA=25) | `CMob.cpp:289` |
| EXP base | `GetExpApply(extra, mobExp, myLevel, mobLevel)` | `MobKilled.cpp:405` |
| EXP party split | `isExp = NumMob * MobExp / 100` distributed to party | `MobKilled.cpp:426-454` |
| EXP curve | Hardcoded level-banded scaling (<=200 … <=400) | `MobKilled.cpp:483-504` |
| EXP bonus | `+= exp*ExpBonus/100` when 0 < ExpBonus < 500 | `MobKilled.cpp:534-535` |
| EXP cap | Clamped to `g_pNextLevel[MAX_LEVEL+1]` | `MobKilled.cpp:575-581` |
| Ranking | EXP gain pushes MSG_UpdateExpRanking to DBSrv | `MobKilled.cpp:585-587` |
| Drops | Item/rune/gold rolled via `rand()%N` drop tables | `MobKilled.cpp:1503,1811,2162,...` |
| Boss chain | Killing quest mobs spawns next boss (GenerateMob) | `MobKilled.cpp:2304` |

### Detailed breakdown

---

### Business Rule: Mob AI State Machine and Aggro

**Overview**: Each mob runs a per-second behavior cycle that transitions between idle, roaming,
combat, fleeing, and returning, based on nearby enemies.

**Detailed description**: The `Mode` field takes values from `CMob.h:26-35`. `ProcessorSecTimer`
(`CMob.cpp:75`) drives either `StandingByProcessor` (non-combat: roaming via `GetRandomPos`,
returning home, waiting on `WaitSec`) or `BattleProcessor` (`CMob.cpp:233`, combat: pursue and
attack the `CurrentTarget`). Aggro is maintained in `EnemyList[MAX_ENEMY]` (13 slots): mobs add
attackers with `AddEnemyList` (`CMob.cpp:330`), drop them with `RemoveEnemyList`, and pick a
focus with `SelectTargetFromEnemyList` (`CMob.cpp:405`); `GetEnemyFromView` (`CMob.cpp:1308`)
acquires new targets within sight. Movement uses route/segment data (`SetSegment`,
`GetNextPos`), and step cadence is gated by `WaitSec` decremented in 6-unit steps
(`CMob.cpp:174-205`). Boss mobs receive special-cased behavior keyed off `GenerateIndex`
(e.g. `KEFRA_BOSS` overrides range to 25 at `CMob.cpp:289`).

**Rule workflow**: tick → has enemies? → BattleProcessor (chase/attack) else StandingBy (roam/return) → update target/position.

---

### Business Rule: Experience Calculation, Party Split, and Curve

**Overview**: Killing a mob grants EXP to the killer and nearby party members, scaled by a
custom level curve, modified by bonuses, and capped at the level table.

**Detailed description**: On death, base EXP is `GetExpApply(pMob[conn].extra,
pMob[target].MOB.Exp, killerLevel, mobLevel)` (`MobKilled.cpp:405`). For parties the value is
proportioned: `isExp = NumMob * MobExp / 100` where `NumMob` reflects participants, and the loop
re-applies `GetExpApply` per party member (`MobKilled.cpp:426-454`). A hardcoded ladder then
scales EXP by the receiver's level band — distinct branches for `myLevel <= 200, <=300, <=356,
<=360, <=370, <=380, <=390, <=400` with inline comments stating the intended average EXP per
band (`MobKilled.cpp:483-504`). An event/item multiplier applies when `0 < ExpBonus < 500`:
`exp += exp * ExpBonus / 100` (`MobKilled.cpp:534-535`). The total is added to `MOB.Exp` but
clamped so it never exceeds `g_pNextLevel[MAX_LEVEL+1]` (`MobKilled.cpp:575-581`). Each gain logs
to the daily EXP log and emits `MSG_UpdateExpRanking` to DBSrv with the player's ranking record
(`MobKilled.cpp:585-587`).

**Rule workflow**: kill → base EXP → party split → level-band scale → apply ExpBonus → cap to level table → persist + rank.

---

### Business Rule: Drop Generation (items, runes, gold)

**Overview**: A killed mob rolls one or more drops from RNG-weighted tables; some mobs drop
quest runes or trigger boss spawns.

**Detailed description**: `MobKilled` contains numerous RNG-gated drop branches. General item
drops use rolls such as `rand() % 60` (`MobKilled.cpp:1503`) and `rand() % 14`
(`:1811,1832,2508`), selecting item indices (e.g. `421 + rand()%7`, `:1835`) and calling the
ground-item creator (commented `CreateItem(AlvoX, AlvoY, &item, rand()%4, 1)` patterns at
`:1846,1875`). The rune-quest system rolls `rand() % 100` gates (`:2162`) and picks from
per-tier `PistaRune[tier][...]` tables (`:2185-2493`), and killing certain quest mobs spawns the
next boss via `GenerateMob(RUNEQUEST_LV3_MOB_BOSS_INITIAL + rand()%7, 0, 0)` (`:2304`). The mob's
`DropBonus` field (`CMob.h:80`) can bias drops. Gold/position scatter uses `rand()` offsets
(`:232-238`).

**Rule workflow**: death → for each drop table → rand roll vs threshold → build STRUCT_ITEM → create ground item / spawn boss.

---

### Business Rule: Spawning and Respawn (CNPCGenerator)

**Overview**: Mobs are populated from a text generator file per region and regenerate on a timer
after death.

**Detailed description**: `CNPCGenerator::ReadNPCGenerator` (`CNPCGene.cpp:45`) parses
`NPCGener.txt` (with `ParseString`, `CNPCGene.cpp:130`), reading region bounds (`ReadRegion`,
`:295`), associated drop lists (`DropList`, `:321`), and level lists (`LevelList`, `:598`). Each
generated mob keeps a `GenerateIndex` linking it back to its generator so that after death the
generator can respawn it. Respawn cadence is governed by `WaitSec` and `RateRegen` (`CMob.h:117`).
`CNPCSummon` (`CNPCGene.cpp:672`) handles dynamically summoned creatures (pets/summons).

**Rule workflow**: load generator file → spawn mobs per region → on death schedule regen via WaitSec → respawn.

---

## 4. Component Structure

```
TMSrv/CMob.h    class CMob { STRUCT_MOB MOB; Affect[]; EnemyList[13]; Route/Segment fields;
                  WeaponDamage/ForceDamage/ReflectDamage; DropBonus/ExpBonus; extra; ... }
                global pMob[MAX_MOB]  (players AND monsters)
TMSrv/CMob.cpp  AI: ProcessorSecTimer, StandingByProcessor, BattleProcessor, aggro list,
                  pathing (SetSegment/GetNextPos/GetRandomPos), GetCurrentScore/UpdateScore,
                  CheckGetLevel, GetEnemyFromView
TMSrv/CNPCGene.cpp  CNPCGenerator (ReadNPCGenerator/ParseString/ReadRegion/DropList/LevelList),
                  CNPCSummon
TMSrv/MobKilled.cpp  MobKilled(target, conn, PosX, PosY): EXP, party split, drops, boss chains,
                  ranking updates
```

## 5. Dependency Analysis

```
Internal:
  CMob ──> Basedef (STRUCT_MOB, STRUCT_AFFECT, BASE_GetMobAbility, g_pNextLevel)
  MobKilled ──> CItem/STRUCT_ITEM (drops), CUser (killer session), DBSrv ranking msg
  CNPCGenerator ──> filesystem (NPCGener.txt), GenerateMob, STL list<>
External:
  - C runtime rand() for EXP/drop RNG; iostream (cout) for load diagnostics
  - STL <list> for generator lists
```

## 6. Afferent and Efferent Coupling

Afferent = units depending on CMob; efferent = its dependencies. `STRUCT_MOB`/`pMob[]` is the
most depended-upon entity in the server because it represents every actor.

| Unit | Afferent (est.) | Efferent (est.) | Critical |
|------|-----------------|-----------------|----------|
| CMob / pMob[] (STRUCT_MOB) | Very High (combat, skills, items, players) | Basedef | High |
| MobKilled | High (every kill) | CMob, CItem, CUser, DBSrv, RNG | High |
| CNPCGenerator | Medium (startup + respawn) | filesystem, GenerateMob | Medium |

## 7. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| NPCGener.txt | Filesystem | Spawn/region/drop/level config | File IO | text | cout error if unreadable (`CNPCGene.cpp:182,199`) |
| DBSrv | Internal | EXP ranking persistence | CPSock binary | MSG_UpdateExpRanking | fire-and-forward |
| Game client | TCP (indirect) | Combat results, drops, EXP display | CPSock binary | mob/score/item messages | via SendFunc |

## 8. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| State machine | `Mode` (MOB_*) + processors | `CMob.h:26-35`, `CMob.cpp:75` | Per-mob AI behavior |
| Unified entity | `STRUCT_MOB` for players and monsters | `CMob.h:40,143` | Shared stat/score/combat code |
| Data-driven spawn | `CNPCGenerator` from text file | `CNPCGene.cpp:45` | Configurable world population |
| Object pool | `pMob[MAX_MOB]` | `CMob.h:143` | Pre-allocated actor slots |

## 9. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| High | Maintainability | `MobKilled` is a 3506-line function with deep nesting | Very hard to verify EXP/drop correctness |
| High | Balance hardcoding | EXP curve hardcoded in level-band branches (`MobKilled.cpp:483-504`) | Rebalancing requires code edits, not config |
| Medium | RNG | `rand()` used for drops/EXP rolls | Predictable/biased outcomes |
| Medium | Special-casing | Boss/quest behavior keyed by literal ids/`GenerateIndex` | Brittle, scattered constants |
| Low | Diagnostics | Load failures only print to cout | Easy to miss missing spawn data |

## 10. Test Coverage Analysis

No automated tests exist for the mob subsystem or any module in the repository (no test project,
framework, or `*test*` sources). The EXP/drop economy — central to game balance and a frequent
exploit target — is validated only at runtime. Recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| CMob / MobKilled / CNPCGenerator | 0 | 0 | None | No tests present in repo |
