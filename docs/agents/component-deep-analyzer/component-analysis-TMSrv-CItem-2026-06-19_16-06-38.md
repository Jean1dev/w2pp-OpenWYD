# Component Deep Analysis Report: TMSrv-CItem

**Component**: TMSrv-CItem (Item Subsystem)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/TMSrv/CItem.cpp` (34), `CItem.h` (46), `Source/Code/ItemEffect.h` (169),
plus item handlers `_MSG_UseItem.cpp`, `_MSG_CombineItem*.cpp`, `_MSG_GetItem.cpp`,
`_MSG_DropItem.cpp`, `_MSG_TradingItem.cpp`, `_MSG_Sell.cpp`, and item helpers in `Basedef.cpp`

---

## 1. Executive Summary

The item subsystem governs how items are represented, dropped on the ground, picked up,
equipped, used, refined, traded, and sold. Its pieces are spread across three layers:

- **Data model**: `STRUCT_ITEM` (in `Basedef.h`) is a compact record carrying `sIndex`
  (item id) and three dynamic effect slots `stEffect[3]` of `{cEffect, cValue}` pairs
  (`Basedef.h:513-520`), on top of per-id static effects (`MAX_STATICEFFECT`). The legal effect
  *types* are the `EF_*` constants enumerated in `ItemEffect.h` (requirements, bonuses, grades,
  trade flags, mount stats, dates).
- **World entity**: `CItem` (`CItem.h:24`) wraps a `STRUCT_ITEM` with world placement
  (`PosX/PosY`, `Decay`, `State`, `Money`, `Open`), and the global array `pItem[MAX_ITEM]`
  (`CItem.h:45`) holds items lying on the ground awaiting pickup or decay.
- **Behavior**: the rules live in the `_MSG_*` item handlers and `BASE_GetItemAbility` /
  `BASE_GetItemSanc` / `BASE_SetItemSanc` helpers in `Basedef.cpp`.

The most intricate rule is **refinement** ("sanc"/+level): items are upgraded probabilistically
using a per-level rate table `g_pCelestialRate[]`, capped per item class, consuming a refine
stone on each attempt (`_MSG_UseItem.cpp:223-310`). A notable observation is that the per-use
1-second anti-spam cooldown in the refine path is **commented out** (`_MSG_UseItem.cpp:209-221`),
leaving refine/use actions ungated by that timer.

---

## 2. Data Flow Analysis

```
Definition load:  item data files -> static effect tables (BASE_*), consumed by handlers
Ground item:      mob death / drop -> CItem instance in pItem[MAX_ITEM] (PosX,PosY,Decay)
Pickup:           _MSG_GetItem -> validate -> move STRUCT_ITEM into CUser.SelChar inventory
Equip/use:        _MSG_UseItem -> BASE_GetItemAbility checks (EF_LEVEL/EF_CLASS/EF_POS/...)
                    -> apply effects to character score (pMob[conn].GetCurrentScore)
Refine:           _MSG_UseItem / _MSG_CombineItem* -> sanc cap check -> rand vs g_pCelestialRate
                    -> success: BASE_SetItemSanc(dest, +1); consume stone
                    -> failure: _NN_Fail_To_Refine (item outcome per branch)
