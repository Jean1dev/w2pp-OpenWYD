# Fase 1 — Especificação do Protocolo de Fio (w2pp-OpenWYD)

> **Objetivo:** permitir que um servidor escrito em QUALQUER linguagem fale exatamente com o
> cliente `WYD.exe` (build 7662 + `ClientPatch_v7662.dll`), byte-for-byte. Este é o artefato de
> **prioridade máxima** da migração: sem ele o cliente mantido não conecta.
>
> **Regra de evidência:** tudo aqui é citado como `arquivo:linha`. Onde não foi possível confirmar
> pela fonte, está marcado **UNVERIFIED**.
>
> **Reuso:** este documento aprofunda — não repete — o deep-dive
> `docs/agents/component-deep-analyzer/component-analysis-CPSock-2026-06-19_16-06-38.md`. Leia
> aquele para a análise de risco/segurança da camada; aqui está a especificação reproduzível.

---

## 0. Glossário rápido (para quem não conhece o código)

| Termo | Significado |
|-------|-------------|
| **TMSrv** | *The Message Server* — o servidor de jogo (gameplay). Fala direto com o cliente. |
| **DBSrv** | *Database Server* — persistência de contas/personagens. Fala só com o TMSrv. |
| **BISrv** | *Billing Server* — cobrança/cash. Fala com o TMSrv via um pacote fixo de 196 bytes. |
| **NP / NPTool** | Processo auxiliar de contas (NPServer). Fala com o DBSrv. |
| **HEADER** | Cabeçalho fixo de 12 bytes em todo pacote (exceto o handshake e o link de billing). |
| **`_MSG`** | Macro C que injeta os 12 bytes do HEADER no início de cada struct de mensagem. |
| **Type** | Campo de 2 bytes no HEADER que identifica a mensagem (inclui bits de direção). |
| **pKeyWord** | Tabela estática de 512 bytes usada para ofuscar/desofuscar o payload. |

---

## 1. Camada de transporte

Fonte: `Source/Code/CPSock.h`, `Source/Code/CPSock.cpp`. O mesmo código é compilado nos três
servidores (TMSrv, DBSrv, BISrv).

### 1.1. Layout do `HEADER` (12 bytes)

Definido em `CPSock.h:42-50` e replicado como macro `_MSG` em `Basedef.h:1205-1210`. **Little-endian**
(x86, Win32). Sem padding — os campos são naturalmente alinhados e somam exatamente 12 bytes.

| Offset | Campo        | Tipo           | Bytes | Significado |
|-------:|--------------|----------------|------:|-------------|
| 0      | `Size`       | `short` (int16)| 2     | Tamanho TOTAL do pacote em bytes, incluindo o próprio HEADER. Lido como `unsigned short` em `CPSock.cpp:390`. |
| 2      | `KeyWord`    | `char` (int8)  | 1     | Índice aleatório `iKeyWord` (0–255) na tabela `pKeyWord`. Semente da ofuscação. |
| 3      | `CheckSum`   | `char` (int8)  | 1     | Checksum aditivo de 1 byte do payload (ver §1.4). |
| 4      | `Type`       | `short` (int16)| 2     | Identificador da mensagem (base + bits de direção). Ver §2. |
| 6      | `ID`         | `short` (int16)| 2     | Índice de conexão/jogador (slot em `pUser[]`, 0..`MAX_USER`-1). |
| 8      | `ClientTick` | `uint32`       | 4     | Timestamp/tick. No envio o servidor grava `CurrentTime` (`CPSock.cpp:541`). |

> **Atenção (memory map dos campos):** em `ReadMessage` o código lê `SockType` como `unsigned int`
> a partir do offset **4** (`CPSock.cpp:394`) e `SockID` como `unsigned int` a partir do offset
> **6** (`CPSock.cpp:395`). São leituras de conveniência que sobrepõem campos; o layout canônico é o
> da tabela acima (`Type` = 2 bytes em 4, `ID` = 2 bytes em 6). O servidor novo deve seguir a
> tabela, não essas leituras sobrepostas.

A ofuscação e o checksum operam **a partir do offset 4** (bytes `Size`/`KeyWord`/`CheckSum` ficam em
claro). Ver o `for (int i = 4; i < Size; i++)` em `CPSock.cpp:430` (decode) e `:558` (encode).

### 1.2. Handshake / INITCODE

- Constante: `INITCODE = 0x1F11F311` (`CPSock.h:40`).
- Os **primeiros 4 bytes** de toda conexão TCP nova devem ser exatamente `INITCODE` (little-endian:
  `11 F3 11 1F`). Validação em `CPSock.cpp:366-383`: enquanto `Init == 0`, lê 4 bytes como
  `unsigned int`; se `!= INITCODE` retorna `ErrorCode=2`; senão `Init=1` e consome os 4 bytes
  (`nProcPosition += 4`).
- O lado que **conecta** (links servidor→servidor) envia o INITCODE com
  `send(tSock, &InitCode, 4, 0)` em `CPSock.cpp:250` (`ConnectServer`). O **cliente** também envia
  esses 4 bytes ao abrir a conexão com o TMSrv.
- É um gate de versão/filtro de tráfego, **não** autenticação (constante pública fixa).

**O que o servidor novo deve fazer:** ao aceitar uma conexão, exigir e consumir 4 bytes
`0x1F11F311` antes de processar qualquer HEADER. Conexões que não comecem assim devem ser
derrubadas.

### 1.3. Enquadramento (framing) no stream TCP

Lógica em `CPSock::ReadMessage` (`CPSock.cpp:353-467`):

1. Se `Init==0`, consumir o INITCODE (§1.2).
2. Esperar até ter pelo menos `sizeof(HEADER)` (12) bytes bufferizados (`CPSock.cpp:386-387`).
3. Ler `Size` (offset 0). Rejeitar se `Size > MAX_MESSAGE_SIZE (8192)` **ou** `Size < sizeof(HEADER)
   (12)` → zera buffers e retorna `ErrorCode=2` (`CPSock.cpp:397-406`).
4. Esperar até `Size <= Rest` (bytes realmente recebidos) — frame incompleto fica no buffer
   (`CPSock.cpp:408-411`).
5. Expor o ponteiro do frame e avançar `nProcPosition += Size` (`CPSock.cpp:414-416`).

Limites de tamanho:
- **Mínimo:** 12 bytes (um HEADER puro, ex.: `MSG_STANDARD`).
- **Máximo:** `MAX_MESSAGE_SIZE = 8192` bytes (`CPSock.h:38`).
- Buffers de recv/send: 128 KB cada (`RECV_BUFFER_SIZE`/`SEND_BUFFER_SIZE`, `CPSock.h:35-36`).

O stream é puramente delimitado por `Size`; não há marcador de fim. Várias mensagens podem chegar
coladas no mesmo `recv()` e são desenquadradas em loop pelo chamador.

### 1.4. Algoritmo de ofuscação (keyword transform) — reproduzível

A tabela `pKeyWord[512]` (`unsigned char`) está hardcoded em `CPSock.cpp:29-46` (rótulo
"7.xx keys"). Cópia completa no **Apêndice A**.

Para cada pacote escolhe-se um índice aleatório `iKeyWord` (no envio: `rand()%256`,
`CPSock.cpp:535`). O HEADER guarda `iKeyWord` no campo `KeyWord`. A semente da posição é
`KeyWord = pKeyWord[iKeyWord * 2]` (`CPSock.cpp:392` no decode, `:536` no encode).

**Decode (recebimento)** — `CPSock.cpp:428-453`:

```text
pos = pKeyWord[iKeyWord * 2]          # semente
for i in [4 .. Size):                 # só o payload, header em claro
    rst   = pos % 256
    Sum2 += byte_cifrado[i]           # acumula ANTES de transformar
    Trans = pKeyWord[rst * 2 + 1]     # byte de transformação (ímpares da tabela)
    mod   = i & 0x3                   # posição i mod 4
    if mod == 0: plain[i] = cifrado[i] - (Trans << 1)
    if mod == 1: plain[i] = cifrado[i] + (Trans >> 3)
    if mod == 2: plain[i] = cifrado[i] - (Trans << 2)
    if mod == 3: plain[i] = cifrado[i] + (Trans >> 5)
    Sum1 += plain[i]                  # acumula DEPOIS de transformar
    pos += 1
```

