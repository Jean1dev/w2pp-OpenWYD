# Viabilidade: `alanpetry/OpenWyd` como client web/desktop do w2pp-OpenWYD

> Análise estática, sem sessão capturada. Todas as afirmações citam evidência em
> `arquivo:linha`. Pontos não confirmados estão marcados **UNVERIFIED**.
>
> Repositório analisado: `github.com/alanpetry/OpenWyd` @ `main` (último commit
> 2026-08-04). Comparado contra este repo em `Source/Code/` (C++ legado 7662) e
> `tmserver/internal/protocol/` (porte Go).

---

## Veredito

**Viável com esforço médio — mas o gargalo não é o protocolo.**

A camada de transporte CPSock é **byte-a-byte idêntica** entre os dois projetos
(tabela de ofuscação com o mesmo SHA-256, mesmo INITCODE, mesmo algoritmo de
checksum, mesmo header de 12 bytes), e 60 dos 68 opcodes do client deles já
existem no meu espaço de opcodes. O que diverge são **layouts de struct** em
alguns pontos enumeráveis (`MAX_EQUIP` 16 vs 18, `MAX_CARGO` 128 vs 120,
`STRUCT_SCORE.Level` int vs short) — divergência real, mas fechada e conhecida,
e corrigível **no lado deles**, já que o client deles compila a partir de fonte.

Os bloqueios reais são outros dois, e nenhum é técnico de protocolo:
1. **Não existe arquivo de licença** no repositório deles — só uma declaração de
   *intenção* no README.
2. **Gameplay em rede não está validado** — o próprio README lista "validar os
   fluxos multiplayer restantes" como trabalho pendente; o que roda hoje é
   movimento **local** na cena Field.

---

## Sumário das evidências

### O que é 100% compatível (camada de transporte)

| Artefato | Meu lado | Client deles | Resultado |
|---|---|---|---|
| Tabela `pKeyWord` (512 bytes) | `Source/Code/CPSock.cpp:29` | `Projects/TMProject/CPSock.cpp` | **SHA-256 idêntico** (`e47996fe5e92de5d`) |
| Porte Go da tabela | `tmserver/internal/protocol/keytable.go` | — | **mesmo SHA-256** |
| INITCODE | `0x1F11F311` (`Source/Code/CPSock.h:40`) | `521270033` (`CPSock.h:8`) | **mesmo valor** |
| Header do pacote | `_MSG` macro, `Source/Code/Basedef.h:1205` | `MSG_STANDARD`, `Basedef.h:27` | **idêntico**: Size(2) KeyWord(1) CheckSum(1) Type(2) ID(2) Tick(4) = 12 B |
| Algoritmo de ofuscação | `Source/Code/CPSock.cpp:424-455` | `CPSock.cpp:890-912` | **idêntico** (mesmo padrão mod-4, mesmo `pKeyWord[rst*2+1]`) |
| Checksum | `Sum2 - Sum1`, não-rejeitante | `(unsigned char)(Sum2 - Sum1)`, não-rejeitante | **idêntico** |
| `MAX_MESSAGE_SIZE` | 8192 | 8192 | idêntico |

A tabela de ofuscação ser byte-idêntica é o achado mais forte da análise. Ela é o
artefato que historicamente mais diverge entre forks privados de WYD; ser
idêntica confirma que os dois projetos descendem do mesmo vazamento C++
(cabeçalho GPL "Victor Klafke, Charles TheHouse" em `Source/Code/CPSock.cpp:1`).

**Consequência prática:** o `wsproxy` deles conecta no meu `tmServer:8281` e os
frames vão ser desofuscados corretamente. O handshake INITCODE passa. Isso não é
suposição — os quatro constituintes do pipeline (INITCODE → framing por Size →
transform por keyword → checksum) são os mesmos.

### O que é majoritariamente compatível (opcodes)

Comparação por **valor** (não por nome), extraindo `const short _MSG_X = (n | FLAG)`
do meu `Basedef.h` e `constexpr auto MSG_X_Opcode` do `Basedef.h` do client deles:

- **68** opcodes no client deles
- **60** existem no meu espaço de opcodes (88%)
- **8** ausentes: `InitGuldName` (0x01D6), `MessageLog` (0x02BC), `DoJackpotBet`
  (0x02BE), `DelayStart`/`SysQuit` (0x03AE), `UseItem2` (0x03C9),
  `RepurchaseItems` (0x03E8), `Encode` (0x0BFF), `UseDeclarationOfWar` (0x0ED7)

