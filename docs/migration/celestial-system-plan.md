# Plano: Sistema de Evolução Celestial (Mortal→Arch→Celestial→Sub) + /destravar40/90 + /arcana

> Status: PLANO (não implementado). Origem: pedido para fazer `/destravar40`, `/destravar90`,
> `/arcana` funcionarem — eles são peças do sistema de evolução, que ainda não existe no Go.

## Contexto

Hoje o Go modela **só Mortal** (`internal/level`: `MaxLevel=399`, curva `g_pNextLevel`, level-up no kill).
Não há transformação de tier, curva Celestial, gates de quest, nem `QuestInfo`. Os comandos
`/destravar40/90` e `/arcana` só fazem sentido dentro do sistema de evolução. Pelo `docs/game.md`:
NPC Evoluções vende poeira → upa Mortal/Arch/Celestial → quests dos cristais → `/destravar40/90` →
lv 200 Cele → Sub-Cele → `/arcana` → resets.

## O que o legado faz (pesquisado)

- **Tier em `ClassMaster`** (`STRUCT_MOBEXTRA`, `Basedef.h:238-242`): `MORTAL=2 ARCH=1 CELESTIAL=3
  CELESTIALCS=4 SCELESTIAL=5`. No Go: `Entity.ClassMaster`/`CharacterState.ClassMaster` já existem e
  são **carregados** do DB (`store_live.go:104`), mas o `SaveCharacter` é update parcial e **NÃO
  grava** `class_master` (setado só na criação/conversão) → transformar não persiste hoje.
- **`QuestInfo.Celestial`** (`Basedef.h:659-678`): `short ArchLevel, CelestialLevel,
  SubCelestialLevel; char Lv40, Lv90, Add120, Add150, Add180, Add200, Arcana, Reset;` + `char Circle`.
  Nada disso é modelado/persistido no Go.
- **`CheckGetLevel`** (`CMob.cpp:1069`): a curva depende do tier —
  `curexp = (max_level==MAX_LEVEL) ? g_pNextLevel[cur] : g_pNextLevel_2[cur]` (Celestial usa
  `g_pNextLevel_2`). **Gate** (`:1107`): `if (ClassMaster==CELESTIAL && (cur==39 && Lv40==0 ||
  cur==89 && Lv90==0))` → trava o level-up até `/destravar40`/`/destravar90` setarem o flag.
- **Comandos** (`_MSG_MessageWhisper.cpp:628/645/676`):
  - `/destravar40` → `QuestInfo.Celestial.Lv40=1` + msg `_NN_Processing_Complete` + sinal
    `_MSG_CombineComplete`(Parm 1).
  - `/destravar90` → `Lv90=1` + msg + sinal + emotion(14,3) + **dá item 3502 (FuryStone)** (`PutItem`).
  - `/arcana` → `QuestInfo.Circle=1` + **põe item 3507 no `Equip[1]`** + `SendItem` + msg + sinal +
    emotion(14,3).
- **Transformação** (Mortal→Arch→Celestial→Sub): seta `ClassMaster` e reseta level — provavelmente em
  `_MSG_CombineItemShany.cpp` (NPC Evoluções/poeira) — **a lógica exata não está confirmada** e precisa
  do agente.

## Lacunas a preencher (no Go)

1. `g_pNextLevel_2` (curva XP Celestial, ~400 entradas) — **não temos** (array de dados).
2. `QuestInfo.Celestial` (Lv40/Lv90/Circle/CelestialLevel/...) — modelar no `Entity` + **persistir**.
3. Persistir `ClassMaster` no save (hoje só carrega).
4. `CheckGetLevel` por tier (curva + gates) — o Go só tem o caminho Mortal.
5. Transformação de tier (entrada do sistema) — lógica + efeitos (reset de level, base score, skills).
6. Sinais ao cliente: `_MSG_CombineComplete`(0x03A7, já existe) e emotion via `MsgMotion`(0x036A, já
   existe); mensagem de texto ao cliente (hoje só temos `notify`/MessageBoxOk — confirmar o formato do
   `SendClientMessage`/`_NN_*`).

## Fases propostas

**Fase 0 — Captura do agente Windows** (pré-requisito de dados). Ver prompt abaixo.

**Fase 1 — Fundação (modelo + persistência):**
- Modelar `QuestInfo.Celestial` no `Entity` (`Lv40,Lv90,Circle,CelestialLevel,SubCelestialLevel,...`).
- Persistir esses campos + `ClassMaster` no save. Opção A: colunas novas no `character` (migração).
  Opção B: reaproveitar/expandir o que o `STRUCT_MOBEXTRA` já guarda (ver como o dbserver lê hoje).