Trade:            _MSG_Trade/_MSG_TradingItem -> EF_NOTRADE gate -> swap items between sessions
Sell/drop:        _MSG_Sell / _MSG_DropItem -> remove from inventory; ground/coin update
```

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| Data model | Item has 3 dynamic effect slots `stEffect[3]` {cEffect,cValue} | `Basedef.h:513-520` |
| Validation | Mutable effect types restricted to a whitelist | `Basedef.cpp:1854,2084` |
| Equip | Refine target must be an equip; non-volatile-only constraints | `_MSG_UseItem.cpp:172-185` |
| Refine cap | Sealed items cap at sanc 9; sanc>=6 & Vol==4 blocked | `_MSG_UseItem.cpp:202-237` |
| Refine odds | Success if `rand()%115` (adjusted) <= `g_pCelestialRate[level]` | `_MSG_UseItem.cpp:277-283` |
| Refine effect | On success `BASE_SetItemSanc(dest, ref+1, 0)`, consume one stone | `_MSG_UseItem.cpp:285-300` |
| Anti-spam (disabled) | 1-second refine cooldown is commented out | `_MSG_UseItem.cpp:209-221` |
| Trade | `EF_NOTRADE` items cannot be traded | `ItemEffect.h:170`, trade handlers |
| Economy | `EF_DONATE` marks cash-shop items | `ItemEffect.h:151` |
| Volatile/consumable | `EF_VOLATILE` distinguishes refine level / consumable class | `ItemEffect.h:94`, `_MSG_GetItem.cpp:181` |

### Detailed breakdown

---

### Business Rule: Item Effect / Attribute Model

**Overview**: Item power and constraints are expressed as `(EF_* type, value)` pairs rather than
named fields, allowing a generic engine to apply any attribute.

**Detailed description**: Each `STRUCT_ITEM` carries three runtime effect slots accessed via the
`EF1/EFV1 … EF3/EFV3` macros (`Basedef.h:515-520`); the engine also consults per-item static
effects. `ItemEffect.h` defines the full vocabulary: stat bonuses (`EF_DAMAGE`, `EF_AC`, `EF_HP`,
`EF_MP`, `EF_STR/INT/DEX/CON`), requirements (`EF_LEVEL`, `EF_REQ_STR/INT/DEX/CON`, `EF_CLASS`,
`EF_POS`), combat modifiers (`EF_ATTSPEED`, `EF_RANGE`, `EF_CRITICAL`, `EF_HITRATE`, `EF_PARRY`,
resistances), refinement/lifecycle (`EF_VOLATILE`, `EF_GRADE0..5`, `EF_INCUBATE`), trade/economy
(`EF_NOTRADE`, `EF_DONATE`, `EF_HWORDCOIN/LWORDCOIN`), mount stats (`EF_MOUNT*`), and time-bound
fields (`EF_WDAY/HOUR/MIN/YEAR/WMONTH`). Reads go through `BASE_GetItemAbility(item, EF_*)`
(e.g., `_MSG_UseItem.cpp:95`, `_MSG_GetItem.cpp:181`). The set of effect types that may be
*written* onto an item is whitelisted in `Basedef.cpp:1854` and `:2084` (grid, class, pos, wtype,
range, level, requirements, volatile, incubate, item-level, notrade, nosanc, donate), which
constrains what mutators may legitimately change.

**Rule workflow**: read item → for each needed attribute call BASE_GetItemAbility → apply to character/score.

---

### Business Rule: Refinement ("Sanc" / +Level Upgrade)

**Overview**: Equipment is upgraded through a probabilistic refine that raises the item's
"sanc" (sanctification / plus level) up to a class-dependent cap, consuming a refine stone.

**Detailed description**: The refine path validates that the destination (`dest`) is an equip and
that it is not itself a volatile/stone (`_MSG_UseItem.cpp:172-185`), then reads the current level
with `BASE_GetItemSanc(dest)`. Caps are enforced per category: generic items blocked when
`sanc >= 6 && Vol == 4` (`:202`), and "sealed"/special id ranges (itemtype 5, e.g. ids
1234-1237, 1369-1372, 1519-1522, 1669-1672, 1901-1910, 1714) blocked at `sanc >= 9` (`:225-237`).
Over the cap returns `_NN_Cant_Refine_More`. The success roll computes `_rd = rand()%115` (values
over 100 are reduced by 15) and compares it to `_chance = g_pCelestialRate[ref]`, where `ref` is
the current level; success when `_rd <= _chance` (`_MSG_UseItem.cpp:277-283`). On success it sets
`BASE_SetItemSanc(dest, ref+1, 0)`, recomputes the character score
(`pMob[conn].GetCurrentScore`), logs the refine, and consumes one refine stone via
`BASE_SetItemAmount(item, amount-1)` or clears the slot (`:285-300`). Failure emits
`_NN_Fail_To_Refine`. The NPC craft variants (`_MSG_CombineItem*` — Agatha, Ailyn, Alquimia,
Ehre, Lindy, Odin, Shany, Tiny, Extracao) implement parallel refine/craft recipes with their own
caps (e.g. `_MSG_CombineItemOdin.cpp:667`).

**Rule workflow**: select stone+target → cap check → roll rand vs rate table → success: +1 level & consume stone → failure: notify.

---

### Business Rule: Pickup, Drop, Decay of Ground Items

**Overview**: Items in the world are entities with position and a decay lifetime, picked up
subject to ownership/space checks.

**Detailed description**: Ground items are `CItem` entries in `pItem[MAX_ITEM]` carrying
`PosX/PosY`, `Decay`, `State`, and `Money` (`CItem.h:29-37`). `_MSG_GetItem` validates pickup
(distance, inventory space, ownership/lock window) before transferring the `STRUCT_ITEM` into the
player's inventory; `_MSG_DropItem` performs the reverse, placing an item on the ground. Item
metadata such as `EF_VOLATILE` is read during pickup (`_MSG_GetItem.cpp:181`). Decay timers cause
abandoned items to disappear.

**Rule workflow**: drop → create ground CItem with decay → pickup validates → move to inventory or expire.

---

### Business Rule: Trade and No-Trade Constraints

**Overview**: Player trading moves items between two sessions, but certain items are
non-tradable.

**Detailed description**: `_MSG_Trade` / `_MSG_TradingItem` drive the offer/confirm/commit
trade flow, moving items between the two `CUser` sessions' inventories. Items flagged with
`EF_NOTRADE` (`ItemEffect.h:170`) are blocked from being placed in a trade, and donate/quest
items have similar restrictions. `_MSG_Sell` reads `EF_VOLATILE` (`_MSG_Sell.cpp:134`) when
pricing/destroying items at NPC shops.

**Rule workflow**: place item in trade → check EF_NOTRADE/quest/donate → allow or reject → on mutual confirm swap.

---

## 4. Component Structure

```
Item data model:   Basedef.h STRUCT_ITEM { sIndex; stEffect[3]{cEffect,cValue} }  (:513-520)
Effect vocabulary: ItemEffect.h  EF_* constants (1..127)
World entity:      TMSrv/CItem.h class CItem { STRUCT_ITEM ITEM; PosX/Y; Decay; State; Money; ... }
                   global pItem[MAX_ITEM]