Os "conflitos" de nome são só nomenclatura — mesmo valor, rótulo diferente:

| Valor | Client deles | Meu servidor |
|---|---|---|
| 0x020D | `AccountLogin` | `AccountLogin` ✓ |
| 0x020F | `NewCharacter` | `CreateCharacter` |
| 0x0289 | `Recall` | `Restart` |
| 0x0367 | `Attack_Multi` | `Attack` |
| 0x0376 | `SwapItem` | `TradingItem` |
| 0x0FDE | `CharPassword` | `AccountSecure` |
| 0x0AD9 | `AirMove_Start` | `MasterGriff` |

Meu dispatcher Go (`tmserver/internal/handler/dispatch.go`) já registra ~53
handlers cobrindo todos os opcodes críticos do caminho login → seleção →
movimento → combate → itens.

### O que **não** é compatível (layouts de struct) — o trabalho real

Aqui está a divergência concreta. As constantes de dimensionamento diferem:

| Constante | Meu (7662) | Deles (v769) | Evidência |
|---|---|---|---|
| `MAX_EQUIP` | **16** | **18** | `Source/Code/Basedef.h:135` vs `Servidor/Source/Code/Basedef.h:207` |
| `MAX_CARGO` | **128** | **120** | `Source/Code/Basedef.h:137` vs `Servidor/Source/Code/Basedef.h:209` |
| `STRUCT_SCORE.Level` | `int` (4 B) | `short` (2 B) | ambos `Basedef.h` |

Isso propaga para os dois pacotes mais importantes do jogo:

**`STRUCT_SELCHAR`** — meu = **840 bytes** (confirmado em
`tmserver/internal/protocol/selchar.go:11`), deles = **904 bytes**.
Cálculo (alinhamento natural MSVC x86, `STRUCT_ITEM` = 8 B, `STRUCT_SCORE` = 48 B
nos dois):

```
             offset   meu                      deles
SPX/HomeTownX   @0    short[4]    = 8          idêntico
SPY/HomeTownY   @8    short[4]    = 8          idêntico
Name/MobName   @16    [4][16]     = 64         idêntico
Score          @80    4 x 48      = 192        idêntico
Equip         @272    [4][16] x 8 = 512    <-> [4][18] x 8 = 576   <<< DIVERGE
Guild                                8
Coin                                16
Exp                                 32
                    -------------------      -------------------
                     840 bytes                904 bytes
```

**Os primeiros 272 bytes são byte-idênticos.** A divergência começa exatamente em
`Equip`. `STRUCT_SCORE` dá 48 bytes nos dois apesar de `Level` ser int vs short —
o padding de alinhamento absorve a diferença, e como é little-endian, um `Level`
< 65536 até lê corretamente. (Meus campos `Direction`/`ChaosRate` em @14-15 caem
no padding deles — o client deles simplesmente perderia esse dado, sem quebrar
o parse. **UNVERIFIED** se isso causa bug visual.)

**`MSG_CNFAccountLogin` (0x010A)** — o **primeiro pacote servidor→client depois
do login**. Meu = **2008 bytes** (`selchar.go:15`), deles ≈ **1920 bytes**. Além
do tamanho, os campos de cauda diferem: eu emito `Keys[12]`, `STRUCT_QUEST
QuestDiaria`, `BlockPass[16]`, `IsBlocked`; eles esperam `SSN1`, `SSN2`
(`Projects/TMProject/Basedef.h:798-808`).

> Ou seja: o login TCP conecta, o pacote chega desofuscado corretamente, e o
> client **misparseia a tela de seleção de personagem**. É exatamente aí que a
> Rota A ingênua quebra.

**`MSG_CreateMob` (0x0364)** — o pacote de maior tráfego do jogo (emitido para
cada entidade visível). Também diverge:

| Campo | Meu (`Source/Code/Basedef.h`) | Deles (`Projects/TMProject/Basedef.h`) |
|---|---|---|
| `Equip` | `unsigned short[16]` | `unsigned short[18]` |
| pós-Guild | `GuildMemberType` + `Unknow[3]` | `GuildLevel` (+ padding) |
| cauda | `AnctCode[16]`, `Tab[26]`, `Hold` (int) | `Equip2[18]`, `Nick[26]`, `Server` (char) |

### Correções a premissas da tarefa

