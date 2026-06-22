# Go Development Guidelines

> Diretrizes para construir os serviços do projeto (tmServer / dbServer / binServer) em **Go 1.26**,
> alvo **Linux/Docker** e arquitetura de **microservices** (ver `docs/migration/migration-plan.md`).
> Todos os exemplos usam **biblioteca-padrão**; as libs do Project Stack são referência.

## Project Stack

Libs definidas para este projeto (referência — os exemplos usam stdlib):

**Especificadas (decisões da Fase 9):**
- **Banco / Driver**: pgx v5 — driver e toolkit PostgreSQL para Go — https://github.com/jackc/pgx
- **RPC interno**: grpc-go v1.80 — implementação Go do gRPC (links tm↔db, tm↔bin, mTLS) — https://github.com/grpc/grpc-go
- **Testes**: testify v1.x — asserts e mocks — https://github.com/stretchr/testify

**Essenciais (auto-selecionadas — padrões da linguagem):**
- **Formatação**: gofmt + goimports — formatação canônica — https://pkg.go.dev/cmd/gofmt
- **Linting**: golangci-lint v2.12 (inclui staticcheck 0.7, govet, errcheck) — https://golangci-lint.run
- **Logging**: log/slog (stdlib, Go 1.21+) — logs estruturados — https://pkg.go.dev/log/slog
- **Build**: go build + make — https://pkg.go.dev/cmd/go

> Esta seção é só referência rápida. Todos os exemplos de código usam stdlib/recursos nativos;
> os princípios valem independentemente das libs escolhidas.

---

## 1. Core Principles

### 1.1 Filosofia e estilo
- **Formate sempre** com `gofmt`/`goimports` — formatação não é debate em Go.
- Siga o idioma: prefira o explícito ao "esperto"; "clear is better than clever".
- Erros são valores: trate-os, não os esconda.
- Rode `go vet` e `golangci-lint` em todo commit.
- Concorrência por **comunicação** (channels), não por memória compartilhada onde possível.

Config base de lint (`.golangci.yml`) — versione no repo:
```yaml
version: "2"                       # golangci-lint v2
linters:
  enable:
    - errcheck                     # erros não tratados
    - govet
    - staticcheck                  # inclui gosimple/stylecheck
    - ineffassign
    - unused
    - gosec                        # análise de segurança
run:
  timeout: 5m
```

### 1.2 Clareza acima de brevidade
- Nomes comunicam intenção; código auto-explicativo reduz comentários.
- Sem otimização prematura: meça antes (Seções 15-17).
- Zero-value útil: projete tipos que funcionem sem inicialização extra.
- A simplicidade é a meta — o `tmServer` roda como **1 goroutine dona do estado** (Fase 9 §3.5);
  resista a abstrações que escondam o fluxo.

## 2. Project Initialization

### 2.1 Criar novo módulo
```bash
mkdir tmserver && cd tmserver
go mod init github.com/openwyd/tmserver   # define o module path
go mod tidy                                # resolve/limpa dependências
go version                                 # confirmar Go 1.26.x
```

### 2.2 Gerenciar dependências
```bash
go get github.com/jackc/pgx/v5@latest      # adicionar
go get -u ./...                            # atualizar (com cuidado)
go mod tidy                                # remover não usadas + atualizar go.sum
go mod download                            # baixar para o cache
go mod verify                              # checar integridade do go.sum
```

## 3. Project Structure

Layout recomendado (segue o "Standard Go Project Layout"); um repo por serviço ou monorepo:

```
tmserver/
├── cmd/
│   └── tmserver/
│       └── main.go            # entrypoint: wiring, flags, shutdown
├── internal/                  # código privado (não importável por fora)
│   ├── protocol/              # codec CPSock (HEADER, keyword, checksum) — Fase 1
│   ├── world/                 # game-loop, estado, entidades (pMob/pUser)
│   ├── handler/               # handlers _MSG_* (Fase 5)
│   ├── persistence/           # cliente do dbServer (gRPC)
│   └── rng/                   # LCG do MSVC (paridade — Fase 8 §4.0)
├── pkg/                       # libs reutilizáveis (se houver) — opcional
├── api/                       # protobufs / contratos gRPC
├── configs/                   # arquivos de config (não secrets)
├── test/                      # fixtures, golden cases (Fase 8)
├── Dockerfile
├── docker-compose.yaml
├── Makefile
├── go.mod
└── go.sum
```

Regras: lógica privada em `internal/`; `cmd/<bin>` só faz "wiring"; um pacote = uma
responsabilidade coesa. Evite o pacote `util`/`common` genérico.

## 4. Container Development (Docker)

