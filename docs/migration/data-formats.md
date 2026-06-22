# Fase 2 — Formatos de Dados / Persistência (w2pp-OpenWYD)

> **Objetivo:** ler os dados existentes e desenhar o novo schema. Fonte: `Basedef.h` (structs),
> `DBSrv/CFileDB.cpp`, `DBSrv/CRanking.cpp`, `TMSrv/CReadFiles.cpp`, e exemplos reais em `Release/`.
>
> **Reuso:** complementa `docs/agents/component-deep-analyzer/component-analysis-DBSrv-*.md` e
> `component-analysis-Basedef-*.md`. Aqui o foco é o **layout binário/textual exato** para migração.

---

## 0. Modelo de persistência (visão geral)

Não há banco de dados. **Toda persistência é arquivo plano**:

- **Contas/personagens:** um arquivo binário por conta, gravado com `_write()` do bloco
  `STRUCT_ACCOUNTFILE` inteiro (`DBSrv/CFileDB.cpp:2469`). Não há serialização — é o dump cru da
  struct C (com alinhamento/padding do MSVC x86).
- **Conteúdo do jogo (read-only em runtime):** CSV/TXT/BIN carregados na inicialização
  (`ItemList`, `SkillData`, `NPCGener`, mapas, drops, etc.).
- **Estado de mundo persistente:** guildas (`Guilds.txt`, `GuildInfo`), ranking (`Ranking.txt`),
  cidade/castelo (`ChampionCity_*`, `Guild_*`, `Chall_*`).

> **Implicação de migração nº 1:** os arquivos de conta são *dumps de struct C dependentes de
> compilador/arquitetura* (little-endian x86, alinhamento MSVC). Para ler em outra stack é preciso
> reproduzir o layout byte-a-byte (offsets/padding), **não** apenas os campos. Recomenda-se escrever
> um conversor único (one-shot) struct→banco, e não tentar ler o formato cru em produção.

---

## 0.1. Alinhamento natural vs `#pragma pack(1)` — LEIA ANTES DE ESCREVER O CONVERSOR

⚠️ **Este é o detalhe que mais quebra conversores.** No `Basedef.h` convivem **dois regimes de
layout diferentes**, e confundi-los desalinha o arquivo inteiro:

| Regime | Quem usa | Layout | Onde no `Basedef.h` |
|--------|----------|--------|---------------------|
| **Alinhamento NATURAL** (com padding MSVC x86) | **Structs de persistência**: `STRUCT_ACCOUNTFILE`, `STRUCT_ACCOUNTINFO`, `STRUCT_MOB`, `STRUCT_ITEM`, `STRUCT_SCORE`, `STRUCT_AFFECT`, `STRUCT_QUEST` | campos alinhados ao próprio tamanho; **há bytes de padding**; o struct é arredondado ao maior alinhamento de membro | **fora** de qualquer `#pragma pack` (linhas ~500–1045 e 1085–1108) |
| **`#pragma pack(push, 1)`** (sem padding) | **Structs de protocolo/rede** (`MSG_*`) e algumas de serialização (`STRUCT_RANKING`) | campos colados, **zero padding** | `#pragma pack(push,1)` em `Basedef.h:1047,1545,1823,2451` (cada um com seu `pop`) |

**Verificado por evidência:** `grep "pragma pack" Basedef.h` → blocos packed em 1047–1080
(`STRUCT_RANKING`), 1545–1584 (`MSG_AccountLogin` & cia.), 1823–1850, 2451–2485. As structs de save
(`STRUCT_MOB`@556-599, `STRUCT_ACCOUNTFILE`@1085-1108, etc.) **não** estão dentro de nenhum desses
blocos → **alinhamento natural**.

### Por que isso importa na prática

Se você copiar o `pack(1)` da Fase 1 (correto para mensagens) e aplicá-lo ao `STRUCT_MOB` do save,
o tamanho muda e **todos os 4 personagens + cargo + tail do arquivo deslocam**:

- `STRUCT_MOB` com **alinhamento natural = 816 bytes** (valor correto do save).
- `STRUCT_MOB` com **`pack(1)` = 805 bytes** (errado para o save) → erro de **11 bytes por personagem**.

O `816` é **confirmado**, não estimado: bate com o comentário do código
`STRUCT_MOB Char[MOB_PER_ACCOUNT]; // 216 - 3480` (3480−216 = 3264 = 4 × **816**) e com o cálculo
manual do `sizeof` MSVC x86 abaixo. Os campos `long long Exp` e o arredondamento final do struct a 8
bytes são o que produzem o padding.

