## Context

See `proposal.md` for motivation and the three capability specs for observable behavior. The Go server already routes several `_MSG_Quest` modes, implements Mortal-to-Arch creation, and contains a simplified Ideal Stone path. It deliberately omits `QUEST_CAPAREAL` and the King's `CapeMode`/sapphire branches. The Ideal Stone path currently requires the Arch cap and preserves most Arch state, while the local legacy source accepts stored level 355 and performs a full reset.

World state belongs exclusively to `world.World.Run`. Database operations therefore cannot execute synchronously in quest or item handlers; they must use `World.Go` and apply results back in the loop. Item and character mutations also need failure-safe persistence because cape purchases and evolution consume valuable items.

## Goals / Non-Goals

**Goals:**

- Reproduce the relevant `_MSG_Quest.cpp` and `_MSG_UseItem.cpp` rules while honoring the issue-requested sapphire range of 4-32.
- Make cape purchases and tier evolution atomic from the player's perspective.
- Keep Mortal-to-Arch creation, Quest 256, and unrelated King services unchanged.
- Model only the additional legacy fields required by these flows and persist them through every character representation.

**Non-Goals:**

- Completing Celestial-to-CelestialCS, Sub-Celestial, Arcana, or `SaveCelestial[2]` behavior.
- Porting every omitted branch of the King NPC or every cape type outside the legacy switch involved here.
- Replacing the placeholder Celestial experience curve.
- Changing the unmodified 7662 client's protocol or UI.

## Decisions

### Use three handler-level flows under one progression change

`QUEST_CAPAREAL`, King cape service, and Ideal Stone evolution remain separate functions selected by explicit NPC/item identity. This prevents the name collision between `Guarda` grade 4 and the Royal Guard grade 13 and keeps each issue independently testable. A monolithic progression handler was rejected because these operations have different eligibility, persistence, and client effects.

### Treat stored legacy levels as the parity basis

Eligibility uses the zero-based levels found in the legacy source: Royal Cape `[199,254)` and Ideal Stone `>=355`. Client-facing documentation may display these as levels 200-254 and 356+, but handler comparisons and tests use stored values. Keeping the existing `399` Ideal Stone gate was rejected because it directly explains the silent failure in #310 and contradicts `_MSG_UseItem.cpp`.

### Persist the missing progression metadata explicitly

Add typed character fields for `QuestInfo.Arch.MortalLevel` and `QuestInfo.Celestial.ArchLevel`, propagate them through domain, world, store, protobuf/API, and a forward database migration. Explicit columns match the existing Celestial and Arch quest flags and avoid introducing an opaque legacy blob. Inferring these values was rejected because `MortalLevel` is an eligibility input and `ArchLevel` is historical state that cannot be reconstructed after the reset.

### Keep sapphire balance database-owned

Add a singleton persisted kingdom-balance record exposed through the persistence boundary. A confirmed purchase submits the expected quoted revision, kingdom, character payment plan, and cape result; the database transaction locks/revises balance state and commits the character-facing outcome once. The handler obtains the quote asynchronously and returns results to the world loop.

The price begins balanced and changes one step toward the selected kingdom after a successful assignment. The selected kingdom's future price rises while the opposing price falls, clamped to 4-32. Online-session counts were rejected because restarts and temporarily offline players would change prices incorrectly. Process-local state was rejected because multiple tmserver instances could diverge.

### Preserve sapphire value deterministically

Before mutation, build an exact payment plan over items 697 and 4131. Ten-unit items are used only where their full value fits; remaining cost uses one-unit items. If exact payment is impossible, reject without mutation. This differs from the legacy branch that can clear a ten-unit item for a smaller remainder, but avoids item-value loss and satisfies the issue's player-facing economic intent.

### Stage mutations and commit before publishing success

Both cape purchases and Celestial evolution construct a proposed character snapshot without first mutating live world state. Persistence commits the snapshot and shared balance update where applicable. On success, the loop installs the snapshot and emits inventory, equipment, score, effect, and session-transition packets. On failure, live state remains unchanged. Mutate-then-save was rejected because asynchronous save failure could consume an Ideal Stone or sapphires without granting the result.

### Reset Celestial state from authoritative class bases

Ideal Stone evolution follows `_MSG_UseItem.cpp`: classify and save the Arch level band, select item 3500/3501/3502, reset tier/level/experience/base score/allocations/skills and quest gates, set special bonus 855 and the retained skill bit, choose cape 3197/3198/3199, and apply effects 98/106. Existing score derivation helpers are used only where they produce those explicit legacy starting values. Preserving Arch allocations was rejected because it creates a materially stronger and persistence-inconsistent Celestial.

## Risks / Trade-offs

- [The issue requests prices 4-32 while the legacy global supports wider 1-64 states] -> Make 4-32 the normative clamp and document this intentional server rule beside the pricing code.
- [Atomic character and shared-balance persistence may require widening the persistence API] -> Provide one domain-specific operation rather than exposing transaction primitives to the game loop.
- [Old characters lack the new progression metadata] -> Backfill conservative defaults and ensure existing Mortal-to-Arch creation populates `MortalLevel` so newly created Arch characters are eligible correctly.
- [Exact sapphire payment can reject a player whose total value is sufficient but denominations cannot match] -> Return the exact quoted cost and retain every item; players can split denominations before retrying.
- [Protocol packet order can leave the unmodified client visually stale] -> Pin packet ordering with handler tests and follow the established logout/character-selection sequence used by Mortal-to-Arch creation.

## Migration Plan

1. Add nullable-safe/defaulted progression columns and the singleton sapphire-balance table or row, then deploy dbserver support before tmserver behavior.
2. Backfill new Arch metadata conservatively from available character state and make all new Mortal-to-Arch creations record the source Mortal level.
3. Deploy tmserver handlers and enable the three flows together so quoted price, persistence operation, and client behavior use the same contract.
4. Verify both kingdom paths and all Ideal Stone bands against fixtures derived from the local legacy source.
5. Roll back application binaries first if necessary; retain additive columns and balance data because older binaries ignore them. No destructive down migration is required during rollback.
