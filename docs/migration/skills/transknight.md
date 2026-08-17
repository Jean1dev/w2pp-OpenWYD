# Transknight skills 0..23

## Identidade mecanica

Transknight mistura dano fisico/magico de curto alcance com buffs defensivos, sustain e finalizadores. O Go ja cobre o fluxo principal de varias skills de dano por `SkillBaseDamage` + `SkillDamage`, mas ainda faltam utilitarios e efeitos especiais que mexem em posicao, MP, passivas e affects secundarios.

Evidencia usa os codigos definidos em `README.md`.

## Catalogo

| Índice | Nome | Árvore | Tipo | Target | Mana | SkillPoint | Instance | Affect/Tick | Fórmula/Efeito | Status | Evidência |
|-------:|------|--------|------|--------|-----:|-----------:|----------|-------------|----------------|--------|-----------|
| 0 | Giro_da_Fúria | 1 | dano | 3 | 15 | 24 | 4/5 | - | dano elemental type 4 base 5 | IMPLEMENTED | CSV+CAST+DMG |
| 1 | Toque_Sagrado | 1 | debuff | 4 | 20 | 48 | 0/0 | A1/2/0 | affect 1/2/0; slow de movimento e penalidade de INT para robe | IMPLEMENTED | CSV+LEGACY+AFF+SCORE+TEST |
| 2 | Golpe_Duplo | 1 | dano | 1 | 10 | 66 | 1/13 | - | dano elemental type 1 base 13 | IMPLEMENTED | CSV+CAST+DMG |
| 3 | Possuído | 1 | buff | 0 | 105 | 84 | 0/0 | A14/10/150 | `v = Special*3/4 + 10`; `Con += v`, `MaxHP += 2v` (sem o `×3` de ClassMaster do `Basedef.cpp:4056` — ver `audit-affects.md`) | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 4 | Fanatismo | 1 | dano | 1 | 30 | 72 | 4/75 | - | dano elemental type 4 base 75 | IMPLEMENTED | CSV+CAST+DMG |
| 5 | Aura_da_Vida | 1 | buff | 0 | 53 | 81 | 0/0 | T17/75 | tick 17 cura HP por Level/2+Value | IMPLEMENTED | CSV+AFF+SCORE |
| 6 | Fúria_Divina | 1 | controle | 1 | 75 | 90 | 0/0 | - | desloca alvo, respeita imunidades e aciona batalha de grupo | IMPLEMENTED | CSV+LEGACY+TEST |
| 7 | Destino | 1 | dano | 6 | 140 | 224 | 3/190 | - | dano elemental type 3 base 190 | IMPLEMENTED | CSV+CAST+DMG |
| 8 | Carga | 2 | dano | 1 | 12 | 21 | 1/15 | - | dano elemental type 1 base 15 | IMPLEMENTED | CSV+CAST+DMG |
| 9 | Mestre_das_Armas | 2 | passivo | 0 | 0 | 45 | 0/0 | - | mao secundaria contribui 100% do EF_DAMAGE em WeaponDamage | IMPLEMENTED | CSV+LEGACY+SCORE+TEST |
| 10 | Golpe_Mortal | 2 | dano | 1 | 20 | 69 | 1/40 | - | dano elemental type 1 base 40 | IMPLEMENTED | CSV+CAST+DMG |
| 11 | Assalto | 2 | buff | 0 | 47 | 78 | 0/0 | A13/7/150 | +15% dano, DAMAGEMULTI e -10% MaxHP | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 12 | Espada_da_Fênix | 2 | dano | 1 | 25 | 72 | 1/60 | - | dano elemental type 1 base 60 | IMPLEMENTED | CSV+CAST+DMG |
| 13 | Samaritano | 2 | buff | 0 | 25 | 72 | 0/0 | A24/0/150 | **DIVERGÊNCIA (issue #267)**: `v = Special*3/4`; `Con += v`, `MaxHP += 2v`. O legado (`Basedef.cpp:4225`) faz `AC += AC/4+Value` — ver B16 em `ingame-bugs.md`. Cai em todo ataque (`DoRemoveSamaritano`) | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 14 | Noção_de_Combate | 2 | passivo | 0 | 0 | 90 | 0/0 | - | mastery de mitigacao Special[2]/20, clamp 0..15 | IMPLEMENTED | CSV+LEGACY+CAST+TEST |
| 15 | Armadura_Crítica | 2 | passivo | 0 | 150 | 218 | 0/1 | A36/0/150 | affect 36 ativa RSV_DRAIN | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 16 | Perseguição | 3 | debuff | 3 | 25 | 24 | 0/0 | A3/60/0 | reduz resistencias conforme robe | IMPLEMENTED | CSV+AFF+SCORE |
| 17 | Espada_Flamejante | 3 | dano | 4 | 25 | 45 | 2/25 | - | dano elemental type 2 base 25 | IMPLEMENTED | CSV+CAST+DMG |
| 18 | Contra_Ataque | 3 | dano | 1 | 17 | 60 | 1/35 | - | dano elemental type 1 base 35 | IMPLEMENTED | CSV+CAST+DMG |
| 19 | Lâmina_Congelantem | 3 | dano | 1 | 22 | 78 | 3/50 | A7/1/0 | dano elemental type 3 base 50; affect 7 penaliza INT de robe | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE+TEST |
| 20 | Ataque_da_Alma | 3 | dano | 1 | 30 | 66 | 1/65 | T20/2 | dano elemental type 1 base 65; veneno -1000 HP/tick com ReqHp | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE+TEST |
| 21 | Punhalada_Venenosa | 3 | dano | 1 | 38 | 87 | 1/80 | T20/10 | dano elemental type 1 base 80; veneno -1000 HP/tick com ReqHp | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE+TEST |
| 22 | Exterminar | 3 | dano | 1 | 0 | 90 | 2/90 | - | consome MP restante, soma MP+INT ao dano e reposiciona alvo | IMPLEMENTED | CSV+CAST+DMG+LEGACY+TEST |
| 23 | Tempestade_de_Gelo | 3 | dano | 6 | 150 | 227 | 3/210 | A1/2/0 | dano elemental type 3 base 210; affect 1 aplicado com score | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE+TEST |

## Regras comuns

- Cast de classe usa learned bit `1<<(skillnum%24)`, classe `skillnum/24`, Special da arvore `(index%24)/8+1`, e custo `BASE_GetManaSpent`.
- Dano elemental TK usa dois caminhos: arvore 2 escala com `3*weaponDamage + 3*Str + Level + Special + base`; outras arvores usam `Special + base + weaponDamage + Level + Int/4 + Int/40`, depois multiplicador de Magic e 5/4.
- TK com bit 14 aprendido fornece mastery de mitigacao de skill `Special[2]/20`, clamped em `0..15`.
- TK com bit 9 aprendido faz a mao secundaria contribuir 100% do `EF_DAMAGE` no `WeaponDamage`.

## Lacunas e riscos

- Captura real do cliente ainda pode validar detalhes visuais finos dos `MSG_Action` de Furia Divina e
  Exterminar, mas a regra server-side esta implementada por fonte local.

## Testes minimos esperados

- Castar uma skill de dano de cada arvore contra mob e player.
- Garantir que `Fúria_Divina` nao afeta merchants/clan 6/geradores imunes quando implementada.
- Validar `Exterminar`: MP vai a zero, `ReqMp` ecoa zero, dano inclui MP anterior e reposicionamento.
- Regressao de poison tick para skills 20/21.
- Aprendizado da 8a skill com 7 pre-requisitos e 50M gold.