### Mapa de padding do `STRUCT_MOB` (alinhamento natural, x86)

```text
off  tam  campo
  0   16  MobName[16]
 16    1  Clan
 17    1  Merchant
 18    2  Guild            (ushort, alinha em 18 — ok)
 20    1  Class
 21   (1) >>> PAD para alinhar Rsv (ushort) <<<
 22    2  Rsv
 24    1  Quest
 25   (3) >>> PAD para alinhar Coin (int) <<<
 28    4  Coin
 32    8  Exp              (long long, alinhamento 8 — 32%8==0, ok)
 40    2  SPX
 42    2  SPY
 44   48  BaseScore        (STRUCT_SCORE, 48)
 92   48  CurrentScore
140  128  Equip[16]        (16 × STRUCT_ITEM[8])
268  512  Carry[64]        (64 × 8)
780    4  LearnedSkill     (long = 4 no Win32)
784    4  Magic
788    2  ScoreBonus
790    2  SpecialBonus
792    2  SkillBonus
794    1  Critical
795    1  SaveMana
796    4  SkillBar[4]
800    1  GuildLevel
801   (1) >>> PAD para alinhar RegenHP (ushort) <<<
802    2  RegenHP
804    2  RegenMP
806    4  Resist[4]
810   (6) >>> PAD final: struct arredonda ao alinhamento 8 (por causa de Exp) <<<
816  ----  sizeof(STRUCT_MOB)
```

> Nota: `long` e ponteiros são **4 bytes** no alvo Win32 (x86). Se algum dia o projeto for compilado
> x64, `long long` continua 8 mas ponteiros viram 8 — irrelevante para o save atual, que é x86.

### O que o conversor DEVE fazer

1. **Não usar `pack(1)`** ao mapear as structs de save. Reproduzir o alinhamento natural acima
   (offsets/padding), seja declarando a struct equivalente sem packing, seja lendo por offset
   explícito.
2. **Travar o layout no build de referência** com asserções, para detectar qualquer divergência de
   compilador/flags antes de rodar a migração:

   ```cpp
   // rodar uma vez no projeto C++ original (ou num shim) para confirmar o contrato binário
   static_assert(sizeof(STRUCT_ITEM)        ==   8, "STRUCT_ITEM != 8");
   static_assert(sizeof(STRUCT_SCORE)       ==  48, "STRUCT_SCORE != 48");
   static_assert(sizeof(STRUCT_AFFECT)      ==   8, "STRUCT_AFFECT != 8");
   static_assert(sizeof(STRUCT_ACCOUNTINFO) == 216, "STRUCT_ACCOUNTINFO != 216");
   static_assert(sizeof(STRUCT_MOB)         == 816, "STRUCT_MOB != 816");
   static_assert(offsetof(STRUCT_MOB, Coin)         ==  28, "Coin off");
   static_assert(offsetof(STRUCT_MOB, Exp)          ==  32, "Exp off");
   static_assert(offsetof(STRUCT_MOB, BaseScore)    ==  44, "BaseScore off");
   static_assert(offsetof(STRUCT_MOB, Equip)        == 140, "Equip off");
   static_assert(offsetof(STRUCT_MOB, Carry)        == 268, "Carry off");
   static_assert(sizeof(STRUCT_QUEST)       ==  56, "STRUCT_QUEST != 56");
   static_assert(sizeof(STRUCT_MOBEXTRA)    == 552, "STRUCT_MOBEXTRA != 552");
   static_assert(sizeof(STRUCT_ACCOUNTFILE) == 7952, "STRUCT_ACCOUNTFILE != 7952");
   static_assert(offsetof(STRUCT_ACCOUNTFILE, Char)    == 216, "Char off");
   static_assert(offsetof(STRUCT_ACCOUNTFILE, Cargo)   == 3480, "Cargo off");
   static_assert(offsetof(STRUCT_ACCOUNTFILE, Coin)    == 4504, "Coin off");
   static_assert(offsetof(STRUCT_ACCOUNTFILE, affect)  == 4572, "affect off");
   static_assert(offsetof(STRUCT_ACCOUNTFILE, mobExtra)== 5600, "mobExtra off");
   ```

   > ⚠️ **Premissa do cálculo de 7952: `time_t` = 8 bytes** (padrão do MSVC desde VS2005; este projeto
   > usa toolset v143/VS2022). `STRUCT_MOBEXTRA` e `STRUCT_QUEST` contêm vários `time_t`
   > (`LastNT`, `DivineEnd`, `LastPenalty`, `DonateInfo.LastTime`, `LastTimeQuest`). Se o build
   > definir `_USE_32BIT_TIME_T`, cada `time_t` cai para 4 bytes e os tamanhos mudam
   > (MOBEXTRA e ACCOUNTFILE encolhem). O `static_assert` acima é justamente o que detecta isso — é a
   > forma definitiva de fechar o número no build de referência.

