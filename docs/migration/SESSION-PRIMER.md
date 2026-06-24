# Session primer — w2pp-OpenWYD (cole no início de uma nova sessão)

> Resumo de contexto + fluxo de trabalho para continuar a migração do WYD para Go.
> Leia também `CLAUDE.md` (raiz) e `docs/migration/ingame-bugs.md` (rastreador de bugs in-game).

## 1. O que é o projeto

Reescrita **big-bang em Go** do servidor do MMORPG **WYD (With Your Destiny)**, mirando o
**cliente Windows `WYD.exe` original e SEM modificações** ("Cavaleiros de Kersef").
- **ClientVersion = `12000`** (esse build manda 12000; o tmserver roda com `-client-version 12000`).
- Fonte C++ legada em `Source/Code/` (decompile **parcial** — algumas funções só têm declaração).
- Conteúdo de jogo + binários legados em `Release/` (montado read-only nos containers).
- Serviços novos em `tmserver/`, `dbserver/`, `binserver/`. Módulo Go: `github.com/jeanluca/w2pp-openwyd`.

### Status atual (o que JÁ funciona contra o cliente real)
Login → seleção/criação de personagem (com equipamento visual correto) → entrar no mundo →
andar → ver outros players → ver ~10.500 NPCs (nomes visíveis, aparência correta) → **lojas de NPC**
(abrir, comprar, vender, item aparece no inventário ao vivo, economia de gold) → **teleporte entre
cidades** (por tile, Armia↔Noatum↔Azran/Erion/Nippleheim) → **persistência** de inventário, gold e
**última cidade** (spawn na área default da última cidade) ao deslogar/relogar.

## 2. Arquitetura (o essencial)

Três microserviços; só a borda **cliente↔tmServer** fala o protocolo legado (CPSock). Links internos
são **gRPC (+mTLS opcional)**.
- **tmServer** (`tmserver/`): jogo. Porta **8281** (CPSock game) e **80** (HTTP status `serv00.htm`,
  listeners SEPARADOS — o cliente sonda o status no :80 antes de abrir o :8281). Dono de TODO o estado
  do mundo em **um único goroutine** (`world.World.Run`) — **sem locks**. Handlers rodam DENTRO do
  loop e mutam estado direto. Chamadas bloqueantes (dbServer/billing) vão para fora do loop via
  `World.Go(...)` e o resultado re-entra no loop por callback.
- **dbServer** (`dbserver/`, porta **7514**): persistência (PostgreSQL/pgx v5). Subcomandos:
  `serve`, `convert`, `seed-account`. Migrations embutidas em `dbserver/migrations/*.up.sql`
  (aplicadas em ordem no boot).
- **binServer** (`binserver/`, porta **3000**): billing (allow-all por padrão).

### Regras de fidelidade que importam
- **Sem criptografia real**: CPSock é obfuscação por tabela estática (`pKeyWord`) + checksum não
  rejeitante. Header CPSock = **12 bytes**.
- **Layout binário é offset-explícito**. Structs de save = alinhamento natural MSVC x86 (NÃO pack(1)).
  Não confie no alinhamento do Go — leia/escreva por offset. Tamanhos-chave: `STRUCT_MOB`=816,
  `STRUCT_ITEM`=8, `STRUCT_SELCHAR`=840.
- **Paridade de RNG** é meta (LCG do MSVC reimplementado em `tmserver/internal/rng`). Preserve a ordem
  das chamadas de rand em código de gameplay.

## 3. O AGENTE DO WINDOWS (como obter dados que não temos)

O usuário tem, **na máquina Windows**, o **servidor ORIGINAL que COMPILA e roda** (a fonte completa,
mais completa que nossa cópia parcial em `Source/Code/`) + uma instância do Claude Code lá, com um
**dumper compilado** (`_layout_probe/dump_layout.cpp`) que imprime `sizeof`/`offsetof` **verificados
pelo compilador** (MSVC x86).

**Quando usar:** sempre que precisar de um layout byte-exato de struct/pacote, de valores de tabela
hardcoded, ou da lógica de uma função que **não existe** na nossa cópia da fonte (decompile parcial).
Eu (assistente) **não falo com o agente direto** — eu **escrevo um prompt** e o usuário cola lá, roda,
e traz o resultado de volta (geralmente um `.md`).

**Como escrever um bom prompt para o agente:**
- Dê o contexto (migração WYD→Go, header CPSock=12B, o que já funciona).
- Peça **offsets + tipos** de cada campo, **tamanho total** (via o dumper), e o valor do **Type** do
  pacote (com `FLAG_GAME2CLIENT 0x0100` / `FLAG_CLIENT2GAME 0x0200`).
- Peça o **código** da função relevante (ex.: `SendFunc.cpp`, `_MSG_*.cpp`) quando a lógica importa.
- Peça para **salvar num arquivo** `captura-wyd-<assunto>.md`.
- Lembre que o agente tem a fonte COMPLETA: se uma função não está na nossa `Source/Code`, ele tem.

Exemplos já entregues: `serv00.htm`/sizes S→C, `STRUCT_MOB`/BaseMob, CreateMob/ShopList/SendItem,
ItemList preços + fórmulas buy/sell, tabela das 5 cidades, tabela de rotas de teleporte
(`GetTeleportPosition`/`DoTeleport`).

## 4. Rodar e testar localmente

Servidor roda nesta máquina Linux; o cliente real roda no **Windows do usuário** apontando o
`serverlist.bin` para o IP desta máquina (status `http://<ip>/serv00.htm` no :80, game no :8281).