### 4.1 Filosofia
Containerizar garante ambiente idêntico entre devs e paridade dev/prod, sem deps locais. O alvo do
projeto é Linux/Docker (Fase 9), então Docker é padrão.

### 4.2 Arquivos
`Dockerfile` (dev), `docker-compose.yaml` (app + PostgreSQL), `.dockerignore`.

### 4.3 Dockerfile para desenvolvimento
```dockerfile
# imagem oficial Alpine, menor, versão fixada
FROM golang:1.26-alpine
WORKDIR /app
RUN apk add --no-cache git make           # apenas deps de runtime/ferramentas
COPY go.mod go.sum ./
RUN go mod download
COPY . .
CMD ["sleep", "infinity"]                  # mantém o container vivo p/ dev
```

### 4.4 Docker Compose
```yaml
services:
  tmserver:
    build: .
    volumes:
      - .:/app                 # hot reload de fonte
      - go-cache:/go/pkg/mod   # cache de módulos
    depends_on:
      db:
        condition: service_healthy
    ports: ["8281:8281"]       # porta do TMSrv (cliente)
  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_PASSWORD: dev
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      retries: 5
volumes:
  go-cache:
```

### 4.5 .dockerignore
```
.git
*.md
test/fixtures
bin/
*.log
```

### 4.6 Comandos essenciais
| Ação | Comando |
|------|---------|
| Subir ambiente | `docker compose up -d` |
| Ver logs | `docker compose logs -f tmserver` |
| Shell no container | `docker compose exec tmserver sh` |
| Rodar app | `docker compose exec tmserver go run ./cmd/tmserver` |
| Rodar testes | `docker compose exec tmserver go test ./...` |
| Derrubar | `docker compose down` |

### 4.7 Makefile
```makefile
.PHONY: build test lint run fmt
build:  ; go build -o bin/tmserver ./cmd/tmserver
test:   ; go test -race -cover ./...
lint:   ; golangci-lint run
fmt:    ; gofmt -w . && goimports -w .
run:    ; go run ./cmd/tmserver
```

### 4.8 Boas práticas
- Fixe a versão da imagem (`golang:1.26-alpine`), nunca `latest`.
- Em produção, use build multi-stage + imagem `scratch`/`distroless` (binário estático Go).
- Não instale libs da aplicação no Dockerfile de dev além das de runtime.

## 5. Naming Conventions

| Elemento | Convenção | Exemplo |
|----------|-----------|---------|
| Pacote | minúsculo, curto, sem `_`/maiúsculas | `protocol`, `world` |
| Tipo/Struct | `MixedCaps` (PascalCase) | `type UserSession struct` |
| Função/Método exportado | `MixedCaps` | `func DecodeHeader(...)` |
| Função/método privado | `mixedCaps` (camelCase) | `func applyDamage(...)` |
| Variável | `camelCase`, curta no escopo curto | `conn`, `mobID` |
| Constante | `MixedCaps` (não `ALL_CAPS`) | `MaxUser = 1000` |
| Interface | substantivo + sufixo `-er` quando 1 método | `Reader`, `Codec` |
| Arquivo | `snake_case.go` | `mob_killed.go` |

- Erros: variáveis `errX` (`errNotFound`), tipos `XError` (`SancError`).
- Não "stutter": em `package user`, `user.User` é ruim → `user.Account`.
- Getters sem `Get` (`u.Name()`, não `u.GetName()`).

```go
// BAD: stutter + nomes ruidosos + ALL_CAPS
package protocol
const MAX_PACKET_SIZE = 8192
type ProtocolPacket struct{ PacketData []byte }
func (p *ProtocolPacket) GetPacketData() []byte { return p.PacketData }

// GOOD: idiomático
package protocol
const MaxPacketSize = 8192
type Packet struct{ Data []byte }
func (p *Packet) Bytes() []byte { return p.Data }
```

## 6. Types and Type System

### 6.1 Declaração
```go
type ItemSlot uint8

type Item struct {
    Index   uint16            // sIndex (Fase 2 §1.5)
    Effects [3]Effect         // arrays de tamanho fixo = layout previsível
}

type MobState int
const (
    MobEmpty MobState = iota   // 0
    MobPeace
    MobAttack
)
```

### 6.2 Type safety
- Use tipos nomeados em vez de `int`/`string` "crus" para domínios (`type GuildID uint16`).
- Evite `interface{}`/`any` salvo em fronteiras genéricas; prefira **generics** (Go 1.18+):
```go
func Map[T, U any](s []T, f func(T) U) []U {
    out := make([]U, len(s))
    for i, v := range s { out[i] = f(v) }
    return out
}
```
- Habilite o zero-value: um `bytes.Buffer{}` já é usável.