3. Lembrar que as **mensagens de rede** (Fase 1) **continuam `pack(1)`** — o conversor de save e o
   parser de protocolo seguem regimes diferentes; isolar os dois no código.

---

## 1. Arquivo de conta (`account/<Key>/<NOME>`)

### 1.1. Convenção de nome e diretório

`DBSrv/CFileDB.cpp:2415-2494` (`DBWriteAccount`) e `:2543+` (`DBReadAccount`):

1. `check = strupr(AccountName)` — nome **em maiúsculas** (`:2423`).
2. Rejeita nomes reservados do DOS: `COM0..9` e `LPT0..9` (`:2425-2428`) — herança Win32.
3. `BASE_GetFirstKey(check, First)` deriva o subdiretório (`:2432`). Pelo exemplo real
   `Release/DBsrv/run/account/A/antonio`, `First` = **primeira letra** do nome (uppercased): `"A"`.
4. Caminho final: `./account/<First>/<check>` (`:2436`).
5. Abre com `_open(..., O_RDWR|O_CREAT|O_BINARY, _S_IREAD|_S_IWRITE)`, `_lseek(0)`,
   `_write(account, sizeof(STRUCT_ACCOUNTFILE))` — sobrescreve o arquivo inteiro a cada save.

> **UNVERIFIED:** `BASE_GetFirstKey` está só declarada (`Basedef.h:2789`); a implementação não está
> no código-fonte disponível (provável lib/objeto). O comportamento "primeira letra maiúscula" é
> **inferido do exemplo** `account/A/antonio`. Confirmar com mais amostras ou desmontando o binário.
>
> **Implicação nº 2 (case-sensitivity):** o nome do arquivo no disco do exemplo é `antonio`
> (minúsculo) num dir `A` (maiúsculo). No Windows (FS case-insensitive) `ANTONIO`/`antonio` colidem;
> em Linux/containers **não**. A migração precisa normalizar (ex.: sempre lowercase) e indexar por
> nome canônico.

### 1.2. `STRUCT_ACCOUNTFILE` (`Basedef.h:1085-1108`)

O bloco gravado. Os comentários de offset no código são de uma versão **antiga** (ver nota de
tamanho abaixo); a struct atual é:

| Campo | Tipo | Significado |
|-------|------|-------------|
| `Info` | `STRUCT_ACCOUNTINFO` | dados da conta (ver §1.3) |
| `Char[4]` | `STRUCT_MOB[MOB_PER_ACCOUNT]` | os 4 personagens (`MOB_PER_ACCOUNT=4`, `Basedef.h:131`) |
| `Cargo[128]` | `STRUCT_ITEM[MAX_CARGO]` | baú compartilhado da conta (`MAX_CARGO=128`) = 1024 bytes |
| `Coin` | `int` | gold no cargo/conta |
| `ShortSkill[4][16]` | `uchar` | barra de skills por personagem (64 bytes) |
| `affect[4][32]` | `STRUCT_AFFECT` | buffs persistidos por char (`MAX_AFFECT=32`) = 4·32·8 = 1024 bytes |
| `mobExtra[4]` | `STRUCT_MOBEXTRA` | dados extras (cidadania, fama, quests, master, …) |
| `Donate` | `int` | saldo de cash/donate |
| `TempKey[52]` | `char` | chave temporária / HWID |
| `ReceivedItem` | `bool` | flag de item recebido |
| `QuestDiaria` | `STRUCT_QUEST` | quest diária ativa |
| `BlockPass[16]` | `char` | senha de bloqueio (trava de char) |
| `IsBlocked` | `bool` | conta/char bloqueado |

