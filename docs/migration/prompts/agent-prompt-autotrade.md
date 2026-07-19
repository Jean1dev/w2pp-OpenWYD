# Agent prompt — capturar a LOJA PESSOAL / AUTOTRADE (SendAutoTrade / ReqBuy / ReqTradeList / Deprivate / CreateMobTrade)

Cole isto no Claude Code da máquina Windows (que tem a FONTE COMPLETA do WYD + o dumper
`_layout_probe/dump_layout.cpp`, MSVC x86). Salve o resultado em `captura-wyd-autotrade.md`.

---

## Contexto

Estou migrando o servidor do WYD para Go (`w2pp-openwyd`), mirando o **cliente `WYD.exe`
original sem mods, ClientVersion 12000** (protocolo base 7640). Header CPSock (`_MSG`) = 12
bytes: `short Size; char KeyWord; char CheckSum; short Type; short ID; unsigned int ClientTick;`.
Já funcionam: login, criação/seleção de char, entrar no mundo, andar, ver outros players,
NPCs, **lojas de NPC** (abrir/comprar/vender), **baú da conta (Cargo)** com deposit/withdraw,
combate, EXP, **troca P2P (`_MSG_Trade` 0x0383)**.

**Vou implementar a issue #115: a LOJA PESSOAL (autotrade)** — o player abre uma banquinha
AFK com título e vende itens **tirados do próprio Cargo (baú da conta)** para quem passa
perto. Já li os 4 handlers na minha cópia PARCIAL da fonte
(`Source/Code/TMSrv/_MSG_SendAutoTrade.cpp`, `_MSG_ReqBuy.cpp`, `_MSG_ReqTradeList.cpp`,
`_MSG_Deprivate.cpp`) e o `Basedef.h`. Tenho a lógica de validação/transação. **O que me
falta são os LAYOUTS byte-exatos e 4 funções cujo corpo NÃO existe na minha cópia.**

## O que eu JÁ SEI (confirme se bate na fonte completa; corrija se divergir)

- Opcodes (`Basedef.h`): `_MSG_SendAutoTrade = (151|FLAG_GAME2CLIENT|FLAG_CLIENT2GAME)`
  (0x0397); `_MSG_ReqBuy = (152|...)` (0x0398); `_MSG_ReqTradeList = (154|...)` (0x039A,
  STANDARDPARM); `_MSG_Deprivate = (140|FLAG_CLIENT2GAME)` (0x028C, STANDARDPARM);
  `_MSG_ItemSold = (155|...)` (0x039B, STANDARDPARM2: Parm1=vendedor Parm2=pos);
  `_MSG_CreateMobTrade = (99|...)` (0x0363). `FLAG_GAME2CLIENT=0x0100`,
  `FLAG_CLIENT2GAME=0x0200`. **Confirme que os NÚMEROS batem no build 12000.**
- `MAX_AUTOTRADE = 12`, `MAX_AUTOTRADETITLE = 24`, `MAX_CARGO = 128`, `STRUCT_ITEM = 8` bytes.
- `MSG_SendAutoTrade` (Basedef.h:~2293): `_MSG` + `char Title[24]` + `STRUCT_ITEM Item[12]` +
  `char CarryPos[12]` (-1 = vazio) + `int Coin[12]` + `short Tax` + `short Index`. É C↔S
  (bidirecional): o cliente manda para ABRIR e o servidor devolve a MESMA struct para LISTAR
  a loja (via `SendAutoTrade`).
- `MSG_ReqBuy` (Basedef.h:~2311): `_MSG` + `int Pos` + `unsigned short TargetID` + `int Price`
  + `int Tax` + `STRUCT_ITEM item`.
- Regras já lidas: só em vila (`BASE_GetVillage` 0..4) e fora do retângulo proibido
  `x∈[2123,2148],y∈[2139,2157]`; `NewbieEventServer` precisa ser != 0; blacklist de sIndex
  {508,3993,747,509,522,526,527,528,529,530,531,446}; preço 0 rejeitado; `EF_NOTRADE` rejeitado;
  DOIS `memcmp` (oferta vs `AutoTrade` armazenado, e oferta vs `Cargo[CarryPos]` real);
  imposto só se `Price>=100000`: `imposto=(Price/100)*Tax; final=Price-imposto`; proceeds do
  vendedor vão para o **Coin do Cargo/banco** (`SendCargoCoin`), não o Coin do char; caps de 2G;
  range check `±VIEWGRIDX/Y` (33) entre comprador e vendedor.

## PROBLEMA: 4 funções SEM corpo na minha cópia parcial

`SendAutoTrade`, `GetCreateMobTrade`, `DoDeprivate`, `RemoveTrade` só têm DECLARAÇÃO
(`SendFunc.h`, `GetFunc.h`, `Server.h`) — o corpo não está na minha árvore. Preciso deles da
fonte COMPLETA.

## PERGUNTAS (responda com evidência `arquivo.cpp:linha` da fonte COMPLETA)

### 1. Layouts byte-exatos (via o dumper `dump_layout.cpp` — `sizeof` + `offsetof` de CADA campo)
Rode o dumper e cole a saída para:
- `MSG_SendAutoTrade` — tamanho total e offset de `Title`, `Item`, `CarryPos`, `Coin`, `Tax`,
  `Index`. **Quero saber se há PADDING** (natural align do MSVC) em algum ponto.
- `MSG_ReqBuy` — tamanho total e offset de `Pos`, `TargetID`, `Price`, `Tax`, `item`.
  **CRÍTICO: tem padding depois do `TargetID` (ushort) antes do `Price` (int)?** (Suspeito 2
  bytes de padding, mas preciso confirmar — muda todo o parser.)
