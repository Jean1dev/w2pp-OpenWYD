---
name: railway-runtime-troubleshooting
description: Troubleshoot production incidents for the WYD Go server on Railway using the Railway CLI. Use when investigating tm-server/db-server/bin-server/webserver slowness, disconnects, client freeze, missing commands, crashes, deploy interruptions, network drops, DB latency, queue saturation, or regressions visible in Railway logs/metrics.
---

# Railway Runtime Troubleshooting

Use this skill to investigate production runtime incidents in the WYD Go server stack on Railway.
Treat Railway data as production evidence: collect narrow time windows first, avoid leaking secrets,
and distinguish server failure from client-side freezes.

## Ground Rules

- Work from the repository root so Railway project linking and local code references line up.
- Do not deploy, restart, redeploy, or change Railway variables unless the user explicitly asks.
- Do not print full secret values from `railway variables`; redact tokens, URLs, passwords and keys.
- Convert user-reported Brazil local time (`America/Sao_Paulo`, usually UTC-03) to UTC before querying logs.
- Prefer exact incident windows: 2 minutes before the reported symptom through 2-5 minutes after.
- Always compare logs, metrics, deployments and recent code changes before naming a root cause.
- If the server logs show no WARN/ERROR/crash and the socket remains open, consider a client freeze or S->C packet/view issue, not only backend slowness.

## Setup

Run Railway from the repo root. If `railway` is not in PATH, use the local install path:

```bash
export PATH="$HOME/.railway/bin:$PATH"
railway whoami
railway status
```

If the directory is not linked, link to the known production project/service:

```bash
export PATH="$HOME/.railway/bin:$PATH"
railway link --project 08049b1a-6753-4274-b436-0dff658a5df1 --environment production --service tm-server
railway status
```

If service names are uncertain, inspect `railway status` and use the service names it reports. Known names in this project include `tm-server`, `db-server`, `bin-server`, and `webserver`.

## Incident Triage Workflow

1. Record the user's report as exact local and UTC times:

```bash
date
date -u
```

Example conversion: `21:33 -03` on `2026-07-18` is `2026-07-19T00:33:00Z`.

2. Pull raw tm-server logs for a tight window:

```bash
export PATH="$HOME/.railway/bin:$PATH"
railway logs --service tm-server \
  --since 2026-07-19T00:32:30Z \
  --until 2026-07-19T00:36:30Z
```

3. Pull a filtered version that removes high-volume noise:

```bash
railway logs --service tm-server \
  --since 2026-07-19T00:32:30Z \
  --until 2026-07-19T00:36:30Z \
  | grep -viE 'recv packet|served status probe|movement:'
```

4. Search for hard failures over a wider window:

```bash
railway logs --service tm-server --since 24h --filter "tmserver stopped"
railway logs --service tm-server --since 24h --filter "panic"
railway logs --service tm-server --since 24h --filter "level=ERROR"
railway logs --service tm-server --since 24h --filter "level=WARN"
railway logs --service tm-server --since 24h --filter "session out queue full"
```

5. Check deploy activity around the symptom:

```bash
railway deployment list --service tm-server
railway deployment list --service db-server
railway logs --service tm-server --since 24h --filter "Starting Container"
railway logs --service db-server --since 24h --filter "Starting Container"
```

Interpretation:

- `Starting Container` close to the incident often means auto-deploy/restart interrupted active sessions.
- Clean shutdown logs such as `world loop stopped` and no panic point to deploy/SIGTERM, not an application crash.
- No restart plus a later `EOF` often means the client closed or was killed.

6. Check resource metrics for CPU/memory saturation:

```bash
railway metrics --service tm-server
railway metrics --service db-server
```

Interpretation:

- Very low CPU/memory during the incident weakens the "server got slow" hypothesis.
- Memory climbing to limit or sudden restart suggests OOM/restart. Confirm with deployment/runtime logs.

7. Check Railway network flow logs when disconnects look like proxy/network failures:

```bash
railway logs --service tm-server --network --lines 200
railway logs --service tm-server --network --lines 200 | grep -iE 'drop|reset|error|refused|timeout'
```

Interpretation:

- No drops/resets plus clean `EOF` points away from Railway network loss.
- Proxy or transport errors should be correlated with the same UTC timestamp as gameplay symptoms.

8. Compare db-server behavior if login/save/cargo/shop operations seem delayed:

```bash
railway logs --service db-server --since 2026-07-19T00:32:30Z --until 2026-07-19T00:36:30Z
railway logs --service db-server --since 24h --filter "level=ERROR"
railway metrics --service db-server
```

Interpretation:

- Account login completing in ~100-300 ms points away from DB slowness.
- Missing account responses, repeated gRPC errors, or long gaps between tm-server "relaying to dbServer" and "OK" point toward db-server or network-to-db issues.

## Packet and Gameplay Analysis

Use tm-server `recv packet` logs to reconstruct the last client actions:

```bash
railway logs --service tm-server \
  --since 2026-07-19T00:32:30Z \
  --until 2026-07-19T00:36:30Z \
  | grep 'recv packet'
```