### 6.3 Alocação e inicialização
```go
mobs := make([]Mob, 0, MaxUser)        // pré-aloca capacidade (evita regrow)
grid := make(map[Pos]*Mob, 1024)
item := &Item{Index: 1100}             // struct literal com campos nomeados
```
Para layout binário fiel (paridade de save, Fase 2), leia por **offset explícito**, não confie em
`unsafe`/alinhamento da linguagem.

## 7. Functions and Methods

### 7.1 Assinaturas
Erro é sempre o **último** retorno. `context.Context` é o **primeiro** parâmetro.
```go
// DecodePacket lê um frame CPSock desofuscado a partir de buf.
func DecodePacket(ctx context.Context, buf []byte) (Packet, error) {
    if len(buf) < headerSize {              // 12 bytes (Fase 1 §1.1)
        return Packet{}, fmt.Errorf("decode: buffer %d < header %d", len(buf), headerSize)
    }
    // ...
    return p, nil
}
```

### 7.2 Retornos e erros (Good vs Bad)
```go
// BAD: engole o erro, retorno ambíguo
func loadMob(id int) Mob {
    f, _ := os.Open(path(id))          // erro ignorado!
    var m Mob
    _ = binary.Read(f, le, &m)         // falha silenciosa
    return m                            // pode retornar lixo
}

// GOOD: erro explícito com contexto
func loadMob(id int) (Mob, error) {
    f, err := os.Open(path(id))
    if err != nil {
        return Mob{}, fmt.Errorf("loadMob %d: %w", id, err)
    }
    defer f.Close()
    var m Mob
    if err := binary.Read(f, le, &m); err != nil {
        return Mob{}, fmt.Errorf("loadMob %d decode: %w", id, err)
    }
    return m, nil
}
```

### 7.3 Boas práticas
- Responsabilidade única; ≤ 3-4 parâmetros (use struct de opções para mais).
- Sem efeitos colaterais ocultos; mutação de estado do mundo só dentro do game-loop (Fase 9).
- `defer` para cleanup (close, unlock) — logo após adquirir o recurso.
- Retorne early (Seção 19.1) para reduzir aninhamento.

## 8. Error Handling

### 8.1 Filosofia
Go usa **valores de erro** (sem exceptions). Crie com `errors.New`/`fmt.Errorf`, embrulhe com `%w`,
defina tipos para o domínio.
```go
var ErrItemDup = errors.New("item duplication detected")

type SlotError struct {
    Slot int
    Max  int
}
func (e *SlotError) Error() string {
    return fmt.Sprintf("slot %d out of range [0,%d)", e.Slot, e.Max)
}

// wrapping com contexto preservando a cadeia:
if err := persist(mob); err != nil {
    return fmt.Errorf("save mob %d: %w", mob.ID, err)
}
```

### 8.2 Convenções (Good vs Bad)
```go
// BAD: mensagem genérica, contexto perdido
if err != nil {
    return errors.New("error")
}

// GOOD: contexto + inspeção por errors.Is/As
if err != nil {
    return fmt.Errorf("combine item %d slot %d: %w", idx, slot, err)
}
// no chamador:
if errors.Is(err, ErrItemDup) { /* trata caso específico */ }
var se *SlotError
if errors.As(err, &se) { log.Warn("bad slot", "slot", se.Slot) }
```

### 8.3 Boas práticas
- **Nunca** ignore erro silenciosamente (`_ = err` exige comentário justificando).
- Adicione contexto útil (IDs, operação, valores) ao embrulhar.
- `panic` só para bugs de programação irrecuperáveis; nunca para fluxo normal.
- Logue o erro **uma vez**, na fronteira de I/O — não em cada camada (Seção 23).
- Em loops sobre conexões, um erro de uma conn não derruba o loop do servidor.

## 9. Concurrency and Parallelism

### 9.1 Modelo
Go usa **goroutines** (leves) + **channels**. Padrão do projeto (Fase 9 §3.5): I/O concorrente, mas
**estado de mundo serializado** numa única goroutine "dona".
```go
// 1 goroutine dona do estado; ingresso por channel (sem locks no estado)
type World struct{ in chan Command }

func (w *World) Loop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return                       // graceful shutdown
        case cmd := <-w.in:
            cmd.apply(w)                 // mutação do estado só aqui
        }
    }
}
```

### 9.2 Sincronização
- **Channels** para passar trabalho/posse de dados entre goroutines.
- `sync.Mutex`/`RWMutex` para proteger estado pequeno e local; `sync.WaitGroup` para esperar grupos.
- `sync.Once` para init único; `atomic` para contadores.
```go
var wg sync.WaitGroup
for _, conn := range conns {
    wg.Add(1)
    go func(c net.Conn) { defer wg.Done(); serve(c) }(c)
}
wg.Wait()
```

