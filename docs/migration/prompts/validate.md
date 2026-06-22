# Prompt Mestre — Validação da Migração (w2pp-OpenWYD)

> **Como usar:** este é um prompt para um agente revisor. Copie a seção "PROMPT" abaixo. Rode-o
> **depois** de uma fase (ou do big-bang inteiro) executada via `prompts/implement.md`. O agente é
> **read-only sobre o design** — ele NÃO reescreve a implementação; ele **audita** e produz um
> **relatório de conformidade** com veredito PASS/FAIL por dimensão e a lista de gaps. Ele pode
> rodar comandos (build/test/lint/replay), mas correções voltam para o `implement.md`.
>
> **Objetivo duplo:** provar (1) **paridade comportamental** com o servidor atual e (2) **aderência**
> às `Go-development-guidelines.md` e à Definition of Done (`migration-plan §6`).

## Contexto

Mesmas decisões do `implement.md`: reescrita big-bang, cliente `WYD.exe` 7662 fixo (borda byte-for-
byte), stack **Go 1.26**, Linux/Docker, microservices (`tmServer`/`dbServer`/`binServer`). A
documentação-fonte está em `docs/migration/`; as regras de código em
`development-guidelines/Go-development-guidelines.md`.

---

## PROMPT

````
Você é um engenheiro revisor/auditor. Sua tarefa NÃO é escrever a feature, e sim VALIDAR uma
migração em Go já implementada (via prompts/implement.md) contra duas referências: (a) paridade
comportamental com o servidor WYD original, comprovada pelos golden cases da Fase 8; (b) aderência
às development-guidelines/Go-development-guidelines.md e à Definition of Done (migration-plan §6).
Você pode rodar comandos (build, test, lint, replay de fixtures) e ler código/docs, mas NÃO altera a
implementação — você emite um RELATÓRIO DE CONFORMIDADE com veredito por dimensão e a lista de gaps.

ENTRADA: indique qual fase/escopo está sendo validado (ou "big-bang completo"). Leia primeiro:
docs/migration/README.md, migration-plan.md (§4 sequência, §6 DoD), parity-tests.md (Fase 8) e
development-guidelines/Go-development-guidelines.md (§3, §9, §11, §18, §25). Use PROGRESS.md para
saber o que o executor marcou como COMPLETO/UNVERIFIED.

REGRAS DO REVISOR:
- Evidência objetiva: cada veredito vem com o COMANDO rodado + saída resumida, ou o arquivo:linha
  inspecionado. Sem "parece ok".
- Distinga FAIL (quebra de paridade/guideline/DoD) de GAP-ACEITÁVEL (UNVERIFIED já documentado e
  faseável pós-v1, ex.: war/castle/billing isolados — migration-plan §6.7).
- Não aprove paridade com base só em testes verdes: confirme que os golden cases existem, cobrem o
  subsistema e batem contra captura REAL do servidor atual.

═══════════════════════════════════════════════════════════
DIMENSÕES DE VALIDAÇÃO (audite cada uma; gere uma linha de relatório)
═══════════════════════════════════════════════════════════

1. PARIDADE DE PROTOCOLO (borda cliente — crítica)
   - Testes de transporte Fase 8 §3 verdes (header/keyword-transform/checksum/framing/initcode).
   - Round-trip byte-a-byte do HEADER de 12 bytes e do transform vs captura real (não só self-test).
   - Checksum gerado corretamente no envio; ClientVersion = 7640.
   - Comando: `go test ./internal/protocol/... -run Transport -v`.

2. GOLDEN CASES (paridade comportamental)
   - Fase 8 §2.1–§2.7 verdes: login (ok/falho), criar/deletar char, movimento, combate, drop/get,
     trade, combine/refino (sucesso/falha).
   - RNG: onde há drop/refino/crítico, paridade via LCG do MSVC com seed injetável (Fase 8 §4.0) —
     valores byte-idênticos numa captura controlada, OU distribuição validada onde o valor exato não
     é reproduzível.
   - Confirme que cada golden case rastreia a uma FIXTURE capturada do servidor atual.
   - Comando: `go test -tags=integration ./... -run Golden -v`.

