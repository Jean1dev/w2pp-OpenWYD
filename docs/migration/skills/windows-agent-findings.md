# Achados do agente Windows - skills

Este documento consolida as respostas `WIN-SKILL-001..008` recebidas do agente Windows. A fonte
Windows provou contratos importantes por codigo legado e dumper MSVC x86; captura ao vivo do cliente
real WYD.exe 12000/7662 continuou `NAO_ENCONTRADO` por falta de ferramentas de captura/driver,
cliente com XTrap, ausencia de credenciais e servidor/cliente parados naquela maquina.

## Status

| Prompt | Status | Resultado util para o Go |
|--------|--------|--------------------------|
| WIN-SKILL-001 | Respondido por fonte/layout; captura `NAO_ENCONTRADO` | `MSG_Attack` 168 B e contrato de `Dam[i].Damage`: `-2` melee, `-1` skill, `0` slot vazio. |
| WIN-SKILL-002 | Respondido por fonte/layout; captura `NAO_ENCONTRADO` | `ReqHp/ReqMp` sao alvos server-side, nao eco do cliente. |
| WIN-SKILL-003 | Respondido por fonte; captura `NAO_ENCONTRADO` | servidor aplica piso de 800 ms por `ClientTick`; `Delay` do CSV nao e threshold server-side. |
| WIN-SKILL-004 | Respondido por fonte/layout; captura `NAO_ENCONTRADO` | affect usa tick de 8 s, `AffectTime/4`, score grid + affect completo self. |
| WIN-SKILL-005 | Respondido por fonte/dados; captura `NAO_ENCONTRADO` | `85 = Explosão_Etérea`, `86 = Escudo_Dourado`; so 85 custa gold. |
| WIN-SKILL-006 | Respondido por dumper MSVC x86 | offsets e tamanhos byte-exatos de pacotes usados por skills. |
| WIN-SKILL-007 | Respondido por fonte; captura `NAO_ENCONTRADO` | `SecLearnedSkill` e campo morto/reservado; skills `200..247` usam `LearnedSkill & (1 << (skillnum % 24))`. |
| WIN-SKILL-008 | Respondido por fonte; captura `NAO_ENCONTRADO` | affects/ticks `40/41/43/44/45/46/47/48` sao icon-only/no-op. |

## `_MSG_Attack` e marcadores de dano

- `_MSG_Attack` tem Type `0x0367` e size `168`.
- `MSG_Attack` usa alinhamento natural MSVC x86. `STRUCT_DAM` tem 8 bytes.
- Offsets absolutos principais: `CurrentHp@16`, `CurrentExp@24`, `PosX@34`, `PosY@36`,
  `TargetX@38`, `TargetY@40`, `Motion@46`, `DoubleCritical@48`, `CurrentMp@52`,
  `SkillIndex@56`, `ReqMp@58`, `Dam@60`.
- `Dam[i].TargetID = 60 + i*8`; `Dam[i].Damage = 64 + i*8`.
- O servidor original nao confia em dano enviado pelo cliente. Valores aceitos no fio:
  `-2` = melee/fisico, `-1` = skill com `SkillIndex` valido, `0` = alvo vazio/sem efeito.
- Qualquer outro `Dam[i].Damage` e tratado como crack no legado.
- Captura ao vivo nao confirmou o valor exato de `SkillIndex` emitido pelo cliente em melee; a fonte
  tolera `SkillIndex < 0` como nao-skill.

## `ReqHp` e `ReqMp`

- `pUser[conn].ReqHp` e `pUser[conn].ReqMp` sao alvos persistentes do servidor.
- No login de personagem: `ReqHp=CurrentScore.Hp` e `ReqMp=CurrentScore.Mp`.
- `SetReqHp`/`SetReqMp` clampa para nao negativo, clampa HP/MP atual contra Max e garante
  `Req >= atual`.
- Cast com MP suficiente desconta `ManaSpent` de `CurrentScore.Mp` e de `ReqMp`, chama `SetReqMp`,
  e ecoa `CurrentMp`/`ReqMp` atualizados no `_MSG_Attack`.
- Cast sem MP nao desconta nada, envia `MSG_SetHpMp` e aborta a skill.
- `MSG_SetHpMp`: Type `0x0181`, size `28`, `Hp@12`, `Mp@16`, `ReqHp@20`, `ReqMp@24`, `ID=conn`.
- Regen e Apply convergem HP/MP atual ate o alvo com cap por tick: 2000 HP e 3000 MP. Cura sobe
  `ReqHp`; DoT/veneno baixa `ReqHp`.

## Delay e anti-speed

- O gate vinculante do servidor e `ClientTick >= LastAttackTick + 800` para qualquer ataque.
- Ha um gate de skill de 700 ms, mas ele e redundante porque o de 800 ms roda antes.
- `g_pSpell[skillnum].Delay` e lido e convertido para ms, com `Delay-1` se `RSV_CAST`, mas o valor
  calculado nao e usado como threshold de rejeicao neste build.
