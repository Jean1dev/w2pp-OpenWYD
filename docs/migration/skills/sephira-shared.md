# Sephira/shared skills 96+

## Identidade mecanica

O bloco `96+` mistura livros Sephira, utilitarios e a faixa `200..247` do segundo conjunto. As respostas
Windows `WIN-SKILL-007/008` fecharam a regra viva deste build:

- `SecLearnedSkill` existe no `STRUCT_MOBEXTRA`, mas e campo morto/reservado; nao autoriza, nao ensina e
  nao valida skill.
- O gate de cast para qualquer `0 <= skillnum < 248`, incluindo `96..103` e `200..247`, usa
  `STRUCT_MOB.LearnedSkill & (1 << (skillnum % 24))`.
- A arvore/especial usado no cast e `CurrentScore.Special[(skillnum % 24)/8 + 1]`.
- `SaveCelestial[slot].LearnedSkill` e quem troca o conjunto ativo em sistemas Celestial/Sub-Celestial;
  isso ainda e preservado como dado de save, mas a promocao/troca de tier nao faz parte desta fase.
- Affects/ticks `40/41/43/44/45/46/47/48` sao icon-only/no-op no servidor original: entram no slot,
  aparecem no cliente e decaem, mas nao alteram score, dano, resist, regen, target, on-hit ou cooldown.
- A "Soul permanente" entregue pela Kibita e outra flag: ela ativa diretamente o bit 30 de
  `LearnedSkill`. Nao ensina a skill 102 (cujo gate vivo e o bit `102%24 == 6`) e nao preenche
  `MobExtra.Soul`, que continua sendo o atributo lido pelo affect 29 de `Limite_da_Alma`.

Evidencia usa os codigos definidos em `README.md`.

## Catalogo