Common WYD packet clues:

- `0x020d`: account login.
- `0x0213`: character login.
- `0x0334`: whisper/chat command; slash commands such as `/armia`, `/reino`, `/gm` arrive here.
- `0x0291`: change city request.
- `0x0290`: paid teleport tile request.
- `0x036c`, `0x0366`, `0x0368`: movement/action.
- `0x0399`: PK/K toggle.
- `0x03ae`: delay quit/close flow; if routed false or unanswered, close behavior may look hung.

Interpretation patterns:

- Packets continue arriving after the reported "lag": client input still reaches server; investigate S->C responses, view state, or client rendering.
- Last packet is a normal command, then silence until `EOF`: likely client stopped sending, froze, or was killed.
- Socket stays open for minutes with no EOF: likely half-open client process/window not responding.
- `session out queue full` means S->C writer backpressure; this is a server-side symptom and should be fixed/instrumented.
- Repeated command packets with no corresponding game effect require code inspection of the relevant handler under `tmserver/internal/handler/`.

## Code Correlation

After identifying the packet/action sequence, inspect recent code changes and the matching handler:

```bash
git status --short
git fetch origin main --quiet
git log origin/main --oneline --since="2026-07-18 00:00"
git diff <old-good-commit>..<new-bad-commit> --stat
git diff <old-good-commit>..<new-bad-commit> -- tmserver/internal/handler tmserver/internal/world tmserver/internal/protocol
```

Map logs to handlers:

- Chat/whisper commands: `tmserver/internal/handler/chat.go`.
- Teleport and movement: `tmserver/internal/handler/movement.go`, `view.go`.
- Login/enter world: `tmserver/internal/handler/character.go`.
- Combat/EXP/mob death: `combat.go`, `mobkilled.go`.
- Items and consumables: `item.go`.
- PK toggle: `pkmode.go`.

For protocol, wire formats or game mechanics, consult local docs and legacy source before editing:

```bash
rg -n "MsgName|opcode|handler|legacy function" docs/migration Source/Code/TMSrv tmserver/internal
```

Use legacy references as evidence when possible. Example: `DoTeleport` in `Source/Code/TMSrv/Server.cpp` calls `GridMulticast`, so the Go teleport path should reconcile old and new view windows rather than re-entering the world.

## Optional Database State Check

Only inspect DB state when it helps confirm a gameplay state such as saved position, last city, level or HP. Do not print credentials.

```bash
export PATH="$HOME/.railway/bin:$PATH"
railway variables --service db-server --json
```

Redact variables. If no `psql` exists locally, write a small temporary Go program that reads a redacted DSN from the environment and prints only safe fields needed for diagnosis, such as character name, slot, level, class_master, pos_x, pos_y, last_city, hp/max_hp. Delete scratch files afterward unless the user asks to keep them.

## Root-Cause Decision Tree

- If `Starting Container` or deployment timestamp overlaps symptom: classify as deploy interruption.
- If panic/error/restart/OOM appears: classify as server crash and inspect stack/logs.
- If metrics show CPU/memory saturation: classify as resource pressure and find high-volume logs or loops.
- If db-server shows delayed/error responses matching gameplay gap: classify as DB/backend dependency issue.
- If network logs show drops/resets at the timestamp: classify as Railway/proxy/network issue.
- If tm-server stays healthy, no queue full, no WARN/ERROR, and client later closes with `EOF`: classify as client freeze or S->C/game-state bug.
- If the last actions involve teleport/view changes, inspect `doTeleport`, `moveMulticast`, `enterWorldView`, `CreateMob`, `RemoveMob`, and old/new view reconciliation first.
- If the last actions involve item/EXP/level-up bursts, inspect S->C packet sequence and whether the client receives malformed or excessive packets.

## Reporting Format

Return a concise incident report in Portuguese:

```markdown
**Resumo**
<1-3 frases com causa provável e impacto.>

**Evidências**
- Janela analisada: <local time> / <UTC>.
- Logs tm-server: <eventos importantes>.
- Deploys: <nenhum / deploy X em timestamp>.
- Métricas: CPU/mem relevantes.
- Network/db-server: <resultado>.

**Conclusão**
<diga se foi server crash, deploy, DB, rede, fila S->C, client freeze ou bug de handler.>

**Próximos passos**
- <fix/instrumentação/teste/redeploy, em ordem prática>.
```

Always state what was not observable. For example: "Os logs atuais mostram pacotes recebidos, mas não contam pacotes S->C nem duração de tick; para a próxima reprodução, instrumentar fila de saída, contadores S->C e slow loop."

## Recommended Instrumentation After Inconclusive Incidents

If logs cannot prove the S->C side, recommend adding:

- Per-session outbound counters by packet type.
- Queue depth samples and WARN on high depth.
- Slow world-loop iteration WARN when one event/tick exceeds a threshold.
- Teleport/view reconciliation logs with old/new position and counts of RemoveMob/CreateMob sent.
- Handler-specific logs for high-risk commands/items, kept at INFO only around reproduction windows or DEBUG behind a flag.
