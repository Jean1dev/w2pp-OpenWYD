## Context

See `proposal.md` for motivation and `specs/quest-item-rewards/spec.md` for observable behavior. The legacy server classifies items `4117..4121` with `EF_VOLATILE=191`, maps `itemID - 4117` to five rows in `QuestsRate.txt`, and performs the reward in `_MSG_UseItem.cpp`. The Go server already has content-rate loaders, item dispatch, party state, level progression, EXP notification, slot synchronization, and generic amount helpers, but volatile 191 is not dispatched and the item IDs are absent from the stack whitelist.

All reward and inventory mutations must remain inside the single-owner world loop. No new asynchronous work or world-state locking is needed.

## Goals / Non-Goals

**Goals:**

- Keep quest reward values operator-configurable through the shipped legacy content file.
- Reuse one tier-aware reward path for the consumer and party recipients so caps, level-ups, and client state remain consistent.
- Make reward application and single-unit consumption one loop-owned operation.
- Cover the complete five-item family even though issue #304 directly reports only its first three tiers.

**Non-Goals:**

- Implement or repair the quest arenas, drops, or Hidra second-floor teleport.
- Change the CPSock protocol, persistence schema, general monster EXP distribution, or general party rules.
- Reproduce the legacy source's accidental extra 10% award to a solo consumer or consuming leader.
- Require a fairy or other equipped item before quest rewards can stack.

## Decisions

### Add a typed quest-rate content model

Add a five-row content type and loader alongside the existing `CompRate` and `SancRate` loaders. Start from the defaults compiled in `CReadFiles.cpp`, then overlay valid `Exp`, `Coin`, and `Level` directives from `QuestsRate.txt`, matching the legacy parser's configuration model while validating row indices, numeric bounds, and usable level intervals.

When `-content` is supplied, `loadContent` will treat this shipped settings file like the other required rate files and pass the loaded table into the handler configuration. Without a content tree, the dispatcher will use a default table so tests and no-content development mode remain functional.

Alternative considered: hard-code the shipped `Release` values directly in the handler. This would ignore an existing operator configuration surface and diverge whenever server rates are tuned.

### Dispatch the whole volatile-191 family through one handler

Add a volatile 191 dispatch case, then validate the concrete index is in `4117..4121` before deriving its tier. This avoids indexing malformed or future volatile-191 items outside the configured table.

The handler will select Mortal columns for Mortal and Arch columns for Arch. Other class tiers are not given an invented mapping: they will follow the legacy active selection behavior, which uses the non-Mortal columns, unless later capture evidence establishes a different rule.

Alternative considered: switch on each item ID separately. That duplicates identical behavior and makes tier coverage easier to drift.

### Centralize direct experience award behavior

Introduce or extract a helper for a known direct EXP amount that clamps to the tier's existing experience ceiling, emits the EXP panel for the amount actually applied, invokes the existing multi-level progression routine, and sends the required score/ETC and reward emotion state. Use it for both the consumer and party recipients.

Quest EXP is a fixed configured award, so it must not pass through monster level scaling, equipment/fairy EXP bonuses, or global monster EXP events.

Alternative considered: construct a synthetic monster and call the kill-reward path. That would incorrectly apply PvE scaling and bonuses and would obscure the fixed-reward contract.

### Resolve the party once and exclude the consumer

Normalize the consumer's party leader as follows: a member uses its nonzero `Leader`; a leader is recognized by its nonempty `PartyList`; otherwise the consumer is solo. Award the integer `questExp / 10` share to the active leader and active member entries, deduplicating connection IDs and excluding the consumer.

This deliberately follows the issue's “demais membros” requirement rather than the legacy block that can award an additional share to the consumer. Party membership is enough; no proximity restriction is added because the legacy reward path contains none.

Alternative considered: literal line-for-line legacy behavior. It would grant 110% to a solo character or consuming leader, contradicting the reported expected behavior.

### Extend the existing stack policy

Add `4117..4121` to the same `isSplittable` policy used by inventory merge and split. Reuse `EF_AMOUNT`, the existing compatible-effect comparison, and the current maximum of 120. After a successful reward, call the established single-unit consumption and slot synchronization path; on validation failure, only resynchronize the unchanged slot.

Alternative considered: a quest-only merge implementation. A second stack engine would create inconsistent caps and duplicate anti-duplication logic.

## Risks / Trade-offs

- **Legacy source has contradictory party semantics** -> Follow the issue's explicit “other members” wording, exclude the consumer, and lock this behavior with leader/member/solo tests.
- **Class tiers beyond Mortal and Arch are underspecified** -> Preserve the active legacy ternary behavior by selecting the non-Mortal configuration columns and add a focused characterization test.
- **Malformed party lists could award twice** -> Deduplicate recipient connection IDs and require a live entity plus an active playing session.
- **A partial reward mutation could consume an item incorrectly** -> Validate all use preconditions before mutation and perform rewards plus consumption synchronously in the world loop.
- **Strict content loading can expose previously unnoticed bad files** -> Return contextual parser errors at startup and retain compiled defaults only for the intentional no-content mode.
- **Large rewards can cross several levels** -> Reuse the existing loop-based level progression and test multi-level and capped cases.

## Migration Plan

1. Add and test the quest-rate parser and compiled defaults.
2. Wire the rate table through tmserver startup and dispatcher configuration.
3. Add direct reward and party distribution behavior with regression tests.
4. Extend stack merge/split coverage and stacked-use tests.
5. Run focused handler/content tests, followed by the repository test suite.

Rollback consists of reverting the handler dispatch, stack whitelist, and quest-rate wiring together. No persisted-data or protocol migration is required; existing `EF_AMOUNT` stacks remain valid either way.