### 9.3 Boas práticas
- Quem cria a goroutine controla seu fim — propague `context.Context` para cancelamento/timeout.
- Sempre `select` com `ctx.Done()` em loops de longa duração.
- Feche channels do lado do **produtor**, nunca do consumidor.
- Rode os testes com `-race` (Seção 11.4) para detectar data races.

Graceful shutdown padrão de um serviço (sinal → cancela context → drena):
```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    srv := newServer()
    go srv.Run(ctx)                 // game-loop + listeners observam ctx.Done()

    <-ctx.Done()                    // bloqueia até SIGINT/SIGTERM
    slog.Info("shutting down")
    shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(shutCtx); err != nil {  // salva estado, fecha conns
        slog.Error("shutdown", "err", err)
    }
}
```

### 9.4 Armadilhas comuns
- Captura de variável de loop em goroutine (Go 1.22+ corrigiu o escopo por iteração; ainda assim seja
  explícito ao passar args).
- Goroutine leak: goroutine bloqueada num channel sem leitor — sempre tenha saída via `ctx`.
- Mutar `map` de múltiplas goroutines sem lock = panic/corrupção.

```go
// BAD: goroutine sem cancelamento → leak se ninguém ler `out`
func watch(out chan int) {
    go func() {
        for { out <- poll() }          // bloqueia para sempre se `out` parar de ser lido
    }()
}

// GOOD: vida atrelada ao context; sai limpo
func watch(ctx context.Context, out chan<- int) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case out <- poll():
            }
        }
    }()
}
```

## 10. Interfaces and Abstractions

### 10.1 Design
Interfaces pequenas e definidas **no consumidor**, não no produtor.
```go
// Codec é tudo que o game-loop precisa da camada de protocolo.
type Codec interface {
    Decode(b []byte) (Packet, error)
    Encode(p Packet) ([]byte, error)
}
```

### 10.2 Implementação
Go usa **duck typing estrutural**: um tipo satisfaz a interface implicitamente.
```go
type CPSock struct{ key [512]byte }
func (c *CPSock) Decode(b []byte) (Packet, error) { /* ... */ return Packet{}, nil }
func (c *CPSock) Encode(p Packet) ([]byte, error) { /* ... */ return nil, nil }

var _ Codec = (*CPSock)(nil)   // checagem em tempo de compilação que satisfaz a interface
```

### 10.3 Composição
- Componha interfaces pequenas (`io.ReadWriter = Reader + Writer`).
- Aceite interfaces, **retorne tipos concretos** ("accept interfaces, return structs").
- Evite interfaces "gordas" antecipadas — extraia quando houver ≥2 implementações reais.

## 11. Unit Tests

### 11.1 Estrutura
Arquivo `_test.go` no mesmo pacote; testes começam com `Test`. Use o pacote `testing` (stdlib).
```go
package protocol

import "testing"

func TestChecksum(t *testing.T) {
    got := checksum([]byte{0x01, 0x02, 0x03})
    want := byte(0x06)
    if got != want {
        t.Errorf("checksum = %d; want %d", got, want)   // t.Errorf não aborta; t.Fatalf aborta
    }
}
```

### 11.2 Table-driven tests
Padrão idiomático de Go para múltiplos casos:
```go
func TestDecodeHeader(t *testing.T) {
    tests := []struct {
        name    string
        in      []byte
        want    Header
        wantErr bool
    }{
        {"valid", []byte{0x0c, 0, 1, 2, 0x0d, 2, 0, 0, 0, 0, 0, 0}, Header{Size: 12}, false},
        {"too short", []byte{0x01}, Header{}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {           // subteste nomeado
            got, err := DecodeHeader(tt.in)
            if (err != nil) != tt.wantErr {
                t.Fatalf("err = %v; wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && got.Size != tt.want.Size {
                t.Errorf("size = %d; want %d", got.Size, tt.want.Size)
            }
        })
    }
}
```

### 11.3 Asserts
Stdlib usa `if`+`t.Errorf`. O projeto usa **testify** para reduzir verbosidade
(`assert.Equal(t, want, got)`), mas a stdlib basta e mantém zero deps nos exemplos.

```go
// BAD: mensagem inútil, não diz o que falhou
if got != want { t.Fatal("failed") }

// GOOD: mostra esperado vs obtido (debug rápido)
if got != want { t.Fatalf("decode(%q) = %d; want %d", in, got, want) }
```

