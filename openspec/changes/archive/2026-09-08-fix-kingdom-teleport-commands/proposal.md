## Why

The kingdom teleport command set currently exposes nonstandard `/red` and `/blue` shortcuts and gives `/reino` behavior that conflicts with the legacy `/king` and `/kingdom` distinction. This allows a character to teleport directly to the opposing king and provides a known path into the client-freeze symptom reported in issue #318.

## What Changes

- Add `/rei` and `/king` aliases that route a kingdom-caped character to the king associated with the equipped cape.
- Add `/reino` and `/kingdom` aliases that route a character to the commerce area associated with the equipped kingdom cape, or to the neutral kingdom center for no cape, a white cape, a green cape, or another neutral cape.
- **BREAKING**: Remove `/red` and `/blue` from the recognized public teleport commands so they fall through as ordinary whisper targets and can no longer bypass cape-based routing.
- Use the legacy fixed destination coordinates for king and kingdom-commerce commands rather than the existing randomized `/reino` destinations.
- Treat `/rei` and `/king` as handled no-ops for characters without a kingdom cape, matching the legacy command's lack of a neutral destination.
- Add command-level regression coverage for aliases, cape classifications, exact destinations, and retired commands.
- Keep the broader client-freeze investigation out of scope; this change removes the reported invalid command path but does not claim to repair every area-transition freeze.

## Capabilities

### New Capabilities

- `kingdom-teleport-commands`: Defines cape-aware king and kingdom-commerce teleport commands, their aliases, destinations, and retired shortcuts.

### Modified Capabilities

None.

## Impact

- Affects chat command dispatch and kingdom teleport routing in `tmserver/internal/handler/chat.go`.
- Affects handler-level protocol tests in `tmserver/internal/handler/chat_test.go`.
- Reuses the existing equipped-cape classification and authoritative `doTeleport` path inside the single-owner world loop.
- Does not change wire formats, persistence, external APIs, dependencies, or general disconnect handling.
