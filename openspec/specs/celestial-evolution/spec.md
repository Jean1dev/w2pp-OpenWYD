# Celestial Evolution Specification

## Purpose

Define the legacy-compatible Ideal Stone transition from Arch to Celestial so valid characters evolve reliably with complete, persistent starting state.

## Requirements

### Requirement: Ideal Stone validates all Arch prerequisites
Right-clicking item 1742 SHALL evolve only an Arch at stored level 355 or higher whose body armor slot is empty and whose persisted Mortal progression level is at least 99. A rejected attempt MUST preserve the stone and provide visible feedback.

#### Scenario: Minimum eligible Arch evolves
- **WHEN** an Arch at stored level 355 with an empty body armor slot and Mortal progression level at least 99 right-clicks an Ideal Stone
- **THEN** the server begins the Arch-to-Celestial transition

#### Scenario: Arch is below the minimum level
- **WHEN** an Arch below stored level 355 right-clicks an Ideal Stone
- **THEN** the stone and character state remain unchanged and the player receives visible requirement feedback

#### Scenario: Body armor slot is occupied
- **WHEN** an otherwise eligible Arch right-clicks an Ideal Stone while the body armor slot is occupied
- **THEN** the stone and character state remain unchanged and the player receives visible armor feedback

#### Scenario: Mortal progression is insufficient
- **WHEN** an otherwise eligible Arch with Mortal progression level below 99 right-clicks an Ideal Stone
- **THEN** the stone and character state remain unchanged and the player receives visible requirement feedback

### Requirement: Evolution records the Arch achievement band
The transition SHALL persist the legacy Arch-level band: 1 for levels 355-369, 2 for 370-379, 3 for 380-397, 4 for level 398, and 5 for level 399 or higher.

#### Scenario: Each Arch level band is converted
- **WHEN** eligible Arch characters from each defined level band evolve
- **THEN** each resulting Celestial stores the corresponding band value

### Requirement: Evolution creates complete Celestial starting state
The transition SHALL set the tier to Celestial and stored level and experience to zero, reset base attributes, mastery allocations, damage, armor, HP, MP, learned skills, free-point state, and Celestial quest gates to their legacy starting values, set the legacy Celestial special bonus, and consume exactly one Ideal Stone.

#### Scenario: Successful evolution resets character state
- **WHEN** an eligible Arch evolves
- **THEN** no Arch level allocation or disallowed learned skill remains in the resulting Celestial state
- **AND** the character has the class-appropriate base attributes, HP and MP, base armor 230, base damage 0, special bonus 855, and the legacy retained skill bit

#### Scenario: Stacked Ideal Stone is used
- **WHEN** the eligible Arch uses an Ideal Stone stack with quantity greater than one
- **THEN** exactly one unit is consumed and the remaining stack is returned to inventory

### Requirement: Evolution grants level-band and kingdom equipment
The resulting Celestial SHALL receive body item 3500 for Arch levels below 380, item 3501 for levels 380-398, or item 3502 for level 399 or higher. The cape slot SHALL become item 3197 for Hekalotia, 3198 for Akelonia, or 3199 for a neutral character, and the legacy body visual effects SHALL be applied.

#### Scenario: Level and kingdom determine equipment
- **WHEN** an eligible Arch evolves
- **THEN** the emitted and persisted body item and cape match the source level band and kingdom

### Requirement: Evolution is persisted before session transition
All tier, progression, score, skill, inventory, equipment, and visual-effect changes MUST be persisted as one successful character outcome before the server returns the player to character selection. Blocking persistence work MUST execute outside the single-owner game loop, with completion applied back inside the loop.

#### Scenario: Successful save returns to character selection
- **WHEN** every evolution state change is persisted successfully
- **THEN** nearby clients receive the required entity refresh, the player returns to character selection, and subsequent login loads the complete Celestial state

#### Scenario: Persistence fails
- **WHEN** persistence cannot commit the complete evolution outcome
- **THEN** the server MUST NOT expose a partially evolved or item-consuming character state and SHALL report the failure to the player
