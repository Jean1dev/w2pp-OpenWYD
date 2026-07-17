# Projetos Relacionados — Análise Comparativa

**Gerado em:** 2026-06-19
**Complementado em:** 2026-07-15 (§4 — cliente descompilado; ver *Adendo*)
**Base:** documentos em `docs/agents/` (PROJECT-OVERVIEW, architectural-report, component-deep-dives) confrontados com repositórios GitHub.

## Projetos analisados

| # | Repositório | Linguagem | O que é | Licença |
|---|---|---|---|---|
| 1 | [seitbnao/W2PP](https://github.com/seitbnao/W2PP) | C++ (VS2015) | Emulador de servidor WYD a partir do decompile do *Polly's Server Release* | GPL-3.0 |
| 2 | [open-wyd/open-wyd-scripts](https://github.com/open-wyd/open-wyd-scripts) | Lua 5.3 (+XML) | Camada de scripts data-driven para o engine "Open WYD" | — |
| 3 | [kevinkouketsu/Wyd2Client](https://github.com/kevinkouketsu/Wyd2Client) | C# (WPF/MVVM) | Cliente/simulador + biblioteca de rede do protocolo WYD | educacional |
| 4 | [EricSantos00/tm-project](https://github.com/EricSantos00/tm-project) | C++ (VS2019+) | **Descompilação do cliente WYD** (`TMProject`) — upstream, 846 commits | GPL-3.0 |
| 5 | [MacedaoRG/w2Project-SeitTbNao](https://github.com/MacedaoRG/w2Project-SeitTbNao) | C++ (VS2019+) | Cópia do #4 **+ `Dataserver`/`Gameserver`** — 4 commits | GPL-3.0 |

> **§4 e §5 são o mesmo código-base.** README idêntico e mesmos autores (Eric Santos *SKEWED*, Wed Souza *FREEDOM*, Kevin Kouketsu *shepher*). São tratados juntos na §4.

---

## Resumo executivo

Os três projetos **têm relação com este projeto**, mas em níveis muito diferentes:

- **W2PP é o ancestral direto deste repositório.** O próprio nome da pasta — `w2pp-OpenWYD` — é a fusão de **W2PP** + **OpenWYD**. A árvore `Source/Code/` aqui é estruturalmente idêntica à de W2PP. Não é "semelhante": é a mesma base de código, com as alterações do autor (fork do "WYD cdk / Cavaleiros de Kersef").
- **open-wyd-scripts representa o "futuro arquitetural"** que este projeto não tem: conteúdo de jogo (eventos, itens, NPCs, teleportes, spawns) descrito em **Lua + XML** em vez de hardcoded em C++. Complementa por contraste — mostra exatamente o que falta aqui.
- **Wyd2Client complementa pelo lado cliente/protocolo:** é uma reimplementação em C# do mesmo protocolo de fio (HEADER + keyword-table) que o `CPSock` deste projeto. Serve como documentação viva do protocolo e como base para bots/testes de carga.

**Adendo 2026-07-15 (§4):** **TMProject** (`tm-project` / `w2Project-SeitTbNao`, o mesmo código-base) é a categoria que faltava — o **fonte C++ do cliente descompilado**, o outro lado do fio. Resultado da conferência: o **transporte CPSock bate 100%** com o nosso Go (`INIT_CODE`, `pKeyWord[512]`, transform e checksum idênticos), o que confirma o `protocol-spec` de forma independente e **fecha a questão do `-reject-checksum` sem precisar de captura**. Em compensação, a camada de mensagens **diverge** (é outro build, não o 7662) — serve de referência, não de fonte de verdade. Achado com risco: o cliente tem uma *keyword queue* que **rejeita pacote**, e nós não a implementamos.

---

## 1. seitbnao/W2PP — **Ancestral direto / fork irmão**

### Evidência da relação (não é coincidência, é a mesma base)

A raiz `Code/` de W2PP e a `Source/Code/` deste projeto têm exatamente os mesmos arquivos e módulos centrais:

| W2PP (`Code/`) | Este projeto (`Source/Code/`) |
|---|---|
| `Basedef.cpp` / `Basedef.h` | `Basedef.cpp` / `Basedef.h` |
| `CPSock.cpp` / `CPSock.h` | `CPSock.cpp` / `CPSock.h` |
| `ItemEffect.h` | `ItemEffect.h` |
| `ClientPatch_v7662/` | `ClientPatch_v7662/` |
| `DBSrv/` | `DBSrv/` |
| `TMSrv/` | `TMSrv/` |

As ferramentas standalone descritas no README do W2PP (DBSrv, TMSrv, DropTool, ExpTool, NPTool, EDITAPPMOB/EDITAPNPC, AttributeMap_Editor) são as **mesmas** listadas no `architectural-report` deste projeto (seção 2.1).

### Linhagem reconstruída

```
Polly's Server Release (decompile, © Hanbitsoft)
        │  (Klafke + TheHouse)
        ▼
   W2PP (seitbnao, mantido por Luis Gustavo / Woz Farias / Eric Santos)
        │
        ▼
   "WYD cdk" / Cavaleiros de Kersef  (Cavaleiros de Kersef.sln)
        │  (fork de Jeanluca)
        ▼
   w2pp-OpenWYD  ← ESTE PROJETO
```

O README local confirma: *"fork do WYD cdk com as minhas alterações"*, e a solution chama-se `Cavaleiros de Kersef.sln`.

### Semelhanças (herdadas)
- Mesmo modelo: cluster de 3 processos (TMSrv/DBSrv/BISrv), reator single-thread com `WSAAsyncSelect`.
- Mesmo protocolo `CPSock` (HEADER + tabela `pKeyWord` de 512 bytes + checksum).
- Mesma persistência file-based (`STRUCT_ACCOUNTFILE`, senha em texto plano).
- Mesmos riscos catalogados no `architectural-report` (checksum que não rejeita pacote, key table estática compartilhada, senhas em plaintext, IP de billing hardcoded).

### O que cada um traz de diferente
- **W2PP** é mais "limpo/upstream" (5 arquivos + 3 pastas no `Code/`, sem BISrv no tree público, sem os editores extras).
- **Este projeto** está **à frente** de W2PP: tem BISrv, os editores adicionais (ZerarSkill, SearchPass, EDITAPPSHOP), conteúdo de NPC/skills atualizado (vide commits recentes "NPCS Atualizados", "Corrigido Skill Exterminar") e toolset mais novo (v143/VS2022 vs VS2015 do W2PP).

### O que faz sentido aproveitar do W2PP
- **Diff upstream:** comparar `Basedef.h`/`CPSock.cpp` linha a linha pode revelar correções de bugs feitas pela comunidade do W2PP (mantido até fev/2025) que ainda não estão aqui.
- **`license.txt` + GPL-3.0:** como este é um fork de base GPL-3.0, vale confirmar a conformidade de licenciamento (o repo local tem `LICENSE`, mas a herança GPL deveria ser explicitada).

---

## 2. open-wyd/open-wyd-scripts — **O modelo data-driven que falta aqui**

### Relação
Pertence à organização **open-wyd** (open-wyd-scripts, owlauncher, open-wyd-forum, ows-clientdata, e um `open-wyd` privado/sem descrição). É a parte "OpenWYD" do nome deste repositório. Diferente do W2PP, **não é a mesma base de código** — é uma camada de scripting (Lua 5.3 + XML) para um engine "Open WYD" cujo núcleo C/C++ não é público.

### O que traz de novo (e que este projeto NÃO tem)
O engine alvo do open-wyd-scripts separa **conteúdo** de **binário**: eventos, itens, mercadores/NPCs, teleportes, spawns e comportamento de criaturas são descritos em Lua + XML, com APIs como `iGameServer`, `iSend`, `FindItem`. É o modelo TFS/OpenTibia aplicado a WYD.

Isso ataca diretamente as dívidas técnicas apontadas no `PROJECT-OVERVIEW` / component deep-dives deste projeto:

| Dívida técnica catalogada aqui | Como o modelo OpenWYD resolveria |
|---|---|
| Curva de EXP hardcoded em faixas de nível (`TMSrv/MobKilled.cpp:483-504`) | Tabela/Lua data-driven |
| 10 handlers `_MSG_CombineItem*` quase duplicados | Lógica em script reutilizável |
| `Server.cpp` ~10.5k linhas, `MobKilled.cpp` função de 3.5k linhas | Regras movidas para scripts |
| 58 arquivos `_MSG_*.cpp` (toda regra em C++) | Eventos/itens scriptáveis sem recompilar |
| Conteúdo só editável via ferramentas binárias (DropTool, ExpTool…) | Arquivos Lua/XML versionáveis em git |

### Complementa / coisas que fazem sentido para este projeto
- **Adotar um interpretador Lua embutido** no TMSrv seria a evolução natural — permitiria mover balanceamento/eventos para fora do C++.
- **owlauncher** (atualiza IP dos canais via DNS) resolve diretamente um problema apontado no `architectural-report`: IPs hardcoded em `localip.txt`/`serverlist.txt`/`biserver.txt` e nas pastas `serverlist bin/` por IP.
- **ows-clientdata** pode ser referência para empacotar os dados de cliente que aqui estão soltos em `Release/Common/`.

### Coisas faltantes (gap)
Este é o maior gap arquitetural: o projeto local é **100% hardcoded**, sem nenhuma camada de script — exatamente o oposto da filosofia OpenWYD. Migrar não é trivial (engines diferentes), mas o open-wyd-scripts serve como **blueprint da arquitetura-alvo**.

---

## 3. kevinkouketsu/Wyd2Client — **Lado cliente / referência de protocolo**

### Relação
Cliente/simulador WYD em **C# (WPF, MVVM)** com biblioteca de rede reutilizável. Kevin Kouketsu é desenvolvedor conhecido no reverse-engineering de WYD. Conecta-se ao **WYD Global (RaidHut oficial)**, mas implementa o **mesmo protocolo de fio** que o `CPSock` deste projeto (HEADER + estruturas WYD). A dependência `Wyd2.Common` é creditada a André Santa Cruz (ptr0x), com "estruturas básicas do WYD".

### Estrutura
- `W2Open.Common` — utilitários e estruturas do protocolo WYD (espelho C# do `Basedef.h`/`CPSock`).
- `Wyd2.Network` — camada de rede orientada a eventos (dispatcher).
- `Wyd2Client` — app WPF (login, criar/deletar personagem, movimento, teleporte, chat normal/sussurro, sistema de macros extensível).

### Como complementa este projeto
- **Documentação viva do protocolo:** o `architectural-report` aponta que o protocolo só existe como `CPSock.cpp` em C++; o Wyd2Client é uma segunda implementação independente do mesmo wire format — ótima para validar/entender os pacotes e as `STRUCT_*` de `Basedef.h`.
- **Cliente de teste / bot / load-test:** o projeto local não tem **nenhum teste automatizado** (apontado no PROJECT-OVERVIEW). Um cliente programável em C# permite scripts de fumaça (login, criar char, mover, chat) contra o TMSrv local — preenche a lacuna de testes ponta-a-ponta.
- **Padrão de macros/dispatcher** é reaproveitável para automação de QA.

### O que traz de novo
Arquitetura limpa (MVVM, dispatcher orientado a eventos) e a ideia de **desacoplar a biblioteca de protocolo** do app — algo que aqui está fundido em `CPSock`/`Basedef` (alto acoplamento, `Basedef.h` incluído por 16 arquivos).

### Coisas faltantes / atenção
- Conecta no servidor **oficial RaidHut**, então as estruturas podem divergir da versão 7.59/7662 usada aqui — exigiria ajuste de offsets/versão antes de apontar para o TMSrv local.
- Limitação conhecida: problemas de sincronização de threads entre envio e UI.

---

## 4. TMProject (`tm-project` + `w2Project-SeitTbNao`) — **O outro lado do fio**

> **Adendo de 2026-07-15.** Diferente de §1–§3, esta seção foi escrita já sob a ótica da
> **reescrita em Go** (`tmserver/`), não do legado C++. Os "matches" abaixo foram conferidos
> byte a byte contra o código Go/legado deste repo — as citações `arquivo:linha` são reais.

### Relação: uma categoria nova no estudo

Todo o estudo até aqui olhava **servidor** (§1 W2PP, §2 open-wyd-scripts) ou uma **reimplementação**
do cliente (§3 Wyd2Client, C#). O TMProject é a coisa que faltava: o **código-fonte descompilado do
cliente WYD em C++** — a contraparte exata do `CPSock` do nosso `tmserver`. Como este projeto tem
como alvo o **`WYD.exe` 7662 não-modificado** (CLAUDE.md), ter o "outro lado do fio" em código-fonte
é qualitativamente diferente de ter mais um servidor.

**Ligação com §3:** *Kevin Kouketsu* é contribuidor do TMProject **e** autor do Wyd2Client. O
Wyd2Client é, na prática, o descendente C# desta mesma linhagem de engenharia reversa.

### Os dois repositórios são o mesmo projeto

| | `EricSantos00/tm-project` | `MacedaoRG/w2Project-SeitTbNao` |
|---|---|---|
| Papel | **Upstream** | Cópia/snapshot |
| Commits | 846 | 4 |
| Stars / forks | 33 / 35 | 1 / 10 |
| CI | `.github/workflows/` | ausente |
| Issues / PRs | 8 / 5 | 0 / 0 |
| Conteúdo extra | — | **`Projects/Dataserver` + `Projects/Gameserver`** |

Ou seja: **para o cliente, use o `tm-project`** (é o mesmo código, mantido e com CI). O
`w2Project-SeitTbNao` só interessa pelo servidor que ele empacota junto — e esse servidor **não é o
nosso**: os arquivos são `RequestLogin.cpp`, `RequestAttack.cpp`, `Connect.cpp`, `Encrypt.cpp`/
`Decrypt.cpp`, `CMob.cpp`, sem `Server.cpp`/`MobKilled.cpp` nem os 58 `_MSG_*.cpp` de `Source/Code/`.
É uma **linhagem de servidor independente** da Polly/W2PP → vale como *segunda opinião* sobre a
semântica dos handlers, não como upstream para diff.

### O achado principal: o transporte CPSock bate 100%

O `Projects/TMProject/CPSock.cpp` do cliente traz a `pKeyWord[512]` completa, o transform e o
checksum. Conferido contra o nosso Go:

| Item | Cliente descompilado | Este projeto | Resultado |
|---|---|---|---|
| `INIT_CODE` | `521270033` (`CPSock.h:9`) | `INITCODE 0x1F11F311` (`Source/Code/CPSock.h:40`) | ✅ **idênticos** (`0x1F11F311 == 521270033`) |
| Tabela `pKeyWord[512]` | `CPSock.cpp:10` | `tmserver/internal/protocol/keytable.go:14` | ✅ **idêntica** (bytes conferidos) |
| Transform por `i & 0x3` | `<<1`, `>>3`, `<<2`, `>>5` | mesmos shifts (`transform.go:28-35`) | ✅ **idêntico** |
| Checksum | `(unsigned char)(Sum2 - Sum1)` | `checksum.go:7-9` | ✅ **idêntico** |
| Buffers | `131072` recv/send | `128*1024` | ✅ idêntico |

Isso converte várias premissas do `protocol-spec.md` de "lido do servidor legado" para
**confirmado nos dois lados do fio, de forma independente**.

### Confirmação forte: o checksum não-rejeitante é do cliente, não do patch

`checksum.go:11-14` hoje atribui o comportamento não-rejeitante a duas coisas: o servidor legado e o
**ClientPatch que NOPa a checagem** (`Hook.cpp:211-214`). O decompile mostra que o **próprio CPSock do
cliente já não rejeita** — ele marca o erro e devolve o pacote assim mesmo:

```c
if ((unsigned char)(Sum2 - Sum1) != CheckSum)
{
    *ErrorCode = 1;
    *ErrorType = Size;
}
return pMsg;          // <- entrega o pacote de qualquer jeito
```

Ou seja, o `-reject-checksum` off-by-default **não depende do ClientPatch** para estar correto: é
intrínseco ao cliente. Isso fecha o ponto do CLAUDE.md ("only enable once a capture confirms correct
checksums") **sem precisar de captura**.

### Gap real encontrado: a *keyword queue* (que o cliente REJEITA)

O cliente tem `SendQueue`/`RecvQueue[MAX_KEYWORD_QUEUE=16]` + `EncodeByte`, e faz uma validação
anti-tamper que — diferente do checksum — **descarta o pacote**:

```c
if (RecvQueue[0] != 0)      // <- só valida se a fila estiver "armada"
{
    ...                      // deriva qKeyword da fila / EncodeByte
    if (~qKeyword != iKeyWord)
    {
        *ErrorCode = 3;
        return 0;            // <- REJEITA
    }
}
```

O nosso `tmserver/internal/protocol/` **não implementa fila nenhuma** (só tabela + transform +
checksum). Hoje isso é inofensivo, porque a checagem é *gated* por `RecvQueue[0] != 0` e nunca
armamos a fila. Mas é uma **restrição latente**: se algum dia implementarmos a mensagem que popula
essa fila, o servidor passa a ser obrigado a seguir a sequência de keywords — senão o cliente
**dropa os pacotes em silêncio** (sem erro visível, sintoma clássico de "o cliente ignora meu
pacote"). Vale registrar no `protocol-spec.md` antes que alguém queime um dia debugando isso.

### Caveat crítico: **o cliente descompilado NÃO é o nosso 7662**

Enquanto o *transporte* bate 100%, a *camada de mensagens* diverge. `MSG_AccountLogin` (opcode
`0x20D` nos dois):

| Campo | Cliente descompilado (`Basedef.h:898-907`) | Este projeto (`messages.go:27-34` / `Source/Code/Basedef.h:1546-1560`) |
|---|---|---|
| Senha | `AccountPass[**16**]` | `AccountPassword[**12**]` (`ACCOUNTPASS_LENGTH`, `Basedef.h:126`) |
| Conta | `AccountName[16]` | `AccountName[16]` ✅ |
| Padding | `TID[52]` | `Zero[52]` ✅ |
| Versão | `int Version` | `int32 ClientVersion` ✅ |
| Flag | `int Force` | `int32 DBNeedSave` ✅ |
| MAC | `unsigned int Mac[4]` | `int32 AdapterName[4]` ✅ |
| **Total do body** | **108 bytes** | **104 bytes** (`MsgAccountLoginBodySize`) |

Campo a campo, na mesma ordem, com a mesma semântica — **exceto a senha (16 vs 12)**. Como o login
real funciona neste projeto contra o cliente 7662 (`test`/`test123`, ver `scripts/run-local.sh`),
os **nossos 12 bytes estão certos para o nosso alvo** e o decompile é de **outro build**.

**Conclusão prática, e é a regra de uso deste repo:**
- **Transporte (CPSock)** — estável entre versões, bate exato → pode ser usado como *ground truth*.
- **Structs de mensagem / offsets** — divergem por versão → **referência, nunca fonte de verdade**.
  Qualquer struct copiada de lá tem que ser confrontada com `Source/Code/Basedef.h` antes de virar Go.

Nota: o `12000` que aparece no `Basedef.h` deles **não** é versão de cliente (é tabela numérica) —
não confundir com o nosso `W2PP_CLIENT_VERSION=12000`. A versão-alvo do decompile permanece
**UNVERIFIED**.

### O que dá pra aproveitar

- **Tabela de opcodes do lado cliente:** 69 constantes `MSG_*_Opcode` nomeadas no `Basedef.h`
  (`MSG_Trade_Opcode = 0x383`, `MSG_UseItem_Opcode = 0x373`, `MSG_Action_Opcode = 0x36C`,
  `MSG_Motion_Opcode = 0x36A`, `MSG_MessageWhisper_Opcode = 0x334`…). O `0x383` bate com o que já
  documentamos como trade P2P. Serve para **atacar os 5 `UNVERIFIED` do `protocol-spec.md`** e para
  dar nome a opcodes que hoje só conhecemos por número.
- **Semântica de renderização:** o cliente tem o código de verdade de mesh/efeito/affect
  (`TMEffect*`, `UpdateAffect` `0x3B9`, `UpdateEquip` `0x36B`). Vários bugs já debugados às cegas
  (brilho de refino, mesh de transformação BM, nick PK) têm a resposta **em código** aqui, em vez de
  tentativa-e-erro contra o `.exe`.
- **Licença:** GPL-3.0, igual à herança do §1 — mesma pendência de conformidade já apontada lá.

### Riscos / atenção

- **Descompilação ≠ original.** O próprio README avisa: *"este projeto é uma descompilação e o mesmo
  pode e contém problemas"*. Onde ele divergir do `Source/`, o `Source/` (que roda) ganha.
- **Não vale copiar código.** O valor é *documental* (entender o cliente), não de reuso — somos uma
  reescrita em Go, e colar C++ descompilado traria junto os bugs e o risco de licença/copyright
  (direitos da Hanbitsoft, ressalvados pelos próprios autores).

---

## Matriz consolidada

| Projeto | Tipo de relação | Mesma base de código? | Complementa | Traz de novo | Aplicável aqui |
|---|---|---|---|---|---|
| **W2PP** | Ancestral / fork irmão | **Sim** (idêntica) | — (é a origem) | Correções upstream da comunidade | Diff/merge de fixes; conformidade GPL |
| **open-wyd-scripts** | Mesma "família" OpenWYD, engine distinto | Não | Sim (arquitetura-alvo) | Scripting Lua+XML data-driven; owlauncher (DNS); ows-clientdata | Blueprint p/ tirar regras do C++; resolver IPs hardcoded |
| **Wyd2Client** | Lado cliente do mesmo protocolo | Não (porta C#) | Sim (cliente/protocolo) | Lib de protocolo desacoplada; cliente programável | Doc de protocolo; cliente de teste/bot; cobrir lacuna de testes |
| **tm-project** (TMProject) | **Cliente descompilado** — o outro lado do fio | Não (é o cliente) | **Sim (o que faltava)** | Fonte C++ do cliente; `pKeyWord`+checksum confirmados; 69 opcodes nomeados | Confirmar `protocol-spec`; matar `UNVERIFIED`s; debugar bugs visuais em código |
| **w2Project-SeitTbNao** | Cópia do TMProject **+ servidor** | Não | Parcial | `Dataserver`/`Gameserver` de **linhagem distinta** (`Request*.cpp`) | 2ª opinião sobre handlers; **não** é upstream p/ diff |

## Recomendações priorizadas para w2pp-OpenWYD

1. **Sincronizar com W2PP upstream** — diff em `Basedef.*`/`CPSock.*` para puxar correções da comunidade (W2PP foi mantido até 2025); registrar a herança GPL-3.0.
2. **Resolver IPs hardcoded** — adotar a abordagem do **owlauncher** (DNS) no lugar de `biserver.txt`/`localip.txt`/`serverlist bin/` fixos (risco "High" no architectural-report).
3. **Testes ponta-a-ponta** — usar **Wyd2Client** (ajustado p/ versão 7662/7.59) como cliente automatizado para cobrir a ausência total de testes.
4. **Roadmap de scripting** — usar **open-wyd-scripts** como referência para, a médio prazo, migrar balanceamento (curva de EXP, drops, eventos) de C++ hardcoded para dados/script, reduzindo `Server.cpp`/`MobKilled.cpp` e os 58 `_MSG_*.cpp`.

### Adendo 2026-07-15 — decorrentes da §4

5. **Registrar a *keyword queue* como restrição latente** no `protocol-spec.md` — o cliente
   **rejeita** (`ErrorCode=3`) quando `~qKeyword != iKeyWord`, e só não nos afeta porque a fila nunca
   é armada. É o item de maior risco de "bug fantasma" futuro do adendo.
6. **Fechar o `-reject-checksum`** — o decompile prova que o não-rejeitar é do próprio cliente, não
   do ClientPatch. Atualizar o comentário de `checksum.go:11-14` (que hoje credita o `Hook.cpp`) e
   decidir a Fase 7 sem depender de captura.
7. **Usar a tabela de 69 opcodes** do `Basedef.h` do cliente para atacar os 5 `UNVERIFIED` do
   `protocol-spec.md` — sempre confrontando com `Source/Code/Basedef.h`, **nunca** copiando struct
   (o `MSG_AccountLogin` de 108 vs 104 bytes mostra que os layouts divergem por versão).
8. **Consultar o cliente antes de debugar bug visual às cegas** — mesh/affect/refino têm o código de
   verdade em `TMProject/`; foi assim que gastamos tentativa-e-erro em brilho de refino e mesh de BM.

---

*Observações baseadas nos READMEs/metadados públicos dos repositórios (jun/2026) e nos relatórios em `docs/agents/`. O núcleo C/C++ do engine "Open WYD" não é público; a comparação com open-wyd-scripts é arquitetural, não de código.*

*Adendo (jul/2026, §4): baseado na leitura direta de `CPSock.h/.cpp` e `Basedef.h` do `tm-project`, conferidos byte a byte contra `tmserver/internal/protocol/` e `Source/Code/`. O `tm-project` é uma **descompilação de um build de cliente que não é o nosso 7662** (divergência confirmada no `MSG_AccountLogin`); a versão exata dele permanece **UNVERIFIED**.*
