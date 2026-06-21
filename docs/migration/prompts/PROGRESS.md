# PROGRESS — Execução da migração (Go big-bang)

> Tracker da reescrita fase a fase (`migration-plan.md §4`, `implement.md`). Cada fase só avança com
> a anterior verde: `go build ./...`, `go test -race ./...`, golden cases da fase, checklist §25.
> Status: TODO / EM ANDAMENTO / COMPLETO.

Módulo Go: `github.com/jeanluca/w2pp-openwyd` (monorepo, single module, **`go 1.25`** —
bump exigido por `golang.org/x/crypto`; toolchain 1.25.x auto-baixado / `golang:1.26-alpine` no
Docker, alvo canônico). Layout por serviço: `tmserver/`, `dbserver/`, `binserver/` (cada um com
`cmd/<svc>` + `internal/`), `api/` compartilhado. **Reorg na Fase 2:** Fase 1 saiu de `cmd/`+
`internal/` na raiz para `tmserver/...` (regra de `internal/` do Go).

| Fase | Escopo | Status | UNVERIFIED / pendências |
|-----:|--------|--------|--------------------------|
| — | Scaffolding (go.mod, Makefile, Docker, compose, .golangci.yml, subtrees por serviço, api/) | **COMPLETO** | — |
| 1 | Codec CPSock + protocolo (`tmserver/internal/protocol`) | **COMPLETO** (subset runnable) | vetores de transporte reais; `_AUTH_GAME`; colisões de base §3.1 |
| 2 | dbServer: conversor + savefmt + domain + schema/pgx | **COMPLETO** (núcleo) / gRPC server → Fase 4 | layouts legados 4294 / 7500–7600; largura `time_t`; internos de `STRUCT_MOBEXTRA` |
| 3 | Game-loop do tmServer (`tmserver/internal/world`) | **COMPLETO** | handlers reais (Fase 4); gRPC client p/ dbServer (adapter da port); sentinel do grid |
| 3 | Game-loop do tmServer | TODO | — |
| 4 | Handlers por subsistema (lotes) | TODO | — |
| 5 | Conteúdo (loaders) | TODO | — |
| 6 | War/Castle + binServer | TODO | `_AUTH_GAME` (billing) |
| 7 | Hardening | TODO | — |

---

## Fase 1 — Codec CPSock + protocolo — COMPLETO

`internal/protocol/`:
- `keytable.go` — `pKeyWord[512]` verbatim (protocol-spec.md §4.4 / CPSock.cpp:29-46) + teste de
  comprimento/spot-bytes.
- `header.go` — `Header` (12 B, offset explícito LE, §1.1) + constantes de transporte (§4.5):
  `HeaderSize`, `InitCode`, `MaxMessageSize`, `AppVersion=7640`, `SkipCheckTick`, `MaxUser`.
- `transform.go` — keyword transform encode/decode byte-a-byte, aritmética `uint8` com wrap (§1.4).
- `checksum.go` — `CheckSum = Sum2 - Sum1` (§1.5); correto no envio.
- `framing.go` — `Framer`: gate INITCODE + framing por `Size` + validação de bounds antes de alocar
  (§1.2-1.3, guidelines §18.3); `ErrBadInitCode`/`ErrBadSize`.
- `types.go` — `Type` + flags de direção (§2) + catálogo acionável C→S/S→C (§3.1-3.2).
- `messages.go` — codecs explícitos: `MsgAccountLogin` (116), `MsgCreateCharacter` (36),
  `MsgDeleteCharacter` (44), `MsgCharacterLogin` (20), `MsgAction` (52) (§3.5); `AuthGameSize=196`.
- `codec.go` — `Encode`/`Decode` (iKeyWord injetável; transform cobre `[4:Size)` = Type/ID/Tick + corpo).

### Critério de pronto
- `go build ./...` ✅ · `go test -race ./...` ✅ · `go vet ./...` ✅ · `gofmt -l` limpo ✅.
- Cobertura `internal/protocol`: **85.9%** (>70% no código crítico).
- Subset runnable de `parity-tests.md §3` verde: header/transform/checksum/framing/initcode/oversize +
  end-to-end in-process (INITCODE→framing→decode→transform→checksum com `MSG_AccountLogin`).

### Achados / notas
- **Checksum valida só a chave/transform, não os dados:** `Sum2 - Sum1` reduz-se à soma dos offsets do
  transform → invariante a alterar bytes de dados. Confirma a natureza fraca/não-rejeitante (§1.5).
  Teste de mismatch corrompe o byte de checksum, não um byte de payload.
