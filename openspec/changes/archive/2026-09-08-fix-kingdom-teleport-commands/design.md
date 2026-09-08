## Context

See `proposal.md` for motivation. Chat slash commands arrive as whisper packets whose target name contains the command keyword. The current dispatcher handles `/red`, `/blue`, and `/reino`; the first two bypass cape classification, while `/reino` combines king and neutral-center behavior with randomized destinations.

The existing cape classifier already maps every known kingdom cape to Hekalotia or Akelonia, and `doTeleport` already performs authoritative movement and view reconciliation inside the single-owner world loop. The legacy handler provides distinct fixed destinations for `/king` and `/kingdom`.

## Goals / Non-Goals

**Goals:**

- Keep all command routing synchronous inside the world loop.
- Centralize the cape-to-destination decision so Portuguese and English aliases cannot drift.
- Preserve normal whisper fallback for keywords that are no longer commands.
- Make coordinates deterministic and directly testable against the legacy behavior.

**Non-Goals:**

- Diagnose or repair the broader area-transition client freeze.
- Change general teleport packet encoding or visibility reconciliation.
- Introduce command authorization, persistence, or configurable destinations.
- Change how kingdom capes are classified elsewhere in the game.

## Decisions

### Separate static city teleports from cape-aware kingdom commands

Remove `red` and `blue` from the static teleport-command table and dispatch the four supported aliases through cape-aware handlers. This prevents a generic coordinate lookup from bypassing faction selection.

Alternative considered: retain `/red` and `/blue` but validate the cape. This preserves nonstandard commands that issue #318 explicitly identifies as uncommon and keeps redundant public paths, so it is rejected.

### Reuse equipped-cape classification with a neutral clan input

Both handlers will derive the effective kingdom from the equipped cape while deliberately ignoring the entity's stored guild clan. A character without a kingdom cape is neutral for command routing even if guild state carries a kingdom-like clan value.

Alternative considered: use the entity's stored clan directly, as the legacy local variable did. The current project already treats equipped cape as the externally visible kingdom identity, and the requested behavior is explicitly cape-based.

### Use distinct destination tables and fixed legacy coordinates

The king handler maps Hekalotia to `(1748,1574)` and Akelonia to `(1748,1880)`. The commerce handler maps Hekalotia to `(1690,1618)`, Akelonia to `(1690,1842)`, and neutral to `(1702,1726)`. No random spread is added.

Alternative considered: preserve the existing random offset used by generic city commands. Fixed destinations match the legacy `/king` and `/kingdom` behavior and avoid landing on an adjacent tile with different map or NPC characteristics.

### Consume neutral king commands without teleporting

`/rei` and `/king` remain recognized for neutral characters but produce no movement. This mirrors the legacy handler, which has no neutral destination, and prevents the keyword from falling through to an offline-whisper notice.

Alternative considered: send neutral characters to the kingdom center. That would collapse the distinction between the king and kingdom-commerce commands and contradict the issue's “respectiva capa” rule.

### Let retired color keywords use normal whisper fallback

Once removed from command dispatch, `red` and `blue` follow the existing non-command whisper behavior. If no matching player is online, the caller receives the standard not-connected response. No special deprecation notice is introduced.

Alternative considered: keep them as handled no-ops. Falling through proves they are no longer commands and preserves the dispatcher's established behavior for unknown keywords.

## Risks / Trade-offs

- [Existing users may still type `/red` or `/blue`] -> The change is intentional and covered as a breaking behavior in the proposal; supported PT/EN aliases replace them.
- [A known kingdom cape could be missing from the shared classifier] -> Exercise the classifier's existing base, elite, and advanced cape families in table-driven tests.
- [The reported client freeze may still occur after other teleports] -> Do not describe this change as a general disconnect fix; retain the separate freeze investigation and telemetry.
- [Fixed tiles could differ from server-specific content placement] -> Use coordinates present in the shipped legacy handler and assert them exactly in regression tests.

## Migration Plan

1. Deploy the command routing and regression tests together.
2. Verify each alias with blue, red, and neutral cape states against the shipped map content.
3. Monitor existing teleport and session-send telemetry for freezes without changing the broader incident conclusion.
4. Roll back the command-dispatch commit if the fixed destinations prove incompatible; no stored data or schema rollback is required.
