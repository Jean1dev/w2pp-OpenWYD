# Huntress skills 72..95

## Identidade mecanica

Huntress concentra dano fisico a distancia, furtividade, evasao, buffs/debuffs proprios e utilitarios ligados a Soul/retarget. A implementacao Go cobre dano, Ilusao, buffs, passivas principais, invisibilidade, custo 85/86 e os fluxos HT de combine/Extracao.

Evidencia usa os codigos definidos em `README.md`.

## Catalogo

| Índice | Nome | Árvore | Tipo | Target | Mana | SkillPoint | Instance | Affect/Tick | Fórmula/Efeito | Status | Evidência |
|-------:|------|--------|------|--------|-----:|-----------:|----------|-------------|----------------|--------|-----------|
| 72 | Ataque_Fatal | 1 | dano | 1 | 15 | 21 | 1/30 | - | dano elemental type 1 base 30 | IMPLEMENTED | CSV+CAST+DMG |
| 73 | Ilusão | 1 | utilitário | 0 | 45 | 39 | 0/0 | - | `MSG_Action3`, MP, learned bit e cadencia 900 ms | IMPLEMENTED | CSV+LEGACY+CAST+TEST |
| 74 | Encantar_Gelo | 1 | passivo | 0 | 0 | 66 | 0/0 | - | bit 2 habilita bonus de dano por arma HT | IMPLEMENTED | CSV+SCORE+TEST |
| 75 | Agressividade | 1 | buff | 0 | 30 | 51 | 0/0 | A27/1/150 | affect 27 ativa `RSV_FROST` quando arma tem `EF_WTYPE=101` | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 76 | Imunidade | 1 | buff | 0 | 42 | 84 | 0/0 | A19/15/150 | affect 19 aplica `RSV_BLOCK` | IMPLEMENTED | CSV+AFF+SCORE |
| 77 | Meditação | 1 | buff | 0 | 90 | 87 | 0/0 | A21/15/150 | affect 21 reduz AC e aumenta `DAMAGEMULTI` | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 78 | Lança_de_Ferro | 1 | passivo | 0 | 0 | 96 | 0/0 | - | passiva sem cast; caminho fisico HT tolera multi-target legado | IMPLEMENTED | CSV+LEGACY+CAST |
| 79 | Tempestade_de_Raios | 1 | dano | 6 | 75 | 233 | 1/0 | - | branch especial usa `Damage*180/100`, `BASE_GetDamage` e divide por 2 | IMPLEMENTED | CSV+CAST+DMG+TEST |
| 80 | Golpe_Felino | 2 | dano | 1 | 10 | 18 | 1/30 | - | dano elemental type 1 base 30 | IMPLEMENTED | CSV+CAST+DMG |
| 81 | Ligação_Espectral | 2 | buff | 0 | 108 | 39 | 0/0 | A37/0/150 | affect 37 soma `ForceDamage` a golpes positivos | IMPLEMENTED | CSV+LEGACY+SCORE+TEST |
| 82 | Perícia_do_caçador | 2 | passivo | 0 | 15 | 69 | 0/0 | A29/1/150 | bit 10 faz off-hand contar 100%; affect 29 aplica Soul modelado | IMPLEMENTED | CSV+LEGACY+SCORE+TEST |
| 83 | Alquimia | 2 | utilitário | 0 | 30 | 54 | 0/0 | - | rota `_MSG_CombineItemAlquimia` usa engine de combine e exige classe HT | IMPLEMENTED | CSV+LEGACY+COMBINE |
| 84 | Extração | 2 | utilitário | 0 | 30 | 84 | 0/0 | - | `_MSG_CombineItemExtracao` consome catalisador 1774 e transforma item refinado | IMPLEMENTED | CSV+LEGACY+COMBINE |
| 85 | Explosão_Etérea | 2 | buff | 0 | 120 | 90 | 0/0 | A31/150/7 | aplica affect 31 "Escudo Dourado"; cobra gold `100*Level` | IMPLEMENTED | CSV+CAST+AFF+WIN+TEST |
| 86 | Escudo_Dourado | 2 | dano | 5 | 25 | 81 | 1/35 | - | dano elemental type 1 base 35; sem custo especial de gold | IMPLEMENTED | CSV+CAST+DMG+WIN+TEST |
| 87 | Troca_de_Espíritos | 2 | buff | 0 | 90 | 242 | 0/160 | A38/0/150 | affect 38 troca metade de MaxMP por MaxHP | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 88 | Lâmina_das_Sombras | 3 | dano | 1 | 9 | 24 | 1/0 | - | dano elemental type 1 base 0 | IMPLEMENTED | CSV+CAST+DMG |
| 89 | Evasão_Aprimorada | 3 | buff | 0 | 24 | 42 | 0/0 | A26/1/150 | affect 26 aplica `RSV_PARRY` | IMPLEMENTED | CSV+AFF+SCORE |
| 90 | Toxina_da_Serpente | 3 | passivo | 0 | 35 | 51 | 0/0 | A30/1/6 | affect 30 soma `ForceMobDamage` contra mobs | IMPLEMENTED | CSV+LEGACY+SCORE+TEST |
| 91 | Visão_Caçadora | 3 | passivo | 0 | 0 | 69 | 0/0 | A38/2/0 | bit 18 adiciona critical por `Special[3]` e Dex | IMPLEMENTED | CSV+LEGACY+SCORE+TEST |
| 92 | Olhos_de_Águia | 3 | buff | 0 | 30 | 75 | 0/0 | A36/1/150 | affect 36 ativa `RSV_DRAIN` quando arma tem `EF_WTYPE=41` | IMPLEMENTED | CSV+AFF+SCORE+TEST |
| 93 | Lâmina_Aérea | 3 | passivo | 0 | 0 | 90 | 0/0 | - | bit 21 adiciona proc fisico em `MSG_AttackTwo` | IMPLEMENTED | CSV+LEGACY+CAST+TEST |
| 94 | Proteção_das_Sombras | 3 | passivo | 0 | 0 | 96 | 0/0 | - | bit 23 soma AC por `Special[3]/3+10` | IMPLEMENTED | CSV+SCORE+TEST |
| 95 | Invisibilidade | 3 | buff | 0 | 90 | 233 | 0/160 | A28/1/0 | affect 28 aplica `RSV_HIDE`; mobs ignoram alvo invisivel | IMPLEMENTED | CSV+AFF+SCORE+AI |