| Índice | Nome | Árvore | Tipo | Target | Mana | SkillPoint | Instance | Affect/Tick | Fórmula/Efeito | Status | Evidência |
|-------:|------|--------|------|--------|-----:|-----------:|----------|-------------|----------------|--------|-----------|
| 96 | Poder_Superior | 1 | buff | 0 | 60 | 75 | 0/0 | A39/5/3 | affect 39 aplica +100 ExpBonus; gate vivo usa bit 0 | IMPLEMENTED | CSV+CAST+AFF+SCORE+WIN |
| 97 | Canhão_Guardião | 1 | dano | 4 | 0 | 0 | 2/2000 | - | dano elemental type 2 base 2000; exige item 746 no grid | IMPLEMENTED | CSV+CAST+DMG+WIN |
| 98 | Muro_de_Espinhos | 1 | desconhecido | 6 | 42 | 36 | 0/0 | - | cria Vinha em celula valida usando template configurado | IMPLEMENTED | CSV+CAST+LEGACY+WIN |
| 99 | Ressureição | 1 | desconhecido | 0 | 0 | 35 | 0/0 | - | morto pode castar e revive com HP/MP randomicos | IMPLEMENTED | CSV+CAST+LEGACY+WIN |
| 100 | Concentração | 1 | passivo | 0 | 0 | 40 | 0/0 | A20/0/0 | branch legado de Accuracy nao tem consumidor server-side; gameplay no-op | IMPLEMENTED | CSV+LEGACY+WIN |
| 101 | Força_Espectral | 1 | passivo | 0 | 0 | 39 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 102 | Limite_da_Alma | 1 | buff | 0 | 75 | 0 | 0/0 | A29/0/150 | affect 29 aplica Soul no score quando `Soul` esta populado | IMPLEMENTED | CSV+AFF+SCORE+WIN |
| 200 | Proteção_Divina | 2 | buff | 0 | 300 | 255 | 0/0 | A6/0/150 | affect 6 modelado; gate vivo usa bit 8 | IMPLEMENTED | CSV+CAST+AFF+SCORE+WIN |
| 201 | Bênção_Divina | 2 | desconhecido | 0 | 15 | 350 | 0/0 | - | sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 202 | Blaze_Luncher | 2 | passivo | 1 | 500 | 350 | 0/0 | T12/0 | tick/affect 12 modelado quando aplicado; linha passiva sem consumidor extra | IMPLEMENTED | CSV+AFF+SCORE+WIN |
| 203 | Clod_Attack | 2 | passivo | 0 | 0 | 350 | 0/0 | A41/0/0 | affect 41 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 204 | Mãos_Sangrentas | 2 | passivo | 0 | 0 | 285 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 205 | Mestre_das_Armas | 2 | passivo | 0 | 0 | 350 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 206 | Fencing_Master | 2 | passivo | 0 | 0 | 300 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 207 | WindStorm | 2 | passivo | 0 | 0 | 300 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 208 | Espelho_Mágico | 3 | passivo | 0 | 0 | 300 | 0/0 | - | unico uso de `SecLearnedSkill` era comentario morto; gameplay no-op | IMPLEMENTED | CSV+LEGACY+WIN |
| 209 | Conexão_de_Gelo | 3 | passivo | 0 | 0 | 350 | 0/0 | A45/0/0 | affect 45 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 210 | Freezing_Counter | 3 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 211 | Seal_Master | 3 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 212 | Cenote | 3 | passivo | 0 | 120 | 355 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 213 | Proteção_Absoluta | 3 | buff | 0 | 400 | 360 | 0/0 | A6/0/0 | affect 6 modelado; gate vivo usa bit 21 | IMPLEMENTED | CSV+CAST+AFF+SCORE+WIN |
| 214 | Life_Wave | 3 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 215 | Holy_Power | 3 | passivo | 0 | 0 | 300 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 216 | Magia_Misteriosa | 1 | buff | 0 | 125 | 325 | 0/0 | A42/0/3 | affect 42 HP/MP modelado; gate vivo usa bit 0 | IMPLEMENTED | CSV+CAST+AFF+SCORE+WIN |
| 217 | Congelamento_Proficiente | 1 | passivo | 0 | 0 | 340 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 218 | Thunder_Force | 1 | passivo | 0 | 0 | 350 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 219 | Thunder_Lising | 1 | passivo | 0 | 0 | 350 | 0/0 | A1/255/2 | affect 1 ja modelado quando aplicado | IMPLEMENTED | CSV+AFF+SCORE+WIN |
| 220 | Remover_Memória | 1 | passivo | 0 | 0 | 310 | 0/0 | A44/0/0 | affect 44 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 221 | Incapacitador | 1 | debuff | 1 | 500 | 330 | 0/0 | A5/0/0 | affect 5 DEX% modelado; gate vivo usa bit 5 | IMPLEMENTED | CSV+CAST+AFF+SCORE+WIN |
| 222 | Special_Master | 1 | passivo | 0 | 0 | 300 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 223 | Another_Change | 1 | desconhecido | 1 | 800 | 320 | 0/0 | - | sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 224 | Anti_Magia | 2 | buff | 0 | 255 | 260 | 0/0 | A43/0/0 | affect 43 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 225 | Chama_Resistente | 2 | buff | 0 | 600 | 300 | 0/0 | T46/100 | tick 46 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 226 | Resi_Decrease | 2 | debuff | 1 | 300 | 350 | 0/0 | T3/50 | tick 3 aplica debuff de resist; gate vivo usa bit 10 | IMPLEMENTED | CSV+CAST+AFF+SCORE+WIN |
| 227 | Manaburn_Master | 2 | passivo | 0 | 0 | 350 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 228 | Unidade_Mental | 2 | passivo | 0 | 0 | 255 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 229 | Invocação_Final | 2 | summon | 0 | 600 | 305 | 11/9 | - | value 9 mapeia template 8 com `pSummonBonus` zero; gate vivo usa bit 13 | IMPLEMENTED | CSV+CAST+SUMMON+WIN |
| 230 | Poworful_Summon | 2 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 231 | Ice_Armor | 2 | passivo | 0 | 0 | 300 | 0/0 | A1/255/1 | affect 1 ja modelado quando aplicado | IMPLEMENTED | CSV+AFF+SCORE+WIN |
| 232 | Concha_Resistente | 3 | passivo | 0 | 0 | 275 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 233 | Espinhos_Fortalecidos | 3 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 234 | Weapon_Power | 3 | passivo | 0 | 0 | 300 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 235 | Last_Resistance | 3 | buff | 0 | 500 | 320 | 0/0 | A48/15/0 | affect 48 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 236 | Contra_Ataque | 3 | passivo | 0 | 0 | 285 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 237 | Ataque_Rápido_Proficiente | 3 | passivo | 0 | 0 | 350 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 238 | Skill_Master | 3 | passivo | 0 | 0 | 350 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 239 | Freezing | 3 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 240 | Ponto_do_Mestre | 1 | passivo | 0 | 0 | 355 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 241 | Absorção_de_Alma | 1 | desconhecido | 1 | 0 | 350 | 0/0 | - | sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 242 | Plunder | 1 | passivo | 0 | 0 | 320 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 243 | Reinforce_Soul | 1 | passivo | 0 | 0 | 300 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 244 | Tiro_Direto | 1 | passivo | 0 | 0 | 255 | 0/0 | A40/0/0 | affect 40 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 245 | Garra_Habilidosa | 1 | passivo | 0 | 0 | 355 | 0/0 | - | passiva sem consumidor server-side neste build | IMPLEMENTED | CSV+WIN |
| 246 | Bleeding | 1 | buff | 1 | 300 | 350 | 0/0 | A47/100/2 | affect 47 icon-only/no-op no servidor original | IMPLEMENTED | CSV+AFF+WIN |
| 247 | Ambush | 1 | passivo | 0 | 0 | 350 | 0/0 | A1/50/1 | affect 1 ja modelado quando aplicado | IMPLEMENTED | CSV+AFF+SCORE+WIN |