**Tamanho do arquivo (importante para migração):** `DBSrv/CRanking.cpp:175` aceita um arquivo de
conta se `(fsize >= 7500 && fsize <= 7600) || fsize == sizeof(STRUCT_ACCOUNTFILE)`. Ou seja:
- Existem **arquivos legados ~7500–7600 bytes** e arquivos no **tamanho atual** =
  `sizeof(STRUCT_ACCOUNTFILE)` = **7952 bytes** (calculado com alinhamento natural e `time_t`=8; ver
  detalhamento e `static_assert` em §0.1, e os componentes `STRUCT_MOBEXTRA`=552 / `STRUCT_QUEST`=56
  em §1.5). Note que `7952 > 7600`, então o tamanho atual cai no ramo `== sizeof(...)` do `CRanking`,
  não no intervalo legado 7500–7600.
- O exemplo `account/A/antonio` tem **4294 bytes** → formato **ainda mais antigo** (sem
  `affect`/`mobExtra`/`QuestDiaria`). **UNVERIFIED** qual versão; provável pré-expansão.

> **Implicação nº 3:** o conversor de migração precisa **detectar a versão pelo tamanho do arquivo**
> e mapear cada layout. Não assumir um único formato. Registrar os tamanhos conhecidos: 4294
> (muito antigo), 7500–7600 (intermediário), `sizeof(STRUCT_ACCOUNTFILE)` (atual).

### 1.3. `STRUCT_ACCOUNTINFO` (`Basedef.h:1017-1032`)

| Offset | Campo | Tipo | Bytes | Notas |
|------:|-------|------|------:|-------|
| 0  | `AccountName` | `char[16]` | 16 | login (`ACCOUNTNAME_LENGTH`) |
| 16 | `AccountPass` | `char[12]` | 12 | **⚠ SENHA EM TEXTO PLANO** (`ACCOUNTPASS_LENGTH`) |
| 28 | `RealName` | `char[24]` | 24 | nome real do dono |
| 52 | `SSN1` | `int` | 4 | doc/identidade |
| 56 | `SSN2` | `int` | 4 | |
| 60 | `Email` | `char[48]` | 48 | |
| 108| `Telephone` | `char[16]` | 16 | |
| 124| `Address` | `char[78]` | 78 | |
| 202| `NumericToken` | `char[6]` | 6 | PIN numérico (2º fator) — também em texto plano |
| 208| `Year` | `int` | 4 | controle "uma vez por dia" |
| 212| `YearDay` | `int` | 4 | |
| | **≈ 216 bytes** | | | (bate com comentário "Info 0-216" em `Basedef.h:1087`) |

> **Dívida de segurança crítica (corrigir na migração):** `AccountPass` e `NumericToken` são
> armazenados **em claro**. Na stack nova: hash forte (argon2id/bcrypt) para a senha; o PIN também
> deve ser hash/HMAC. Ver registro de risco na Fase 9.

### 1.4. `STRUCT_MOB` (personagem persistido) (`Basedef.h:556-599`)

Players **e** mobs compartilham esta struct (ver Fase 3). Persistida dentro de `Char[4]`. Campos:

| Campo | Tipo | Significado |
|-------|------|-------------|
| `MobName[16]` | `char` | nome |
| `Clan` | `char` | clã/raça |
| `Merchant` | `uchar` | id de mercador |
| `Guild` | `ushort` | id da guilda |
| `Class` | `uchar` | classe |
| `Rsv` | `ushort` | reservado |
| `Quest` | `uchar` | progresso de quest (legado) |
| `Coin` | `int` | gold carregado |
| `Exp` | `long long` | experiência (8 bytes) |
| `SPX`,`SPY` | `short` | posição de "save point" (gema estelar) |
| `BaseScore` | `STRUCT_SCORE` | atributos-base (48 bytes, ver §1.5) |
| `CurrentScore` | `STRUCT_SCORE` | atributos atuais (48 bytes) |
| `Equip[16]` | `STRUCT_ITEM` | equipados (`MAX_EQUIP=16`) = 128 bytes |
| `Carry[64]` | `STRUCT_ITEM` | inventário (`MAX_CARRY=64`) = 512 bytes |
| `LearnedSkill` | `long` | bitmask de skills (4 categorias) |
| `Magic` | `uint` | |
| `ScoreBonus` | `ushort` | pontos de atributo livres |
| `SpecialBonus` | `ushort` | pontos especiais |
| `SkillBonus` | `ushort` | pontos de skill |
| `Critical` | `uchar` | chance de crítico |
| `SaveMana` | `uchar` | |
| `SkillBar[4]` | `uchar` | 4 primeiros slots da barra |
| `GuildLevel` | `uchar` | 0=membro … define líder |
| `RegenHP`,`RegenMP` | `ushort` | regeneração |
| `Resist[4]` | `char` | resistências fogo/gelo/raio/magia |

