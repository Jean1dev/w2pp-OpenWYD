# Arch Crystal Quest

## Purpose

Restore the four sequential Arch crystal quests with persistent progress and rewards that survive reconnection without duplication.

## Requirements

### Requirement: Crystal stages are sequential and persistent
The server SHALL allow an Arch with stored level at least 355 to use crystal items 4106 through 4109 only in order, persist a stage from 0 through 4, and reject repeats or out-of-order items without consuming them.

#### Scenario: Wrong crystal order or ineligible character
- **WHEN** a player uses a crystal other than the next required item, is not Arch, or is below stored level 355
- **THEN** the item and character state remain unchanged and the player receives a readable explanation

### Requirement: Crystal completion applies legacy rewards
On each successful stage the server SHALL consume one crystal, subtract 100000000 EXP without going below zero, and apply the legacy bonuses: stage 1 +80 MP, stage 2 +30 AC, stage 3 +80 HP, and stage 4 +60 HP/+60 MP/+20 AC.

The resulting stored level SHALL be the lesser of the current level and the level supported by the remaining EXP on the Arch curve. For each lost level, the server SHALL reverse the class HP/MP increments (with a zero floor), reconstruct AC from the resulting level and crystal stage, recalculate free attribute and skill points, and subtract two free specialization points with a zero floor. Allocated attributes, mastery, learned skills, quest stages and Arch unlock flags SHALL be preserved; no point debt SHALL be introduced. Current HP/MP SHALL be capped to the resulting effective maxima without healing. These changes SHALL be committed atomically with the crystal consumption before publishing success. This replaces the no-deleveling decision recorded in issue #327 and applies only to future crystal uses.

#### Scenario: Remaining EXP crosses a level boundary
- **WHEN** a successful crystal use leaves less EXP than required for the current level
- **THEN** the level and its associated gains decrease, crystal rewards remain, and the resulting state is identical after relogin

#### Scenario: Recover eligibility for the next crystal
- **WHEN** a crystal reduces the stored level below 355
- **THEN** the next crystal is rejected until the character regains stored level 355, without resetting completed stages or unlock flags

#### Scenario: Remaining EXP sustains the current level
- **WHEN** the remaining EXP is at or above the minimum for the current level
- **THEN** the level is unchanged, even if excess EXP would support a higher level

#### Scenario: Complete all crystals
- **WHEN** an eligible Arch uses 4106, 4107, 4108, then 4109
- **THEN** stage becomes 4, each item is consumed once, and all four bonuses are present after relogin

#### Scenario: Persistence fails
- **WHEN** a valid crystal use cannot be saved
- **THEN** the crystal, quest stage, EXP, level and attributes remain unchanged and the player can retry

#### Scenario: Evolution removes Arch bonuses
- **WHEN** an Arch with crystal bonuses evolves to Celestial
- **THEN** the Celestial receives fresh base stats while retaining the historical crystal stage