- **Transform cobre `[4:Size)`** — inclui Type/ID/ClientTick do header, não só o corpo; só
  Size/KeyWord/CheckSum (bytes 0-3) ficam em claro.

### UNVERIFIED / a fechar por captura
- **Vetores de transporte reais** (`test/fixtures/transport/*.json`, schema §7a): só há o
  `_schema_example.json` (hex vazio → `TestTransportVectors` faz Skip). Capturar do TMSrv vivo
  (proxy/Wyd2Client, parity-tests.md §5) e soltar no diretório torna o teste assertivo **sem mudar
  código**. Prova a paridade que o round-trip não prova.
- **`_AUTH_GAME` (196 B)** — layout interno UNVERIFIED (§4.3); `TestAuthGameLayout` faz Skip. Fase 6.
- **Colisões de base de `Type`** (`_MSG_Action2/3` §3.1) — confirmar struct real do cliente 7662 na
  Fase 5.

---

## Fase 2 — dbServer + conversor + PostgreSQL — COMPLETO (núcleo)

`dbserver/`:
- `internal/savefmt/` — codec por **offset explícito**, alinhamento natural MSVC-x86 (NÃO pack(1),
  data-formats.md §0.1) do **formato atual (7952 B)**: `STRUCT_ITEM`(8)/`SCORE`(48)/`AFFECT`(8)/
  `MOB`(816)/`QUEST`(56)/`MOBEXTRA`(552)/`ACCOUNTINFO`(216)/`ACCOUNTFILE`(7952). `Encode`+`Decode`.
  `DetectVersion` por tamanho (4294 / 7500–7600 / 7952).
- `internal/domain/` — modelo relacional alvo (data-formats.md §4).
- `internal/convert/` — `savefmt → domain` + **hash argon2id** de senha/PIN/blockpass (nunca claro) +
  nome canônico lowercase; `WalkAccounts` (runner one-shot).
- `internal/store/` — pgx: `Migrate` (migrations embarcadas) + `SaveAccount` transacional.
- `migrations/0001_init.{up,down}.sql` — schema account/character/item/affect.
- `cmd/dbserver convert -accounts <dir> [-dsn <pg>]` — runnable (dry-run / persiste).
- `api/db/v1/db.proto` — contrato gRPC do DBSrv (gerar com `make proto`).

### Critério de pronto
- `go build ./...` ✅ · `go test -race ./...` ✅ · `go vet ./...` (+ `-tags=integration`) ✅ ·
  `gofmt -l` limpo ✅.
- **static_assert equivalente** verde: sizeof (816/552/56/7952/216/48/8) + offsets âncora
  (Char@216, Cargo@3480, Coin@4504, affect@4572, mobExtra@5600) + offsets internos de `MOB`
  (Coin@28, Exp@32, BaseScore@44, Equip@140, Carry@268).
- **Round-trip** `Decode∘Encode == identidade` (DoD "dump round-trip confere"), via `AccountFile`
  sintético (não há amostra real no formato 7952).
- Conversor roda na amostra real: `account/A/antonio` detectado como **legacy-4294** e **pulado**
  (UNVERIFIED), sem chute.

### Achados / decisões
- **Checksum/alinhamento dois regimes**: save = alinhamento natural (este pacote); rede = pack(1)
  (Fase 1). Isolados.
- **`MOBEXTRA` como blob preservado**: a aritmética interna de §1.5 é auto-inconsistente (162→168
  rotulado "pad 4" mas são 6 B); então os 552 B são mantidos em `Raw` (round-trip exato) e só
  `ClassMaster`@0 / `Citizen`@1 são expostos. Resto UNVERIFIED.
- **Bump go 1.25**: `x/crypto` (argon2) exige Go ≥ 1.25 → go.mod subiu de 1.22 p/ 1.25 (toolchain
  auto-baixado; Docker 1.26 ≥ 1.25).
- **gRPC server adiado p/ Fase 3**: o contrato (`api/db/v1/db.proto`) está definido; o servidor e o
  client são escritos quando o tmServer (consumidor) for ligado (Fase 3). Codegen via `make proto`
  (requer protoc-gen-go/-grpc).

