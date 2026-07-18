# Prompt para o agente Windows — Curva de EXP Celestial (`g_pNextLevel_2`)

> Cole isto na instância do Claude Code na máquina Windows (que tem a **fonte completa**
> que compila + o dumper `_layout_probe`). Escopo **estreito**: só falta o **array de dados**
> `g_pNextLevel_2[]`. A **lógica** já foi portada para o Go a partir da nossa cópia parcial
> (`CMob.cpp:1069 CheckGetLevel`, os gates 39/89, os ganhos por tier) — não precisamos do
> código de novo, só dos números.

## Contexto

Migração do WYD (cliente 12000, header CPSock=12B) de C++ para Go. O tier Celestial
(`ClassMaster` CELESTIAL=3 / CELESTIALCS=4 / SCELESTIAL=5) já está modelado no servidor Go:
persistência de `class_master` + os flags `QuestInfo.Celestial.Lv40/Lv90/Circle`, os comandos
`/destravar40` `/destravar90` `/arcana`, e o level-up por tier com os gates (issue #117, fase 1).

O único bloqueio para valores **corretos** é a curva de XP Celestial. `CheckGetLevel`
(`CMob.cpp:1092-1093`) seleciona `g_pNextLevel_2[cur]` quando `max_level == MAX_CLEVEL`, mas o
array literal mora em `Basedef.cpp` (**ausente** na nossa cópia — mesma situação da curva Mortal
`g_pNextLevel`, que veio da captura em `captura-wyd-levelup.md`). Hoje o Go roda contra um
**placeholder sintético** (`tmserver/internal/level/nextlevel.go`, `celestialPlaceholderCurve`).

## O que preciso (valores byte-exatos)

1. **`g_pNextLevel_2[]` — o array completo.** Onde é preenchido (hardcoded em `Basedef.cpp`,
   fórmula no boot, ou lido de arquivo?). Se for tabela, me dê **todos os valores** (índices
   `0..MAX_CLEVEL`, i.e. 0..199 — o `extern` é `[MAX_CLEVEL + 202]`), como `long long`. Se for
   fórmula, a fórmula exata + o valor de teto usado no clamp.
   - Confirmar o índice base: nível celestial 1 → índice 0 ou 1? (a curva Mortal usa
     `nextLevel[cur+1]` como próximo limiar).
   - `MAX_CLEVEL` exato (nossa cópia diz `199`, `Basedef.h:178`) e o teto `g_pNextLevel_2[MAX_CLEVEL+1]`.

2. **Confirmar `CheckGetLevel` (`CMob.cpp:1069-1187`)** — só para bater com o que portamos:
   - Os gates: `CELESTIAL && (cur==39 && Lv40==0 || cur==89 && Lv90==0) → return 0` (`:1107`).
     Existe algum outro gate Celestial (ex.: `Add120/150/180/200` em níveis 120/150/180/200)?
   - Os ganhos por nível no ramo Celestial (`:1147`): confirmamos **só** `Ac++` +
     `BASE_GetBonusScorePoint` (sem `SkillBonus`/`SpecialBonus`). Bate?
   - Como `CelestialLevel`/`SubCelestialLevel` (em `QuestInfo.Celestial`) se relacionam com
     `MOB.BaseScore.Level` (o nível "vivo") — são espelhos, ou níveis separados por tier?

3. **`QuestInfo.Celestial` layout** (via dumper, se prático): `offsetof`/tipo de
   `Lv40, Lv90, Add120..200, Arcana, Reset, ArchLevel, CelestialLevel, SubCelestialLevel` e o
   `char Circle`, além do offset de `QuestInfo` em `STRUCT_MOBEXTRA` e `sizeof(QuestInfo)`.
   (Modelamos só Lv40/Lv90/Circle por enquanto; o resto é para o trabalho futuro de transformação.)

## Onde salvar

Salve em `docs/migration/captura-wyd-celestial.md` (valores + offsets + qualquer trecho de
código que difira do nosso). Quando chegar, substituímos `celestialPlaceholderCurve` pelo array
real e adicionamos os anchors em `TestNextLevel2Placeholder` (vira golden, como
`TestNextLevelTable`).
