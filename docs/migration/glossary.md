# Fase 10 — Glossário (w2pp-OpenWYD)

> Termos de domínio WYD/PT-BR e do código, com definição e onde aparecem. Para quem **não** conhece
> C++/Win32 nem o universo WYD.

## Servidores e infraestrutura

| Termo | Definição | Onde |
|-------|-----------|------|
| **TMSrv** | *The Message Server* — servidor de jogo; fala com o cliente. | `Source/Code/TMSrv/` |
| **DBSrv** | *Database Server* — persistência de contas/personagens (arquivos). | `Source/Code/DBSrv/` |
| **BISrv** | *Billing Server* — cobrança/cash; pacote fixo de 196 bytes. | `Source/Code/BISrv/`, Fase 1 §1.6 |
| **NPServer / NPTool** | Processo de contas/cash externo; fala com o DBSrv. | `_MSG_NP*`, Fase 1 §3.4 |
| **CPSock** | Camada de socket/pacote compartilhada pelos 3 servidores. | `CPSock.cpp/.h` |
| **HEADER** | Cabeçalho fixo de 12 bytes de todo pacote. | Fase 1 §1.1 |
| **INITCODE** | Constante `0x1F11F311` que abre toda conexão. | Fase 1 §1.2 |
| **pKeyWord** | Tabela estática de 512 bytes da ofuscação. | Fase 1 §4.4 |
| **ClientPatch** | DLL que altera o `WYD.exe` em memória (checksum off, SkillDelay/4). | `ClientPatch_v7662/` |
| **Reactor** | Loop único de eventos WinSock (`MainWndProc`) que processa tudo. | Fase 3 §5, Fase 6 |

## Entidades e estado

| Termo | Definição | Onde |
|-------|-----------|------|
| **STRUCT_MOB** | Struct compartilhada por **players e mobs** (nome, score, equip, carry). | `Basedef.h:556` |
| **pMob[]** | Array global de entidades: índice `<1000`=player, `≥1000`=mob/NPC. | Fase 3 §1 |
| **pUser[]** | Array global de sessões (`CUser`): socket, conta, modo. | Fase 3 §2.1 |
| **conn / ID** | Índice da conexão `[0,1000)`; liga `pUser[conn]`↔`pMob[conn]`; vai no HEADER. | Fase 3 §1 |
| **Mode** | Estado da sessão (`USER_*`) ou da entidade (`MOB_*`). | Fase 3 §3 |
| **mob vs NPC vs summon** | mob = monstro hostil; NPC = não-jogador fixo (loja/guarda); summon = invocado. | `CMob`, `CNPCGene`, `CNPCSummon` |
| **cargo** | Baú compartilhado da conta (`Cargo[128]`). | Fase 2 §1.2 |
| **coin** | Gold (moeda). | `STRUCT_MOB.Coin` |

## Itens e refino

| Termo | Definição | Onde |
|-------|-----------|------|
| **Anct / Ancient** | Atributo "ancião" aplicado a item via combine (joia + item). | Fase 5 `_MSG_CombineItem` |
| **Refino** | Aprimorar item (+1..+N); sucesso por tabela de taxa. | Fase 4 §3, `SancRate.txt` |
| **sanc (sanctificação)** | Nível de refino do item (0..N). | `BASE_GetItemSanc`, Fase 4 §4 |
| **Combine** | Receita de itens → resultado (Anct e variantes Ehre/Tiny/Odin/…). | Fase 4 §3, Fase 5 |
| **capsule / cápsula** | Item de cash que entrega prêmios (via DBSrv). | `_MSG_DBCapsuleInfo` |
| **selo / seal** | Trava em item; "PutoutSeal" retira. | `_MSG_PutoutSeal` |
| **Grade** | Raridade do item: 1=Normal 2=Místico 3=Arcano 4=Lendário. | `Basedef.h:1185` |
| **STRUCT_ITEM** | Item em 8 bytes: `sIndex` + 3 pares efeito/valor. | Fase 2 §1.5 |

## Personagem e progressão

| Termo | Definição | Onde |
|-------|-----------|------|
| **Classe / Clan** | Classe do personagem (TransKnight, etc.) e raça/clã. | `STRUCT_MOB.Class/Clan` |
| **ClassMaster / tier** | Evolução: MORTAL → ARCH → SD/SP/DK/CS (Celestial). | Fase 4 §1.4 |
| **quest Arch (SD/SP/DK/CS)** | Linha de quests de evolução: Sub Deus, Arch, Supreme, Deuses Kefra, Celestiais. | `Rates.txt`, Fase 4 |
| **Score / Special** | Atributos (Str/Int/Dex/Con) e pontos especiais de skill. | `STRUCT_SCORE` |
| **ScoreBonus / SkillBonus** | Pontos livres para distribuir (`_MSG_ApplyBonus`). | Fase 5 |
| **affect** | Buff/debuff temporizado. | `STRUCT_AFFECT`, Fase 2 §1.5 |
| **FREEEXP** | Nível até onde o jogo é grátis (billing). | Fase 7 §2.1 |
| **cra point** | "Crack point" — penalidade anti-cheat acumulada (`AddCrackError`). | Fase 3 §2.1 |

## Eventos e PvP

| Termo | Definição | Onde |
|-------|-----------|------|
| **Castle / Zakum** | Evento de castelo; matar boss dá a "chave"/prêmio. | `CCastleZakum`, Fase 6 §8 |
| **GTorre / Guerra de Torre** | Guild war agendada por hora (`GTorreHour`). | Fase 4 §6, Fase 7 |
| **RVR** | Realm vs Realm; região e hora dedicadas (`RVRHour`). | `Regions.txt`, Fase 7 |
| **Battle Royal (BR)** | Modo BR (`BrState`, `BRHour`). | Fase 7 |
| **PK / PKMode** | Player Kill — modo de combate entre jogadores. | `_MSG_PKMode` |
| **Pesadelo (Nightmare)** | Áreas/eventos de alta dificuldade (`maxNightmare`). | Fase 4 §1.3, Fase 7 |
| **Imposto de zona** | Líder de guilda cobra Exp/Coin de uma zona dominada. | `_MSG_Challange`, Fase 5 |

## Termos técnicos (código)

| Termo | Definição | Onde |
|-------|-----------|------|
| **`_MSG` (macro)** | Injeta os 12 bytes do HEADER no início de cada struct de mensagem. | `Basedef.h:1205` |
| **FLAG_* (direção)** | Bits OR'd no `Type` indicando o sentido (C2G/G2C/G2DB/…). | Fase 1 §2 |
| **SKIPCHECKTICK** | `ClientTick` mágico (235543242) para pacotes internos do servidor. | `Basedef.h:232` |
| **ESCENE_FIELD** | ID 30000 usado em mensagens originadas pelo servidor. | `Basedef.h:230` |
| **MaxCarry / MAX_CARGO / MAX_EQUIP** | Limites de slots de item. | Fase 3 §6 |
| **AccountPass / NumericToken** | Senha e PIN — **em texto plano** (dívida). | Fase 2 §1.3 |
