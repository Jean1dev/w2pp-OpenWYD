# Contratos — Movimento & Visão (lote 2)

> Handlers de deslocamento, emotes e culling de visão. Fonte: `Source/Code/TMSrv/_MSG_*.cpp`.

---

## `_MSG_Action` (0x036C / `_MSG_Action2` 0x0366 / `_MSG_Action3` 0x0368) — movimento/rota
- **Gatilho/struct:** `MSG_Action` (`PosX/Y`, `Effect`, `Speed`, `Route[24]`, `TargetX/Y`). Três
  Types caem no mesmo handler (`_MSG_Action.cpp`).
- **Validações (ordem):**
  1. `Mode == USER_PLAY` (senão `SendHpMode`);
  2. `Hp != 0` (senão `SendHpMode` + `AddCrackError(5,3)`);
  3. se em trade → `RemoveTrade` de ambos; se `TradeMode` → `RemoveTrade`.
  4. **Anti-speedhack por tick:** usa `m->ClientTick` (movetime) vs `CurrentTime`. Limite mínimo de
     **900 ms** entre ações (`LastIllusionTick + 900`) e janela máx. de **15000 ms**
     (`movetime > CurrentTime+15000` ou `< CurrentTime-120000`) → `AddCrackError(1,104/105)`.
- **Variantes:** `_MSG_Action3` = "Skill Ilusão" (exige `Class==3` e skill aprendida `LearnedSkill&2`;
  gasta `g_pSpell[73].ManaSpent`). `_MSG_Action`/`Action2` = andar normal.
- **Efeitos:** atualiza posição/rota do `pMob[conn]`, recalcula grid e **multicast** aos jogadores na
  visão (`GridMulticast`). Consome MP nas variantes de skill.
- **Anti-cheat/risco:** este é o handler central de **anti-speedhack** (ticks). `SKIPCHECKTICK
  (235543242)` desativa a checagem na primeira ação. Preservar a lógica de tick **exatamente** para
  paridade (Fase 8 §2.3). Constantes de tempo/skill hardcoded.

## `_MSG_Motion` (0x036A) — emotes/animações
- **Gatilho/struct:** `MSG_Motion` (`Motion`, `Parm`, `NotUsed`).
- **Validações:** `Hp != 0` **e** `Mode == USER_PLAY` (senão `SendHpMode` + `AddCrackError(4,6)`).
- **Efeitos:** loga e **multicast** do motion aos jogadores na visão (`GridMulticast`).
- **Risco:** baixo; só visual. Validar bounds de `Motion` no novo código (não há aqui).

## `_MSG_ChangeCity` (0x0291) — definir cidade de spawn
- **Gatilho/struct:** `MSG_STANDARD` (usa `pMob[conn].TargetX/Y` atuais, não campos do pacote).
- **Efeitos:** `city = BASE_GetVillage(tx,ty)`; se `0..4`, grava nos **bits altos** de
  `MOB.Merchant`: `Merchant = (Merchant & 0x3F) | (city << 6)`. Loga.
- **Risco:** `Merchant` é um campo multiuso (bit-packed) — documentar o layout de bits ao migrar
  (cidade nos bits 6-7). Sem checagem de `Mode`/`Hp`.

## `_MSG_ReqTeleport` (0x0290) — teleporte pago
- **Gatilho/struct:** sem corpo útil (usa posição atual). Lógica em `_MSG_ReqTeleport.cpp`.
- **Validações:** bloqueia teleporte numa região específica (`posx/4==491 && posy/4==443` →
  "Only by water scroll"). Calcula destino+custo via `GetTeleportPosition`.
- **Efeitos:** se o clan do jogador é dono da zona 4 (`g_pGuildZone[4].Clan==clan`) → teleporte
  grátis (`goto label_tel`). Senão cobra `reqcoin` de `MOB.Coin`; metade do imposto vai para a `Exp`
  do "mestre da guilda" cobradora (`GuildImpostoID[4]`). `DoTeleport`. Sem dinheiro → "Not enough money".
- **Risco:** economia de imposto de zona acoplada; valores/zonas hardcoded. `Exp` usada como cofre de
  imposto (cuidado no schema — Fase 2). Preservar a regra de isenção do dono da zona.

## `_MSG_NoViewMob` (0x0369) — re-sincronizar visão de um mob
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = MobID`).
- **Validações:** `MobID` em `(0, MAX_MOB)`.
- **Efeitos:** se o mob está vazio → `SendRemoveMob`. Se em visão (`GetInView`) → `SendCreateMob` +
  `SendPKInfo`; senão `SendRemoveMob`. Para players, exige `Mode==USER_PLAY`.
- **Risco:** baixo; é reconciliação de visão pedida pelo cliente. Sem efeito de estado.