**Encode (envio)** — operação inversa, `CPSock.cpp:556-581`:

```text
pos = pKeyWord[iKeyWord * 2]
for i in [4 .. Size):
    Sum1 += plain[i]                  # acumula ANTES
    rst   = pos % 256
    Trans = pKeyWord[rst * 2 + 1]
    mod   = i & 0x3
    if mod == 0: cifrado[i] = plain[i] + (Trans << 1)
    if mod == 1: cifrado[i] = plain[i] - (Trans >> 3)
    if mod == 2: cifrado[i] = plain[i] + (Trans << 2)
    if mod == 3: cifrado[i] = plain[i] - (Trans >> 5)
    Sum2 += cifrado[i]                # acumula DEPOIS
    pos += 1
```

Notas de implementação para paridade exata:
- Aritmética em `unsigned char` (8 bits, wrap mod 256). `Trans << 2` pode estourar 8 bits no
  cálculo intermediário (em C é promovido a `int`), mas o resultado é truncado ao gravar em
  `pMsg[i]`/`pSendBuffer[...]` (que são `char`). Replicar com truncamento de 8 bits.
- O índice na tabela usa `*2` e `*2+1`: bytes **pares** da tabela são "KeyWord seeds", bytes
  **ímpares** são "Trans". `pos` cresce 1 por byte e é reduzido mod 256 → percorre os 256 pares.
- A "criptografia" é reversível e a chave está no repositório → **obfuscação, não cifragem**
  (detalhe de risco no deep-dive CPSock §3/§9).

### 1.5. Checksum (não-rejeitante)

- No envio: `CheckSum = Sum2 - Sum1` (aritmética `unsigned char`), gravado no HEADER
  (`CPSock.cpp:583-584`). `Sum1` = soma dos bytes em claro; `Sum2` = soma dos bytes cifrados.
- No recebimento: recalcula `Sum = Sum2 - Sum1` e compara com `HEADER.CheckSum` (`CPSock.cpp:455`).
- **Mismatch NÃO rejeita o pacote.** Em `CPSock.cpp:457-466` o código seta `*ErrorCode = 1` e mesmo
  assim executa `return pMsg;` (comentário em `:457`: *"return packet, even check_sum not match"*).
  A decisão de descartar fica a cargo do chamador, que é inconsistente entre servidores.