### 11.4 Comandos
| Ação | Comando |
|------|---------|
| Todos os testes | `go test ./...` |
| Teste específico | `go test -run TestChecksum ./internal/protocol` |
| Com cobertura | `go test -cover ./...` |
| Relatório HTML de cobertura | `go test -coverprofile=c.out ./... && go tool cover -html=c.out` |
| Detector de race | `go test -race ./...` |
| Verboso | `go test -v ./...` |

## 12. Mocks and Testability

### 12.1 Estratégias
Mock manual via interface pequena (preferido) ou geração (`testify/mock`, `go.uber.org/mock`).
```go
type fakePersist struct{ saved []Mob }
func (f *fakePersist) Save(_ context.Context, m Mob) error {
    f.saved = append(f.saved, m); return nil
}
```

### 12.2 Injeção de dependência
DI manual via construtor (sem framework) — claro e testável.
```go
type Handler struct{ p Persister }                    // depende da interface
func NewHandler(p Persister) *Handler { return &Handler{p: p} }
// no teste: NewHandler(&fakePersist{})
```

### 12.3 Test doubles
- **Fake**: implementação simples em memória (ex.: `fakePersist`).
- **Stub**: retorna respostas fixas. **Spy**: registra chamadas. **Mock**: verifica expectativas.
- Para o servidor de jogo: um **cliente headless** (Fase 8) é o melhor double de integração.

## 13. Integration Tests

### 13.1 Estrutura
Separe com build tag no topo do arquivo:
```go
//go:build integration

package persistence_test
```

### 13.2 Execução seletiva
```bash
go test ./...                       # só unitários (tag integration não compila)
go test -tags=integration ./...     # inclui integração
```

### 13.3 Dependências reais
Use **testcontainers-go** para subir PostgreSQL real (ou o `docker-compose` de dev). Para o
`tmServer`, os **golden cases** (Fase 8) com o cliente capturado são o teste de integração-chave.

## 14. Load and Stress Tests

### 14.1 Ferramentas
Servidor TCP custom (não HTTP) → use um **cliente de carga próprio em Go** (goroutines abrindo N
conexões CPSock) ou ferramentas genéricas (`k6`, `vegeta`) para os endpoints gRPC internos.

### 14.2 Benchmarks de carga
Simule fan-out (Fase 9 NF1: até `MaxUser=1000`/canal):
```go
func BenchmarkServeConns(b *testing.B) {
    srv := newTestServer(b)
    b.SetParallelism(1000)               // ~1000 conexões simultâneas
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() { srv.handleOnce(loginPacket) }
    })
}
```

### 14.3 Testes de concorrência
Reproduza o cenário de **dup de item** (Fase 8 §2.5): N goroutines fazendo `get` no mesmo item;
asserir que só uma sucede. Rode com `-race`.

## 15. Profiling and Diagnostics

### 15.1 CPU e memória
`runtime/pprof` (stdlib) ou `net/http/pprof` (endpoint):
```go
import _ "net/http/pprof"            // registra /debug/pprof/*
go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
```
```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30   # CPU
go tool pprof http://localhost:6060/debug/pprof/heap                 # memória
```

### 15.2 Ferramentas de diagnóstico
- `go tool trace` — execução/escalonamento de goroutines.
- `GODEBUG=gctrace=1` — traços do GC (relevante para latência, Fase 9 NF4).
- `dlv` (Delve) — debugger.

### 15.3 Análise
```bash
go test -cpuprofile cpu.out -memprofile mem.out -bench .
go tool pprof -http=:8080 cpu.out    # UI web do profile
```
Foque o hot-path: codec CPSock e o game-loop. Meça **antes** de otimizar.

## 16. Benchmarks

### 16.1 Escrever benchmarks
Funções `Benchmark*` no `_test.go`; o `b.N` é ajustado pelo runtime.
```go
func BenchmarkDecode(b *testing.B) {
    buf := samplePacket()
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = DecodePacket(context.Background(), buf)
    }
}
```

### 16.2 Sub-benchmarks
```go
func BenchmarkEncode(b *testing.B) {
    for _, size := range []int{16, 256, 4096} {
        b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
            p := packetOfSize(size)
            for i := 0; i < b.N; i++ { _, _ = Encode(p) }
        })
    }
}
```

### 16.3 Execução e comparação
```bash
go test -bench=. -benchmem ./internal/protocol     # roda com alocações
go test -bench=BenchmarkDecode -count=10 > new.txt # múltiplas amostras
benchstat old.txt new.txt                          # comparar regressões
```

## 17. Optimization

### 17.1 Princípios
Meça primeiro (Seção 15), ataque o hot-path, documente trade-offs. Não troque clareza por
microganhos sem benchmark provando.