- `MSG_CreateMobTrade` — tamanho total e offset de TODOS os campos (`PosX/PosY`, `MobID`,
  `MobName`, `Equip[16]`, `Affect[MAX_AFFECT]`, `Guild`, `GuildMemberType`, `Unknow[3]`,
  `Score` (STRUCT_SCORE), `CreateType`, `AnctCode[16]`, `Tab[26]`, `Desc[24]`).
- `STRUCT_SCORE` — tamanho total e todos os offsets (para eu montar o bloco `Score` dentro do
  CreateMobTrade).
- Confirme `MSG_STANDARDPARM` e `MSG_STANDARDPARM2` (tamanho e offsets de `Parm`/`Parm1`/`Parm2`).

### 2. `SendAutoTrade(int conn, int otherconn)` — o pacote S→C que LISTA a loja
Cole o corpo COMPLETO. Quero confirmar:
- Ele monta e envia um `MSG_SendAutoTrade` (0x0397) de volta para `conn` com o conteúdo de
  `pUser[otherconn].AutoTrade`? Ou usa outro Type/struct?
- Qual é o `HEADER.ID` do pacote (é `conn`? `otherconn`? `ESCENE_FIELD`/30000?)?
- Copia `Title`/`Item[]`/`CarryPos[]`/`Coin[]`/`Tax` inteiros? Zera/omite algum campo?

### 3. `GetCreateMobTrade(int mob, MSG_CreateMobTrade *sm)` — a APARÊNCIA da loja no mundo
Cole o corpo COMPLETO. **O ponto mais importante:**
- **Qual VALOR exato é escrito em `sm->CreateType`?** Esse é o flag que faz o cliente 12000
  desenhar o personagem na pose de "loja aberta / sentado". É uma constante nomeada? Um enum?
  (Ex.: 0=normal, N=autotrade?) Cole a linha `sm->CreateType = ...`.
- Como `sm->Desc` é preenchido (é o `AutoTrade.Title`)? E `Tab[26]`? E `AnctCode[16]`?
- Que campos vêm do `pMob[mob].MOB` (Equip, MobName, Score, Guild, Affect)?
- O handler `_MSG_SendAutoTrade.cpp` faz `sm_cmt.Score.Con = 0` depois — por quê? algum outro
  campo de Score é neutralizado para a mob-loja?

### 4. `DoDeprivate(int conn, int target)` — FECHAR a loja
Cole o corpo COMPLETO. Quero saber EXATAMENTE o que acontece ao fechar:
- Zera `pUser[conn].TradeMode` e o `AutoTrade`? (memset?)
- Qual pacote S→C é enviado / multicast para NEUTRALIZAR a pose de loja nos clientes que veem
  o vendedor? É um `_MSG_RemoveMob` + `_MSG_CreateMob` normal? Um `_MSG_CreateMob` (0x0364)
  re-emitido? Um `GridMulticast` de quê exatamente?
- O parâmetro `target` (do `MSG_STANDARDPARM.Parm`) é usado para quê? (fechar a loja de outro?
  expulsar?) Em que condições?
- Ele é chamado de outros lugares além do handler `_MSG_Deprivate`? (ex.: no `RemoveTrade`, no
  logout, na morte?)

### 5. `RemoveTrade(int conn)` — cancelar trade/loja
Cole o corpo COMPLETO. Quero saber:
- Ele fecha a loja pessoal (chama `DoDeprivate` / zera `AutoTrade`/`TradeMode`) OU só mexe no
  `Trade` (troca P2P face-a-face)?
- É chamado no fluxo de LOGOUT/DISCONNECT? (Preciso saber o que exatamente limpar quando a
  conexão do vendedor cai com a loja aberta.)

### 6. Tabela de imposto e persistência
- Cole os valores iniciais de `g_pGuildZone[0..4].CityTax` (e `[4]`, a zona central) —
  onde `g_pGuildZone` é inicializado (`Basedef.cpp`? um `.txt`?). Preciso dos números para a
  minha tabela de imposto por vila.
- Confirme: o estado da loja (`pUser[conn].AutoTrade`, `TradeMode`) fica SÓ em memória (`CUser`)
  e **NÃO** é gravado no `STRUCT_ACCOUNTFILE`/save do char, certo? Ou seja, **a loja NÃO
  sobrevive a relogin**. (Se sobreviver, me diga onde é salva.)
- `LojinhaTimer` (CUser) — onde é INCREMENTADO (o `ProcessSecMinTimer.cpp:~950` só reseta)?
  Não é crítico, mas se for barato, cole.

### 7. Sequência S→C completa dos 3 fluxos (para eu ordenar os pacotes certo)
Liste, em ordem, TODOS os pacotes S→C que o servidor original manda em cada caso:
- **Abrir loja** (`_MSG_SendAutoTrade` recebido): o que vai para o próprio vendedor e o que é
  multicast para os vizinhos?
- **Comprar** (`_MSG_ReqBuy` recebido): o que vai para o COMPRADOR (item/coin), para o
  VENDEDOR (cargo/coin) e o multicast `_MSG_ItemSold` para os vizinhos?
- **Fechar** (`_MSG_Deprivate` recebido): o que neutraliza a pose nos vizinhos?

---

**Formato:** para cada função, cole o código C++ literal com `arquivo.cpp:linha`. Para os
layouts, cole a saída crua do dumper. Marque como **UNVERIFIED** qualquer coisa que você
inferir em vez de ler direto. Salve tudo em `captura-wyd-autotrade.md`.