```bash
docker compose up -d --build            # sobe tudo (db, dbserver, binserver, tmserver)
docker compose run --rm dbserver seed-account -name test -pass test123   # conta de teste
docker compose logs --since 2m tmserver # logs (todo pacote logado: "recv packet type=0x....")
```

**Teste de login HEADLESS (sem o cliente Windows) — use ANTES de culpar o código de jogo:**
```bash
W2PP_E2E_ADDR=127.0.0.1:8281 W2PP_E2E_ACCOUNT=test W2PP_E2E_PASSWORD=test123 \
W2PP_E2E_VERSION=12000 go test -tags=e2e -run TestE2ESmokeLogin ./tmserver/internal/world/ -v
```
`CNFAccountLogin (0x10a)` = chain tmServer→dbServer→Postgres OK. **Tem que mandar
`W2PP_E2E_VERSION=12000`** ou o servidor responde `0x102` (version mismatch).

Build/test padrão: `go build ./...`, `go test -race ./...`, `make lint`.

## 5. LIÇÕES DE DEBUG (aprendidas no sangue — leia antes de gastar horas)

1. **Servidor-correto ≠ cliente-correto.** O servidor pode calcular tudo certo e o cliente mostrar
   errado se o **pacote S→C** carrega o valor errado. Ex. real: a persistência da cidade funcionava
   (load `last_city` certo, spawn calculado certo), mas o `CNFCharacterLogin` mandava a **posição do
   template (Armia)** em vez do spawn calculado → cliente sempre desenhava Armia. **Sempre confirme o
   que VAI NO PACOTE**, não só o estado do servidor. Adicione logs dos dois lados (load no dbserver +
   spawn no tmserver) e compare.

2. **Imagem Docker stale é traiçoeira.** `docker compose up --build` às vezes **reusa um layer em
   cache** e NÃO recompila com o código novo — especialmente após `make proto` (o `db.pb.go`
   regenerado). Sintoma: o DB tem o valor certo mas o serviço retorna 0/antigo; ou o cliente ignora um
   campo novo do proto (desserializa sem o field). **Fix:** `docker compose build --no-cache <svc>` e
   reinicie. Quando um campo persistido "não carrega" mas o DB está certo, **suspeite da imagem antes
   do código**. Depois de mexer no `.proto`, rebuild `--no-cache` nos DOIS lados (tmserver+dbserver).

3. **gRPC stale / rede Docker.** "Login travado / conectando" quase sempre é o link gRPC
   tmServer↔dbServer, não o jogo: (a) redeploy só do tmserver deixa a conexão stale → reinicie os dois;
   (b) **IPAM do Docker esgotado** ("all predefined address pools have been fully subnetted") após
   muitos ciclos down/up/run → o compose tem um subnet fixo `172.28.0.0/24` em `networks.default.ipam`
   pra contornar; limpeza total = `sudo systemctl restart docker`.

4. **Não reinicie serviços no meio de um teste do usuário.** Restart no meio dispara save-on-shutdown /
   reconexão gRPC e gera falhas transitórias que parecem bug de código. Faça o deploy, ESPERE o usuário
   testar, e só então mexa.

5. **Persistência: save assíncrono vs reload.** Saves on logout/disconnect são `World.Go` (fora do
   loop). Para o logout-para-seleção (mesma conexão, reload rápido), use `World.SaveCharacterThen` (só
   confirma ao cliente DEPOIS do save commitar). `shutdown()` espera saves em voo via `saveWG`.

## 6. Fatos/constantes úteis

- ClientVersion **12000**. Conta teste **test/test123**. Char inicial: 1.000.000 de gold, spawn Armia.
- **5 cidades** (`world/city.go`): Armia(0) (2086,2093), Azran(1) (2494,1707), Erion(2) (2453,2000),
  Nippleheim(3) (3652,3122), Noatum(4) (1050,1706). "Última cidade" salva em 2 bits (0–3; Noatum não é
  salvável → cai em Armia). Spawn = `CitySpawn(cidade) + rand%15`.
- **Teleporte** (`world/teleport.go`): por TILE; cliente pisa e manda `_MSG_ReqTeleport` (0x0290, só
  header); servidor resolve destino+custo pela posição. Noatum é hub (cidades pagam 700 p/ ir, voltam
  de graça). `DoTeleport` = `MSG_Action` (0x036C) com `Effect=1` + grid reconcilia visão.
- Tipos de pacote ficam em `tmserver/internal/protocol/types.go`. Codecs S→C com testes byte-exatos em
  `protocol/*_test.go` (CreateMob, ShopList, SendItem, UpdateEtc, CNFCharacterLogin, SELCHAR).

## 7. Próximas frentes (roadmap)

Já feito: login, chars, NPCs (nomes/aparência), lojas (buy/sell + SendItem), teleporte, persistência
(itens/gold/cidade). A fazer (escolher com o usuário):
- **Banco/Cargo** (NPC Guarda_Carga, Merchant=2; depósito 0x0388 / saque 0x0387).
- **NPCs de combinação/refino** (Odin/Lindy/Shany).
- **IA/movimento/combate de mob** (NPCs hoje são estáticos).
- Persistência de stats/skills mais completa; mais rotas de teleporte (campos/dungeons já parciais).

Sempre: ler `docs/migration/` antes de mexer em wire/format/gameplay; comentar o **porquê** (paridade);
testes table-driven `-race`; o snapshot/golden de protocolo são os testes críticos de paridade.