### 17.2 Otimizações comuns
- **Pré-alocar** slices/maps com capacidade conhecida (`make([]T, 0, n)`).
- **Reusar buffers** com `sync.Pool` no caminho de rede (encode/decode de pacotes).
- Evitar conversões `[]byte`↔`string` repetidas no hot-path.
```go
var bufPool = sync.Pool{New: func() any { return make([]byte, 0, 4096) }}
func encode(p Packet) []byte {
    b := bufPool.Get().([]byte)[:0]
    defer bufPool.Put(b)
    // ... preenche b ...
    return append([]byte(nil), b...)   // copia o resultado antes de devolver ao pool
}
```

### 17.3 Otimização de memória
- Structs: ordene campos do maior para o menor para reduzir padding.
- Prefira arrays de valor a slices de ponteiro quando o tamanho é fixo (cache locality).
- `b.ReportAllocs()` em benchmarks para flagrar alocações no hot-path.

```go
// BAD: campos desalinhados → padding desperdiçado (24 bytes)
type Mob struct {
    Alive bool   // 1 + 7 pad
    ID    int64  // 8
    Kind  bool   // 1 + 7 pad
}

// GOOD: do maior para o menor → 16 bytes
type Mob struct {
    ID    int64  // 8
    Alive bool   // 1
    Kind  bool   // 1 (+6 pad final)
}
```
> Use `go vet -fieldalignment` (ou `fieldalignment -fix`) para detectar/corrigir automaticamente.

### 17.4 Performance básica
- `strings.Builder` para concatenação em loop (não `+=`).
- Evite `defer` em loops muito quentes (custo por chamada) — só onde a clareza compensa.
- O GC do Go 1.26 (Green Tea, default) é eficiente; ajuste `GOGC`/`GOMEMLIMIT` se necessário.

## 18. Security

### 18.1 Práticas essenciais
- **Nunca** hardcode secrets — use env/secret manager (Fase 7: IPs/admin hoje em texto plano).
- Valide TODO input do cliente (o cliente é não-confiável — Fase 1/5): bounds de slot/grid, tamanhos.
- **mTLS** nos links internos gRPC (tm↔db, tm↔bin) — Fase 9.
- Hash de senha/PIN (argon2id/bcrypt) na migração — nunca persistir em claro (Fase 2 §1.3).
- Rate limiting por conexão; dependências atualizadas; menor privilégio.

### 18.2 Ferramentas
```bash
govulncheck ./...        # vulnerabilidades conhecidas (golang.org/x/vuln)
go vet ./...             # erros suspeitos
golangci-lint run        # inclui gosec via config
```

### 18.3 Segurança nas fronteiras
- Cópia defensiva ao cruzar a fronteira de rede (não confie no `Size` do header sem validar — Fase 1).
- Sanitize strings vindas do cliente (nomes, chat) antes de logar/persistir.
- Trate o checksum/keytable como **obfuscação**, não criptografia — planeje sessão real pós-cutover.

## 19. Code Patterns

### 19.1 Early return
```go
func handle(conn int, p Packet) error {
    if !valid(conn) { return ErrBadConn }     // sai cedo
    if p.Size < headerSize { return ErrShort }
    return process(conn, p)                    // caminho feliz sem aninhar
}
```

### 19.2 Separação de responsabilidades
Lógica pura separada de I/O: o cálculo de dano (Fase 4) é função pura testável; o `Send*`/socket é I/O.
```go
func computeDamage(att, def Score, mastery int) int { /* puro, testável */ }
func (h *Handler) onAttack(...) { dmg := computeDamage(...); h.send(...) }  // I/O à parte
```

### 19.3 DRY
Extraia duplicação real (ex.: as 9 variantes de combine → 1 engine parametrizada, Fase 5). Mas evite
abstração prematura: "a little copying is better than a little dependency".

### 19.4 Escopo de variável
Declare no menor escopo possível; use o `if` com init: `if err := f(); err != nil { ... }`.

## 20. Dependency Management

### 20.1 Princípios
- Stdlib primeiro (`database/sql`, `log/slog`, `net`, `encoding/binary`).
- Dependências bem mantidas e mínimas; versão explícita no `go.mod`.
- Avalie custo de cada dep nova (supply chain, manutenção).

### 20.2 Comandos
```bash
go list -m all                  # listar dependências
go mod why github.com/jackc/pgx/v5   # por que essa dep existe
govulncheck ./...               # checar vulnerabilidades
go mod tidy                     # limpar não usadas
go get -u=patch ./...           # só updates de patch (mais seguro)
```

## 21. Comments and Documentation

### 21.1 Comentários de código
Comente o **porquê**, não o **o quê**. Bom comentário explica decisão/quirk:
```go
// rand()%115 com achatamento >=100 -> -15: paridade exata com o cliente (Fase 8 §4.0).
roll := rng.Intn(115)
if roll >= 100 { roll -= 15 }
```

