# Captura WYD — Level-up (XP, HP/MP, bônus, pacotes)

> Retorno do agente Windows (fonte completa `Source\Code`, structs verificados pelo
> `_layout_probe`). Função de level-up = `CMob::CheckGetLevel()` (`CMob.cpp:1069`),
> chamada após ganho de exp (`_MSG_Attack.cpp:1764`).
>
> **Implementado** (MORTAL solo) em `tmserver/internal/level` + `handler/mobkilled.go`
> (`grantExp`). Este doc é a evidência + o que ficou **deferido**.

## 1. Constantes (em código: `level.MaxLevel`, etc.)
| const | valor | fonte |
|---|---:|---|
| `MAX_CLASS` | 4 | Basedef.h:175 |
| `MAX_LEVEL` | 399 | Basedef.h:177 |
| `MAX_CLEVEL` | 199 | Basedef.h:178 |
| `MAX_HP`/`MAX_MP` | 1000000000 | Basedef.h:263-264 |

Classes: 0=TK 1=FM 2=BM 3=HT. MORTAL/ARCH → `g_pNextLevel`, max=MAX_LEVEL; CELESTIAL* →
`g_pNextLevel_2`, max=MAX_CLEVEL.

## 2. Curva de XP
- `g_pNextLevel[]` (Basedef.cpp:625) — **hardcoded**, índice=nível, `[0]=0`, `[1]=500`, `[2]=1124`.
  Sobe de nível quando `MOB.Exp >= g_pNextLevel[Level+1]`. Teto `[400]=4100000000`.
  → **Transcrita e validada** (anchors + monotonia) em `tmserver/internal/level/nextlevel.go`.
- `g_pNextLevel_2[]` (Celestial) = `n*20000000`; `[199]=3980000000`, `[200]=4000000000`,
  `[201]=8200000000`. **DEFERIDO** (não modelamos tiers celestiais).

## 3. HP/MP por nível (em código: `level.IncHP/IncMP`, `BaseSIDCHM`)
`g_pIncrementHp[4]={3,1,1,2}`, `g_pIncrementMp[4]={1,3,2,1}` (Basedef.cpp:63-64).
`BaseSIDCHM[4][6]` = Str,Int,Dex,Con,HP,MP por classe (Basedef.cpp:44):
```
TK {8,4,7,6,80,45}  FM {5,8,5,5,60,65}  BM {6,6,9,5,70,55}  HT {8,9,13,6,75,60}
```
`BASE_GetHpMp` (Basedef.cpp:1214): `MaxHp = baseHP + (Con-baseCon)*2 + Level*incHp`;
`MaxMp = baseMP + (Int-baseInt)*2 + Level*incMp`. (No level-up usamos o incremento
`MaxHp += incHp[cls]`, equivalente.)

## 4. `GetExpApply` (em código: `level.ExpApply`, path MORTAL)
`GetFunc.cpp:1028`. `mult% = (target+1)*100/(attacker+1)`, teto 200; se `mult<80 && attacker+1>=50`
→ `mult*2-100` (pune killer muito acima do mob); `exp=(exp*mult+1)/100`. ARCH=50% base + quest-gates
(355/370); CELESTIAL trata attacker=MAX_LEVEL + quest-gates (40/90) — **esses ramos DEFERIDOS**.

## 5. Pontos de atributo (em código: `level.ScoreBonus`, MORTAL)
`BASE_GetBonusScorePoint` (Basedef.cpp:898): `ScoreBonus = leveluse - usados`, onde
`leveluse = lvl*5 (+5/nv≥254, +10/nv≥299, -8/nv≥354)` e `usados = Σ(stat - base)`. Idempotente
(função de level+stats) → recalculado no level-up, não precisa persistir.
**No level-up MORTAL/ARCH** (CMob.cpp:1115): `SkillBonus += 3` (+4 se Level≥200), `SpecialBonus += 2`,
`Ac++`. **DEFERIDO** (Entity não modela Skill/Special bonus nem base-score separado p/ o Ac++).

## 6. Pacotes S→C no level-up (`CheckGetLevel` retorna 1..4 = ¼/½/¾/subiu)
| evento | pacote | Type | tam | status |
|---|---|---|---:|---|
| seg 1–4 | `MSG_UpdateScore` (SendScore) | 0x0336 | 152 | **feito** (`sendScore`) |
| seg 1–4 | `MSG_Motion` (SendEmotion 14,3) | 0x036A | 20 | **feito** (`EncodeMotion`, efeito de level-up) |
| seg 1–4 | `MSG_MessagePanel` | 0x0101 | — | DEFERIDO (texto "Level Up") |
| **seg 4** | `MSG_UpdateEtc` (SendEtc) | 0x0337 | 48 | parcial (mandamos UpdateEtc só p/ gold) |
| **seg 4** | `MSG_CreateMob` (multicast) | 0x0364 | 232 | **DEFERIDO** (recriar player na visão p/ novo nível/visual) |
| **seg 4** | `DoItemLevel(conn)` (só MORTAL) | — | — | **DEFERIDO** (itens de recompensa por nível, ver `LevelItem.txt`) |

`MSG_Motion` (0x036A, 20B): header(12) + `Motion` u16@12 (=14) + `Parm` u16@14 (=3) + `NotUsed` int@16.
`MSG_UpdateScore` (0x0336, 152B): header + STRUCT_SCORE(48)@12 + Critical@60 + SaveMana@61 +
Affect[32]@62 + Guild@126 + GuildLevel@128 + Resist[4]@130 + RegenHP@134 + RegenMP@135 + CurrHp@136 +
CurrMp@140 + Magic@144 + Special[4]@148. (Nosso `EncodeUpdateScore` preenche o subconjunto que o mundo
rastreia.)

## 7. EXP de party — **NÃO CONFIÁVEL (artefato de decompilação)**
`PARTYBONUS=100` (50–200, gameconfig) e `UNK_1=30` são confiáveis; `g_EmptyMob` aparece como
`MAX_USER=1000` mas é mislabel de decompilação (daria exp absurda). A fórmula
`(UNK_1+myLevel)*isExp/(UNK_1+myLevel)` reduz a `isExp` (num=den). **Recomendação:** quando fizer
party, cada membro recebe `GetExpApply(...)` e aplica `PARTYBONUS` como %, ignorando `g_EmptyMob`/UNK
até validar por captura de tráfego. Por isso a distribuição de party fica DEFERIDA.

## Fluxo (implementado em `grantExp`)
1. `exp = ExpApply(mob.Exp, killerLevel, mobLevel)`; `killer.Exp += exp` (clamp `MaxExp`).
2. enquanto `Exp >= NextLevelExp(Level)` e `Level<MaxLevel`: `Level++`; `MaxHp/MaxMp += inc`;
   no fim cura full + recalcula `ScoreBonus`.
3. se subiu: `MSG_UpdateScore` + `MSG_Motion(14,3)` (self + in-view).