## Regras comuns

- Cast vivo usa `LearnedSkill & (1 << (skillnum % 24))` para `0..247`; `96` usa bit 0, `200` usa bit 8,
  `247` usa bit 7.
- O poder/custo usa `CurrentScore.Special[(skillnum % 24)/8 + 1]`, nao `SecLearnedSkill`.
- Livros Sephira usam `EF_VOLATILE` `31..38`, setam bits `24..31` em `LearnedSkill` e consomem apenas
  uma unidade quando o livro esta stackado; esses bits sao persistidos por paridade, mas o branch de cast
  `1<<(skillnum-72)` e morto neste build.
- `SecLearnedSkill` e `SaveCelestial[].SecLearnedSkill` sao campos mortos/reservados neste build.
- `SaveCelestial[slot].LearnedSkill` preserva a mascara por conjunto/tier Celestial; a troca de tier em si
  fica fora desta fase.
- Affects/ticks `40/41/43/44/45/46/47/48` sao aplicados genericamente como icone/timer e decaem, sem
  efeito de score/combat/tick.
- Skill 97 tem formula especial `15*Level + base`, mas o legado tambem exige item 746 na celula do cast.
- Skill 99 e constante magica que permite personagem morto agir e revive o proprio player com HP/MP randomicos.

## Lacunas e riscos

- Promocao/troca Celestial/Sub-Celestial ainda precisa de uma fase propria para usar `SaveCelestial[]` como
  conjunto ativo. A Fase 6 apenas preserva os campos e implementa a regra de cast do conjunto carregado.
- Captura ao vivo do cliente continua util para documentar UI/bytes, mas nao bloqueia a paridade server-side
  provada por fonte para este build.

## Testes minimos esperados

- Livro Sephira ensina bit correto, consome item e persiste learned mask.
- Livro Sephira stackado consome uma unidade e atualiza `UpdateEtc.Learn`.
- Cast `96` usa bit `0`/Special da arvore 1 e aplica affect 39.
- Cast `97` falha sem item 746 e causa dano com item valido.
- Cast `98` cria Vinha em celula valida e falha fora de grid/area proibida.
- Cast `99` revive o personagem morto.
- Cast `200..247` usa `LearnedSkill` modulo 24, sem `SecLearnedSkill`.
- Affects `40/41/43/44/45/46/47/48` nao alteram score/dano/resist/regen/tick.