### 21.2 Documentação de API (GoDoc)
Doc comment começa com o nome do identificador e é frase completa:
```go
// DecodePacket lê um frame CPSock desofuscado de buf e retorna o pacote.
// Retorna erro se buf for menor que o header de 12 bytes.
func DecodePacket(ctx context.Context, buf []byte) (Packet, error) { /* ... */ }
```
Visualize com `go doc ./internal/protocol` ou `pkgsite` local.

### 21.3 Documentação de pacote
Um comentário antes do `package` (geralmente em `doc.go`):
```go
// Package protocol implementa o codec CPSock do WYD: HEADER de 12 bytes,
// transform por keyword-table e checksum (ver docs/migration/protocol-spec.md).
package protocol
```

## 22. Database

### 22.1 Abordagem
Go suporta SQL cru (`database/sql`), query builders e geradores de código (sqlc). Trade-offs:
- **`database/sql` (stdlib)**: portável, sem mágica; mais boilerplate.
- **Driver nativo (pgx)**: melhor performance e tipos PostgreSQL; é a escolha do projeto (Fase 9).
- **sqlc**: gera código tipado a partir de SQL — boa opção; mantém SQL explícito.

Os exemplos abaixo usam **`database/sql`** (stdlib) para serem agnósticos.

### 22.2 Conexão e driver
```go
import (
    "database/sql"
    "time"
    _ "github.com/jackc/pgx/v5/stdlib"   // driver registra-se via blank import
)

func OpenDB(ctx context.Context, dsn string) (*sql.DB, error) {
    db, err := sql.Open("pgx", dsn)       // não conecta ainda; só valida o DSN
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    db.SetMaxOpenConns(25)                // pool: limite de conexões
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    if err := db.PingContext(ctx); err != nil {  // valida a conexão de fato
        db.Close()
        return nil, fmt.Errorf("ping db: %w", err)
    }
    return db, nil
}
```

Query parametrizada (NUNCA concatene SQL):
```go
func AccountByName(ctx context.Context, db *sql.DB, name string) (Account, error) {
    const q = `SELECT id, name, pass_hash FROM account WHERE name = $1`
    var a Account
    err := db.QueryRowContext(ctx, q, name).Scan(&a.ID, &a.Name, &a.PassHash) // bind seguro
    if errors.Is(err, sql.ErrNoRows) {
        return Account{}, ErrAccountNotFound
    }
    if err != nil {
        return Account{}, fmt.Errorf("account %q: %w", name, err)
    }
    return a, nil
}
```

### 22.3 Migrations
Migrações são SQL versionado (`0001_init.up.sql`/`.down.sql`), aplicado por ferramenta
(`golang-migrate`, `goose`) na subida do serviço. Mantenha-as idempotentes e revisáveis. O conversor
one-shot dos saves legados (Fase 2) é um job separado, não uma migração de schema.

```sql
-- 0001_account.up.sql
CREATE TABLE account (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,          -- canônico lowercase (Fase 2 §1.1)
    pass_hash  TEXT NOT NULL,                 -- argon2id; nunca texto plano
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_character_account ON character(account_id);
```
```bash
migrate -path ./migrations -database "$DSN" up      # aplicar
migrate -path ./migrations -database "$DSN" down 1  # reverter última
```

### 22.4 Boas práticas
- **Sempre** prepared/parametrizado (`$1`, `?`) — nunca string-concat (anti SQL injection).
- Índices para queries frequentes (ex.: `account(name)`, `character(account_id)`).
- Connection pooling (configurado em `OpenDB`); feche `rows` com `defer rows.Close()`.
- Transações explícitas com `defer tx.Rollback()` (no-op após `Commit`):
```go
tx, err := db.BeginTx(ctx, nil)
if err != nil { return err }
defer tx.Rollback()
if _, err := tx.ExecContext(ctx, q, args...); err != nil { return err }
return tx.Commit()
```
- Trate timeouts via `context` em toda chamada (`...Context`).

```go
// BAD: string-concat → SQL injection + sem context
q := "SELECT id FROM account WHERE name = '" + name + "'"
row := db.QueryRow(q)

// GOOD: parametrizado + context com timeout
ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
defer cancel()
row := db.QueryRowContext(ctx, "SELECT id FROM account WHERE name = $1", name)
```

## 23. Logs and Observability

### 23.1 Níveis de log
`DEBUG` (diagnóstico), `INFO` (eventos normais), `WARN` (anomalia recuperável), `ERROR` (falha de
operação), e nível fatal só no boot (`os.Exit`). Em produção, default `INFO`.

