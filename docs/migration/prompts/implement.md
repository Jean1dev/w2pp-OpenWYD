# Prompt Mestre — Execução da Migração (w2pp-OpenWYD)

> **Como usar:** este é um prompt para um agente de codificação. Copie a seção "PROMPT" abaixo. Ele
> assume que a documentação de engenharia reversa em `docs/migration/` já está COMPLETA (gerada via
> `DOCUMENTATION-PROMPT.md`) e que a stack/arquitetura já foi decidida (Fase 9 → **Go**, Linux/Docker,
> microservices). **Rode FASE A FASE — um agente/lote por vez.** Não tente o big-bang inteiro numa
> sessão: o escopo é grande (codec binário + 3 serviços + 58 handlers). Cada fase só começa com a
> anterior verde (compila, testes `-race` passam, golden cases da fase verdes).
>
> Complementa: depois de cada fase, rode `prompts/validate.md` para auditar conformidade.

## Contexto da migração (decisões já tomadas)

- **Stack:** **Go 1.26** (decisão da Fase 9 — `migration-plan.md §3`). Stack do time; melhor
  velocidade até a paridade.
- **Alvo:** **Linux/Docker**, arquitetura de **microservices**: `tmServer` (jogo, stateful, CPSock↔
  cliente) / `dbServer` (gRPC, PostgreSQL) / `binServer` (billing, gRPC, novo). Topologia em
  `migration-plan.md §3.5`.
- **Cliente:** MANTIDO (`WYD.exe` 7662 + `ClientPatch_v7662.dll`). ⇒ A borda cliente↔`tmServer` é
  **byte-for-byte** no protocolo CPSock; os links internos (`tm↔db`, `tm↔bin`) são modernos
  (gRPC+mTLS).
- **Estratégia:** REESCRITA TOTAL (big-bang). ⇒ É obrigatório provar **paridade comportamental** com
  o servidor atual via os golden cases da Fase 8 (`parity-tests.md`).
- **Leitura obrigatória antes de codar:** `development-guidelines/Go-development-guidelines.md`
  (layout, Docker, concorrência, testes, DB, segurança, checklist) e os artefatos de
  `docs/migration/` relevantes à fase.

---

## PROMPT

````
Você é um engenheiro de software sênior em Go implementando a REESCRITA TOTAL (big-bang) de um
emulador de servidor de MMORPG (WYD "With Your Destiny"), a partir de uma documentação de engenharia
reversa já pronta em docs/migration/. O servidor original é C++/Win32 (Source/Code/). O cliente
WYD.exe 7662 é MANTIDO sem alterações, então a borda cliente↔tmServer DEVE ser 100% compatível no
protocolo de fio (CPSock). A stack-alvo é Go 1.26, Linux/Docker, microservices (tmServer/dbServer/
binServer) — decisões da Fase 9.

OBJETIVO: produzir CÓDIGO Go real, testado e idiomático, fase a fase, com paridade comportamental
comprovada pelos golden cases da Fase 8.

═══════════════════════════════════════════════════════════
LEITURA OBRIGATÓRIA (antes de escrever qualquer código)
═══════════════════════════════════════════════════════════
- development-guidelines/Go-development-guidelines.md — É A LEI DO CÓDIGO. Em especial:
  §3 layout (cmd/ + internal/), §4 Docker, §5 nomenclatura, §6.3 layout binário por offset
  explícito, §7-8 funções/erros, §9 concorrência (1 goroutine dona + channels), §11 testes,
  §18 segurança, §22 banco (pgx), §25 checklist de pré-commit.
- docs/migration/README.md — índice das 10 fases, status e achados de maior impacto.
- docs/migration/migration-plan.md — §3.5 topologia, §4 SEQUÊNCIA (a ordem desta execução),
  §5 riscos, §6 Definition of Done.
- docs/migration/parity-tests.md (Fase 8) — formato de golden case, harness de replay, paridade
  EXATA de RNG via LCG do MSVC. É a sua rede de segurança.
- Por fase, o artefato-fonte correspondente (protocol-spec / data-formats / domain-model /
  game-rules / handlers/).

═══════════════════════════════════════════════════════════
REGRAS GERAIS (válidas para TODAS as fases)
═══════════════════════════════════════════════════════════
1. EVIDÊNCIA. Toda fórmula, offset, constante mágica ou regra de jogo deve rastrear até a fonte:
   cite docs/migration/<arquivo>.md §<seção> e, quando precisar do original, Source/Code/.../
   arquivo.cpp:linha. NÃO invente offsets, tamanhos, taxas ou fórmulas.
2. UNVERIFIED. Para os itens marcados UNVERIFIED no README (layout de _AUTH_GAME, coef. Dex/Str por
   classe×arma, largura de time_t, semântica do AttributeMap, NPCs de quest longos): NÃO chute.
   Marque `// UNVERIFIED: <o que falta> (docs/migration/...)` e crie um teste pendente (t.Skip com
   motivo) para fechar por captura/build depois.
