## 1. Hunting Scroll Routing

- [x] 1.1 Add the `EF_VOLATILE` 195 constant, supported item-index bounds, and the six-by-ten legacy destination table in the item handler; verify every coordinate matches `Source/Code/TMSrv/_MSG_UseItem.cpp`.
- [x] 1.2 Add a dedicated hunting-scroll use path that validates item index and `WarpID` before accessing the table, consumes and synchronizes exactly one item on success, and invokes the shared authoritative teleport operation; verify invalid inputs preserve inventory and position.
- [x] 1.3 Route consumable class 195 from `_MSG_UseItem` to the dedicated hunting-scroll behavior and add structured use logging; verify supported items no longer reach the unknown-consumable rejection path.

## 2. Verification

- [x] 2.1 Add table-driven handler tests for all 60 combinations of six supported scrolls and ten destinations, verifying the emitted teleport target matches the legacy table.
- [x] 2.2 Add handler tests for single-unit removal and multi-unit stack decrement, verifying the authoritative inventory slot update sent to the client.
- [x] 2.3 Add boundary tests for item indices outside 3432 through 3437 and `WarpID` values 0 and 11, verifying there is no consumption or teleport and the unchanged slot is synchronized.
- [x] 2.4 Run `go test -run 'Test.*HuntingScroll' ./tmserver/internal/handler` and `go test ./tmserver/internal/handler ./tmserver/internal/protocol`, verifying both focused and related package tests pass.