- Constantes de tier (`Mortal=2 Arch=1 Celestial=3 CelestialCS=4 SCelestial=5`).

**Fase 2 — Leveling Celestial:**
- `CheckGetLevel` por tier: seleciona `g_pNextLevel` (Mortal) vs `g_pNextLevel_2` (Celestial); aplica os
  gates 39→40 (Lv40) e 89→90 (Lv90). Fiar no caminho de XP/kill (`mobkilled.go`/`level`).

**Fase 3 — Os comandos** (em `handler/chat.go` `runCommand`, ao lado de teleportes/buffs):
- `/destravar40` → `e.Celestial.Lv40=1`; `MsgCombineComplete`(parm 1); persistir.
- `/destravar90` → `e.Celestial.Lv90=1`; dar item 3502 (carry); `MsgCombineComplete` + `MsgMotion(14,3)`.
- `/arcana` → `e.Celestial.Circle=1`; pôr 3507 no `Equip[1]` (+ `SendItem`); sinal + emotion.
- Gate de uso: idealmente só efetivo p/ `ClassMaster==CELESTIAL` (senão é cosmético).

**Fase 4 — Transformação (entrada do sistema):** NPC Evoluções/poeira que muda `ClassMaster` e reseta o
level/score — a partir da captura do agente. (Maior; pode virar um plano próprio.)

## Prompt para o agente Windows (Fase 0)

````markdown
# Captura WYD: sistema de evolução Celestial (curvas, QuestInfo, CheckGetLevel, transformação)

Contexto: migração WYD→Go, header CPSock 12B, structs de save em alinhamento natural MSVC x86. Você
tem a fonte COMPLETA + o dumper `_layout_probe/dump_layout.cpp`. Preciso modelar o tier de evolução
(MORTAL=2/ARCH=1/CELESTIAL=3/CELESTIALCS=4/SCELESTIAL=5) e os comandos de destrave. Salve em
`captura-wyd-celestial.md` (offsets+tipos+tamanhos+código).

## A) Curva de XP Celestial
- O array completo `g_pNextLevel_2[]` (todas as entradas, idealmente 0..400) — valores `long long`.
- Confirme `g_pNextLevel[]` (Mortal) se diferir do que já temos (captura-wyd-levelup.md).

## B) STRUCT_MOBEXTRA.QuestInfo (layout exato via dumper)
- `offsetof`/tipo de cada campo de `QuestInfo` (Mortal/Arch/Celestial/Circle), `sizeof(QuestInfo)`, e
  o offset de `QuestInfo` dentro de `STRUCT_MOBEXTRA`. Em especial os campos `Celestial.Lv40/Lv90/
  CelestialLevel/SubCelestialLevel/Add120..200/Arcana/Reset` e `Circle`.
- Como `ClassMaster` é guardado em `STRUCT_MOBEXTRA` (offset/tipo) e se é persistido no arquivo de conta.

## C) CheckGetLevel (código completo, CMob.cpp:1069)
- O corpo inteiro: seleção de curva por tier, os gates (39→40/89→90), e o que muda no level-up
  Celestial (HP/MP/score/AC/skill/special bonus por nível, `g_pCelestialRate[15]`).
- Como `CelestialLevel`/`SubCelestialLevel` se relacionam com `MOB.CurrentScore.Level` (o nível "vivo").

## D) Transformação de tier (Mortal→Arch→Celestial→Sub)
- Onde e como `ClassMaster` (extra.ClassMaster) é alterado: mostre o(s) handler(es) (provável
  `_MSG_CombineItemShany.cpp` / NPC Evoluções) — o item de poeira/Lac usado, a receita, e o efeito
  (reset de Level/BaseScore/skills, `SaveCelestial[2]`, o que zera e o que mantém).
- A relação `SaveCelestial[2]` (parece guardar o estado por tier) — quando é salvo/restaurado.

## E) Comandos de destrave (confirmar)
- Confirme `_MSG_MessageWhisper.cpp` /destravar40 (:628), /destravar90 (:645), /arcana (:676): os flags
  setados, os itens dados (3502/3507), e os sinais (`SendClientSignalParm(_MSG_CombineComplete,1)`,
  `SendEmotion(14,3)`, `SendClientMessage(_NN_Processing_Complete)`) — valores/Type dos pacotes.

Salve em `captura-wyd-celestial.md`.
````

## Recomendação

Rodar a **captura do agente** primeiro; depois Fases 1→3 (modelo+persistência+comandos) dão os
comandos com persistência e os itens de recompensa, e a Fase 2 faz o "destrave" gatear de verdade
quando houver chars Celestial. A Fase 4 (transformação) é a entrada e pode ser um plano separado.