## Regras comuns

- Dano HT usa `3*weaponDamage + 3*Str + Level/2 + Special + base`, sem multiplicador de Magic, depois 5/4.
- Passivas HT modeladas: bit 2 bonus por arma, bit 7 +200 damage, bit 10 off-hand 100%, bit 18 critical, bit 21 proc de Lamina Aerea e bit 23 AC.
- `AffectType` 19/21/26/27/28/29/30/31/36/37/38 tem efeito de score, RSV ou dano.
- `Skillnum==79` usa branch legado proprio: base 180% do dano, depois `BASE_GetDamage` e divisao por 2.
- `Skillnum==85` e `Explosão_Etérea`, mas aplica o affect 31 chamado "Escudo Dourado"; o bloco legado cobra gold `100*Level` antes do gasto de MP. `Skillnum==86` e `Escudo_Dourado` e nao tem custo especial de gold.
- Duracao do affect 31 (issues #236/#229): a skill 85 tem `AffectTime` bruto 30 (contra 600 das linhas longas), ou seja e a cauda curta da tabela. O ÷8 uniforme da #92 a truncava para 3 (−57%, nao a metade). Com o loader de volta ao ÷4 e o piso de 60 s de `world.AffectDuration` — que so vale para affects nao agressivos, e a skill 85 e `Aggressive=0` — ela fica em **64 s estaveis em qualquer mastery**, em vez de escalar com `Special`. Ver `TestCastBuffDurationCurve`.

## Lacunas e riscos

- `MobExtra.Soul` ja e decodificado do save legado, trafega pelo DB/gRPC interno e alimenta `Entity.Soul` para os efeitos de score.
- O marcador visual secundario `Dam[1]` da Lamina Aerea foi deferido para evitar reprocessar slot visual como alvo real no loop Go; o dano server-side ja e aplicado.
- Receitas/taxas exatas de Alquimia seguem parametrizadas pela engine de combine; a restricao de classe HT esta no handler.

## Testes minimos esperados

- Dano das skills 72, 79, 80 e 88 com weapon damage controlado.
- Ilusao via movimento/Action3, MP e learned bit.
- Imunidade, Meditacao, Evasao, Frost, Drain, ForceDamage, Soul e Invisibilidade atualizando score/RSV.
- Skill 85 cobrando gold `100*Level` e aplicando affect 31; skill 86 sem custo especial de gold.
