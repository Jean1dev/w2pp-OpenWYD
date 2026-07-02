# Prompt p/ o agente do Windows — captura SKILLS/AFFECTS (cole lá e traga o .md)

Contexto: estamos migrando o servidor do WYD (build cliente 12000, header CPSock = 12 bytes,
`FLAG_GAME2CLIENT 0x0100` / `FLAG_CLIENT2GAME 0x0200`) para Go. A frente de **skills + buffs**
já foi implementada a partir da nossa cópia parcial da fonte; ficaram pontos UNVERIFIED que só a
sua fonte COMPLETA + o dumper (`_layout_probe/dump_layout.cpp`) respondem. Salve tudo num arquivo
`captura-wyd-skills.md`.

## 1. Layouts (rode o dumper: sizeof + offsetof de CADA campo)

- `MSG_SetShortSkill` (Type esperado 0x0378): esperamos body = `Skill1[4]` + `Skill2[16]` = 20B
  (total 32B). Confirme os offsets e o total.
- `MSG_UpdateScore` (0x0336): esperamos total 152 com `Affect[32]` (u16) no offset absoluto 62
  (body 50), `Guild`@114(body), `Resist[4]`@118, `CurrHp`@124, `Magic`@132, `Special[4]` u8 @136.
  Confirme (pack(1)).
- `MSG_SetHpDam` (0x018A): esperamos 20B: `int Hp; int Dam`. Confirme.
- `MSG_Attack`: confirme `SkillIndex`@(body44) e `ReqMp`@(body46), e o que o cliente 12000 manda
  em `SkillIndex` num ataque MELEE (esperamos -1) e nos `Dam[i].Damage` (-2 melee / -1 skill).

## 2. Funções que NÃO estão na nossa cópia (cole o código completo)

- `GetParryRate` (chance de parry/dodge 0..1000) — hoje confiamos em parry 0.
- `BASE_GetDoubleCritical` — hoje confiamos no byte DoubleCritical do cliente (inseguro).
- O significado de `MSG_Attack.Progress`/`cProgress` se existir nessa build.
- `SetReqHp`/`SetReqMp` e como `pUser[conn].ReqMp` é INICIALIZADO (login?) — precisamos saber se o
  eco de `ReqMp` no ataque deve partir de um contador server-side ou do valor do cliente.
- Onde `MOB.SaveMana` e `MOB.Magic` são setados para PLAYERS (efeito de equip? EF_?; classe base?).
- O papel exato de `MOB.RegenMP` no roll de resist de affect (`_MSG_Attack.cpp`:
  `rand()%100 > RegenMP + AffectResist + difLevel`) — de onde vem o RegenMP de um player?
- `CurrentWeather`: onde muda (timer?), valores possíveis (0/1/2?) e o broadcast pro cliente.
- `pTransBonus[]` (tabela de transformação BM, Type 16) — dump completo dos valores.
- `STRUCT_MOBEXTRA`: offsets de `Soul`/`ClassMaster` (p/ Type 29 e tiers).
- `BASE_GetBonusSpecialPoint` se existir (nós derivamos SpecialBonus como +2/level — confirme).

## 3. Comportamento a capturar no cliente real (WYD.exe 12000)

- Um clique de "aprender skill" no class master: confirme que sai
  `MSG_ApplyBonus{BonusType:2, Detail:5000+idx, TargetID:<npc>}`.
- Um cast de buff curto (ex.: skill 41 Velocidade): capture o `MSG_UpdateScore` de resposta e
  cronometre o tempo REAL até o ícone expirar — queremos validar a unidade de 8s/tick do affect
  (450 ticks = 1h). Anote `Affect[i]` (u16) do pacote.
- O valor de `Delay` efetivo entre skills consecutivas (o ClientPatch divide o delay exibido por 4).

## 4. Tabelas

- `g_pSpell` pós-load de 3 linhas de controle (indexes 0, 5, 200) — sizeof(STRUCT_SPELL) e os
  valores parseados (confirma nosso parser: 22 conversões, coluna 23 ignorada, AffectTime/4).
- `BaseSIDCHM[4][4]` (atributos base por classe — usamos no ScoreBonus derivado).