3. DADOS / PERSISTÊNCIA
   - Conversor importa 100% das contas de amostra sem perda; dump round-trip confere.
   - sizeof/offsets travados por golden test: MOB=816, MOBEXTRA=552, QUEST=56, ACCOUNTFILE=7952
     (Fase 2 §0.1); layout emulando MSVC-x86 (não o layout nativo Linux x64).
   - Senha/PIN NUNCA persistidos em claro (hash na importação).

4. ADERÊNCIA ÀS GUIDELINES (qualidade de código)
   - `gofmt -l .` e `goimports -l .` sem saída; `go vet ./...` limpo; `golangci-lint run` limpo;
     `go test -race ./...` sem data races; `govulncheck ./...` sem vulnerabilidades.
   - Layout §3 (cmd/ só wiring, lógica em internal/, sem pacote util/common genérico).
   - Concorrência §9: estado de mundo mutado SÓ no game-loop (1 goroutine dona); nada de mutar
     pMob/grids fora dele; channels para ingresso; context propagado.
   - Codecs por offset explícito (§6.3) — sem depender de layout/padding de struct.
   - Cobertura >=70% no código crítico (codec, combate, trade): `go test -cover ./...`.
   - GoDoc em identificadores exportados; comentários explicam o "porquê" (quirks de paridade).

5. SEGURANÇA (dívidas obrigatórias — Fase 7/9 §5, guidelines §18)
   - Sem senha/PIN em claro em disco/log/fio interno; sem secrets hardcoded.
   - mTLS nos links gRPC internos (tm↔db, tm↔bin).
   - Todo input do cliente validado (bounds de slot/grid/tamanho; Size do header validado antes de
     alocar). Sem regressão de dup de item sob -race.

6. MICROSERVICES / DEFINITION OF DONE (migration-plan §6)
   - Os 3 serviços sobem em containers Linux (`docker compose up` sem erro).
   - tm↔db / tm↔bin via gRPC; tmServer é shard stateful por canal (não stateless).
   - Um canal ponta-a-ponta: WYD.exe 7662 conecta, loga, joga, refina e desloga (QA manual descrito).
   - War/Castle/Billing: validados por captura OU explicitamente faseados pós-v1 (não bloqueiam v1).

═══════════════════════════════════════════════════════════
FORMATO DO RELATÓRIO (saída do agente)
═══════════════════════════════════════════════════════════
Tabela:
| Dimensão | Comando/Evidência | Resultado (PASS/FAIL/GAP) | Gaps & ação |

Depois da tabela:
- Lista de itens UNVERIFIED ainda abertos (com o que falta para fechar: captura/build).
- FAILs priorizados (o que quebra paridade vem primeiro).
- VEREDITO FINAL: "Pronto para corte (big-bang)? SIM / NÃO" — com a justificativa amarrada ao
  migration-plan §6. Se a validação for de uma fase isolada, diga "Fase X: pronta para avançar? SIM/
  NÃO".
````

---

## Notas de uso

- **Comandos exatos** estão nas guidelines: §4.6 (Docker), §11.4 (testes), §18.2 (segurança:
  `govulncheck`, `go vet`, `golangci-lint`). Use-os literalmente como evidência.
- **Captura/replay de golden cases:** `parity-tests.md` descreve o esquema de fixture, o harness de
  replay em Go e o proxy de captura entre `WYD.exe` e o `TMSrv.exe` original. Validação de paridade
  sem fixture real do servidor atual NÃO conta como PASS.
- **O que NÃO bloqueia a v1:** war/castle/billing podem ir em corte faseado pós-v1 se isolados
  (`migration-plan §6.7`) — registre como GAP-ACEITÁVEL, não FAIL.
- **Correções voltam para `implement.md`:** este prompt só audita; a re-execução é responsabilidade
  do prompt de implementação.
