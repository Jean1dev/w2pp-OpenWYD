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

#### Scenario: Complete all crystals
- **WHEN** an eligible Arch uses 4106, 4107, 4108, then 4109
- **THEN** stage becomes 4, each item is consumed once, and all four bonuses are present after relogin

#### Scenario: Persistence fails
- **WHEN** a valid crystal use cannot be saved
- **THEN** the crystal, quest stage, EXP and attributes remain unchanged and the player can retry

#### Scenario: Evolution removes Arch bonuses
- **WHEN** an Arch with crystal bonuses evolves to Celestial
- **THEN** the Celestial receives fresh base stats while retaining the historical crystal stage