**Tamanho persistido:** **`sizeof(STRUCT_MOB) = 816 bytes`** (alinhamento natural x86) — valor
**confirmado** pelo comentário do código (`Char[4] = 216→3480`, 3264 = 4×816) e pelo mapa de padding
em **§0.1**. Atenção: isso vale **sem `#pragma pack`**; aplicar `pack(1)` (como nas mensagens da
Fase 1) daria 805 bytes e desalinharia o arquivo — ver §0.1. O `sizeof` do `STRUCT_ACCOUNTFILE`
**completo** ainda depende de `STRUCT_MOBEXTRA`/`STRUCT_QUEST` e deve ser travado por `static_assert`
no build de referência (§0.1).

### 1.5. Structs auxiliares

`STRUCT_ITEM` (`Basedef.h:500-522`) — **8 bytes**:
| `short sIndex` (2) | `stEffect[3]` união `short`/`{uchar cEffect; uchar cValue;}` (6) |
Macros `EF1/EFV1..EF3/EFV3` acessam os 3 pares efeito/valor. Item "vazio" = `sIndex==0`.

`STRUCT_SCORE` (`Basedef.h:524-546`) — **48 bytes**: `int Level,Ac,Damage` + `uchar
Merchant,AttackRun,Direction,ChaosRate` + `int MaxHp,MaxMp,Hp,Mp` + `short Str,Int,Dex,Con` +
`short Special[4]`.

`STRUCT_AFFECT` (`Basedef.h:735-741`) — **8 bytes**: `uchar Type; uchar Value; ushort Level;
uint Time;` (buff/debuff temporizado).

`STRUCT_QUEST` (`Basedef.h:865-882`) — **56 bytes** (align 8) — quest diária: `short IndexQuest,
Nivel,IdMob1,QtdMob1,IdMob2,QtdMob2,IdMob3,QtdMob3` (16) `; long ExpReward` (4) `; int GoldReward`
(4) `; STRUCT_ITEM Item[2]` (16) `; time_t LastTimeQuest` (pad→40, +8) `; short MobCount1..3` (6) →
54 → arredonda a 56.

`STRUCT_MOBEXTRA` (`Basedef.h:620-733`) — **552 bytes** (align 8) — dados extras por personagem
(cidadania, fama, soul, progresso de quests Mortal/Arch/Celestial, `SaveCelestial[2]`, donate/NT,
penalidades). Composição (alinhamento natural x86, `time_t` = 8 bytes):

| Bloco | Tipo | Tam | Off |
|-------|------|----:|----:|
| `ClassMaster`,`Citizen`,`SecLearnedSkill(long)`,`Fame`,`Soul`,`MortalFace` | mistos | 16 | 0 |
| `QuestInfo` | struct aninhada (Mortal 35 + Arch 35 + Celestial 44 + Circle 1 + EMPTY[30]) | 146 | 16 |
| `SaveCelestial[2]` | struct (136 cada, align 8 por `long long Exp`) | 272 | 168* |
| `LastNT`,`NT`,`KefraTicket`,`DivineEnd`,`LastPenalty`,`CheckTimeKersef`,`Hold` | `time_t`/int mix | 40 | 440 |
| `DayLog{long long;int}`, `DonateInfo{time_t;int}` | 16 + 16 | 32 | 480 |
| `EMPTY[9]` | `int[9]` | 36 | 512 |
| | **total** | **552** | (548 → pad 8) |

\* pad de 4 bytes antes de `SaveCelestial` (162 → 168, alinhamento 8).

`STRUCT_SELCHAR` (`Basedef.h:1002-1015`) — projeção enviada na tela de seleção (não persistida
isolada; derivada de `STRUCT_ACCOUNTFILE` por `DBGetSelChar`): posições, nomes, `STRUCT_SCORE[4]`,
`STRUCT_ITEM Equip[4][16]`, guilda, coin, exp dos 4 chars.

