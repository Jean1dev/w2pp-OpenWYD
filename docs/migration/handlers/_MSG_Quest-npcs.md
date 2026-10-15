# `_MSG_Quest` (0x028B) — enumeração dos NPCs de quest

> Detalha o handler de interação com NPC de quest (resumo em
> [lote2-quest-ranking-cash.md](lote2-quest-ranking-cash.md)). Fonte:
> `Source/Code/TMSrv/_MSG_Quest.cpp` (2753 linhas). Linhas conferidas via `#pragma region`/`case`.

## Despacho (NPC → `npcMode`)
- **Entrada:** `MSG_STANDARDPARM2` — `Parm1 = npcIndex` (o NPC), `Parm2 = confirm` (0 = "perguntar",
  ≠0 = "confirmar/executar").
- **Pré:** `npcIndex` em `[MAX_USER, MAX_MOB)`; cancela trade/autotrade.
- **Resolução do tipo (`:50-160`):** o "tipo de NPC" vem de **`MOB.Merchant`** e, quando
  `Merchant==100`, do **grade** do item equipado no NPC
  (`BASE_GetItemAbilityNosanc(Equip[0], EF_GRADE0)`). Se `npcMode` continua `-1` → `return`.
  Depois entra num `switch(npcMode)`.

| Merchant | Grade | `npcMode` | Linha case | Propósito |
|---------:|------:|-----------|-----------:|-----------|
| 100 | 0 | `QUEST_COVEIRO` | 293 | Quest Arch 256 — etapa 1 (lvl 39–115) |
| 100 | 1 | `QUEST_JARDINEIRO` | 338 | Quest Arch 256 — etapa 2 (lvl 115–190) |
| 100 | 2 | `QUEST_KAIZEN` | 382 | Quest Arch 256 — etapa 3 (lvl 190–265) |
| 100 | 3 | `QUEST_HIDRA` | 426 | Quest Arch 256 — etapa 4 (lvl 265–320) |
| 100 | 4 | `QUEST_ELFOS` | 470 | Quest Arch 256 — etapa 5 |
| 100 | 5 | `LIDER_APRENDIZ` | 2625 | líder aprendiz (sub-quest/promoção) |
| 100 | 7/8/9 | `PERZEN` | 2398 | NPC Perzen |
| 100 | 13 | `QUEST_CAPAREAL` | 2342 | quest capa real |
| 100 | 14 | `CAPAVERDE_TELEPORT` | 2167 | teleporte (capa verde) |
| 100 | 15 | `MOLARGARGULA` | 2236 | quest "molar gárgula" (flag Mortal) |
| 100 | 16 | `TREINADORNEWBIE4` | 2046 | tutorial novato 4 |
| 100 | 22 | `SOBREVIVENTE` | 2597 | NPC sobrevivente — troca implementada (#325) |
| 100 | 30 | `GUARDA_REAL_EVT1` | 2664 | guarda real (evento) |
| 72 | — | `UXMAL` | 1313 | NPC Uxmal — inscrição na Pista de Runas (#355) |
| 36 | — | `TREINADORNEWBIE1` | 1895 | tutorial novato 1 |
| 40 | — | `TREINADORNEWBIE2` | 1938 | tutorial novato 2 |
| 41 | — | `TREINADORNEWBIE3` | 1992 | tutorial novato 3 |
| 8 | — | `CAPAVERDE_TRADE` | 2188 | trade (capa verde) |
| 78 | — | `BLACKORACLE` | 2257 | oráculo negro |
| 120 | — | `CARBUNCLE_WIND` | 2364 | carbúnculo (vento) |
| 4 | — | `GOLD_DRAGON` | 1471 | dragão dourado |
| 10 | — | `AMU_MISTICO` | 253 | quest "Terra Mística" (flag Mortal; exige party) |
| 11 | — | `EXPLOIT_LEADER` | 1626 | líder (exploit/evento) |
| 12 | — | `JEFFI` | 514 | NPC Jeffi |
| 13 | — | `SHAMA` | 612 | NPC Shama |
| 14 / 15 | — | `KING` | 693 | NPC rei |
| 19 | — | `COMP_SEPHI` | 2102 | composição Sephi |
| 26 | — | `KINGDOM` | 1127 | kingdom broker |
| 30 | — | `ZAKUM` | 238 | info da quest Zakum (ocupação da área) |
| 31 | — | `MESTREHAB` | 1745 | mestre de habilidade |
| 68 | — | `GODGOVERNMENT` | 2565 | governo (god government) |
| 58 | — | `MOUNT_MASTER` | 170 | curar/ressuscitar montaria (`Equip[14]`) |
| 62 | — | `ARZAN_DRAGON` | 1397 | dragão de Arzan |
| 76 | — | `URNAMMU` | 1247 | NPC Urnammu |
| 74 | — | `KIBITA` | 2430 | cidadania, buff condicional e Soul permanente |
| 200 | — | `CURANDEIRO` | 2717 | curandeiro |

> **Total:** 38 tipos de NPC → 36 blocos `case`/`#pragma region` (KING cobre Merchant 14 e 15).

## Padrão comum de quest (vale para a maioria dos `case`)
1. **Pré-condições:** classe (`extra.ClassMaster` MORTAL/ARCH/CELESTIAL…), **faixa de nível**
   (min/max), flags de quest em `extra.QuestInfo` (Mortal/Arch/Celestial) e/ou itens no inventário.
2. **`confirm == 0`** → `SendSay(npcIndex, ...)`: o NPC **pergunta/explica** (preço, item exigido,
   etc.). Sem efeito de estado.
3. **`confirm != 0`** → **executa**: consome item/coin, concede recompensa (item/exp/teleporte),
   e **grava a flag de progresso** (`extra.QuestInfo.*` ou `pMob[conn].QuestFlag`). Loga.

## Detalhe — cadeia "Quest 256" (Arch) — `COVEIRO → JARDINEIRO → KAIZEN → HIDRA → ELFOS`
Etapas sequenciais de progressão de nível (rumo ao cap 256/Arch). Cada NPC:
- exige `ClassMaster ∈ {MORTAL, ARCH}` (senão `_NN_Level_Limit2`);
- exige **faixa de nível** específica (senão `_NN_Level_limit`);
- consome um **ticket** (item por etapa) do inventário (senão "traga o item X");
- seta `pMob[conn].QuestFlag` e **teleporta** o jogador para a área da prova (jitter `rand()%5-3`).

| Etapa | NPC (grade) | Faixa de nível | Ticket (`sIndex`) | `QuestFlag` | Teleporte (aprox.) |
|------:|-------------|----------------|------------------:|------------:|--------------------|
| 1 | COVEIRO (0) ✅ | 39 – 115 | 4038 | 1 | (2398, 2105) |
| 2 | JARDINEIRO (1) ✅ | 115 – 190 | 4039 | 2 | (2234, 1714) |
| 3 | KAIZEN (2) ✅ | 190 – 265 | 4040 | 3 | (464, 3902) |
| 4 | HIDRA (3) ✅ | 265 – 320 | 4041 | 4 | (668, 3756) |
| 5 | ELFOS (4) ✅ | 320 – 350 | 4042 | 5 | (1322, 4041) |

> **Nomes in-game** (a fonte legada só usa os apelidos internos acima; estes são os nomes que o
> cliente mostra na tooltip do ticket, levantados nas issues #261/#263/#264/#265):
> 1 Coveiro = *Defensor da Alma* · 3 Kaizen = *Ressureição do Cavaleiro Negro* (NPC "Patrulha", na
> Dungeon Negro) · 4 Hidra = *Hidra Imortal* (mesmo NPC "Patrulha") · 5 Elfos =
> *Início da Infelicidade* (NPC "Guarda", no Submundo).
>
> A tooltip mostra a faixa **1-based** (ex.: "Mortal 321~350" para o passo 5), enquanto a checagem
> legada usa o nível armazenado 0-based `[min, max)` — por isso os títulos das issues citam o limite
> superior +1. Os valores da tabela são os do `_MSG_Quest.cpp`, e batem com a tooltip.

## Exemplos de NPCs não-cadeia (amostra detalhada)
- **`MOUNT_MASTER` (Merchant 58, `:170`):** cura/ressuscita a montaria (`Equip[14]`, `sIndex`
  2330–2390). `confirm==0` mostra preço (`g_pItemList[mount].Price`); `confirm!=0` cobra gold e, por
  RNG (`vit -= rand()%3`), restaura a vitalidade do mount **ou** o destrói (`memset`). Recalcula score.
- **`ZAKUM` (Merchant 30, `:238`):** apenas **informa** quem está na área da quest Zakum
  (`GetUserInArea(2180,1160,2296,1270)`); sem efeito.
- **`AMU_MISTICO` (Merchant 10, `:253`):** quest "Terra Mística" — exige `ClassMaster==MORTAL`, flag
  `QuestInfo.Mortal.TerraMistica==0` e estar **em party**; ao confirmar seta
  `QuestInfo.Mortal.TerraMistica = 1`.
- **`TREINADORNEWBIE1..4`, `CAPAVERDE_*`, `KING`, `KINGDOM`, `CURANDEIRO`, traders/info:** NPCs de
  tutorial/teleporte/troca/serviço — mesmo padrão (perguntar/confirmar), efeitos pontuais
  (teleporte, item inicial, cura, troca). Passo-a-passo fino: abrir a linha do `case` indicada na
  tabela de despacho.

## Detalhe — `KIBITA` (Merchant 74)

O `case KIBITA` (`_MSG_Quest.cpp:2430-2562`) ignora `confirm` e avalia três serviços nesta ordem:

1. **Cidadania:** se `Citizen==0` e há pelo menos 4.000.000 de gold, cobra o valor, define
   `Citizen=ServerIndex+1`, atualiza a cidadania da guilda quando o jogador é líder e encerra o case.
2. **Buff temporário:** existe somente sob `#ifdef KIBITA_SOUL` (não há definição desse macro na
   árvore versionada). Em dias úteis, às 21h, Mortal abaixo do nível 369 entrega o item 420, recebe
   affect 29 por `AFFECT_1H` e é teleportado para perto de `(2454,1843)`.
3. **Soul permanente ✅:** Mortal com `CurrentScore.Level>=369` e sem o bit 30 de `LearnedSkill`
   entrega a Pedra Secreta da classe. O slot é limpo integralmente, o bit 30 é ativado, `Equip[15]`
   é substituído pela capa do reino e `CharLogOut` força o reload antes de `_MSG_SendArchEffect`.

| Classe (`MOB.Class`) | Pedra Secreta | Item |
|---------------------:|----------------|-----:|
| 0 — TransKnight | Água | 5334 |
| 1 — Foema | Sol | 5336 |
| 2 — Beastmaster | Terra | 5335 |
| 3 — Huntress | Vento | 5337 |

| Reino (`MOB.Clan`) | Capa (`Equip[15]`) |
|--------------------:|-------------------:|
| 7 — Hekalotia | 3194 |
| 8 — Akelonia | 3195 |
| qualquer outro | 3196 |

O bit 30 é uma flag permanente de progressão. Este ramo não escreve `STRUCT_MOBEXTRA.Soul`; esse
campo elemental é outro conceito, consumido pelo affect 29. No servidor Go somente o terceiro ramo
está implementado; cidadania e o bloco condicional continuam pendentes.

## Notas de migração
- **Estado de quest** mora em `STRUCT_MOBEXTRA.QuestInfo` (Mortal/Arch/Celestial, Fase 2 §1.5) e em
  `pMob[conn].QuestFlag` (estado volátil da etapa atual) — mapear cada flag no schema novo.
- **Tudo hardcoded:** faixas de nível, `sIndex` de tickets/recompensas, coordenadas de teleporte e o
  mapeamento Merchant/grade→quest. Forte candidato a **conteúdo data-driven** (ex.: tabela
  `quest`/`quest_step` ou scripts) — alinha com o gap apontado na comparação de projetos
  (open-wyd-scripts) e em game-rules (Fase 4).
- **Identificação do NPC por `Merchant` + `EF_GRADE0` do item equipado** é um quirk a preservar na
  leitura dos dados de NPC (Fase 2 — `NPCGener`/BaseMob) durante a migração.
- **Grade 0 é ambíguo — e isso é legado, não bug.** `BASE_GetItemAbilityNosanc` (`:34`) devolve `0`
  quando o `Equip[0]` não tem efeito `EF_GRADE0`, então templates Merchant-100 sem esse efeito
  (`FirstTrainer`, `Guarda_`, `Treinador1` em `Release/TMsrv/run/npc/`) caem em `QUEST_COVEIRO`
  junto com o Coveiro (que tem `EF_GRADE0 0` explícito). A porta do grade 0 no Go reproduz isso de
  propósito; desambiguar quebraria a paridade.
- **UNVERIFIED (passo-a-passo fino):** recompensas/etapas internas exatas dos blocos longos
  (`JEFFI`, `SHAMA`, `KING`, `KINGDOM`, `URNAMMU`, `ARZAN_DRAGON`, `GOLD_DRAGON`, `EXPLOIT_LEADER`,
  `MESTREHAB`, `GODGOVERNMENT`) — o **despacho e o propósito** estão mapeados; abrir a linha
  do `case` ao implementar cada um.

> **Status:** todos os **38 tipos de NPC / 36 `case`** mapeados (gatilho Merchant/grade → modo →
> propósito → linha) + a cadeia Arch detalhada. Enumeração da Fase 5 concluída.

## Sobrevivente — Hall de Kefra (#325)

O NPC `MOB.Merchant=100`, `EF_GRADE0=22` converte o primeiro item **4127**
(Pergaminho Selado) nos slots acessíveis do inventário em **100 acessos** ao Hall
(o valor legado; cada servidor ajusta com `-kefra-entries-per-scroll` /
`W2PP_KEFRA_ENTRIES_PER_SCROLL`, #343).
A interação é imediata, tanto com `confirm=0` quanto com `confirm!=0`, e limpa
inteiramente o slot selecionado, conforme `_MSG_Quest.cpp:2598-2623`. Não exige
espaço livre. Se a soma exceder `int32`, nada muda.
O servidor envia o slot vazio e a mensagem `_DN_CHANGE_COUNT` (420) com o saldo.

**Desvio intencional (#343):** sem pergaminho acessível, o legado fica em silêncio;
o Go responde com `_DN_CHANGE_COUNT` e o saldo atual (inclusive 0), para o jogador
consultar os acessos restantes no próprio NPC.

A mensagem sai como no `SendClientMessage` (`SendFunc.cpp:27-43`): `MSG_MessagePanel`
com `HEADER.ID` 0 e o texto em Windows-1252. O cliente 7662 descarta o painel com
outro ID (`TMScene.cpp:1371`), e até o #343 o saldo nunca aparecia na tela. A mesma
correção vale para o saldo do Pesadelo (`/nt` e a troca de entradas), que usa a
mesma string.

O template real `npc/Sobrevivente` tem `MOB.Merchant=100` no byte 17, mas
`CurrentScore.Merchant=68` (o valor exposto por `Entity.Merchant`). O despacho
aceita o discriminador legado do template e também `Entity.Merchant=100`, sempre
com grade 22, sem alterar os dados enviados ao cliente.

O saldo é individual por personagem e acompanha inventário e atributos no
salvamento normal. A entrada no Hall usa o bloco `(2364,3892)` (coordenadas
arredondadas para múltiplos de quatro), exige saldo positivo e desconta um acesso.
O destino é `(2364+rand()%3,3906+rand()%3)`, com duas chamadas ao RNG MSVC, X antes
de Y (`GetFunc.cpp:994-1004`). Uma tentativa sem saldo não consome RNG nem teleporta.
Outras rotas e o ciclo de vida do boss Kefra não fazem parte desta implementação.

## Uxmal — Pista de Runas (#355, #373)

O template `npc/Uxmal` tem `MOB.Merchant=72` no byte 17, mas
`CurrentScore.Merchant=104` (o valor de `Entity.Merchant`, que também aparece
nos treinadores). Por isso o despacho usa só o byte 17 (`isUxmalNPC`).

**Inscrição** (`_MSG_Quest.cpp:1313-1395`, `handler/runequest.go`). As
verificações seguem a ordem do legado e cada recusa envia uma mensagem de
`Language.txt`:

1. Só aceita nos minutos 16–19, 36–39 e 56–59. Fora disso: "Quest em progresso.".
2. Só aceita líder de grupo ou jogador solo.
3. Procura o primeiro slot do inventário com a Pista da Runas (5134). A sala é a
   sanc da pista, limitada a 6.
4. A sala 0 tem 2 vagas e as demais têm 3. Sala cheia: "Há muitos jogadores.".
5. Rejeita o mesmo conn já inscrito na sala.
6. Registra a inscrição, consome a pista e envia "Entrada registrada.".

**Entrada** nos minutos 0, 20 e 40 (`ProcessSecMinTimer.cpp:1082-1148`):

- Só entram inscrições cujo nome ainda confere, com o líder parado no bloco
  (25,13) e ainda líder ou solo.
- O líder é teleportado para `PistaPos[sala][grupo]`, junto com os membros em
  jogo que estejam no mesmo bloco.
- Spawn por sala: Lich (5654 para o grupo 1, 5653 para o grupo 2), Torre
  (5706–5764) e o boss Amon (5789). O Labirinto sorteia `MobCount = 8 + rand()%8`
  com o RNG MSVC e spawna 5854–5898 para o grupo 1.

**Saída** nos minutos 15, 35 e 55 (`:1150-1429`):

- Remove todos os mobs da caixa x 3310–3588, y 1005–1663.
- Revive com HP 1 quem estiver morto ali dentro.
- Teleporta todos para (3294,1701).
- Limpa as inscrições.

Os timers rodam uma vez por rodada de 20 minutos. Depois de um restart no meio da
rodada o servidor não refaz a entrada, mas ainda executa a saída. Os blocos -1 da
pista (`world.IsRuneQuestGenerator`) não spawnam no boot nem entram na fila de
respawn. A população permanente das salas (blocos com `MinuteGenerate 1`)
continua no timer de minuto, como no legado.

**Salas** (#373, `handler/runequest_kills.go`). O crédito vai para
`partyleader = Leader`, ou para o próprio matador quando `Leader <= 0`. A
**recompensa padrão** é:

- uma runa de `PistaRune[sala][rand()%N]` para o líder;
- outra para cada membro de `PartyList` em `USER_PLAY`;
- a pista 5134 com `EF_SANC = sala+1`, só para o líder.

A runa do líder é sorteada mesmo quando ele não pode recebê-la. O
`world.RuneQuestRunes` guarda só as N entradas sorteáveis: a última de cada linha
do legado nunca sai.

Hooks de morte antes do `DeleteMob` (`MobKilled.cpp:2158-2500`), chamados em
`mobKilled` logo após a água, antes de ouro, EXP e loot:

| Sala | Regra |
|---|---|
| +0 Lich | `rand()%100`, depois apaga todos os Lichs (`DeleteMob(c,3)`, inclusive o que está morrendo). Com `< 20`, recompensa padrão com sanc 1. Senão, `GenerateMob` 5653 2× e 5654 2×. |
| +1 Torre | Matar a torre `5706+t` zera o `MobCount[t]`. Um mob 5709–5764 pontua para o primeiro grupo `t` cuja torre está viva e cujo líder é o `partyleader`. |
| +2 Amon | O boss 5789 dá a recompensa padrão com sanc 3. |
| +3 Sulrang | Com `Party[0]` inscrito e `MobCount == 0`: `MobCount = 1` e `GenerateMob(5948 + rand()%7)`. |
| +4 Labirinto | Só o último mob do bloco (`CurrentNumMob == 1`, ainda contando o que morre) decrementa o grupo do `partyleader`, e só se os outros dois tiverem `MobCount >= 1`. No 0, o grupo vai para (3351,1334), com os membros do grupo 1 em x 3352, e spawna o 5849. O 5849 dá a recompensa padrão com sanc 5. |
| +5 Balrog | O 5899 coloca `Party[0].MobCount = 1` e dá a recompensa padrão com sanc 6. |
| +6 Coelho | Cada mob 5775–5785 decrementa o contador; no 1→0 spawna o 5767. O 5767 dá só as runas. |

Depois do `DeleteMob` (`:2958-2986`):

- Os mobs da Torre (5709–5764) e do Amon (5790–5848) voltam na hora.
  - Exceção: blocos que a fila de respawn de 15s já repõe (`MinuteGenerate <= 0`
    fora do evento), para não spawnar duas vezes.
- Cada Kalintz (5948–5955) spawna o próximo (`rand()%7`) e incrementa o
  `MobCount` do grupo do `partyleader`.

**Prêmios na saída** (`ProcessSecMinTimer.cpp:1186-1416`):

- Rodam depois da limpeza e antes de zerar as inscrições.
- Na Torre (sanc 2) e no Sulrang (sanc 4), o grupo com `MobCount` estritamente
  maior que os outros dois recebe a recompensa padrão.
- O nome do líder precisa ainda conferir.

**Portais do Balrog** (`:254-514`), a cada tick, para cada líder inscrito em
`Pista[5]`. O líder precisa estar vivo, ainda ser líder ou solo, e o nome precisa
conferir.

1. `rand()%3` é sorteado **antes** da checagem do tile.
2. A posição `& 0xFFFC` é comparada com `world.RuneQuestBalrogPortals`, a tabela
   `PistaBalrogPortalPos` já com o destino de cada resultado (os ramos que o
   sorteio 0–2 não alcança foram removidos).
3. Se o destino é a sala do boss e `Party[0].MobCount == 0`, gera o 5899 antes de
   cobrar.
4. Sem a Semente de Cristal (4032), não há salto. Com ela, uma é consumida e o
   grupo vai para `PistaBalrogPos[destino]`.

**Contador do Coelho:**

- **Envio:** a cada 6 ticks, `_MSG_MobLeft` (0x3B0, `MSG_STANDARDPARM`, ID
  `ESCENE_FIELD`) vai para os jogadores em x 3330–3447 e y 1475–1524. O
  `SendSignalParmArea` tem a borda superior exclusiva (`:536`).
- **Devolução:** o contador sobe, limitado a 100 e só se for diferente de 0, em
  dois casos:
  - um coelho 5775–5785 chega ao fim da rota (`Processor & 0x10000`, `:2004`);
  - um coelho 5768–5774 entra no portão x 3423–3442, y 1492–1511 (`:1856`,
    `:2049`).
- **Remoção:** qualquer mob que entra no portão, exceto o 5767, é apagado com
  `DeleteMob(.,3)`.

**Guarda do Amon** (`_MSG_Action.cpp:177-189`, `movement.go`). Um movimento para
x 3378–3398, y 1427–1448 é bloqueado se o jogador não for o líder do próprio
grupo, ou se não for líder inscrito em `Pista[2]`. O jogador vai para
`PistaPos[2][rand()%3]`.

**Troca de líder** (`Server.cpp:8205-8220`, `leaderLeaveParty`). Toda inscrição
com o id e o nome do líder que sai passa para o novo líder. O `TransferGroup`
(`:8317-8346`) ainda não tem port.

**Armadilhas do Labirinto** (`:1715-1804`, `SendFunc.cpp:541-600`,
`Server.cpp:6081-6160`), a cada 3 ticks, nas 44 caixas de `world.RuneQuestTraps`:

- Se algum líder inscrito em `Pista[4]` está na caixa (borda superior
  exclusiva), nada acontece.
- Senão, se há algum jogador, o `EnvEffect(32)` sai do centro.
- Depois, todo jogador vivo dentro da caixa (borda inclusiva) perde 2000 HP, ou
  fica com 1.

**`DeleteMob` sem morte.** O legado decrementa o `CurrentNumMob` para qualquer
tipo diferente de 0 (`Server.cpp:7820`). O `world.RemoveGeneratorMob` faz isso
para os tipos 3 usados aqui e para o fim de rota dos mobs de rota 3. Antes, um
bloco cujos mobs saíam pelo fim da rota ficava no teto e nunca era reposto.

**Desvios:**

- **Alvos errados corrigidos.** O RNG continua o mesmo. Os três casos:
  - Torre, grupo 3: a runa do líder ia para o líder do grupo 1 (`:1275`).
  - Sulrang, grupo 2: o ticket ia para `Party[3]`, fora do array (`:1373`).
  - Armadilhas: a checagem de líder testava `Party[1]` duas vezes e nunca
    `Party[2]` (`SendFunc.cpp:588`, `Server.cpp:6111`).
- **Cadência.** O timer do legado é de 500ms (`Server.cpp:4086`) e o tick é de 1s.
  Por isso:
  - o portal do Balrog faz metade dos `rand()` por segundo;
  - `SecCounter%12` vira 6 ticks;
  - `SecCounter%5` (2,5s) vira 3 ticks.
- **Mob que mata mob.** A morte de um mob por outro mob que não seja summon
  (`mobai.go`) não passa pelos hooks. O legado ainda descontaria o Coelho e
  repovoaria a Torre nesse caso, que é raro dentro das salas. **UNVERIFIED**
  se ele ocorre na prática.
- **Lich.** Quando o hook do Lich já apagou o mob que está morrendo, `mobKilled`
  só consome o sorteio do `DieSay` e não remove de novo o slot, que pode já ser
  de um Lich novo.
