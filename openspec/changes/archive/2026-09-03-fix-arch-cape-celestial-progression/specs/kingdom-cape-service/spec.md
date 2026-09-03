## Purpose

Restore the Kings' sapphire-funded cape service so characters can join and maintain a kingdom while the shared price encourages balanced faction distribution.

## ADDED Requirements

### Requirement: King quotes the current kingdom price
Each King SHALL quote the sapphire cost for that King's kingdom before confirmation. The two kingdom prices SHALL move inversely, remain within 4 and 32 sapphire units inclusive, and be derived from shared persisted balance state rather than only the current process's online population.

#### Scenario: Player asks for the price
- **WHEN** an eligible player opens a King interaction without confirming it
- **THEN** the King reports that kingdom's current sapphire cost without consuming items or changing state

#### Scenario: Opposing prices reflect imbalance
- **WHEN** persisted balance state makes one kingdom more populated than the other
- **THEN** joining the more populated kingdom costs more and joining the less populated kingdom costs less, with both costs clamped to `[4,32]`

### Requirement: Sapphire denominations are counted and consumed atomically
The service SHALL count item 697 as one sapphire unit and item 4131 as ten units. It MUST validate the full cost before changing inventory, cape, clan balance, or persisted state, and a successful operation MUST consume no more than the quoted cost.

#### Scenario: Insufficient sapphires
- **WHEN** a player confirms the service with fewer sapphire units than the quoted cost
- **THEN** the server preserves all inventory and cape state and sends visible cost feedback

#### Scenario: Mixed denominations satisfy the cost
- **WHEN** a player has a sufficient combination of one-unit and ten-unit sapphire items and confirms the service
- **THEN** the server consumes the required value, updates the cape, and emits inventory and equipment updates

#### Scenario: Ten-unit item exceeds the remaining cost
- **WHEN** consuming a ten-unit sapphire item would exceed the remaining quoted cost
- **THEN** the service MUST preserve value through a deterministic change or denomination strategy rather than silently destroying excess sapphire value

### Requirement: King applies legacy-compatible cape transitions
The service SHALL classify the equipped cape into its legacy kingdom and cape mode, reject interaction with the opposing King, preserve already-completed capes, and apply the cape appropriate to the character tier and selected kingdom.

#### Scenario: Neutral eligible cape joins Hekalotia
- **WHEN** a character with an eligible neutral cape confirms Hekalotia's service and pays the quoted price
- **THEN** the equipped cape becomes the corresponding Hekalotia cape and the shared balance state records the choice

#### Scenario: Neutral eligible cape joins Akelonia
- **WHEN** a character with an eligible neutral cape confirms Akelonia's service and pays the quoted price
- **THEN** the equipped cape becomes the corresponding Akelonia cape and the shared balance state records the choice

#### Scenario: Opposing kingdom cape is presented
- **WHEN** a cape identifies the character with the kingdom opposing the selected King
- **THEN** the server makes no inventory, cape, or balance change

#### Scenario: Cape service is already complete
- **WHEN** the equipped cape is already in the completed mode for that King
- **THEN** the King acknowledges the character without charging sapphires or altering balance state

### Requirement: Balance updates are safe under concurrent requests
The persisted kingdom balance adjustment and the character's payment/cape result MUST have a single committed outcome, and a failed balance update MUST NOT leave the character charged without the corresponding cape result.

#### Scenario: Persistence rejects a balance update
- **WHEN** the shared balance update fails during a confirmed cape purchase
- **THEN** the server preserves or restores the character's sapphires and cape and reports failure