Duas premissas do enunciado não se sustentaram:

1. **O client deles não manda `ClientVersion = 1059`.** O `APP_VERSION 1059`
   (`Servidor/Source/Code/Basedef.h:173`) é do **servidor** deles. O client
   hardcoda **`1758`**, em dois lugares:
   `Projects/TMProject/TMFieldScene.cpp:18790` e
   `Projects/TMProject/TMSelectServerScene.cpp:1417` (`stAccountLogin.Version = 1758`).
   Como os dois valores discordam entre si, o servidor deles aparentemente não
   rejeita por versão. **UNVERIFIED** — não li o `_MSG_AccountLogin.cpp` deles a
   fundo. Para mim isso é irrelevante de qualquer forma: é um `int32` atrás da
   flag `-client-version`.

2. **`Source/` deste repo é só servidor.** Contém `TMSrv`, `DBSrv`, `BISrv`,
   ferramentas e `ClientPatch_v7662` (um *patcher* que injeta hooks no `.exe`,
   não o fonte do client). Não há nenhum fonte de renderização/cena
   (`find Source -iname "*render*" -o -iname "*d3d*" -o -iname "*scene*"` → vazio).
   **A Rota B, como formulada, é impossível: não existe fonte do client 7662 aqui.**

---

## Rota A vs Rota B

A Rota B original está descartada (sem fonte do client 7662). Mas a análise
revelou uma terceira rota, que é a melhor das três — e ela só existe porque o
client **deles** vem com fonte completo (`Projects/TMProject/`, ~igual ao
client original C++: `TMHuman.cpp`, `RenderDevice.h`, `TMSkill*.cpp`, `CFrame.cpp`).

| | **Rota A** — apontar client deles pro meu server | **Rota B** — recompilar 7662 com a toolchain deles | **Rota C** (recomendada) — fork do client deles, retarget pra 7662 |
|---|---|---|---|
| **Como** | `wsproxy → tmserver:8281`, ajustar `-client-version`, e então adaptar **meu servidor** aos layouts v769 | Usar WASM toolchain + wsproxy contra fonte do client 7662 | Editar `Basedef.h` do client deles (`MAX_EQUIP` 16, `MAX_CARGO` 120→128, `Level` int) e recompilar o WASM |
| **Esforço** | Alto | **Impossível** | **Médio** |
| **Risco** | **Alto** — força meu servidor a emitir layout v769, quebrando o alvo `WYD.exe` 7662 nativo e o `STRUCT_ACCOUNTFILE` = 7952 | — | Baixo-médio — mudanças concentradas em `Basedef.h` + alguns sites de struct |
| **Retorno** | Client web, mas com o servidor comprometido | — | Client web **sem tocar no servidor**; meu tmServer continua fonte única de verdade |
| **Bloqueio** | Não posso servir dois layouts ao mesmo tempo | Sem fonte do client 7662 | Licença (ver abaixo) |

**Por que a Rota C ganha:** o client deles é compilado a partir de fonte, então a
divergência de struct é corrigível *no client*, não no servidor. Isso preserva o
invariante mais importante do projeto — o `tmServer` continua falando o dialeto
7662 que o `WYD.exe` nativo espera. Mudar meu servidor pra falar v769 (Rota A)
significaria manter dois dialetos ou abandonar o client nativo.

**Fator favorável à manutenção do fork:** os 70 commits recentes deles são
*todos* na camada de renderização (`feat(wasm): render ...`, `feat(wasm): ship HD
terrain`). O `Basedef.h` não é onde o churn acontece. Um fork que altera
constantes de protocolo tende a rebasear sem conflito.

---

## Próximos passos (priorizados)

Ordem desenhada para **falhar barato**: cada passo invalida o plano antes do
passo seguinte custar caro.

1. **Resolver a licença primeiro — antes de escrever qualquer código.**
   Não há `LICENSE` no repo deles. Abrir issue pedindo que adicionem
   `LICENSE` GPL-3.0. Sem isso, tudo abaixo é trabalho com risco jurídico
   (detalhes na seção de riscos). *Custo: 1 issue. Desbloqueia ou mata o plano.*

2. **Validar o transporte end-to-end, sem tocar em struct.**
   Subir `wsproxy` apontando pro meu `tmserver:8281`
   (`--target-host tmserver --target-port 8281`) e confirmar que o INITCODE passa
   e que o `MSG_AccountLogin` chega desofuscado no meu handler.
   Critério de sucesso: meu `tmServer` loga um `Type=0x020D` com `AccountName`
   legível. *Isso valida a tabela `pKeyWord` na prática — o achado mais forte da
   análise — com custo quase zero.*