### UNVERIFIED / a fechar
- **Layouts legados (4294 / 7500–7600)**: não modelados (a amostra `antonio` mostra layout de
  `ACCOUNTINFO` diferente, space-padded). `TestRealAntonioSample` faz Skip. Reverter por build de
  referência antes de migrar contas legadas.
- **Largura de `time_t`**: premissa = 8 B (base de 552/56/7952). Confirmar no build de referência.
- **Internos de `STRUCT_MOBEXTRA`** (Fame/Soul/SecLearnedSkill/donate/quests): preservados crus.

### Testes que dependem de infra
- `internal/store/store_integration_test.go` (`//go:build integration`): requer PostgreSQL real;
  `W2PP_TEST_DSN=... go test -tags=integration ./dbserver/internal/store/`. Pulado por padrão.

---

## Fase 3 — Game-loop do tmServer — COMPLETO

`tmserver/internal/world/`:
- `world.go` — `World`: estado autoritativo (sessions[1000], entities[25000], grid), **1 goroutine
  dona** (`Run`), `New`, `send` (não bloqueia o loop; dropa cliente lento), shutdown graceful
  (drena/salva). Constantes MAX_* e máquinas de estado `Mode`/`EntityMode` (domain-model.md §3/§6).
- `session.go` — `Session` (CUser subset) + `Entity` (CMob subset, índice compartilhado <1000=player).
- `event.go` — eventos (connect/frame/disconnect) aplicados **só no loop**; goroutines de I/O por
  conexão (`readLoop` usa o `Framer`+`Decode` da Fase 1; `writeLoop` encoda+escreve); guardas do
  dispatcher (Ping no-op, SkipCheckTick dropado — protocol-spec §2); identidade de slot (anti-reuso).
- `server.go` — `Serve`/`acceptLoop` (listener TCP; fecha no `ctx`).
- `grid.go` — grid espacial denso (pMobGrid/pItemGrid), dim configurável; sentinel 0xFFFF (UNVERIFIED).
- `persistence.go` — **port `Persistence`** (o loop só depende da interface) + `NopPersistence`. O
  adapter gRPC real (api/db/v1) entra na Fase 4.
- `tmserver/cmd/tmserver` — wiring real: listen → `world.Serve` → shutdown por sinal.

### Modelo de concorrência (o coração da fase)
- **Toda mutação de estado ocorre na goroutine do `Run`** — sem locks, espelhando o reactor
  single-thread original (domain-model.md §5). Conexões só trocam mensagens com o loop por channels
  (`events` de entrada; `out` por sessão de saída). Mata dup de item por construção.
- `send` é não-bloqueante: fila `out` cheia ⇒ dropa a conexão (sem head-of-line blocking global).
- `emit` faz `select` em `events`/`done` ⇒ goroutines de I/O não travam no shutdown.

### Critério de pronto
- `go build ./...` ✅ · `go test -race ./...` ✅ · `go vet ./...` ✅ · `gofmt -l` limpo ✅.
- **Cliente headless** conecta (INITCODE), faz handshake e troca pacotes (C→loop→S) sem corromper
  estado (`TestHeadlessConnectAndExchange`); Ping/SkipCheckTick dropados.
- **Teste de concorrência** (64 conexões simultâneas, `-race`) sem race (`TestConcurrentConnections`).
- **Shutdown graceful** salva players in-world (`TestGracefulShutdownSavesPlayers`); binário smoke-OK
  (listen→accept→drain→exit 0).

### UNVERIFIED / pendências
- **Handlers reais**: o loop chama um `Handler` plugável; o default é no-op (Fase 4 instala o dispatch
  de `_MSG_*`).
- **gRPC client p/ dbServer**: a port `Persistence` está definida; o adapter sobre `api/db/v1` (e a
  geração via `make proto`) entra quando os handlers de login/char precisarem (Fase 4).
- **Sentinel do grid** (0xFFFF) e semântica de visibilidade/colisão: confirmar por captura (Fase 4).

### Pendências de tooling (checklist §25)
- `golangci-lint` / `goimports` / `govulncheck` NÃO instalados localmente (binário `go` local 1.22.2
  baixa toolchain 1.25.x via `GOTOOLCHAIN` por causa do go.mod). Rodar lint/vuln no container
  `golang:1.26-alpine` (`make lint`, `make vuln`) ou via `go install` antes do corte. `go vet` +
  `gofmt` (disponíveis) cobrem Fases 1–2. `make proto` (protoc + plugins) ainda não rodado.
