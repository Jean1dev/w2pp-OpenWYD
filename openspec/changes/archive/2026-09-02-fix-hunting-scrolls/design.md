## Context

See `proposal.md` for motivation and `specs/hunting-scrolls/spec.md` for observable behavior. The item-use packet already decodes the client's 16-bit `WarpID`, and the handler already owns reusable item-stack and teleport operations. The missing piece is dispatch and routing for `EF_VOLATILE` 195. The legacy server contains the authoritative six-by-ten coordinate table and validates item and destination ranges before mutating state.

All item consumption and player movement must execute in the single-owner world goroutine. No persistence or blocking work is required by this behavior.

## Goals / Non-Goals

**Goals:**

- Preserve the legacy mapping and its zero-based table calculations without changing the wire format.
- Make validation precede both inventory mutation and teleportation.
- Exercise all mapping rows and columns through table-driven tests.

**Non-Goals:**

- Changing scroll item definitions, prices, icons, or acquisition.
- Introducing destination restrictions not present in the legacy handler.
- Generalizing unrelated teleport items or replacing the shared teleport path.

## Decisions

### Use an immutable in-process destination table

Port the legacy `HuntingScrolls[6][10][2]` data as a fixed Go table next to the item-use behavior. Select the row with `itemIndex - 3432` and the column with `WarpID - 1`, but only after explicit bounds checks.

This keeps the gameplay data auditable against the legacy source and avoids a new configuration or database dependency for static compatibility data. Loading the coordinates from runtime configuration was rejected because it adds operational failure modes without a current content-management requirement.

### Dispatch by consumable class and validate by item index

Add consumable class 195 to the existing item-use dispatch, then require item indices 3432 through 3437 inside the dedicated behavior. Dispatching by item index alone was rejected because the current architecture and legacy handler classify this family by `EF_VOLATILE` before applying the narrower item-index guard.

### Preserve invalid requests without side effects

Invalid item indices and `WarpID` values perform no teleport and no consumption. The handler should synchronize the unchanged inventory slot to prevent optimistic client state from appearing consumed, while retaining the legacy gameplay result of no mutation.

Silent return was considered for byte-for-byte control-flow parity, but slot synchronization is consistent with the Go server's established rejection behavior and prevents phantom inventory changes.

### Reuse the shared authoritative teleport and stack primitives

Successful use decrements exactly one unit with the existing stack helper, sends the updated slot, and invokes the existing teleport path. This preserves visibility reconciliation and avoids duplicating movement state changes. The sequence will validate first, consume and synchronize second, then teleport, matching other item-triggered teleports in the Go server.

### Test the mapping exhaustively

Use table-driven tests covering all 60 item/`WarpID` combinations, plus focused tests for single-item removal, stacked decrement, and invalid boundaries. Exhaustive data coverage is appropriate because transcription mistakes in a static coordinate table compile successfully but send players to the wrong location.

## Risks / Trade-offs

- [Coordinate transcription error] → Compare every test case against the six legacy table rows and assert both teleport coordinates.
- [Client sends an unexpected `WarpID`] → Bounds-check before indexing or mutating inventory and re-synchronize the unchanged slot.
- [Teleport succeeds but inventory appears stale] → Use the established stack mutation and slot synchronization helpers on every successful use.
- [Minor divergence from legacy invalid-request response] → Limit the divergence to re-sending unchanged authoritative item state; gameplay state remains identical.

## Migration Plan

No data migration is required. Deploy the updated game server normally; existing scroll items immediately become usable because their catalog metadata and packet layout are already supported. Rollback consists of deploying the previous binary, with no persisted schema or data to reverse.
