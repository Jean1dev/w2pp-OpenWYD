# Status das perguntas para o agente Windows - skills

As oito perguntas foram respondidas e consolidadas em `windows-agent-findings.md`.
Este arquivo agora rastreia o que ficou resolvido e o que ainda exige captura real do cliente.

## Respostas recebidas

| Prompt | Status | Documento de referencia |
|--------|--------|-------------------------|
| WIN-SKILL-001 | Fonte/layout recebidos; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |
| WIN-SKILL-002 | Fonte/layout recebidos; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |
| WIN-SKILL-003 | Fonte recebida; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |
| WIN-SKILL-004 | Fonte/layout recebidos; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |
| WIN-SKILL-005 | Fonte/dados recebidos; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |
| WIN-SKILL-006 | Dumper MSVC x86 recebido | `windows-agent-findings.md` |
| WIN-SKILL-007 | Fonte recebida; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |
| WIN-SKILL-008 | Fonte recebida; captura real `NAO_ENCONTRADO` | `windows-agent-findings.md` |

## Contratos agora desbloqueados

- `MSG_Attack` e layouts S->C sensiveis a skill tem offsets MSVC x86 provados.
- `Dam[i].Damage` e marcador: `-2` melee, `-1` skill, `0` vazio; dano real e server-authoritative.
- `ReqHp/ReqMp` sao alvos server-side inicializados no login e convergidos por tick.
- Cast sem MP envia `MSG_SetHpMp` (`0x0181`, 28 B) e aborta.
- O servidor aplica piso de 800 ms entre ataques; `Delay` do CSV nao e cooldown server-side.
- Buffs usam tick de 8 s, `AffectTime/4`, `Time=(AffectTime+1)*Delay/100`, `MSG_UpdateScore` grid
  e `MSG_SendAffect` self.
- Huntress `85 = Explosão_Etérea` e `86 = Escudo_Dourado`; so a 85 cobra gold especial.
- `SecLearnedSkill` e campo morto/reservado neste build; cast `200..247` usa `LearnedSkill % 24`.
- Affects/ticks `40/41/43/44/45/46/47/48` sao icon-only/no-op no servidor original.

## Pendencias de captura, nao bloqueantes para a implementacao server-side

- Captura C->S real do cliente WYD.exe 12000/7662 para confirmar o `SkillIndex` exato em melee.
- Captura visual de cooldown/timer para comparar UI do cliente com o contrato server-side.
- Captura de cast HT 85/86 para documentar gold/MP/dano antes/depois no cliente real.