3. SEGUE AS GUIDELINES. gofmt/goimports sempre; go vet + golangci-lint run limpos; erros com
   contexto (fmt.Errorf("...: %w")); nada de `_ = err` sem justificativa; nomes idiomáticos
   (MixedCaps, sem ALL_CAPS, sem stutter).
4. LAYOUT BINÁRIO POR OFFSET EXPLÍCITO (NF2/NF8 — risco crítico). Use encoding/binary, little-endian,
   campo a campo. NÃO confie em layout/padding de struct Go nem em unsafe. DOIS regimes:
   - Protocolo (MSG_*): equivalente a pack(1) — testar byte-a-byte vs captura (Fase 8 §3).
   - Save (STRUCT_ACCOUNTFILE etc.): ALINHAMENTO NATURAL (NÃO pack(1)), emulando MSVC-x86 mesmo
     rodando em Linux x64. Travar com golden tests usando os sizeof documentados como constantes:
     MOB=816, MOBEXTRA=552, QUEST=56, ACCOUNTFILE=7952 (Fase 2 §0.1).
5. CONCORRÊNCIA = PARIDADE. tmServer roda como UMA goroutine dona do estado de mundo (sem locks no
   estado) alimentada por channels; goroutines de I/O por conexão. NUNCA mute estado de mundo fora
   do game-loop (mata dup de item — risco da Fase 9). Propague context.Context; rode tudo com -race.
6. SEGURANÇA (dívidas obrigatórias da migração, Fase 7/9 §5): hash de senha/PIN (argon2id/bcrypt) na
   IMPORTAÇÃO — nunca persistir em claro; mTLS nos links gRPC internos; validar TODO input do cliente
   (bounds de slot/grid/tamanho — o cliente é não-confiável); checksum CORRETO no envio. Sem secrets
   hardcoded (vira env/secret).
7. CRITÉRIO DE PRONTO POR FASE (não avance sem isto):
   - `go build ./...` compila.
   - `go test -race ./...` verde.
   - Os golden cases da fase (Fase 8) verdes.
   - Checklist de pré-commit das guidelines §25 cumprido (fmt, vet, lint, govulncheck, cobertura
     >=70% no código crítico, GoDoc no exportado).
   - Atualize docs/migration/prompts/PROGRESS.md marcando a fase (TODO/EM ANDAMENTO/COMPLETO) com
     o que ficou UNVERIFIED.

═══════════════════════════════════════════════════════════
FASES (siga a SEQUÊNCIA do migration-plan §4 — uma por vez)
═══════════════════════════════════════════════════════════

──────────────────────────────────────────
FASE 1 — CODEC CPSOCK + PROTOCOLO   → tmserver/internal/protocol
──────────────────────────────────────────
Fonte: docs/migration/protocol-spec.md (+ CPSock.h/.cpp, Basedef.h).
- HEADER de 12 bytes (Size, KeyWord, CheckSum, Type, ID, ClientTick): codec por offset explícito,
  little-endian.
- Transform por keyword-table (pKeyWord, 512 bytes) — reproduzir byte-a-byte (encode/decode).
- Checksum — fórmula exata; no ENVIO, gerar checksum correto (dívida: original é não-rejeitante).
- INITCODE / handshake e framing (delimitação no stream TCP, tamanho mín/máx; validar Size do
  header antes de alocar — Fase 1 §framing / guidelines §18.3).
- Catálogo de Types como constantes Go; structs MSG_* com codec gerado/explícito.
- ClientVersion = 7640 (Basedef.h:102), NÃO 7662.
CRITÉRIO: testes de transporte da Fase 8 §3 verdes contra captura real (header/transform/checksum/
framing/initcode).

──────────────────────────────────────────
FASE 2 — dbServer + CONVERSOR + PostgreSQL   → dbserver/
──────────────────────────────────────────
Fonte: docs/migration/data-formats.md (+ DBSrv/CFileDB.cpp, CReadFiles.cpp; exemplos em Release/).
- Serviço gRPC (contratos em api/) + schema PostgreSQL (migrations versionadas, pgx — guidelines §22).
- CONVERSOR one-shot dos arquivos de conta: detectar versão por TAMANHO (4294 legado / 7500–7600 /
  7952 atual); codec por offset explícito emulando MSVC-x86 (long=4, time_t=8). Travar com
  static_assert-equivalente (golden test de sizeof/offset — Fase 2 §0.1).
- Hash de senha/PIN na importação (NUNCA gravar claro). Nome de conta canônico lowercase.
CRITÉRIO: conversor importa 100% das contas de amostra sem perda; dump round-trip confere.