3. **Instrumentar os dois lados para o diff byte-a-byte.**
   Os dois lados já têm quase tudo pronto:
   - **Lado deles:** o `wsproxy` já tem `TransportTrace`
     (`webclient/server/wyd_tcp_proxy.py:22`), que grava chunk, direção,
     tamanho e **SHA-256** por transferência, em JSONL — sem decodificar o
     payload. É exatamente a ferramenta de captura necessária.
   - **Lado meu:** o client deles tem `g_wyd_socket_debug` com
     `last_sent_opcode` / `last_recv_opcode` (`CPSock.cpp:920`).
   - **Falta:** um dump hex pré-ofuscação no meu `protocol.Encode`, atrás de
     flag. Comparar contra o `sizeof` esperado pelo client.

4. **Confirmar a quebra prevista no `MSG_CNFAccountLogin`.**
   Predição desta análise: o login passa, e o client quebra ao parsear a tela de
   seleção de personagem, porque eu mando **2008** bytes e ele espera **~1920**.
   Se essa predição se confirmar, a análise está calibrada e a Rota C é o
   caminho. Se quebrar *antes* disso, há divergência não mapeada e a estimativa
   de esforço precisa ser refeita.

5. **Só então: fork + retarget do `Basedef.h` deles.**
   `MAX_EQUIP` 18→16, `MAX_CARGO` 120→128, `STRUCT_SCORE.Level` short→int, e a
   cauda de `MSG_CNFAccountLogin`/`MSG_CreateMob`. Recompilar o WASM e repetir o
   passo 4.

6. **Adiar desktop/Electron indefinidamente.** Não há código (ver riscos).

---

## Riscos e bloqueios

### 🔴 Bloqueador — licenciamento indefinido

**Não existe arquivo `LICENSE` no repositório deles.** O que existe é uma
declaração de intenção no `README.md:117`:

> "The OpenWyd-authored source changes are **intended to be available** under
> GPL-3.0 or later."

"Intended to be available" não é uma concessão de licença. Sem `LICENSE`, o
padrão do copyright é **todos os direitos reservados** — vendorar `webclient/` ou
`wsproxy` neste repo e distribuir junto não está autorizado hoje.

Agravantes:
- O repo deles empacota o binário original do client e assets, sobre os quais
  eles **explicitamente não reivindicam direitos** (`README.md:110-116`). Isso é
  material de terceiros que eu não posso redistribuir de qualquer forma.
- **Inconsistência no meu próprio repo:** o `LICENSE` da raiz é **GPL-2.0**,
  enquanto `Source/license.txt` é **GPL-3.0** e os headers dos fontes dizem
  "version 3 or any later version" (`Source/Code/CPSock.cpp:6`). GPL-2.0-only e
  GPL-3.0 são **incompatíveis**. Isso precisa ser resolvido internamente antes de
  qualquer integração — provavelmente trocando o `LICENSE` da raiz por GPL-3.0
  pra bater com os fontes que já estão aqui.

**Mitigação:** tratar como dependência externa (submodule / serviço separado no
compose), nunca vendorar, até que exista `LICENSE` explícito.

### 🟠 Alto — maturidade do build WASM

O README é honesto sobre isso, e a leitura otimista do enunciado não se sustenta.
O que está em **"What remains"** (`README.md:66-79`):

> "Validate the remaining multiplayer gameplay flows against the Linux server,
> including character creation, movement, combat, teleport, and persistence."

E o que funciona hoje (`README.md:37`): "The Field scene runs with real client
assets and **local movement**" — movimento **local**, não sincronizado.

Ou seja: **o gameplay em rede não está validado nem contra o servidor deles.**
Startup, select-server, select-character e Field estão "available for runtime
investigation" — que é uma formulação bem mais fraca que "funcionando". Combate,
itens, chat e trade não têm validação de rede nenhuma.

Para produção isso é cedo demais. Para prototipagem é utilizável.

### 🟠 Alto — dependência de mantenedor único

