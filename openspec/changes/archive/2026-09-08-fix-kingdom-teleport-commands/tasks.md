## 1. Command Routing

- [x] 1.1 Remove `red` and `blue` from the static teleport command table and verify `/red` and `/blue` no longer emit a teleport action.
- [x] 1.2 Add `/rei` and `/king` dispatch to one cape-aware king handler and verify both aliases select identical behavior.
- [x] 1.3 Add `/reino` and `/kingdom` dispatch to one cape-aware commerce handler and verify both aliases select identical behavior.

## 2. Cape-Aware Destinations

- [x] 2.1 Define the fixed legacy king and commerce destination coordinates and verify no random offset is applied by the kingdom command handlers.
- [x] 2.2 Route Hekalotia and Akelonia kingdom capes through the existing cape classifier and verify each cape family reaches its specified king and commerce destinations.
- [x] 2.3 Route no cape, white cape, green cape, and other neutral capes to `(1702,1726)` for `/reino` and `/kingdom`, and verify `/rei` and `/king` are handled without movement for the same cases.
- [x] 2.4 Execute every successful route through the authoritative teleport path and verify position plus view reconciliation remain covered by the existing teleport tests.

## 3. Regression Verification

- [x] 3.1 Replace the existing randomized `/reino` test with table-driven coverage for all four aliases, representative blue/red cape families, neutral cape states, and exact coordinates; verify `go test -run 'TestCommand(Rei|King|Reino|Kingdom)' ./tmserver/internal/handler` passes.
- [x] 3.2 Add regression cases for `/red` and `/blue` normal-whisper fallback, including absence of `MsgAction`, and verify the targeted handler tests pass.
- [x] 3.3 Run `go test ./tmserver/internal/handler ./tmserver/internal/world` and verify the affected handler and authoritative teleport suites pass.
- [x] 3.4 Run `make test` and verify the full repository suite passes with race detection and coverage.