──────────────────────────────────────────
FASE 3 — GAME-LOOP DO tmServer   → tmserver/internal/world
──────────────────────────────────────────
Fonte: docs/migration/domain-model.md.
- Estado de mundo em memória, autoritativo: índice conn↔player↔mob (players e mobs compartilham
  STRUCT_MOB/pMob[], <1000 = player), grids (4096²/1024²), limites MAX_*.
- 1 goroutine dona + channels (ch_in/ch_out); goroutines de I/O por conexão usando o codec da Fase 1.
- gRPC client para o dbServer. Graceful shutdown (signal → context → drena/salva — guidelines §9).
CRITÉRIO: um cliente headless conecta, faz handshake e troca pacotes básicos sem corromper estado;
teste de concorrência (N goroutines, -race) sem race.

──────────────────────────────────────────
FASE 4 — HANDLERS POR SUBSISTEMA (EM LOTES)   → tmserver/internal/handler
──────────────────────────────────────────
Fonte: docs/migration/handlers/*.md + game-rules.md (fórmulas). São 58 handlers — FAÇA 5–8 POR VEZ.
Ordem: login → criação/seleção de char → movimento → combate → itens (drop/get/use) → trade →
combine/refino → party/guild. Para cada handler implemente: gatilho/struct de entrada, pré-condições
e validações (bounds, versão, estado), efeitos colaterais (mutação SÓ via game-loop, persistência,
mensagens a outros), saídas S→C, casos de erro e checagens anti-cheat.
Regras de jogo (game-rules.md): EXP/party, drop (g_pDropRate, Basedef.cpp:222), refino/combine,
combate (BASE_GetDamage/SkillDamage + acerto/parry/reflect) — fórmulas como FUNÇÕES PURAS testáveis,
separadas do I/O (guidelines §19.2). Consolide as ~9 variantes de combine numa engine parametrizada
(§19.3). Para o RNG: reimplementar o LCG do MSVC + a ordem de chamadas (Fase 8 §4.0) com seed
injetável.
CRITÉRIO POR LOTE: golden cases do subsistema (Fase 8 §2.1–§2.7) verdes.

──────────────────────────────────────────
FASE 5 — CONTEÚDO (LOADERS)   → loaders nos serviços apropriados
──────────────────────────────────────────
Fonte: data-formats.md + game-rules.md. Loaders de ItemList, SkillData, NPCGener, mapas (HeightMap/
AttributeMap), rates (Rates.txt/gameconfig). Validar contra exemplos reais de Release/.

──────────────────────────────────────────
FASE 6 — WAR/CASTLE + binServer   → tmserver/internal/world (war) + binserver/
──────────────────────────────────────────
Fonte: flows.md + config-ops.md. War/Castle (GTorre, Zakum/RVR). binServer DESENHADO DO ZERO (gRPC),
com API e política próprias; billing só após CAPTURAR o _AUTH_GAME (UNVERIFIED — Fase 6 §9). Pode ir
em corte faseado pós-v1 se isolado.

──────────────────────────────────────────
FASE 7 — HARDENING   → todos os serviços
──────────────────────────────────────────
Fonte: migration-plan §5. Fechar dívidas: senhas hasheadas (confirmar), checksum rejeitante no
envio, mTLS nos links internos, planejar sessão/cifra real pós-cutover (sem quebrar o cliente 7662),
rate limiting por conexão, govulncheck limpo.

═══════════════════════════════════════════════════════════
ENTREGÁVEL POR FASE
═══════════════════════════════════════════════════════════
- Módulos Go reais no layout das guidelines §3 (cmd/<bin>/main.go faz só wiring; lógica em internal/).
- Testes unitários + table-driven + golden cases da fase; -race verde.
- Dockerfile/docker-compose/Makefile conforme guidelines §4 quando a fase introduz um serviço novo.
- PROGRESS.md atualizado (status da fase + UNVERIFIED pendentes).
- NÃO avance de fase sem cumprir o "CRITÉRIO DE PRONTO POR FASE" (regra geral 7).
````

---

## Notas de uso

- **Rode fase a fase.** A Fase 1 (codec) destrava todo o resto; sem compat de fio o cliente nem
  conecta. As Fases 4 e 8 (handlers + golden cases) são o coração da paridade.
- **Fase 4 em lotes.** São 58 handlers — peça 5–8 por vez para o agente não estourar contexto, na
  ordem login→char→movimento→combate→itens→trade→combine→party/guild.
- **`parity-tests.md` é a rede de segurança.** Capture golden cases do servidor atual (proxy TCP
  entre `WYD.exe` e o `TMSrv.exe` original, ou o `Wyd2Client` C#) ANTES de implementar o subsistema.
- **Depois de cada fase, rode `prompts/validate.md`** para auditar conformidade antes de seguir.
- **Mantenha o estilo "evidência + file:line + UNVERIFIED"** — é o que garante paridade sem o
  código original aberto ao lado.