### 1.6. Export e exclusão
- `DBExportAccount` grava cópia em `S:/export/account<ServerIndex>/<NOME>` (`CFileDB.cpp:2513`) —
  caminho hardcoded num drive `S:` (ver Fase 7 para hardcodes).
- `DeleteCharacter` (`CFileDB.cpp:2328`) e ranking varrem `./account/*` (`CRanking.cpp:102,144`).

---

## 2. Mapas

| Arquivo | Tamanho | Dimensão | Semântica | Evidência |
|---------|--------:|----------|-----------|-----------|
| `HeightMap.dat` | 16.777.216 (16 MB) | `4096 × 4096`, 1 byte/célula | altura/colisão do terreno; `pHeightGrid[MAX_GRIDY][MAX_GRIDX]` | `Server.h:62,342`; `MAX_GRIDX=MAX_GRIDY=4096` (`Basedef.h:160-161`) |
| `AttributeMap.dat` | 1.048.576 (1 MB) | `1024 × 1024`, 1 byte/célula | atributos de área (andável, água, bloqueio, zona) | `g_pAttribute[1024][1024]` (`Basedef.h:2880`) |

- O mundo é um grid único de **4096×4096** células. A `HeightMap` tem 1 byte por célula. A
  `AttributeMap` é **1/4 da resolução** em cada eixo (1024×1024) → **1 atributo por bloco 4×4** de
  células de altura. **UNVERIFIED** a semântica bit-a-bit de cada byte de atributo (andável/água/
  bloqueio) — extrair por inspeção/captura.
- `pHeightGrid` é usado direto pelo pathfinding `BASE_GetRoute(...)` (`CMob.cpp:931,983,1044,1239`).

> **Migração:** manter os dois `.dat` como assets binários (não há ganho em "schematizar" um
> grid denso). Documentar a regra de indexação `grid[y][x]` (row-major, y externo) e a razão 4:1
> entre os dois mapas.

---

## 3. Arquivos de conteúdo

### 3.1. `ItemList.csv` / `ItemList.bin` → `STRUCT_ITEMLIST` (`Basedef.h:1162-1185`)

`g_pItemList[MAX_ITEMLIST]` (`Basedef.h:2878`). O `.csv` é a fonte editável; o `.bin`
(`Release/DBsrv/run/ItemList.bin`, ~910 KB) é a forma compilada carregada em runtime.

Struct (≈104 bytes; comentários de offset no próprio código):

| Campo | Tipo | Offset (comentado) |
|-------|------|------|
| `Name[64]` | `char` | 0 |
| `IndexMesh`,`IndexTexture`,`IndexVisualEffect` | `short`×3 | |
| `ReqLvl`,`ReqStr`,`ReqInt`,`ReqDex`,`ReqCon` | `short`×5 | requisitos |
| `stEffect[MAX_STATICEFFECT]` | `{short sEffect; short sValue;}` | efeitos estáticos |
| `Price` | `int` | 92 |
| `nUnique` | `short` | 96 |
| `nPos` | `short` | 98 (slot de equip) |
| `Extra` | `short` | 100 |
| `Grade` | `short` | 102 (1=Normal 2=Místico 3=Arcano 4=Lendário) |

Exemplo real (`Release/Common/ItemList.csv:1`):
```
1,TransKnight,0.0,0.0.0.0.0,0,0,1,0,0,EF_CLASS,1,EF_RANGE,1,EF_REGENHP,2,EF_REGENMP,2,EF_CRITICAL,10
```
Formato CSV: `index,Name,...,pares EF_<nome>,valor`. Os `EF_*` mapeiam para constantes de efeito
(ver `ItemEffect.h`). **UNVERIFIED** o mapeamento coluna-a-coluna completo do CSV → struct;
documentar na Fase 4/5 ao detalhar `CItem`/`CReadFiles`.

> **`ItemCSum.h`** (`Release/TMsrv/` e `DBsrv/`) é um header de checksum da `ItemList` — provável
> anti-tamper para garantir que TMSrv e DBSrv usam a mesma lista. Confirmar uso.

### 3.2. `SkillData.csv` → `STRUCT_SPELL` (`Basedef.h:1110-...`)

`MAX_SKILL`/`MAX_SPELL` entradas. Campos por skill: `SkillPoint, TargetType, ManaSpent, Delay,
Range, InstanceType, InstanceValue, TickType, TickValue, AffectType, AffectValue, AffectTime,
Act[8], InstanceAttribute, TickAttribute, ...`.

