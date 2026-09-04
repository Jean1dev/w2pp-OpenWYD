## 1. Quest Rate Content

- [x] 1.1 Add the typed five-tier quest-rate model with legacy compiled defaults and verify unit tests cover the default EXP, coin, and Mortal/Arch level values.
- [x] 1.2 Implement strict `QuestsRate.txt` parsing and overlays for `Exp`, `Coin`, and `Level`, and verify parser tests cover valid shipped content, case handling, missing rows that retain defaults, invalid indices, invalid numeric bounds, malformed directives, and unusable ranges.
- [x] 1.3 Load `Common/Settings/QuestsRate.txt` with the required content tree, expose the rates through tmserver and dispatcher configuration, retain defaults in no-content mode, and verify startup/content tests cover configured, missing, malformed, and fallback cases.

## 2. Reward Application

- [x] 2.1 Add a reusable direct-EXP award path that clamps the applied amount, runs existing multi-level progression, and emits the required EXP, score/ETC, and emotion updates; verify focused tests cover ordinary gain, multiple level-ups, tier gates, and the EXP ceiling.
- [x] 2.2 Dispatch `EF_VOLATILE=191` to a quest-item handler that safely maps only IDs `4117..4121` to tiers, selects the applicable configured class columns, validates the inclusive-minimum/exclusive-maximum level range, and verify table-driven tests cover every tier and both boundaries.
- [x] 2.3 Grant the successful consumer's configured EXP and coin with existing ceilings and state synchronization, and verify tests cover configured values, gold capping, EXP capping, and rejection with no mutation.

## 3. Party Distribution

- [x] 3.1 Resolve leader/member/solo party membership inside the world loop, distribute `questExp / 10` to each distinct active recipient except the consumer, and verify tests cover leader use, member use, solo use, duplicate entries, disconnected entries, and the absence of shared gold.
- [x] 3.2 Apply level progression and client notifications to each party recipient, and verify a party-share regression test covers a recipient crossing a level threshold.

## 4. Stacking and Consumption

- [x] 4.1 Add item IDs `4117..4121` to the existing merge/split policy and verify table-driven inventory tests cover every ID, compatible merges, the 120-unit cap with remainder, valid splits, and invalid split preservation.
- [x] 4.2 Consume and synchronize exactly one unit only after a successful quest reward, and verify handler tests cover multi-unit stacks, the final unit, rejected use, and a subsequent inventory move preserving the authoritative reduced amount.

## 5. Verification

- [x] 5.1 Run `go test -run 'Test.*Quest(Item|Rate|Reward)' ./tmserver/internal/content ./tmserver/internal/handler` (adjusting the focused pattern to the implemented test names) and verify all focused tests pass.
- [x] 5.2 Run `make test` and verify the repository test suite, race detection, and coverage complete without regressions.
