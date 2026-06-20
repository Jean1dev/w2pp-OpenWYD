# PROGRESS — Execução da migração (Go big-bang)

> Tracker da reescrita fase a fase (`migration-plan.md §4`, `implement.md`). Cada fase só avança com
> a anterior verde: `go build ./...`, `go test -race ./...`, golden cases da fase, checklist §25.
> Status: TODO / EM ANDAMENTO / COMPLETO.

Módulo Go: `github.com/jeanluca/w2pp-openwyd` (monorepo, single module, `go 1.22` local /
`golang:1.26-alpine` no Docker — alvo canônico 1.26).

| Fase | Escopo | Status | UNVERIFIED / pendências |
|-----:|--------|--------|--------------------------|
| — | Scaffolding (go.mod, Makefile, Docker, compose, .golangci.yml, cmd/{tm,db,bin}server, dirs) | **COMPLETO** | — |
| 1 | Codec CPSock + protocolo (`internal/protocol`) | **COMPLETO** (subset runnable) | vetores de transporte reais; `_AUTH_GAME`; colisões de base §3.1 |
| 2 | dbServer + conversor + PostgreSQL | TODO | — |
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

### Pendências de tooling (checklist §25)
- `golangci-lint` / `goimports` / `govulncheck` NÃO instalados localmente (Go local = 1.22.2). Rodar no
  container `golang:1.26-alpine` (`make lint`, `make vuln`) ou via `go install` antes do corte. `go vet`
  + `gofmt` (disponíveis) cobrem a Fase 1.