Exemplo real (`Release/Common/SkillData.csv:1`):
```
0,24,3,15,3,5,4,5,0,0,0,0,0,10.0.0.10.0.0.0.0,10.0.0.10.0.0.0.0,4,0,1,13,0,3,0,1,Giro_da_Furia
```
> ⚠ **Delay**: o `ClientPatch` divide `SkillDelay` por 4 no cliente (`Hook.cpp:230-231`) — o
> cooldown efetivo no cliente é 1/4 do tabelado. Detalhar na Fase 4.

### 3.3. `NPCGener.txt` (spawns de mob/NPC)

Texto chaveado por blocos `# [n]` (`Release/TMsrv/run/NPCGener.txt`). Campos por bloco:

```
#  [0]
  MinuteGenerate: -1      # intervalo de respawn (-1 = ?)
  MaxNumMob:      100     # população máxima
  MinGroup/MaxGroup: 4/7  # tamanho do grupo
  Leader:  Ciclope_Forte  # mob líder (nome → ItemList/MobList)
  Follower: Ciclope_Forte
  RouteType: 2 / Formation: 0
  StartX/StartY/StartRange  # ponto de spawn + raio
  StartWait
  DestX/DestY/DestRange/DestWait  # patrulha
```
Carregado por `CNPCGene` (`TMSrv/CNPCGene.cpp`). Existe `NPCGener.new.txt` (variante). Detalhar
parsing exato na Fase 4 (AI/spawn).

### 3.4. Outros arquivos de conteúdo (catálogo)

| Arquivo | Local | Formato | Conteúdo |
|---------|-------|---------|----------|
| `data00.csv` | Common | CSV temporal | série tempo (ranking grind por hora? `2021_01_15_07,0,0,...`) — **UNVERIFIED** |
| `extraitem.bin` / `extraitem.csv` | Common / TMsrv | bin/CSV | itens extras (cash/eventos); `.bin` em runtime |
| `InitItem.csv` | TMsrv | CSV `STRUCT_INITITEM` | itens iniciais por classe (`PosX,PosY,ItemIndex,Rotate`) |
| `ItemDropList.txt` | TMsrv (~240 KB) | TXT | tabela de drop por mob (ver Fase 4) |
| `LevelItem.txt` | TMsrv (~17 KB) | TXT | itens por nível (drop/recompensa) |
| `Guard.txt` | TMsrv | TXT | NPCs guarda (`STRUCT_NPC_GUARD`) |
| `Regions.txt` | TMsrv | TXT `x1,y1,x2,y2 = Nome` | regiões nomeadas (guerra, RvR, cidades) |
| `Rates.txt` | TMsrv | TXT chave/valor | rates de exp/drop/gold (Fase 4/7) |
| `QuestDiaria.txt` | TMsrv | TXT | definição de quests diárias |
| `Language.txt` / `Language.h` | TMsrv | TXT/header | tabela de strings (i18n PT) |
| `Settings/*.txt` | Common | TXT | `CastleQuest`, `CompRate` (refino), `MobMerc`, `QuestsRate`, `SancRate` (Fase 4) |

### 3.5. Estado de mundo persistente (guilda / ranking / cidade)

| Arquivo | Formato | Conteúdo |
|---------|---------|----------|
| `Guilds.txt` | TXT | índice de guildas |
| `GuildInfo` | bin/TXT | `STRUCT_GUILDINFO` (fama, sub-líderes, clã, nível) — `Basedef.h:1033-1045` |
| `Guild_<x>_<y>.txt`, `ChampionCity_<x>_<y>.txt`, `Chall_<x>_<y>.txt` | TXT | estado de guerra/castelo/desafio por região |
| `Ranking.txt` | TXT | ranking (gerado por `CRanking` varrendo `account/`) |
| `serverlist.bin` / `serverlist.txt` | bin/TXT | lista de servidores (canais) — também na Fase 7 |

---

## 4. Modelo de dados alvo (proposta)

Banco **relacional** (PostgreSQL recomendado; ver Fase 9) — os dados são fortemente relacionais e
o volume cabe folgado. Normalizar o que hoje é array de tamanho fixo.