- `SKIPCHECKTICK = 235543242` pula as checagens de tempo.
- `LastAttack = SkillIndex` e gravado para paridade, mas nao decide cooldown.

## Affect, icone e expiracao

- Unidade de affect: 8 segundos por tick (`SECBATTLE=8`; `AFFECT_1H=450`).
- O loader legado divide `AffectTime` do CSV por 4 antes de usar.
- `SetAffect` e `SetTick` usam `Time = (AffectTime + 1) * Delay / 100`, com `Delay = 100 + Level`
  no fluxo de `_MSG_Attack`.
- Depois do cast, `SendScore` envia:
  - `MSG_UpdateScore` Type `0x0336`, size `152`, grid-multicast, com `Affect[i]` u16
    `(Type << 8) | (Time & 0xFF)`.
  - `MSG_SendAffect` Type `0x03B9`, size `268`, somente para o alvo, com `STRUCT_AFFECT[32]`
    completo.
- Na expiracao, o slot e zerado e `SendScore` e reenviado para remover icone/efeito.
- `Time >= 32400000` e permanente no legado.

## Huntress 85/86

- `SkillData.csv` do servidor Windows confirma: `85 = Explosão_Etérea`, `86 = Escudo_Dourado`.
- O comentario legado `Escudo_dourado` no bloco `skillnum == 85` se refere ao affect Type 31
  ("Escudo Dourado"), nao ao nome da skill.
- Skill 85 cobra gold especial `100 * Level` antes do gasto de MP; sem gold suficiente, aborta o cast.
  O `Level` e a variavel local da skill no fluxo legado, derivada do Special da arvore.
- Skill 85 aplica `AffectType=31`, `AffectValue=150`, `AffectTime=30` no CSV, que vira `7` apos
  `AffectTime/4`.
- Skill 86 nao tem hardcode de gold; segue o fluxo normal de skill com `InstanceType=1`,
  `InstanceValue=35`, `Aggressive=1`, `MaxTarget=10`.
- Nao ha tabela de troca/renomeacao de indices HT. A classe vem de `skillnum/24`.

## Layouts MSVC x86

| Pacote | Type | Size | Alinhamento | Offsets relevantes |
|--------|------|-----:|-------------|--------------------|
| `MSG_Attack` | `0x0367` | 168 | natural | `SkillIndex@56`, `ReqMp@58`, `Dam@60`, stride 8 |
| `MSG_UpdateScore` | `0x0336` | 152 | `pack(1)` | `Score@12`, `Affect[0]@62`, `Guild@126`, `CurrHp@136`, `CurrMp@140`, `Special@148` |
| `MSG_SetHpDam` | `0x018A` | 20 | natural | `Hp@12`, `Dam@16` |
| `MSG_SendAffect` | `0x03B9` | 268 | natural | `Affect[i]@12+i*8`; `Type+0`, `Value+1`, `Level+2`, `Time+4` |
| `MSG_SetShortSkill` | `0x0378` | 32 | natural | `Skill1@12`, `Skill2@16` |

## SecLearnedSkill e 200+

- `STRUCT_MOBEXTRA.SecLearnedSkill` existe no offset 4 do MobExtra, mas nao ha leitura/escrita ativa no
  servidor original. A unica leitura localizada em `Basedef.cpp:3873-3884` esta comentada.
- `SaveCelestial[].SecLearnedSkill` tambem nao tem uso ativo neste build.
- A validacao viva de cast para `0 <= skillnum < 248` usa `learn = skillnum % 24` e
  `LearnedSkill & (1 << learn)` (`_MSG_Attack.cpp:196-217`).
- `200..247` nao tem mapa exclusivo de bits: `200->8`, `224->8`, `247->7`.
- O segundo conjunto/tier Celestial troca a mascara inteira via `SaveCelestial[slot].LearnedSkill`
  (`_MSG_UseItem.cpp:3218/3233`), nao por `SecLearnedSkill`.

## Affects 40+

- Busca por consumidores de `STRUCT_AFFECT.Type`/tick em `Basedef.cpp`, `Server.cpp` e timers achou
  consumidores para `0..31,34,35,36,37,38,39,42`, mas nao para `40,41,43,44,45,46,47,48`.
- Esses types ainda podem ser aplicados por `SetAffect`/`SetTick`, enviados em `MSG_UpdateScore` e
  `MSG_SendAffect`, exibidos como icone e decair pelo timer.
- Nao ha efeito server-side para score, dano, resist, regen, target selection, on-hit, item,
  invisibilidade ou cooldown.
- Skills relacionadas: `203(A41)`, `209(A45)`, `220(A44)`, `224(A43)`, `225(T46)`, `235(A48)`,
  `244(A40)`, `246(A47)`.

## Pendencias reais

- Captura byte-a-byte do cliente WYD.exe 12000/7662 ainda nao existe.
- O valor exato de `SkillIndex` que o cliente emite em melee comum permanece nao capturado.
- O comportamento visual exato do cliente durante cooldown/timer continua sem captura, embora o
  contrato server-side esteja provado por fonte.
