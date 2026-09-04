# Quest Item Rewards Specification

## Purpose

Define legacy-compatible rewards, party sharing, validation, consumption, and inventory stacking for the quest reward item family.

## Requirements

### Requirement: Quest reward configuration
The system SHALL associate item IDs `4117` through `4121`, in ascending order, with quest tiers `0` through `4` and SHALL obtain each tier's Mortal and Arch experience reward, gold reward, and Mortal and Arch level range from `Common/Settings/QuestsRate.txt` when a content tree is configured.

#### Scenario: Configured quest rates are loaded
- **WHEN** the server starts with a content tree containing a valid `Common/Settings/QuestsRate.txt`
- **THEN** uses of items `4117` through `4121` use the corresponding configured `Exp`, `Coin`, and `Level` values

#### Scenario: Configured quest rates are unavailable
- **WHEN** the server starts without a configured content tree
- **THEN** the system uses the legacy compiled default quest rates

#### Scenario: Configured quest rates are invalid
- **WHEN** the server starts with a configured content tree whose `QuestsRate.txt` is missing or invalid
- **THEN** startup fails with an error identifying the quest-rate content problem

### Requirement: Eligible quest item use
The system SHALL allow a living, actively playing character to use a quest reward item only when the character's class tier and current level select a configured range whose minimum is inclusive and maximum is exclusive.

#### Scenario: Mortal uses an item inside its level range
- **WHEN** a Mortal uses item `4117` through `4121` at a level greater than or equal to that tier's Mortal minimum and less than its Mortal maximum
- **THEN** the use succeeds with the tier's Mortal reward

#### Scenario: Arch uses an item inside its level range
- **WHEN** an Arch uses item `4117` through `4121` at a level greater than or equal to that tier's Arch minimum and less than its Arch maximum
- **THEN** the use succeeds with the tier's Arch reward

#### Scenario: Character is outside the item level range
- **WHEN** a character uses a quest reward item below the applicable minimum or at or above the applicable maximum
- **THEN** the system refuses the use, informs the character of the level restriction, synchronizes the unchanged inventory slot, and grants no reward

### Requirement: Successful quest item reward
On a successful quest item use, the system SHALL add the tier's full configured experience and gold reward to the consuming character, subject to the existing experience and carried-gold ceilings, SHALL apply all resulting level progression, and SHALL synchronize the character's visible reward and state.

#### Scenario: Reward does not reach a ceiling
- **WHEN** an eligible character successfully uses a quest reward item and both rewards fit below their respective ceilings
- **THEN** the character receives the full configured experience and gold and receives the corresponding EXP notice, reward emotion, level-up updates when applicable, and current character state

#### Scenario: Reward reaches a ceiling
- **WHEN** adding a quest item reward would exceed the experience or carried-gold ceiling
- **THEN** the affected total is capped at that ceiling and the synchronized state reports the capped total

#### Scenario: Reward crosses multiple level thresholds
- **WHEN** the granted experience crosses one or more level thresholds permitted by the character's tier and quest gates
- **THEN** the system applies every permitted level-up through the existing level progression rules

### Requirement: Quest experience party share
After a successful use, the system SHALL grant experience equal to integer division of the consumer's configured experience reward by 10 to each other actively playing character in the same party. The consumer SHALL NOT receive this party share in addition to the full reward, and gold SHALL NOT be shared.

#### Scenario: Party leader consumes an item
- **WHEN** a party leader successfully uses a quest reward item
- **THEN** each connected active party member other than the leader receives the 10% experience share exactly once

#### Scenario: Party member consumes an item
- **WHEN** a non-leader party member successfully uses a quest reward item
- **THEN** the leader and every other connected active party member receive the 10% experience share exactly once, while the consumer receives only the full configured reward

#### Scenario: Character uses an item without a party
- **WHEN** a character who is not in a party successfully uses a quest reward item
- **THEN** no party experience share is granted

#### Scenario: Party contains inactive entries
- **WHEN** party state contains disconnected, missing, or non-playing characters
- **THEN** those entries receive no experience share and do not prevent eligible active members from receiving theirs

#### Scenario: Party share causes level progression
- **WHEN** a party recipient's 10% experience share crosses a permitted level threshold
- **THEN** the system applies the recipient's normal level progression and synchronizes the recipient's EXP notice, reward emotion, and character state

### Requirement: Quest item stack behavior
The system SHALL allow each item ID `4117` through `4121` to merge with an otherwise compatible item of the same ID, split into separate stacks, and hold no more than the existing maximum stack amount.

#### Scenario: Compatible quest item stacks are merged
- **WHEN** a player moves one quest item stack onto a compatible stack with the same item ID
- **THEN** the system transfers as many units as fit up to the existing stack limit and preserves any remainder

#### Scenario: Quest item stack is split
- **WHEN** a player requests a valid split amount smaller than the source quest item stack and has an empty inventory slot
- **THEN** the system reduces the source by that amount and creates a second stack containing the requested amount

### Requirement: Quest item consumption
The system SHALL consume exactly one unit from a quest item stack only after the reward use succeeds and SHALL synchronize and persist the remaining stack or empty slot.

#### Scenario: Successful use from a multi-item stack
- **WHEN** an eligible character successfully uses one quest reward item from a stack containing more than one unit
- **THEN** the stack remains in its slot with its amount reduced by exactly one

#### Scenario: Successful use of the final unit
- **WHEN** an eligible character successfully uses a quest reward item whose effective amount is one
- **THEN** the inventory slot becomes empty

#### Scenario: Rejected use preserves the stack
- **WHEN** use of a quest reward item is rejected
- **THEN** no unit is consumed and the authoritative unchanged stack is sent back to the client