### 23.2 Logs estruturados
Use `log/slog` (stdlib): saída key-value/JSON, com contexto.
```go
import "log/slog"

func setupLogger() *slog.Logger {
    h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,             // nível configurável (env)
    })
    logger := slog.New(h)
    slog.SetDefault(logger)                // default global opcional
    return logger
}
```

### 23.3 Implementação de logging
Logs com campos de contexto (correlação por conexão/conta):
```go
log := slog.With("service", "tmserver", "conn", conn)
log.Info("account login", "account", name, "version", clientVersion)

if err != nil {
    log.Error("combine failed", "slot", slot, "err", err)  // logue na fronteira de I/O
}
// logger por requisição/sessão carregando IDs:
reqLog := slog.With("conn", conn, "account", acc.Name)
reqLog.Debug("packet", "type", fmt.Sprintf("0x%04X", p.Type), "size", p.Size)
```
Regras: não logue secrets/senha; logue o erro **uma vez** (Seção 8.3); mensagens em inglês, estáveis.

```go
// BAD: não estruturado, interpola tudo numa string, vaza senha
log.Printf("login %s pass=%s from %s ok", name, pass, ip)

// GOOD: estruturado, campos tipados, sem secret
slog.Info("login ok", "account", name, "ip", ip, "version", clientVersion)
```

### 23.4 Métricas e observabilidade
- Instrumente I/O e fronteiras: latência por handler, taxa de erro, conexões ativas, throughput.
- Exponha endpoints `/healthz` (liveness), `/readyz` (readiness) e `/metrics` (Prometheus) no serviço.
- Mantenha **cardinalidade de labels** baixa (não use IDs de jogador como label de métrica).
- `context` propaga trace/correlação entre tm↔db↔bin (gRPC interceptors).

## 24. Golden Rules

1. **Simplicidade**: clear is better than clever; o game-loop é single-thread por clareza e paridade.
2. **Erros explícitos**: trate todo erro; embrulhe com contexto; nunca engula.
3. **Testes**: golden cases (Fase 8) + `-race` + cobertura no código crítico.
4. **Documentação**: GoDoc em tudo exportado; comente o "porquê" (quirks de paridade).
5. **Performance medida**: `pprof`/benchmarks antes de otimizar; nunca por palpite.

## 25. Pre-Commit Checklist

### Código
- [ ] `gofmt`/`goimports` aplicado
- [ ] `go vet` e `golangci-lint run` sem erros críticos
- [ ] `go build ./...` compila sem erros

### Testes
- [ ] `go test ./...` passa
- [ ] `go test -race ./...` sem data races
- [ ] Cobertura >= 70% no código crítico (codec, combate, trade)
- [ ] Testes de integração (`-tags=integration`) executados se aplicável
- [ ] Benchmarks validados se houve mudança no hot-path

### Qualidade
- [ ] Erros tratados explicitamente (sem `_ = err` injustificado)
- [ ] Recursos com cleanup (`defer Close()`/`Unlock()`)
- [ ] Sem secrets hardcoded
- [ ] `govulncheck ./...` sem vulnerabilidades

### Documentação
- [ ] Funções/tipos exportados documentados (GoDoc)
- [ ] README atualizado
- [ ] Comentários explicam o "porquê"

### Docker
- [ ] `Dockerfile` válido (imagem fixada)
- [ ] `docker compose up` sobe app + db sem erros

## 26. References

### Documentação oficial
- The Go Programming Language — https://go.dev/doc/
- Effective Go — https://go.dev/doc/effective_go
- Go Code Review Comments — https://go.dev/wiki/CodeReviewComments
- Go 1.26 Release Notes — https://go.dev/doc/go1.26
- Google Go Style Guide — https://google.github.io/styleguide/go/

### Ferramentas essenciais
- go command / módulos — https://pkg.go.dev/cmd/go
- gofmt / goimports — https://pkg.go.dev/cmd/gofmt
- golangci-lint (v2.12) — https://golangci-lint.run
- staticcheck (0.7) — https://staticcheck.dev
- govulncheck — https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck

### Testes e performance
- testing (stdlib) — https://pkg.go.dev/testing
- testify — https://github.com/stretchr/testify
- pprof — https://pkg.go.dev/runtime/pprof
- benchstat — https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
- testcontainers-go — https://golang.testcontainers.org

### Estilo da comunidade
- Uber Go Style Guide — https://github.com/uber-go/guide
- Standard Go Project Layout — https://github.com/golang-standards/project-layout

### Stack do projeto
- pgx (v5) — https://github.com/jackc/pgx
- grpc-go (v1.80) — https://github.com/grpc/grpc-go
- log/slog — https://pkg.go.dev/log/slog