- **O cliente também não verifica:** o `ClientPatch_v7662` troca os saltos condicionais de
  verificação de checksum por `JMP` incondicional (`0xEB`) em três endereços:
  `*(BYTE*)0x53AC6A = 0xEB; *(BYTE*)0x53AD52 = 0xEB; *(BYTE*)0x53AE7E = 0xEB;`
  (`ClientPatch_v7662/Hook.cpp:211-214`, comentário PT: *"Altera o salto das verificações de
  checksum para JMP afim de não verificar o checksum"*).

**O que o servidor novo deve fazer:** para compatibilidade de fio é **obrigatório** calcular o
checksum corretamente no **envio** (o cliente lê o campo mesmo sem validar de forma estrita; manter
correto evita divergência). No **recebimento**, pode-se computar e logar, mas o comportamento atual
é processar mesmo com mismatch. Recomendação para a stack nova: computar e **rejeitar** (corrigindo a
dívida), exceto se a paridade exigir tolerar — registrar como decisão (ver Fase 9).

### 1.6. Link de billing (TMSrv ↔ BISrv) — framing diferente

O link com o BISrv **não** usa HEADER nem ofuscação. É um pacote fixo:

- Tamanho: `g_cGame = sizeof(_AUTH_GAME) = 196` bytes (`CPSock.h:88-92,132`).
- Leitura: `ReadBillMessage` espera 196 bytes e devolve o ponteiro cru, sem transform/checksum
  (`CPSock.cpp:469-496`). Envio: `SendBillMessage` copia 196 bytes diretos (`CPSock.cpp:498-511`).
- A struct `_AUTH_GAME` atual é um placeholder `char Unk[196]` (`CPSock.h:88-91`), com o comentário
  *"NEEDS TO BE FIXED ACCORDING TO WYD 1.2 6.13 SIZE IS 0xC4 (196)"*. O layout interno detalhado
  está comentado como referência "FROM TANTRA" (`CPSock.h:93-130`) — **UNVERIFIED** se bate com este
  servidor; ver §4.3.

---

## 2. Codificação do campo `Type` (esquema de direção)

`Type` (2 bytes) = **`base` OR bits de direção**. Os bits são (`Basedef.h:1212-1221`):

| Constante | Valor | Sentido |
|-----------|------:|---------|
| `FLAG_GAME2CLIENT` | `0x0100` | TMSrv → Cliente |
| `FLAG_CLIENT2GAME` | `0x0200` | Cliente → TMSrv |
| `FLAG_DB2GAME`     | `0x0400` | DBSrv → TMSrv |
| `FLAG_GAME2DB`     | `0x0800` | TMSrv → DBSrv |
| `FLAG_DB2NP`       | `0x1000` | DBSrv → NPServer |
| `FLAG_NP2DB`       | `0x2000` | NPServer → DBSrv |
| `FLAG_NEW`         | `0x4000` | Família de mensagens "novas" (ranking grind/exp) |

O dispatcher do TMSrv (`ProcessClientMessage.cpp:66`) compara `std->Type` diretamente contra essas
constantes — ou seja, **o valor de fio inclui os bits de direção**. Uma mensagem com mais de um
flag (ex.: `_MSG_AccountSecure = 222 | 0xF00`) tem um único valor de fio que casa em qualquer
contexto. A tabela abaixo já traz o valor de fio calculado (decimal e hex).

Guardas globais antes do switch (`ProcessClientMessage.cpp:38-64`):
- `std->ID` deve estar em `[0, MAX_USER)` (`:42`) senão o pacote é logado e descartado.
- `_MSG_Ping` (`0x03A0`) é ignorado/no-op (`:59`).
- Se `isServer==FALSE` e `ClientTick == SKIPCHECKTICK (235543242)`, o pacote é descartado
  (`:63`) — tick "interno" que o cliente não deve usar.

---

## 3. Catálogo de mensagens

> Valores de fio calculados a partir de `Basedef.h` (todas as `const short _MSG_*`/`_S_*`).
> "Handler" = função `Exec_*` chamada no `switch` de `ProcessClientMessage.cpp` (para C→S).
> Structs detalhadas em §3.5. Total de constantes catalogadas: **198** (dump completo no Apêndice B).

### 3.1. Cliente → TMSrv (entradas que o servidor novo PRECISA tratar)

São as únicas com `case` no dispatcher (`ProcessClientMessage.cpp:66-311`). Implementar todas.

| Type (dec / hex) | Constante | Struct | Handler | Notas |
|---:|---|---|---|---|
| 525 / 0x020D | `_MSG_AccountLogin` | `MSG_AccountLogin` | `Exec_MSG_AccountLogin` | login de conta; valida `ClientVersion` |
| 531 / 0x0213 | `_MSG_CharacterLogin` | `MSG_CharacterLogin` | `Exec_MSG_CharacterLogin` | entra no mundo com `Slot` |
| 533 / 0x0215 | `_MSG_CharacterLogout` | `MSG_STANDARD`* | `Exec_MSG_CharacterLogout` | volta à seleção |
| 529 / 0x0211 | `_MSG_DeleteCharacter` | `MSG_DeleteCharacter` | `Exec_MSG_DeleteCharacter` | exige senha |
| 527 / 0x020F | `_MSG_CreateCharacter` | `MSG_CreateCharacter` | `Exec_MSG_CreateCharacter` | cria no `Slot` |
| 4062 / 0x0FDE | `_MSG_AccountSecure` | `MSG_AccountSecure` | `Exec_MSG_AccountSecure` | token numérico (PIN) |
| 819 / 0x0333 | `_MSG_MessageChat` | (chat) | `Exec_MSG_MessageChat` | chat público; comandos `/` |
| 876 / 0x036C | `_MSG_Action` | `MSG_Action` | `Exec_MSG_Action` | movimento (também 0x0366/0x0368) |
| 870 / 0x0366 | `_MSG_Action2` | `MSG_Action` | `Exec_MSG_Action` | colide com `_MSG_PKInfo`/`_MSG_Action2` |
| 872 / 0x0368 | `_MSG_Action3` | `MSG_Action` | `Exec_MSG_Action` | |
| 874 / 0x036A | `_MSG_Motion` | `MSG_Motion` | `Exec_MSG_Motion` | animações/emotes |
| 873 / 0x0369 | `_MSG_NoViewMob` | `MSG_STANDARD`* | `Exec_MSG_NoViewMob` | |
| 649 / 0x0289 | `_MSG_Restart` | `MSG_STANDARD`* | `Exec_MSG_Restart` | reviver/voltar cidade |
| 652 / 0x028C | `_MSG_Deprivate` | `MSG_STANDARDPARM`* | `Exec_MSG_Deprivate` | |
| 654 / 0x028E | `_MSG_Challange` | `MSG_STANDARDPARM`* | `Exec_MSG_Challange` | duelo/desafio |
| 655 / 0x028F | `_MSG_ChallangeConfirm` | (req) | `Exec_MSG_ChallangeConfirm` | |
| 656 / 0x0290 | `_MSG_ReqTeleport` | (teleport) | `Exec_MSG_ReqTeleport` | |
| 635 / 0x027B | `_MSG_REQShopList` | `MSG_REQShopList` | `Exec_MSG_REQShopList` | abre loja NPC |
| 904 / 0x0388 | `_MSG_Deposit` | `MSG_STANDARDPARM`* | `Exec_MSG_Deposit` | cargo/banco |
| 903 / 0x0387 | `_MSG_Withdraw` | `MSG_STANDARDPARM`* | `Exec_MSG_Withdraw` | cargo/banco |
| 894 / 0x037E | `_MSG_RemoveParty` | (party) | `Exec_MSG_RemoveParty` | |
| 895 / 0x037F | `_MSG_SendReqParty` | `MSG_SendReqParty` | `Exec_MSG_SendReqParty` | convida party |
| 939 / 0x03AB | `_MSG_AcceptParty` | `MSG_AcceptParty` | `Exec_MSG_AcceptParty` | |
| 886 / 0x0376 | `_MSG_TradingItem` | `MSG_TradingItem` | `Exec_MSG_TradingItem` | mover item inv↔equip↔trade |
| 820 / 0x0334 | `_MSG_MessageWhisper` | (whisper) | `Exec_MSG_MessageWhisper` | sussurro |
| 657 / 0x0291 | `_MSG_ChangeCity` | `MSG_STANDARD`* | `Exec_MSG_ChangeCity` | |
| 921 / 0x0399 | `_MSG_PKMode` | `MSG_STANDARDPARM`* | `Exec_MSG_PKMode` | modo PK |
| 922 / 0x039A | `_MSG_ReqTradeList` | `MSG_STANDARDPARM`* | `Exec_MSG_ReqTradeList` | |
| 884 / 0x0374 | `_MSG_UpdateItem` | `MSG_UpdateItem` | `Exec_MSG_UpdateItem` | abrir/fechar baú etc. |
| 651 / 0x028B | `_MSG_Quest` | `MSG_STANDARDPARM2`* | `Exec_MSG_Quest` | |
| 888 / 0x0378 | `_MSG_SetShortSkill` | `MSG_SetShortSkill` | `Exec_MSG_SetShortSkill` | barra de skills |
| 871 / 0x0367 | `_MSG_Attack` | `MSG_Attack` | `Exec_MSG_Attack` | ataque (também 0x039D/0x039E) |
| 925 / 0x039D | `_MSG_AttackOne` | `MSG_AttackOne` | `Exec_MSG_Attack` | 1 alvo |
| 926 / 0x039E | `_MSG_AttackTwo` | `MSG_AttackTwo` | `Exec_MSG_Attack` | 2 alvos |
| 626 / 0x0272 | `_MSG_DropItem` | `MSG_DropItem` | `Exec_MSG_DropItem` | dropar no chão |
| 624 / 0x0270 | `_MSG_GetItem` | `MSG_GetItem` | `Exec_MSG_GetItem` | pegar do chão |
| 900 / 0x0384 | `_MSG_QuitTrade` | `MSG_STANDARD`* | `Exec_MSG_QuitTrade` | cancela trade |
| 883 / 0x0373 | `_MSG_UseItem` | `MSG_UseItem` | `Exec_MSG_UseItem` | usar/equipar item |
| 631 / 0x0277 | `_MSG_ApplyBonus` | `MSG_ApplyBonus` | `Exec_MSG_ApplyBonus` | distribuir pontos |
| 919 / 0x0397 | `_MSG_SendAutoTrade` | (auto-trade) | `Exec_MSG_SendAutoTrade` | loja de jogador |
| 920 / 0x0398 | `_MSG_ReqBuy` | `MSG_ReqBuy` | `Exec_MSG_ReqBuy` | comprar de auto-trade |
| 889 / 0x0379 | `_MSG_Buy` | `MSG_Buy` | `Exec_MSG_Buy` | comprar de NPC |
| 890 / 0x037A | `_MSG_Sell` | `MSG_Sell` | `Exec_MSG_Sell` | vender p/ NPC |
| 899 / 0x0383 | `_MSG_Trade` | `MSG_Trade` | `Exec_MSG_Trade` | confirmar trade |
| 934 / 0x03A6 | `_MSG_CombineItem` | (combine) | `Exec_MSG_CombineItem` | refino base |
| 927 / 0x039F | `_MSG_ReqRanking` | `MSG_STANDARDPARM2`* | `Exec_MSG_ReqRanking` | |
| 723 / 0x02D3 | `_MSG_CombineItemEhre` | (combine) | `Exec_MSG_CombineItemEhre` | |
| 960 / 0x03C0 | `_MSG_CombineItemTiny` | (combine) | `Exec_MSG_CombineItemTiny` | |
| 708 / 0x02C4 | `_MSG_CombineItemShany` | (combine) | `Exec_MSG_CombineItemShany` | |
| 949 / 0x03B5 | `_MSG_CombineItemAilyn` | (combine) | `Exec_MSG_CombineItemAilyn` | |
| 954 / 0x03BA | `_MSG_CombineItemAgatha` | (combine) | `Exec_MSG_CombineItemAgatha` | |
| 722 / 0x02D2 | `_MSG_CombineItemOdin` | (combine) | `Exec_MSG_CombineItemOdin` | também 0x02E2 (Odin2) |
| 738 / 0x02E2 | `_MSG_CombineItemOdin2` | (combine) | `Exec_MSG_CombineItemOdin` | |
| 740 / 0x02E4 | `_MSG_DeleteItem` | (del item) | `Exec_MSG_DeleteItem` | |
| 981 / 0x03D5 | `_MSG_InviteGuild` | `MSG_STANDARDPARM2`* | `Exec_MSG_InviteGuild` | ⚠ valor colide com `_MSG_CombineItemLoki`? não — Loki=0x02D5 |
| 741 / 0x02E5 | `_MSG_SplitItem` | `MSG_SplitItem` | `Exec_MSG_SplitItem` | dividir stack |
| 707 / 0x02C3 | `_MSG_CombineItemLindy` | (combine) | `Exec_MSG_CombineItemLindy` | |
| 737 / 0x02E1 | `_MSG_CombineItemAlquimia` | (combine) | `Exec_MSG_CombineItemAlquimia` | |
| 724 / 0x02D4 | `_MSG_CombineItemExtracao` | (combine) | `Exec_MSG_CombineItemExtracao` | |
| 3603 / 0x0E13* | `_MSG_GuildAlly` (na verdade 0x0E12) | `MSG_GuildAlly` | `Exec_MSG_GuildAlly` | ver nota* |
| 3598 / 0x0E0E | `_MSG_War` | `MSG_STANDARD`* | `Exec_MSG_War` | guild war |
| 717 / 0x02CD | `_MSG_CapsuleInfo` | (capsule) | `Exec_MSG_CapsuleInfo` | |
| 972 / 0x03CC | `_MSG_PutoutSeal` | (seal) | `Exec_MSG_PutoutSeal` | |

\* `MSG_STANDARD`/`MSG_STANDARDPARM*` = corpo só com HEADER (+ `Parm`/`Parm1,Parm2,...`). Confirmar a
struct exata por handler na Fase 5. `_MSG_GuildAlly = 18|0xE00 = 0x0E12 (3602)`; o `_MSG_InviteGuild`
e `_MSG_CombineItemLoki` compartilham base 213 mas com flags diferentes (0x03D5 vs 0x02D5) — **não**
colidem no fio. Colisões reais de base entre famílias são desambiguadas pelos bits de direção.

> **Colisões de valor a verificar (UNVERIFIED):** `_MSG_Action2 (0x0366)` divide base 102 com
> `_MSG_PKInfo`; `_MSG_Action3 (0x0368)` base 104. Como ambos carregam `GAME2CLIENT|CLIENT2GAME`, o
> valor de fio é único por constante, mas a base reaproveitada sugere reuso histórico — validar caso
> a caso na Fase 5 qual struct o cliente 7662 realmente envia.

### 3.2. TMSrv → Cliente (saídas que o servidor novo PRECISA produzir)

Principais (lista completa no Apêndice B). Produzidas por `SendFunc.cpp`/handlers.

| Type (dec / hex) | Constante | Struct | Uso |
|---:|---|---|---|
| 257 / 0x0101 | `_MSG_MessagePanel` | `MSG_MessagePanel` (String[128]) | texto no painel |
| 258 / 0x0102 | `_MSG_MessageBoxOk` | `MSG_MessageBoxOk` | popup OK |
| 266 / 0x010A | `_MSG_CNFAccountLogin` | (lista de chars) | confirma login → seleção |
| 272 / 0x0110 | `_MSG_CNFNewCharacter` | `MSG_CNFNewCharacter` | char criado |
| 274 / 0x0112 | `_MSG_CNFDeleteCharacter` | `MSG_CNFDeleteCharacter` | char deletado |
| 276 / 0x0114 | `_MSG_CNFCharacterLogin` | `MSG_CNFCharacterLogin` | entra no mundo (snapshot do char) |
| 278 / 0x0116 | `_MSG_CNFCharacterLogout` | (std) | logout confirmado |
| 281 / 0x0119 | `_MSG_CharacterLoginFail` | (std) | falha ao entrar |
| 284 / 0x011C | `_MSG_AlreadyPlaying` | (std) | conta já online |
| 868 / 0x0364 | `_MSG_CreateMob` | `MSG_CreateMob` | spawn de player/mob na visão |
| 867 / 0x0363 | `_MSG_CreateMobTrade` | | spawn de auto-trade |
| 357 / 0x0165 | `_MSG_RemoveMob` | | despawn |
| 871 / 0x0367 | `_MSG_Attack` | `MSG_Attack` | resultado de ataque/dano |
| 876 / 0x036C | `_MSG_Action` | `MSG_Action` | movimento de terceiros |
| 884 / 0x0374 | `_MSG_UseItem` | `MSG_UseItem` | resultado de uso |
| 386 / 0x0182 | `_MSG_SendItem` | `MSG_SendItem` | atualiza 1 slot |
| 369 / 0x0171 | `_MSG_CNFGetItem` | | confirma pegar item |
| 373 / 0x0175 | `_MSG_CNFDropItem` | `MSG_CNFDropItem` | confirma drop |
| 385 / 0x0181 | `_MSG_SetHpMp` | `MSG_SetHpMp` | HP/MP |
| 822 / 0x0336 | `_MSG_UpdateScore` | (score) | atributos/score |
| 380 / 0x017C | `_MSG_ShopList` | `MSG_ShopList` | lista da loja NPC |
| 899 / 0x0383 | `_MSG_Trade` | `MSG_Trade` | estado do trade |
| 928 / 0x03A0 | `_MSG_Ping` | (std) | keepalive (ignorado no recv) |

### 3.3. Protocolo interno TMSrv ↔ DBSrv

Direções `FLAG_GAME2DB`/`FLAG_DB2GAME`. Despacho no DBSrv via `ProcessDBMessage` (TMSrv side:
`TMSrv/ProcessDBMessage.cpp`). Mesma camada de transporte (HEADER + ofuscação + INITCODE) — o TMSrv
abre a conexão com `ConnectServer` (envia INITCODE).

| Type (dec / hex) | Constante | Sentido | Struct/uso |
|---:|---|---|---|
| 2051 / 0x0803 | `_MSG_DBAccountLogin` | G→DB | valida conta/senha |
| 1046 / 0x0416 | `_MSG_DBCNFAccountLogin` | DB→G | resposta: lista de chars |
| 1057 / 0x0421 | `_MSG_DBAccountLoginFail_Account` | DB→G | conta inexistente |
| 1058 / 0x0422 | `_MSG_DBAccountLoginFail_Pass` | DB→G | senha errada |
| 1060 / 0x0424 | `_MSG_DBAccountLoginFail_Block` | DB→G | bloqueada |
| 2052 / 0x0804 | `_MSG_DBCharacterLogin` | G→DB | carrega char do slot |
| 1047 / 0x0417 | `_MSG_DBCNFCharacterLogin` | DB→G | char carregado |
| 2049 / 0x0801 | `_MSG_DBNewAccount` | G→DB | cria conta |
| 2050 / 0x0802 | `_MSG_DBCreateCharacter` | G→DB | cria char |
| 1048 / 0x0418 | `_MSG_DBCNFNewCharacter` | DB→G | char criado |
| 2057 / 0x0809 | `_MSG_DBDeleteCharacter` | G→DB | deleta char |
| 1049 / 0x0419 | `_MSG_DBCNFDeleteCharacter` | DB→G | deletado |
| 2054 / 0x0806 | `_MSG_SavingQuit` | G→DB | salva e sai |
| 2055 / 0x0807 | `_MSG_DBSaveMob` | G→DB | salva STRUCT_MOB do char |
| 2053 / 0x0805 | `_MSG_DBNoNeedSave` | G→DB | desconecta sem salvar |
| 1055 / 0x041F | `_MSG_DBAlreadyPlaying` | DB→G | conta já em uso |
| 3087 / 0x0C0F | `_MSG_DBSendItem` | G↔DB | `MSG_DBSendItem` (entrega item/cash) |
| 3089 / 0x0C11 | `_MSG_DBSendDonate` | G↔DB | `MSG_DBSendDonate` (donate/cash) |
| 3132 / 0x0C3C | `_MSG_DBCapsuleInfo` | G↔DB | cápsula |
| 1062 / 0x0426 | `_MSG_DBOnlyOncePerDay` | DB→G | limite diário |

(Lista completa de mensagens DB no Apêndice B; structs detalhadas na Fase 2/5.)

### 3.4. Protocolo DBSrv ↔ NPServer (contas/cash externo)

Direções `FLAG_DB2NP`/`FLAG_NP2DB`. Não toca no cliente; relevante para a migração do subsistema de
contas/cash.

| Type (dec / hex) | Constante | Sentido |
|---:|---|---|
| 4097 / 0x1001 | `_MSG_NPReqIDPASS` | DB→NP |
| 8194 / 0x2002 | `_MSG_NPIDPASS` | NP→DB |
| 8195 / 0x2003 | `_MSG_NPReqAccount` | NP→DB |
| 4101 / 0x1005 | `_MSG_NPAccountInfo` | DB→NP |
| 8198 / 0x2006 | `_MSG_NPReqSaveAccount` | NP→DB |
| 12299 / 0x300B | `_MSG_NPCreateCharacter` | NP↔DB |
| 12301 / 0x300D | `_MSG_NPDonate` | NP↔DB |
| 7184 / 0x1C10 | `_MSG_NPAppeal` | DB↔G↔NP |

### 3.5. Structs campo-a-campo (mensagens críticas)

Layouts em `Basedef.h`. Little-endian. Salvo indicação, **sem** `#pragma pack` (alinhamento natural
do MSVC x86: `int`/`uint` em 4, `short` em 2). `_MSG` = 12 bytes do HEADER no início.

#### `MSG_AccountLogin` (C→S, Type 0x020D) — `Basedef.h:1545-1561` (`#pragma pack(push,1)`)

| Offset | Campo | Tipo | Bytes | Significado |
|------:|-------|------|------:|-------------|
| 0  | _MSG | HEADER | 12 | cabeçalho |
| 12 | `AccountPassword` | `char[12]` | 12 | senha **em texto plano** (`ACCOUNTPASS_LENGTH`) ⚠ dívida de segurança |
| 24 | `AccountName` | `char[16]` | 16 | login (`ACCOUNTNAME_LENGTH`) |
| 40 | `Zero` | `char[52]` | 52 | padding/reservado |
| 92 | `ClientVersion` | `int` | 4 | **deve ser igual a `APP_VERSION = 7640`** (ver §5) |
| 96 | `DBNeedSave` | `int` | 4 | flag |
| 100 | `AdapterName` | `int[4]` | 16 | MAC/adapter (anti-cheat/HWID) |
| | **Total** | | **116** | (variante `MSG_AccountLogin_HWID` adiciona `char HwId[50]`) |

Validação no handler: `if (Size < sizeof(MSG_AccountLogin) || m->ClientVersion != ClientVersion)`
rejeita com mensagem de versão (`_MSG_AccountLogin.cpp:44-47`).

#### `MSG_CreateCharacter` (C→S, Type 0x020F) — `Basedef.h:1598-1605`

| Offset | Campo | Tipo | Bytes | Significado |
|------:|-------|------|------:|-------------|
| 0  | _MSG | HEADER | 12 | |
| 12 | `Slot` | `int` | 4 | slot de personagem (0..3) |
| 16 | `MobName` | `char[16]` | 16 | nome (`NAME_LENGTH`) |
| 32 | `MobClass` | `int` | 4 | classe inicial |

#### `MSG_DeleteCharacter` (C→S, Type 0x0211) — `Basedef.h:1608-1616`

| 12 | `Slot` | `int` | 4 | |
| 16 | `MobName` | `char[16]` | 16 | |
| 32 | `Password` | `char[12]` | 12 | confirmação |

#### `MSG_CharacterLogin` (C→S, Type 0x0213) — `Basedef.h:1674-1680`

| 12 | `Slot` | `int` | 4 | qual personagem entra |
| 16 | `Force` | `int` | 4 | forçar (kick sessão anterior?) |

#### `MSG_Action` (movimento, C↔S, Type 0x036C/0x0366/0x0368) — `Basedef.h:2070-2082`

| Offset | Campo | Tipo | Bytes | Significado |
|------:|-------|------|------:|-------------|
| 12 | `PosX`,`PosY` | `short`×2 | 4 | posição atual |
| 16 | `Effect` | `int` | 4 | 0=andando, 1=teleportando (comentário no código) |
| 20 | `Speed` | `int` | 4 | velocidade |
| 24 | `Route` | `char[24]` | 24 | rota (passos), `MAX_ROUTE=24` |
| 48 | `TargetX`,`TargetY` | `short`×2 | 4 | destino |

#### `MSG_Attack` (C↔S, Type 0x0367) — `Basedef.h:2400-2432`

| Offset | Campo | Tipo | Bytes | Significado |
|------:|-------|------|------:|-------------|
| 12 | `Unk_1` | `char[4]` | 4 | |
| 16 | `CurrentHp` | `int` | 4 | |
| 20 | `Unk_2` | `char[4]` | 4 | |
| 24 | `CurrentExp` | `long long` | 8 | exp atual |
| 32 | `unk0` | `short` | 2 | |
| 34 | `PosX`,`PosY` | `ushort`×2 | 4 | |
| 38 | `TargetX`,`TargetY` | `ushort`×2 | 4 | |
| 42 | `AttackerID` | `ushort` | 2 | |
| 44 | `Progress` | `ushort` | 2 | |
| 46 | `Motion` | `uchar` | 1 | |
| 47 | `SkillParm` | `uchar` | 1 | |
| 48 | `DoubleCritical` | `uchar` | 1 | |
| 49 | `FlagLocal` | `uchar` | 1 | |
| 50 | `Rsv` | `short` | 2 | |
| 52 | `CurrentMp` | `int` | 4 | |
| 56 | `SkillIndex` | `short` | 2 | |
| 58 | `ReqMp` | `short` | 2 | |
| 60 | `Dam[13]` | `STRUCT_DAM`×13 | 104 | `MAX_TARGET=13`; cada = `{int TargetID; int Damage;}` (8 bytes) |

> `_MSG_AttackOne` usa `STRUCT_DAM Dam[1]` (`Basedef.h:2483`) e `_MSG_AttackTwo` `Dam[2]` (`:2520`).
> O tamanho do pacote varia com o nº de alvos.

#### `MSG_UseItem` (C↔S, Type 0x0373) — `Basedef.h:2196-2206`

| 12 | `SourType` | `int` | 4 | tipo de origem (inv/equip) |
| 16 | `SourPos` | `int` | 4 | slot de origem |
| 20 | `DestType` | `int` | 4 | tipo destino |
| 24 | `DestPos` | `int` | 4 | slot destino |
| 28 | `GridX`,`GridY` | `ushort`×2 | 4 | posição no grid |
| 32 | `WarpID` | `ushort` | 2 | |

#### `MSG_TradingItem` (C→S, Type 0x0376) — `Basedef.h:2103-2113`

| 12 | `DestPlace` | `uchar` | 1 | inv/equip/trade/cargo destino |
| 13 | `DestSlot` | `uchar` | 1 | |
| 14 | `SrcPlace` | `uchar` | 1 | origem |
| 15 | `SrcSlot` | `uchar` | 1 | |
| 16 | `WarpID` | `int` | 4 | |

#### `MSG_Trade` (C↔S, Type 0x0383) — `Basedef.h:2435-2445`

| 12 | `Item[15]` | `STRUCT_ITEM`×15 | — | `MAX_TRADE=15`; ver tamanho de `STRUCT_ITEM` na Fase 2 |
| .. | `InvenPos[15]` | `char[15]` | 15 | |
| .. | `TradeMoney` | `int` | 4 | gold ofertado |
| .. | `MyCheck` | `uchar` | 1 | confirmou |
| .. | `OpponentID` | `ushort` | 2 | |

#### `MSG_Buy` / `MSG_Sell` (C↔S, 0x0379/0x037A) — `Basedef.h:2124-2141`

`MSG_Buy`: `ushort TargetID; short TargetInvenPos; short MyInvenPos; int Coin;`
`MSG_Sell`: `ushort TargetID; short MyType; short MyPos;`

#### `MSG_ApplyBonus` (C→S, 0x0277) — `Basedef.h:2144-2151`

`short BonusType (0:Score 1:Special 2:Skill); short Detail (0:Str 1:Int 2:Dex 3:Con); ushort TargetID;`

#### `MSG_AccountSecure` (C↔S↔DB, 0x0FDE) — `Basedef.h:1588-1595`

`char NumericToken[6]; char Unknown[10]; int ChangeNumeric;` (PIN de 6 dígitos).

> Demais structs C→S/S→C (combine/refino, party, guild, war, capsule, etc.) na Fase 5, por handler.

---

## 4. Apêndices

### 4.1. Constante de versão exigida pelo cliente 7662

- `APP_VERSION = 7640` (`Basedef.h:102`, comentário `// 6975`). O cliente envia
  `MSG_AccountLogin.ClientVersion`, que o TMSrv compara com `APP_VERSION` (`_MSG_AccountLogin.cpp:26,44`).
  Mismatch → mensagem `_NN_Version_Not_Match_Rerun` e rejeição.
- **Nomenclatura:** "7662" é o build/nome do executável e do `ClientPatch_v7662.dll`; o **número de
  versão de protocolo trafegado** é **7640**. O servidor novo deve aceitar `ClientVersion == 7640`
  (ou tornar configurável).

### 4.2. Ofuscação no cliente (ClientPatch)

`ClientPatch_v7662/Hook.cpp` aplica patches em memória ao `WYD.exe`:
- Desliga verificação de checksum: `0x53AC6A/0x53AD52/0x53AE7E = 0xEB` (`:211-214`).
- Divide `SkillDelay` por 4 em 104 entradas: loop em `0x11DA838 + i*96 + 48` (`:230-231`) — afeta
  cooldown de skills (relevante para Fase 4).
- Hooks de `ReadMessage` (`0x4252D6`), `SendChat` (`0x4676C5`), caixas de item, etc. (`:218-227`).

### 4.3. `_AUTH_GAME` (billing, 196 bytes) — UNVERIFIED

A struct ativa é `char Unk[196]` (`CPSock.h:88-91`). O layout real do WYD 1.2/6.13 está apenas como
comentário "FROM TANTRA" (`CPSock.h:93-130`) e **não foi confirmado** que corresponde a este
servidor. Campos candidatos: `Packet_Type`, `Result`, `S_KEY[32]`, `Session[32]`, `User_ID[52]`,
`User_IP[24]`, billing fields. Para a migração: capturar tráfego real TMSrv↔BISrv (Fase 8) para
confirmar o layout antes de reimplementar.

### 4.4. Tabela `pKeyWord[512]` (chave de ofuscação, completa)

Fonte: `CPSock.cpp:29-46`. Bytes pares = "KeyWord seed", ímpares = "Trans". Reproduzir **idêntica**
no servidor novo.

```c
unsigned char pKeyWord[512] = { // 7.xx keys
  0x84,0x87,0x37,0xd7,0xea,0x79,0x91,0x7d,0x4b,0x4b,0x85,0x7d,0x87,0x81,0x91,0x7c,0x0f,0x73,0x91,0x91,0x87,0x7d,0x0d,0x7d,0x86,0x8f,0x73,0x0f,0xe1,0xdd,0x85,0x7d,
  0x05,0x7d,0x85,0x83,0x87,0x9c,0x85,0x33,0x0d,0xe2,0x87,0x19,0x0f,0x79,0x85,0x86,0x37,0x7d,0xd7,0xdd,0xe9,0x7d,0xd7,0x7d,0x85,0x79,0x05,0x7d,0x0f,0xe1,0x87,0x7e,
  0x23,0x87,0xf5,0x79,0x5f,0xe3,0x4b,0x83,0xa3,0xa2,0xae,0x0e,0x14,0x7d,0xde,0x7e,0x85,0x7a,0x85,0xaf,0xcd,0x7d,0x87,0xa5,0x87,0x7d,0xe1,0x7d,0x88,0x7d,0x15,0x91,
  0x23,0x7d,0x87,0x7c,0x0d,0x7a,0x85,0x87,0x17,0x7c,0x85,0x7d,0xac,0x80,0xbb,0x79,0x84,0x9b,0x5b,0xa5,0xd7,0x8f,0x05,0x0f,0x85,0x7e,0x85,0x80,0x85,0x98,0xf5,0x9d,
  0xa3,0x1a,0x0d,0x19,0x87,0x7c,0x85,0x7d,0x84,0x7d,0x85,0x7e,0xe7,0x97,0x0d,0x0f,0x85,0x7b,0xea,0x7d,0xad,0x80,0xad,0x7d,0xb7,0xaf,0x0d,0x7d,0xe9,0x3d,0x85,0x7d,
  0x87,0xb7,0x23,0x7d,0xe7,0xb7,0xa3,0x0c,0x87,0x7e,0x85,0xa5,0x7d,0x76,0x35,0xb9,0x0d,0x6f,0x23,0x7d,0x87,0x9b,0x85,0x0c,0xe1,0xa1,0x0d,0x7f,0x87,0x7d,0x84,0x7a,
  0x84,0x7b,0xe1,0x86,0xe8,0x6f,0xd1,0x79,0x85,0x19,0x53,0x95,0xc3,0x47,0x19,0x7d,0xe7,0x0c,0x37,0x7c,0x23,0x7d,0x85,0x7d,0x4b,0x79,0x21,0xa5,0x87,0x7d,0x19,0x7d,
  0x0d,0x7d,0x15,0x91,0x23,0x7d,0x87,0x7c,0x85,0x7a,0x85,0xaf,0xcd,0x7d,0x87,0x7d,0xe9,0x3d,0x85,0x7d,0x15,0x79,0x85,0x7d,0xc1,0x7b,0xea,0x7d,0xb7,0x7d,0x85,0x7d,
  0x85,0x7d,0x0d,0x7d,0xe9,0x73,0x85,0x79,0x05,0x7d,0xd7,0x7d,0x85,0xe1,0xb9,0xe1,0x0f,0x65,0x85,0x86,0x2d,0x7d,0xd7,0xdd,0xa3,0x8e,0xe6,0x7d,0xde,0x7e,0xae,0x0e,
  0x0f,0xe1,0x89,0x7e,0x23,0x7d,0xf5,0x79,0x23,0xe1,0x4b,0x83,0x0c,0x0f,0x85,0x7b,0x85,0x7e,0x8f,0x80,0x85,0x98,0xf5,0x7a,0x85,0x1a,0x0d,0xe1,0x0f,0x7c,0x89,0x0c,
  0x85,0x0b,0x23,0x69,0x87,0x7b,0x23,0x0c,0x1f,0xb7,0x21,0x7a,0x88,0x7e,0x8f,0xa5,0x7d,0x80,0xb7,0xb9,0x18,0xbf,0x4b,0x19,0x85,0xa5,0x91,0x80,0x87,0x81,0x87,0x7c,
  0x0f,0x73,0x91,0x91,0x84,0x87,0x37,0xd7,0x86,0x79,0xe1,0xdd,0x85,0x7a,0x73,0x9b,0x05,0x7d,0x0d,0x83,0x87,0x9c,0x85,0x33,0x87,0x7d,0x85,0x0f,0x87,0x7d,0x0d,0x7d,
  0xf6,0x7e,0x87,0x7d,0x88,0x19,0x89,0xf5,0xd1,0xdd,0x85,0x7d,0x8b,0xc3,0xea,0x7a,0xd7,0xb0,0x0d,0x7d,0x87,0xa5,0x87,0x7c,0x73,0x7e,0x7d,0x86,0x87,0x23,0x85,0x10,
  0xd7,0xdf,0xed,0xa5,0xe1,0x7a,0x85,0x23,0xea,0x7e,0x85,0x98,0xad,0x79,0x86,0x7d,0x85,0x7d,0xd7,0x7d,0xe1,0x7a,0xf5,0x7d,0x85,0xb0,0x2b,0x37,0xe1,0x7a,0x87,0x79,
  0x84,0x7d,0x73,0x73,0x87,0x7d,0x23,0x7d,0xe9,0x7d,0x85,0x7e,0x02,0x7d,0xdd,0x2d,0x87,0x79,0xe7,0x79,0xad,0x7c,0x23,0xda,0x87,0x0d,0x0d,0x7b,0xe7,0x79,0x9b,0x7d,
  0xd7,0x8f,0x05,0x7d,0x0d,0x34,0x8f,0x7d,0xad,0x87,0xe9,0x7c,0x85,0x80,0x85,0x79,0x8a,0xc3,0xe7,0xa5,0xe8,0x6b,0x0d,0x74,0x10,0x73,0x33,0x17,0x0d,0x37,0x21,0x19
};
```

### 4.5. Constantes de transporte (resumo)

| Constante | Valor | Fonte |
|-----------|------:|-------|
| `INITCODE` | `0x1F11F311` | `CPSock.h:40` |
| `MAX_MESSAGE_SIZE` | 8192 | `CPSock.h:38` |
| `RECV_BUFFER_SIZE`/`SEND_BUFFER_SIZE` | 131072 (128KB) | `CPSock.h:35-36` |
| `sizeof(HEADER)` | 12 | `CPSock.h:42-50` |
| `g_cGame` (billing) | 196 | `CPSock.h:132` |
| `APP_VERSION` | 7640 | `Basedef.h:102` |
| `SKIPCHECKTICK` | 235543242 | `Basedef.h:232` |
| `MAX_USER` | 1000 | `Basedef.h:116` |
| `MAX_ROUTE` / `MAX_TARGET` / `MAX_TRADE` / `MAX_EQUIP` | 24 / 13 / 15 / 16 | `Basedef.h:184,233,139,135` |

### 4.6. Apêndice B — dump completo dos 198 Types

> Gerado de `Basedef.h`. Formato: `decimal  hex  base [flags]  constante`. Reproduzível com o
> script em `parity-tests.md` (Fase 8). Inclui DB e NP. (Tabela longa — mantida no repositório como
> referência de implementação; ver seção 3 para os subconjuntos acionáveis.)

| dec | hex | base | flags | constante |
|---:|---|---:|---|---|
| 257 | 0x0101 | 1 | GAME2CLIENT | `_MSG_MessagePanel` |
| 258 | 0x0102 | 2 | GAME2CLIENT | `_MSG_MessageBoxOk` |
| 266 | 0x010A | 10 | GAME2CLIENT | `_MSG_CNFAccountLogin` |
| 272 | 0x0110 | 16 | GAME2CLIENT | `_MSG_CNFNewCharacter` |
| 274 | 0x0112 | 18 | GAME2CLIENT | `_MSG_CNFDeleteCharacter` |
| 276 | 0x0114 | 20 | GAME2CLIENT | `_MSG_CNFCharacterLogin` |
| 278 | 0x0116 | 22 | GAME2CLIENT | `_MSG_CNFCharacterLogout` |
| 279 | 0x0117 | 23 | GAME2CLIENT | `_MSG_NewAccountFail` |
| 281 | 0x0119 | 25 | GAME2CLIENT | `_MSG_CharacterLoginFail` |
| 282 | 0x011A | 26 | GAME2CLIENT | `_MSG_NewCharacterFail` |
| 283 | 0x011B | 27 | GAME2CLIENT | `_MSG_DeleteCharacterFail` |
| 284 | 0x011C | 28 | GAME2CLIENT | `_MSG_AlreadyPlaying` |
| 285 | 0x011D | 29 | GAME2CLIENT | `_MSG_StillPlaying` |
| 357 | 0x0165 | 101 | GAME2CLIENT | `_MSG_RemoveMob` |
| 358 | 0x0166 | 102 | GAME2CLIENT | `_MSG_PKInfo` |
| 367 | 0x016F | 111 | GAME2CLIENT | `_MSG_DecayItem` |
| 369 | 0x0171 | 113 | GAME2CLIENT | `_MSG_CNFGetItem` |
| 373 | 0x0175 | 117 | GAME2CLIENT | `_MSG_CNFDropItem` |
| 380 | 0x017C | 124 | GAME2CLIENT | `_MSG_ShopList` |
| 385 | 0x0181 | 129 | GAME2CLIENT | `_MSG_SetHpMp` |
| 386 | 0x0182 | 130 | GAME2CLIENT | `_MSG_SendItem` |
| 389 | 0x0185 | 133 | GAME2CLIENT | `_MSG_UpdateCarry` |
| 394 | 0x018A | 138 | GAME2CLIENT | `_MSG_SetHpDam` |
| 395 | 0x018B | 139 | GAME2CLIENT | `_MSG_UpdateWeather` |
| 397 | 0x018D | 141 | GAME2CLIENT | `_MSG_ReqChallange` |
| 403 | 0x0193 | 147 | GAME2CLIENT | `_MSG_SetClan` |
| 406 | 0x0196 | 150 | GAME2CLIENT | `_MSG_CloseShop` |
| 525 | 0x020D | 13 | CLIENT2GAME | `_MSG_AccountLogin` |
| 527 | 0x020F | 15 | CLIENT2GAME | `_MSG_CreateCharacter` |
| 529 | 0x0211 | 17 | CLIENT2GAME | `_MSG_DeleteCharacter` |
| 531 | 0x0213 | 19 | CLIENT2GAME | `_MSG_CharacterLogin` |
| 533 | 0x0215 | 21 | CLIENT2GAME | `_MSG_CharacterLogout` |
| 622 | 0x026E | 110 | CLIENT2GAME | `_MSG_CreateItem` |
| 624 | 0x0270 | 112 | CLIENT2GAME | `_MSG_GetItem` |
| 626 | 0x0272 | 114 | CLIENT2GAME | `_MSG_DropItem` |
| 631 | 0x0277 | 119 | CLIENT2GAME | `_MSG_ApplyBonus` |
| 635 | 0x027B | 123 | CLIENT2GAME | `_MSG_REQShopList` |
| 649 | 0x0289 | 137 | CLIENT2GAME | `_MSG_Restart` |
| 651 | 0x028B | 139 | CLIENT2GAME | `_MSG_Quest` |
| 652 | 0x028C | 140 | CLIENT2GAME | `_MSG_Deprivate` |
| 654 | 0x028E | 142 | CLIENT2GAME | `_MSG_Challange` |
| 655 | 0x028F | 143 | CLIENT2GAME | `_MSG_ChallangeConfirm` |
| 656 | 0x0290 | 144 | CLIENT2GAME | `_MSG_ReqTeleport` |
| 657 | 0x0291 | 145 | CLIENT2GAME | `_MSG_ChangeCity` |
| 658 | 0x0292 | 146 | CLIENT2GAME | `_MSG_SetHpMode` |
| 707 | 0x02C3 | 195 | CLIENT2GAME | `_MSG_CombineItemLindy` |
| 708 | 0x02C4 | 196 | CLIENT2GAME | `_MSG_CombineItemShany` |
| 717 | 0x02CD | 205 | CLIENT2GAME | `_MSG_CapsuleInfo` |
| 722 | 0x02D2 | 210 | CLIENT2GAME | `_MSG_CombineItemOdin` |
| 723 | 0x02D3 | 211 | CLIENT2GAME | `_MSG_CombineItemEhre` |
| 724 | 0x02D4 | 212 | CLIENT2GAME | `_MSG_CombineItemExtracao` |
| 725 | 0x02D5 | 213 | CLIENT2GAME | `_MSG_CombineItemLoki` |
| 737 | 0x02E1 | 225 | CLIENT2GAME | `_MSG_CombineItemAlquimia` |
| 738 | 0x02E2 | 226 | CLIENT2GAME | `_MSG_CombineItemOdin2` |
| 740 | 0x02E4 | 228 | CLIENT2GAME | `_MSG_DeleteItem` |
| 741 | 0x02E5 | 229 | CLIENT2GAME | `_MSG_SplitItem` |
| 742 | 0x02E6 | 230 | CLIENT2GAME | `_MSG_CombineDedekinto` |
| 745 | 0x02E9 | 233 | CLIENT2GAME | `_MSG_CombineDedekinto2` |
| 819 | 0x0333 | 51 | GAME2CLIENT+CLIENT2GAME | `_MSG_MessageChat` |
| 820 | 0x0334 | 52 | GAME2CLIENT+CLIENT2GAME | `_MSG_MessageWhisper` |
| 822 | 0x0336 | 54 | GAME2CLIENT+CLIENT2GAME | `_MSG_UpdateScore` |
| 823 | 0x0337 | 55 | GAME2CLIENT+CLIENT2GAME | `_MSG_UpdateEtc` |
| 824 | 0x0338 | 56 | GAME2CLIENT+CLIENT2GAME | `_MSG_CNFMobKill` |
| 825 | 0x0339 | 57 | GAME2CLIENT+CLIENT2GAME | `_MSG_UpdateCargoCoin` |
| 845 | 0x034D | 589 | GAME2CLIENT | `_MSG_DisableExpMsg` |
| 867 | 0x0363 | 99 | GAME2CLIENT+CLIENT2GAME | `_MSG_CreateMobTrade` |
| 868 | 0x0364 | 100 | GAME2CLIENT+CLIENT2GAME | `_MSG_CreateMob` |
| 870 | 0x0366 | 102 | GAME2CLIENT+CLIENT2GAME | `_MSG_Action2` |
| 871 | 0x0367 | 103 | GAME2CLIENT+CLIENT2GAME | `_MSG_Attack` |
| 872 | 0x0368 | 104 | GAME2CLIENT+CLIENT2GAME | `_MSG_Action3` |
| 873 | 0x0369 | 105 | GAME2CLIENT+CLIENT2GAME | `_MSG_NoViewMob` |
| 874 | 0x036A | 106 | GAME2CLIENT+CLIENT2GAME | `_MSG_Motion` |
| 875 | 0x036B | 107 | GAME2CLIENT+CLIENT2GAME | `_MSG_UpdateEquip` |
| 876 | 0x036C | 108 | GAME2CLIENT+CLIENT2GAME | `_MSG_Action` |
| 883 | 0x0373 | 115 | GAME2CLIENT+CLIENT2GAME | `_MSG_UseItem` |
| 884 | 0x0374 | 116 | GAME2CLIENT+CLIENT2GAME | `_MSG_UpdateItem` |
| 886 | 0x0376 | 118 | CLIENT2GAME+GAME2CLIENT | `_MSG_TradingItem` |
| 888 | 0x0378 | 120 | CLIENT2GAME+GAME2CLIENT | `_MSG_SetShortSkill` |
| 889 | 0x0379 | 121 | GAME2CLIENT+CLIENT2GAME | `_MSG_Buy` |
| 890 | 0x037A | 122 | GAME2CLIENT+CLIENT2GAME | `_MSG_Sell` |
| 893 | 0x037D | 125 | GAME2CLIENT+CLIENT2GAME | `_MSG_CNFAddParty` |
| 894 | 0x037E | 126 | GAME2CLIENT+CLIENT2GAME | `_MSG_RemoveParty` |
| 895 | 0x037F | 127 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendReqParty` |
| 899 | 0x0383 | 131 | GAME2CLIENT+CLIENT2GAME | `_MSG_Trade` |
| 900 | 0x0384 | 132 | GAME2CLIENT+CLIENT2GAME | `_MSG_QuitTrade` |
| 902 | 0x0386 | 134 | GAME2CLIENT+CLIENT2GAME | `_MSG_CNFCheck` |
| 903 | 0x0387 | 135 | GAME2CLIENT+CLIENT2GAME | `_MSG_Withdraw` |
| 904 | 0x0388 | 136 | GAME2CLIENT+CLIENT2GAME | `_MSG_Deposit` |
| 919 | 0x0397 | 151 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendAutoTrade` |
| 920 | 0x0398 | 152 | GAME2CLIENT+CLIENT2GAME | `_MSG_ReqBuy` |
| 921 | 0x0399 | 153 | GAME2CLIENT+CLIENT2GAME | `_MSG_PKMode` |
| 922 | 0x039A | 154 | GAME2CLIENT+CLIENT2GAME | `_MSG_ReqTradeList` |
| 923 | 0x039B | 155 | GAME2CLIENT+CLIENT2GAME | `_MSG_ItemSold` |
| 925 | 0x039D | 157 | GAME2CLIENT+CLIENT2GAME | `_MSG_AttackOne` |
| 926 | 0x039E | 158 | GAME2CLIENT+CLIENT2GAME | `_MSG_AttackTwo` |
| 927 | 0x039F | 159 | GAME2CLIENT+CLIENT2GAME | `_MSG_ReqRanking` |
| 928 | 0x03A0 | 160 | GAME2CLIENT+CLIENT2GAME | `_MSG_Ping` |
| 929 | 0x03A1 | 161 | GAME2CLIENT+CLIENT2GAME | `_MSG_StartTime` |
| 930 | 0x03A2 | 162 | GAME2CLIENT+CLIENT2GAME | `_MSG_EnvEffect` |
| 931 | 0x03A3 | 163 | GAME2CLIENT+CLIENT2GAME | `_MSG_SoundEffect` |
| 932 | 0x03A4 | 164 | GAME2CLIENT+CLIENT2GAME | `_MSG_GuildDisable` |
| 933 | 0x03A5 | 165 | GAME2CLIENT+CLIENT2GAME | `_MSG_GuildBoard` |
| 934 | 0x03A6 | 166 | GAME2CLIENT+CLIENT2GAME | `_MSG_CombineItem` |
| 935 | 0x03A7 | 167 | GAME2CLIENT+CLIENT2GAME | `_MSG_CombineComplete` |
| 936 | 0x03A8 | 168 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendWarInfo` |
| 939 | 0x03AB | 171 | GAME2CLIENT+CLIENT2GAME | `_MSG_AcceptParty` |
| 940 | 0x03AC | 172 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendCastleState` |
| 941 | 0x03AD | 173 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendCastleState2` |
| 944 | 0x03B0 | 176 | GAME2CLIENT+CLIENT2GAME | `_MSG_MobLeft` |
| 948 | 0x03B4 | 180 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendArchEffect` |
| 949 | 0x03B5 | 181 | GAME2CLIENT+CLIENT2GAME | `_MSG_CombineItemAilyn` |
| 953 | 0x03B9 | 185 | GAME2CLIENT+CLIENT2GAME | `_MSG_SendAffect` |
| 954 | 0x03BA | 186 | GAME2CLIENT+CLIENT2GAME | `_MSG_CombineItemAgatha` |
| 955 | 0x03BB | 187 | GAME2CLIENT+CLIENT2GAME | `_MSG_MobCount` |
| 960 | 0x03C0 | 192 | GAME2CLIENT+CLIENT2GAME | `_MSG_CombineItemTiny` |
| 972 | 0x03CC | 204 | GAME2CLIENT+CLIENT2GAME | `_MSG_PutoutSeal` |
| 981 | 0x03D5 | 213 | GAME2CLIENT+CLIENT2GAME | `_MSG_InviteGuild` |
| 1961 | 0x07A9 | 169 | GAME2CLIENT+CLIENT2GAME+DB2GAME | `_MSG_TransperCharacter` |
| 2777 | 0x0AD9 | 217 | GAME2DB+CLIENT2GAME | `_MSG_MasterGriff` |
| 3357 | 0x0D1D | 29 | DB2GAME+GAME2DB+GAME2CLIENT | `_MSG_MagicTrumpet` |
| 3358 | 0x0D1E | 30 | DB2GAME+GAME2DB+GAME2CLIENT | `_MSG_DBNotice` |
| 3359 | 0x0D1F | 31 | DB2GAME+GAME2DB+GAME2CLIENT | `_MSG_CNFDBCapsuleInfo` |
| 3598 | 0x0E0E | 14 | CLIENT2GAME+DB2GAME+GAME2DB | `_MSG_War` |
| 3602 | 0x0E12 | 18 | CLIENT2GAME+DB2GAME+GAME2DB | `_MSG_GuildAlly` |
| 3603 | 0x0E13 | 19 | CLIENT2GAME+DB2GAME+GAME2DB | `_MSG_GuildInfo` |
| 4010 | 0x0FAA | 170 | GAME2CLIENT+CLIENT2GAME+DB2GAME+GAME2DB | `_MSG_ReqTransper` |
| 4062 | 0x0FDE | 222 | DB2GAME+GAME2DB+CLIENT2GAME+GAME2CLIENT | `_MSG_AccountSecure` |
| 4063 | 0x0FDF | 223 | DB2GAME+GAME2DB+CLIENT2GAME+GAME2CLIENT | `_MSG_AccountSecureFail` |
| 17665 | 0x4501 | 1 | NEW+DB2GAME+GAME2CLIENT | `_MSG_GrindRankingData` |
| 19714 | 0x4D02 | 2 | NEW+DB2GAME+GAME2DB+GAME2CLIENT | `_MSG_UpdateExpRanking` |
| 20480 | 0x5000 | 20480 | — | `_MSG_Exp_Msg_Panel_` |

**Mensagens internas DBSrv/NP** (não trafegam ao cliente): ver lista em §3.3/§3.4 — incluem
`_MSG_DB*` (0x04xx DB→G, 0x08xx G→DB, 0x0Cxx bidirecionais) e `_MSG_NP*` (0x1xxx/0x2xxx/0x3xxx).

---

## 5. O que o servidor novo precisa garantir (checklist de compat de fio)

1. Aceitar e consumir o **INITCODE** `0x1F11F311` no início de cada conexão.
2. Implementar o **framing por `Size`** (12..8192) exatamente como §1.3.
3. Implementar **encode/decode** com a `pKeyWord` idêntica e o transform de §1.4 (truncamento 8 bits).
4. Calcular o **checksum** corretamente no envio (§1.5); no recv, decidir política (recomendado:
   rejeitar, mas atual é tolerar).
5. Reconhecer os **Types** de §3.1 (C→S) e produzir os de §3.2 (S→C) com os mesmos valores e layouts.
6. Aceitar `ClientVersion == 7640` no `MSG_AccountLogin`.
7. Tratar `_MSG_Ping` como no-op e descartar pacotes com `ClientTick == SKIPCHECKTICK` vindos do
   cliente.
8. Manter o link de **billing** como pacote cru de **196 bytes** (sem HEADER/ofuscação) — layout a
   confirmar (Fase 8).

> **Status da Fase 1: PARCIAL→COMPLETO no transporte e no catálogo de Types.** Pontos UNVERIFIED:
> layout interno de `_AUTH_GAME` (billing, §4.3) e structs exatas das mensagens marcadas `(std)`/
> `(combine)` — serão fechadas na Fase 5 (contratos por handler) e Fase 8 (captura real).