Helpers:           Basedef.cpp BASE_GetItemAbility / BASE_GetItemSanc / BASE_SetItemSanc /
                   BASE_SetItemAmount / BASE_CheckItemDate
Handlers (TMSrv/):
  _MSG_UseItem.cpp (6686)   # use/refine/anvil
  _MSG_CombineItem*.cpp     # NPC craft/refine recipes (10 variants)
  _MSG_GetItem.cpp (252)    # pickup
  _MSG_DropItem.cpp (146)   # drop
  _MSG_Trade.cpp / _MSG_TradingItem.cpp (491) / _MSG_QuitTrade.cpp  # trading
  _MSG_SplitItem.cpp / _MSG_UpdateItem.cpp / _MSG_DeleteItem.cpp / _MSG_Sell.cpp
```

## 5. Dependency Analysis

```
Internal:
  CItem ──> Basedef (STRUCT_ITEM)
  item handlers ──> BASE_GetItemAbility/Sanc (Basedef.cpp), CUser.SelChar inventory, pMob score
  refine ──> g_pCelestialRate[] (rate table), g_pMessageStringTable (client messages)
External:
  - C runtime rand() for refine RNG
```

## 6. Afferent and Efferent Coupling

Afferent = units depending on the item subsystem; efferent = its own dependencies. The
`STRUCT_ITEM` model and `BASE_GetItemAbility` helper have very high afferent coupling (mobs,
players, trade, shop, drop all use them).

| Unit | Afferent (est.) | Efferent (est.) | Critical |
|------|-----------------|-----------------|----------|
| STRUCT_ITEM / EF_* model | Very High (players, mobs, trade, shop, drops) | none | High |
| BASE_GetItemAbility/Sanc | Very High (all item handlers) | Basedef tables | High |
| CItem (ground entity) | High (drop/pickup/mob death) | Basedef | Medium |
| Refine logic (_MSG_UseItem) | Medium | item helpers, RNG, score | Medium |

## 7. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| Game client | TCP | Item actions (use/get/drop/trade/refine) | CPSock binary | item message structs | client messages (`_NN_*`), item resync via SendItem |
| Item data files | Filesystem | Static item/effect definitions | File IO | binary tables | loaded at startup |
| DBSrv | Internal | Inventory persistence on save | CPSock binary | STRUCT_ITEM in SELCHAR | save-on-quit |

## 8. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| Type-tag attribute model | `(EF_* , value)` effect slots | `Basedef.h:513`, `ItemEffect.h` | Generic, data-driven item attributes |
| Accessor helpers | `BASE_GetItemAbility/Sanc` | `Basedef.cpp` | Centralized item attribute reads |
| Object pool | `pItem[MAX_ITEM]` ground items | `CItem.h:45` | Pre-allocated world item slots |
| Strategy-ish variants | `_MSG_CombineItem*` per NPC | TMSrv/ | Distinct craft/refine recipes |

## 9. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| High | Anti-spam | Refine/use 1-second cooldown commented out (`_MSG_UseItem.cpp:209-221`) | Unthrottled refine/use actions |
| High | Duplication | Item moves rely on client-supplied slot positions across handlers | Item-dup exposure if any handler mis-validates |
| Medium | Complexity | Deeply nested refine conditions with ambiguous precedence (`_MSG_UseItem.cpp:241-247`) | Hard-to-verify correctness |
| Medium | Fragmentation | 10 near-duplicate `_MSG_CombineItem*` handlers | Inconsistent rule drift between variants |
| Medium | RNG | `rand()` used for refine outcomes | Predictable/biased odds |

## 10. Test Coverage Analysis

No automated tests exist for the item subsystem or any module in the repository (no test
project, framework, or `*test*` sources). The refine economy and item-move handlers — high-value
targets for exploits — are validated only at runtime. Recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| CItem / item handlers | 0 | 0 | None | No tests present in repo |
