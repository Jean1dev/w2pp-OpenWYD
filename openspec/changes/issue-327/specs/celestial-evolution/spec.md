## MODIFIED Requirements

### Requirement: Evolution grants level-band and kingdom equipment
With all four Arch crystal stages complete, the resulting Celestial SHALL receive body item 3500 for stored Arch levels below 380, item 3501 for levels 380-398, or item 3502 for level 399 or higher. With fewer than four stages complete, the body item SHALL instead be Silver Cythera 3500. All granted body items SHALL be +0. The cape slot SHALL become item 3197 for Hekalotia, 3198 for Akelonia, or 3199 for a neutral character, and the legacy body visual effects SHALL be applied. Crystal completion SHALL NOT be an eligibility requirement or alter the recorded Arch achievement band.

#### Scenario: Level and kingdom determine equipment
- **WHEN** an eligible Arch with all four crystal stages complete evolves
- **THEN** the emitted and persisted body item and cape match the source level band and kingdom

#### Scenario: Incomplete quest incurs a penalty
- **WHEN** an eligible Arch with zero through three crystal stages complete evolves
- **THEN** the character receives Silver Cythera 3500 +0 regardless of source level, including stored level 399
- **AND** evolution still persists successfully and records the source Arch achievement band

## ADDED Requirements

### Requirement: Arch locks prevent bypass
Arch level progression SHALL stop at stored levels 354 and 369 until the corresponding Lindy flags are set, including when one reward crosses several level thresholds. Conventional EXP gains SHALL be rejected while a reached lock remains pending. Fairy Dust used while a pending lock exists SHALL preserve both the dust and EXP and instruct the player to complete the Lindy unlock. Existing above-boundary characters SHALL keep their level but SHALL NOT gain further levels until the reached pending locks are cleared. Other tiers SHALL retain their existing progression behavior.

#### Scenario: Locked Fairy Dust
- **WHEN** an Arch reaches a locked boundary and uses Fairy Dust
- **THEN** the dust and experience remain unchanged

#### Scenario: Large EXP reward
- **WHEN** a single reward would advance an Arch past a pending lock
- **THEN** level progression stops at that boundary

### Requirement: Lindy unlocks are ordered and persistent
Lindy SHALL complete only the earliest pending unlock per interaction at or above its stored boundary level. Each unlock SHALL consume the existing seven-position recipe with distinct inventory slots. The first unlock SHALL assign cape 3191/3192/3193 for Hekalotia/Akelonia/neutral and refresh its visual for both the player and nearby observers. The second SHALL require and consume one Fame and preserve the cape. Invalid or repeated requests SHALL preserve ingredients. Save failures SHALL preserve ingredients, flags, fame and equipment.

#### Scenario: Late unlock
- **WHEN** an Arch already above a boundary completes the corresponding Lindy unlock
- **THEN** the flag and required cape/fame changes persist without lowering the character level

#### Scenario: First unlock refreshes nearby clients
- **WHEN** the first unlock commits successfully
- **THEN** the player's client and nearby observers receive updated cape equipment and the cape survives reconnection

#### Scenario: Unlock rejected or save fails
- **WHEN** the recipe, level, flags or Fame are invalid, or the unlock cannot be saved
- **THEN** no ingredients, flags, Fame or equipment are changed
