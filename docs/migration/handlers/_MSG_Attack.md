# Contrato — `_MSG_Attack`

- **Gatilho:** Type `0x0367` (também `_MSG_AttackOne` 0x039D, `_MSG_AttackTwo` 0x039E — todos caem
  em `Exec_MSG_Attack`). Struct `MSG_Attack` (Fase 1 §3.5; inclui `SkillIndex`, `PosX/Y`,
  `TargetX/Y`, `Dam[MAX_TARGET=13]`).
- **Fonte:** `TMSrv/_MSG_Attack.cpp:21+`.

## Pré-condições e validações
1. `pUser[conn].TradeMode == 0` — não pode atacar em auto-trade (`:27-31`).
2. `pUser[conn].Mode == USER_PLAY` — senão `SendHpMode` (`:33-37`).
3. Vivo: `Hp != 0` **ou** `SkillIndex == 99` (ressurreição) — senão `SendHpMode` +
   `AddCrackError(1,8)` (`:40-45`).
4. **Anti-speed (cadência de ataque):** se `ClientTick < LastAttackTick + 800` (ms) → rejeita +
   `AddCrackError(1,107)` (`:59-66`). Limite de **800 ms** entre ataques.
5. **Sanidade de tick:** se `ClientTick < LastAttackTick - 100` → `AddCrackError(4,7)` (`:70-71`).
   Janela de tick válida: `ClientTick15sec .. CurrentTime+15000` (`:79-88`); fora → erro.
6. `ClientTick == SKIPCHECKTICK` pula as checagens (eventos internos do servidor).

## Efeitos colaterais
- Força `m->ID = ESCENE_FIELD` (broadcast no campo) (`:25`).
- Atualiza `pUser[conn].LastAttackTick = ClientTick`, `LastAttack = SkillIndex` (`:73-76`).
- Resolve dano por alvo (até `MAX_TARGET=13`): aplica fórmula de combate (Fase 4 §4), reduz `Hp` do
  alvo, marca `TargetKilled[]`; em morte de mob → `MobKilled` (exp/drop, Fase 4 §1-2).
- Consome MP da skill (`ReqMp`), aplica affects/efeitos de skill.

## Saídas (S→C)
- `_MSG_Attack` (broadcast aos jogadores na visão): posição, `AttackerID`, `Motion`, `SkillIndex`,
  `DoubleCritical`, e `Dam[]` (TargetID+Damage por alvo), `CurrentHp/Mp/Exp` do atacante.
- `_MSG_SetHpMp`/`_MSG_SetHpDam` para HP/MP; `_MSG_CNFMobKill` em morte; `_MSG_RemoveMob` ao despawn.

## Anti-cheat
- Cadência 800 ms + validação de `ClientTick` (anti-speedhack/macro). `AddCrackError` acumula "cra
  points" → logout (Fase 3 §2.1 / TMSrv-Core deep-dive).
- Dano é **server-authoritative**: o servidor recalcula; os campos de dano enviados pelo cliente
  são sobrescritos. (Confirmar que nenhum campo de dano do cliente é confiado — risco de dup/cheat.)

## Riscos (migração)
- A fórmula de dano (`BASE_GetDamage`/`BASE_GetSkillDamage`, pipeline do golpe, acerto/parry/reflect)
  está documentada na **Fase 4 §4** (fonte real em `Basedef.cpp`/`_MSG_Attack.cpp`); usa `rand()` →
  validar por **distribuição** com golden cases (Fase 8). Resta tabelar coef. Dex/Str por classe×arma.
- `SkillIndex == 99` como "ressurreição" é constante mágica — preservar.
- A janela de tick depende de `CurrentTime` (clock do servidor) — reproduzir a unidade (ms).