| Métrica | Valor |
|---|---|
| Criado em | 2026-06-24 (~7 semanas) |
| Último push | 2026-08-04 (10 dias atrás) |
| Estrelas / forks | **2 / 0** |
| Contribuidores | **1** (Alan Petry) |
| Issues+PRs abertos | 487 |
| Branches | `agent/...` — desenvolvimento majoritariamente por agente |

Velocidade alta (70 commits em 4 dias) mas **bus factor 1**, comunidade zero, e
sem sinal de processo de contribuição externa. Não há `CONTRIBUTING.md`.
Risco concreto de ficar órfão. Um fork meu seria, na prática, permanente.

*Atenuante:* como todo o churn está na camada de render e não no protocolo, um
fork sobrevive bem a upstream parado — o que eu precisaria do upstream (parity de
render) é justamente o que eles estão entregando rápido.

### 🟡 Médio — Electron é só texto

Busca por `electron` em todo o repo retorna **duas ocorrências, ambas prosa de
roadmap** no README (`:77` em inglês, `:200` em português). Zero código, zero
dependência no `webclient/package.json`, nenhum diretório de host desktop.

Tratar como **não iniciado**. O esforço adicional seria integral: empacotar o
bundle WASM + os ~512 MB de assets num host Electron, mais a "native bridge" que
o README menciona. Não é uma extensão barata do client web.

### 🟡 Médio — componente extra não mencionado

A integração não é "wsproxy + página estática". O `webclient/app` depende de um
**asset server** próprio com API HTTP: `/api/manifest`, `/api/manifest/bootstrap`,
`/api/resolve`, `/api/assets/mesh-related`, `/api/assets/fields`, etc.
(`webclient/app/src/assetClient.js`, `webclient/app/src/main.js:1797+`).
São **três** componentes a encaixar, não dois — e o bundle de assets é de ~512 MB
na primeira visita (`README.md:57`).

---

## Arquitetura de integração recomendada

**O invariante do single-owner game loop não é ameaçado.** Confirmado: o
`wsproxy` é transporte puro. O docstring dele é explícito
(`webclient/server/wyd_tcp_proxy.py:23`):

> "This deliberately records transport chunks rather than interpreting WYD
> packets. The payload itself is never decoded, changed, or persisted."

Ele termina um WebSocket e reabre um TCP para `--target-host:--target-port`,
repassando bytes crus. Do ponto de vista do `tmServer`, uma conexão vinda do
wsproxy é indistinguível de uma conexão TCP direta: entra pelo `world.Server`,
vira uma goroutine por conexão, e só troca mensagens com o loop por channel.
**Nenhum estado de mundo novo, nenhum caminho de mutação novo, nenhum lock.**

Topologia proposta — três serviços novos, todos fora do caminho do loop:

```
browser ──WS──> wsproxy ──TCP──> tmserver:8281   (loop single-owner intacto)
   │                                  │
   └──HTTP──> webclient (app+assets)  ├── dbserver:7514
                                      ├── binserver:3000
                                      └── webserver:7600
```

Regras de encaixe:
- `wsproxy` sobe com `--target-host tmserver --target-port 8281` e
  **`--no-client-target`** — sem isso ele aceita `?host=&port=` da querystring e
  vira um proxy TCP aberto (SSRF). Não expor sem essa flag.
- Entrar como `git submodule` + serviços próprios no compose, **não** vendorado
  (ver bloqueador de licença).
- O `webserver:7600` (criação de conta) continua sendo o caminho de auth web; o
  client WASM não deve inventar contas anônimas como faz a demo pública deles.

---

## Conclusão

O trabalho de engenharia reversa que este projeto já fez **se paga** aqui: a
compatibilidade da camada de transporte não é sorte, é consequência de os dois
projetos descenderem do mesmo fonte C++, e é verificável estaticamente sem
capturar uma sessão — que era a pergunta central do enunciado. A resposta é sim,
dá para comparar estaticamente, e a comparação foi feita: tabela de ofuscação,
INITCODE, checksum, header e 88% dos opcodes conferem.

A divergência de struct é real mas **enumerável e pequena** — três constantes e
duas caudas de pacote. Não é o abismo que a divergência histórica entre builds de
WYD costuma produzir.

O que recomendo é não deixar a boa notícia do protocolo mascarar o resto: o
projeto deles tem 7 semanas, um mantenedor, nenhuma licença, e gameplay em rede
não validado nem contra o próprio servidor. Vale gastar o passo 1 (issue de
licença) e o passo 2 (validar transporte, custo quase zero) antes de qualquer
compromisso maior.