```text
account
  id PK, name UNIQUE (canonical lowercase), pass_hash, pin_hash,
  real_name, email, telephone, address, ssn1, ssn2,
  donate_balance, cargo_coin, is_blocked, block_pass_hash,
  created_at, last_login_day  -- (Year/YearDay viram timestamp)

character
  id PK, account_id FK, slot (0..3), name UNIQUE,
  class, clan, guild_id FK NULL, level, exp BIGINT, coin,
  str,int,dex,con, score_bonus, special_bonus, skill_bonus,
  max_hp,max_mp,hp,mp, critical, regen_hp, regen_mp,
  resist_fire,resist_ice,resist_thunder,resist_magic,
  learned_skill BIGINT, magic, save_x, save_y,
  pos_x, pos_y, guild_level, citizen, fame, soul, class_master

item        -- normaliza Equip[16] + Carry[64] + Cargo[128]
  id PK, owner_kind ENUM(char_equip,char_carry,account_cargo),
  owner_id FK, slot,
  item_index, eff1,effv1, eff2,effv2, eff3,effv3

skill_slot  -- ShortSkill[4][16] + SkillBar[4]
  character_id FK, bar, slot, skill_id

affect      -- buffs persistidos (affect[char][32])
  character_id FK, type, value, level, expires_at

daily_quest -- STRUCT_QUEST por conta/char
  character_id FK, index_quest, level, mob1..3, qtd1..3, count1..3,
  exp_reward, gold_reward, reward_item1, reward_item2, last_time

guild
  id PK, name, fame, clan, citizen, level, leader_char_id,
  sub1..3_char_id

-- Conteúdo (read-only, carregado de arquivos OU tabelas de referência):
item_list(index PK, name, mesh, texture, vfx, req_lvl/str/int/dex/con,
          static_effects JSONB, price, unique, pos, extra, grade)
skill_data(id PK, skill_point, target_type, mana, delay, range, ...)
npc_gener(id PK, leader, follower, min/max_group, start_xy, dest_xy, ...)
drop_table(mob_index, item_index, rate, ...)
```

Mapeamento de invariantes a preservar:
- `STRUCT_ITEM.sIndex == 0` ⇒ slot vazio (não criar linha `item`).
- Slots têm **posição fixa** (índice no array = significado): preservar `slot` numérico.
- `Exp` é `long long` (BIGINT). `Coin`/`Donate` são `int` (cuidado com overflow ≥ 2^31).
- Senha/PIN: migrar para hash, descartando o texto plano (one-way; usuários redefinem se preciso).
- Os 3 tamanhos de arquivo de conta (4294 / 7500–7600 / atual) exigem **conversores versionados**.

---

## 5. Riscos de dados para a migração

| Risco | Detalhe | Mitigação |
|-------|---------|-----------|
| Senha/PIN em claro | `STRUCT_ACCOUNTINFO.AccountPass`, `NumericToken` | hash na importação; nunca persistir claro |
| Layout dependente de compilador | dumps de struct C com padding MSVC x86; **dois regimes** (save = alinhamento natural; rede = `pack(1)`) | reproduzir alinhamento natural (NÃO `pack(1)`) nas structs de save; `static_assert`/`offsetof` no build de referência — ver **§0.1** |
| Múltiplas versões de arquivo | 4294 / 7500–7600 / `sizeof` | detectar por tamanho; mapear cada layout |
| Case-sensitivity de nomes | `A/antonio` vs `ANTONIO` | normalizar para canônico (lowercase) |
| Caminhos/drives hardcoded | `S:/export/...` (`CFileDB.cpp:2513`) | virar config (Fase 7) |
| Semântica de `AttributeMap` UNVERIFIED | bits por byte não documentados | extrair por inspeção antes de reusar/regerar |

> **Status da Fase 2: PARCIAL.** Layouts macro (conta, MOB, item, score, mapas) documentados com
> evidência e **validados campo-a-campo** contra `Basedef.h`; regimes de alinhamento esclarecidos
> (§0.1). `sizeof` calculados por alinhamento natural: `STRUCT_MOB`=816, `STRUCT_MOBEXTRA`=552,
> `STRUCT_QUEST`=56, **`STRUCT_ACCOUNTFILE`=7952** (premissa `time_t`=8; travar com `static_assert`,
> §0.1). UNVERIFIED a confirmar via build/captura: `BASE_GetFirstKey`, a largura efetiva de `time_t`
> no build, layout dos arquivos legados (4294 / 7500–7600), mapeamento coluna-a-coluna dos CSV e
> semântica bit-a-bit da `AttributeMap`.
