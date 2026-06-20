# Fase 9 — Recomendação de Stack e Plano de Migração (w2pp-OpenWYD)

> Reescrita TOTAL (big-bang), cliente `WYD.exe` 7662 mantido. Baseado nas Fases 1–8.
>
> **Restrições do time (decididas):**
> - **Deploy:** Linux, em **containers Docker** (sem Windows em produção).
> - **Arquitetura:** **microservices** — `tmServer` (jogo), `dbServer` (persistência),
>   `binServer` (billing) — formalizando os 3 processos atuais (TMSrv/DBSrv/BISrv).
> - **Linguagem do time:** **Go** (fator de maior peso na velocidade até a paridade).

---

## 1. Requisitos não-funcionais (derivados do código + restrições do time)

| # | Requisito | Origem | Implicação |
|---|-----------|--------|------------|
| NF1 | Reator de I/O com alto fan-out (até `MAX_USER=1000` conexões/canal) | Fase 3 §6, CPSock | muitos sockets de longa duração por instância |
| NF2 | Parsing de **structs binárias** little-endian, layout MSVC x86 | Fase 1, Fase 2 | codec por offset explícito (não confiar em layout da linguagem) |
| NF3 | Ofuscação/checksum por byte reproduzível | Fase 1 §1.4-1.5 | aritmética `u8` com wrap; `pKeyWord` idêntica |
| NF4 | Baixa latência (combate cadência 800 ms, ticks) | Fase 5 Attack | pausas de GC sob controle (folgado nesta cadência) |
| NF5 | Estado de mundo **em memória**, autoritativo | Fase 3 | grandes arrays (`pMob[25000]`, grids 4096²); serviço **stateful** |
| NF6 | Persistência (arquivos→banco) | Fase 2 | migrar dumps de struct para schema relacional |
| NF7 | Modelo de concorrência que preserve semântica single-thread | Fase 3 §5 | gameplay serializado; I/O concorrente |
| NF8 | Compat de fio byte-for-byte (borda cliente) | Fase 1 | sem margem para erro de layout no link cliente↔tmServer |
| NF9 | **Rodar em Linux/Docker** | restrição do time | binário portável; imagens enxutas; sem deps Win32 |
| NF10 | **Decompor em microservices** (tm/db/bin) | restrição do time | serviços deployáveis e observáveis de forma independente |

---

## 2. Comparação de candidatos

> Reavaliada com as restrições do time (Linux/Docker, microservices) e a **familiaridade do time em
> Go** — que tem peso alto num big-bang (velocidade até paridade).

| Critério | Go | Rust/tokio | C#/.NET 8 | TypeScript/Node |
|----------|-----|-----------|-----------|-----------------|
| Mapear structs binárias | ★★★★ (codec por offset explícito; sem layout automático, mas **explícito = testável e seguro** p/ byte-exato) | ★★★★★ (`#[repr(C)]`, `bytemuck`) | ★★★★ (`StructLayout`/`MemoryMarshal`) | ★★ (`Buffer`/`DataView` manual) |
| Aritmética `u8` com wrap | ★★★★ (tipo `byte`/`uint8`) | ★★★★★ (`wrapping_*`) | ★★★★ | ★★★ |
| Latência / GC (nesta cadência 800 ms) | ★★★★ (GC concorrente, pausas curtas) | ★★★★★ (sem GC) | ★★★★ (GC server) | ★★ |
| Rede / fan-out | ★★★★★ (goroutines) | ★★★★★ (tokio) | ★★★★★ | ★★★★ |
| Concorrência p/ estado do mundo | ★★★★★ (**1 goroutine dona + channels** = preserva single-thread, NF7) | ★★★ (borrow checker força actor cedo) | ★★★★ | ★★★★ (single-thread natural) |
| **Linux / Docker / microservices** | ★★★★★ (binário estático, imagem mínima, gRPC/health nativos) | ★★★★★ (footprint mínimo) | ★★★★ (ok em Linux/containers) | ★★★ |
| **Familiaridade do time** | ★★★★★ (**stack do time**) | ★★ (curva) | ★★★ | ★★★ |
| Reuso do `Wyd2Client` (C#) | ★★★ (como **referência**/ferramenta de golden cases) | ★★ | ★★★★★ (código direto) | ★★ |
| Risco geral | **baixo** | médio | baixo-médio | médio-alto |

---

## 3. Recomendação

### Opção recomendada: **Go** (revisada — alinhada ao time e a Linux/microservices)

> Atualização honesta: a versão anterior recomendava C#, penalizando Go pelo parsing binário. Com
> **o time dominando Go** + alvo **Linux/Docker/microservices**, Go passa a ser a melhor escolha. A
> familiaridade do time é o que mais reduz o risco do big-bang, e a fraqueza do Go aqui é gerenciável.

**Por quê:**
1. **Velocidade até a paridade:** é a linguagem do time. Num big-bang, isso domina os demais fatores.
2. **Concorrência encaixa no modelo do jogo (NF5/NF7):** `tmServer` = **uma única goroutine dona do
   estado de mundo** (sem locks → mata dup de item) alimentada por **channels**, com goroutines de
   I/O por conexão. Idiomático em Go *e* fiel à semântica single-thread original. Ver topologia §3.5.
3. **Linux/Docker/microservices (NF9/NF10):** terreno natural — binário estático, imagem enxuta,
   `gRPC`/health-check/observabilidade nativos.
4. **GC (NF4):** a cadência de combate (800 ms) e ~1000 players/canal dão folga ao GC concorrente do
   Go. Escala-se por **canais** (mais instâncias de `tmServer`), não por loop stateless.

**Trade-off real (e mitigação):** Go **não garante layout de struct** (NF2/NF8). Mitigação: escrever
**codecs binários explícitos** (`encoding/binary`, little-endian, campo a campo) tanto para o
protocolo (Fase 1) quanto para o conversor de save (Fase 2). Isso é mais boilerplate, mas para um
protocolo **fixo** + conversor **one-shot** o codec explícito é até vantajoso: testável, sem padding
surpresa. Travar com **golden tests** usando os `sizeof`/offsets já documentados (816/552/56/7952,
Fase 2 §0.1) como constantes de teste. Onde fizer sentido, **gerar** os structs/codecs a partir do
catálogo de Types (Fase 1) e do mapa de offsets (Fase 2).

### Alternativas (se uma premissa mudar)
- **Rust/tokio:** se footprint/latência em container virarem prioridade máxima e o time topar a
  curva. Melhor em NF2/NF3/NF4; custo = velocidade até paridade. Viável como **polyglot** só no
  `tmServer` (serviço crítico), mantendo o resto em Go — mas isso adiciona custo operacional;
  recomendado **só** se houver gargalo medido.
- **C#/.NET 8:** continua sólido em Linux/containers e tem reuso direto do `Wyd2Client`; perdeu o
  topo apenas por **não ser a stack do time**. Boa escolha se a familiaridade do time mudar.

### Descartado para a v1
- **TS/Node:** o modelo single-thread casa com o atual, mas parsing binário e CPU de combate em
  escala são fracos; descartado por NF2/NF4.

---

## 3.5. Topologia de microservices alvo

```
            CPSock (FIXO, obrigatório)            gRPC + mTLS
 Cliente   ──────────────────────────►  tmServer  ───────────►  dbServer  ──►  PostgreSQL
 WYD.exe       (1 instância por CANAL;   (N shards    ───────────►  binServer (billing, NOVO)
 + ClientPatch  descoberto via serverlist) stateful)      (gRPC)
                                                  └────────────►  NATS (FUTURO: eventos cross-channel)
```

**Princípio-chave — só a borda do cliente é "presa" ao protocolo legado.** O cliente é fixo, então
`tmServer` *tem* que falar CPSock (HEADER + keyword-table + INITCODE, Fase 1). Os links **internos**
(`tm↔db`, `tm↔bin`) são seus dos dois lados → modernize-os; não arraste o CPSock para dentro.

| Serviço | Estado | Responsabilidade | Borda externa | Notas |
|---------|--------|------------------|---------------|-------|
| **tmServer** | **Stateful** (mundo em memória) | gameplay, simulação, handlers, war/castle | **CPSock** ↔ cliente | 1 instância por **canal/shard**; 1 goroutine de game-loop + channels; cliente-fixo manda no protocolo |
| **dbServer** | Stateless (estado em PG) | contas, personagens, ranking, guildas, persistência | **gRPC** (interno) | dono do PostgreSQL e do conversor one-shot (Fase 2); fronteira de estado compartilhado |
| **binServer** | Stateless | billing/cash-shop | **gRPC** (interno) | **desenhar do zero** (o original é stub; `_AUTH_GAME` UNVERIFIED) com API e política próprias |

**Comunicação interna — recomendação: gRPC (+ mTLS) agora; NATS depois.**
- Você vai **reescrever** `dbServer`/`binServer` de qualquer forma → projeta a API deles do zero;
  logo, **gRPC não custa mais** que inventar um TCP próprio, e dá contratos tipados + **auth real
  (mTLS)** + health/streaming nativos em Go. O CPSock interno não agrega (a "cripto" é tabela pública
  fixa) — é dívida; mantenha-o só na borda do cliente.
- **Não ameaça a paridade:** o protocolo interno é invisível ao cliente; basta o comportamento
  observável na borda ser idêntico.
- **NATS (futuro):** adotar quando surgirem eventos **cross-channel** (chat global, guilda/ranking
  entre canais), que pedem pub/sub. Para request/response (login, load/save char) gRPC basta.

**Concorrência do `tmServer` (preservando NF7):**
```
[goroutine por conexão] → decode CPSock → ch_in  ┐
                                                  ├─► [1 goroutine: game loop]  (dona do estado; sem locks)
[goroutine por conexão] ← encode CPSock ← ch_out ┘            │
                                                    gRPC client → dbServer / binServer
```

**Escala & deploy (Linux/Docker):**
- Escalar **por canais**: N `tmServer`, cada um autoritativo sobre seus players, todos atrás de **um**
  `dbServer`. O `serverlist`/`serverlist.bin` continua sendo a "service discovery" **do cliente**.
- `tmServer` exige **TCP sticky de longa duração** (não é HTTP) → expor via LB TCP / host:port por
  canal, **não** ingress HTTP. Descoberta interna via DNS do orquestrador (k8s/compose) ou NATS.
- Um container por serviço; imagem distroless/scratch (binário Go estático).

**Pegadinha Linux para o conversor (Fase 2):** os saves foram escritos por **MSVC x86**
(`long`=4, `time_t`=8 → `ACCOUNTFILE`=7952). Rodando em **Linux x64** o layout nativo difere
(`long`=8 no LP64). O conversor deve **emular o layout Windows-x86** independentemente do SO — em Go
isso é natural, pois você lê por **offset explícito** de qualquer forma. (O *wire* é little-endian e
Linux x64 também é LE → o protocolo em si não sofre; só o alinhamento das structs de save.)

---

## 4. Sequência do big-bang (ordem de reconstrução)

```
1. Codec CPSock + protocolo  ──►  2. dbServer + conversor (PostgreSQL)
        │                              │
        ▼                              ▼
3. Game-loop do tmServer  ──►  4. Handlers por subsistema  ──►  5. Conteúdo (loaders)
        │                                                            │
        ▼                                                            ▼
6. War / Castle / binServer  ───────────────────────────────►  7. Hardening (segurança)
```

1. **Codec + protocolo (Fase 1):** codec binário CPSock (HEADER, framing, INITCODE, keyword
   transform, checksum) + catálogo de Types em Go. Critério: testes de transporte da Fase 8 §3
   verdes contra captura real.
2. **dbServer + persistência (Fase 2):** serviço gRPC + schema PostgreSQL + **conversor one-shot**
   dos arquivos de conta (detectar versões por tamanho: 4294 legado / 7500–7600 / **7952 atual**;
   codec por **offset explícito** emulando MSVC-x86, §3.5). Hash de senhas/PIN na importação.
3. **Game-loop do tmServer (Fase 3):** 1 goroutine dona do estado + channels de I/O (§3.5); estado
   por índice, grids; cliente CPSock; gRPC client p/ dbServer.
4. **Handlers por subsistema (Fase 5):** login → char → movimento → combate → itens (drop/get/use) →
   trade → combine → party/guild. Cada lote valida seus golden cases (Fase 8 §2).
5. **Conteúdo (Fase 2/4):** loaders de `ItemList`, `SkillData`, `NPCGener`, mapas, rates.
6. **War/Castle + binServer (Fase 6):** `binServer` desenhado do zero (gRPC); billing após captura do
   `_AUTH_GAME`.
7. **Hardening:** corrigir as dívidas (senha hash, checksum rejeitante, mTLS interno, keytable →
   sessão real).

---

## 5. Registro de riscos

| Risco | Prob. | Impacto | Mitigação |
|-------|------|---------|-----------|
| Layout binário divergente (padding/offsets) — **dois regimes** | médio | **crítico** | **Protocolo** (`MSG_*`): `pack(1)`, testes byte-a-byte (Fase 8 §3) vs captura. **Save** (`STRUCT_ACCOUNTFILE` etc.): **alinhamento natural, NÃO `pack(1)`**; travar com `static_assert`/`offsetof` (Fase 2 §0.1). Sizes confirmados: MOB=816, MOBEXTRA=552, QUEST=56, ACCOUNTFILE=7952 |
| Largura de `time_t` no build | baixa | alto | o `sizeof(ACCOUNTFILE)=7952` assume `time_t`=8 (padrão MSVC); se `_USE_32BIT_TIME_T`, muda → o `static_assert` (Fase 2 §0.1) detecta antes da migração |
| Paridade de fórmulas (exp/drop/refino) | alto | alto | extrair constantes (Fase 4) + golden cases por distribuição (Fase 8 §4) |
| Formatos de dados multi-versão | médio | alto | conversor versionado por tamanho de arquivo (Fase 2 §1.2) |
| RNG não reproduzível | alto | médio | testar distribuição, não valor; seed injetável (Fase 8 §4) |
| `_AUTH_GAME` (billing) UNVERIFIED | médio | médio | capturar tráfego TM↔BI antes de implementar (Fase 6 §9) |
| Combate via funções `BASE_*` sem fonte | alto | alto | golden cases de ataque (Fase 8 §2.4) em vez de reconstrução |
| **Segurança:** senha/PIN em claro | certo | **crítico** | hash na migração (não persistir claro) |
| **Segurança:** checksum não-rejeitante + keytable estática pública | certo | alto | implementar checksum correto no envio; planejar sessão/cifra real pós-cutover (sem quebrar o cliente 7662 de imediato) |
| Concorrência reintroduzindo dup de item | médio | alto | `tmServer` com **1 goroutine dona do estado + channels** (§3.5); nada de mutar estado fora do game-loop; testes de race (Fase 8 §2.5) |
| Lógica de billing hardcoded (`Unk_*`) | médio | médio | reescrever como política explícita + captura (Fase 5 CharacterLogin) |
| **Go sem layout garantido** de struct | médio | alto (NF2/NF8) | codecs por **offset explícito** (não confiar em layout); golden tests com `sizeof`/offsets da Fase 2 §0.1; opção de **gerar** codecs do catálogo de Types |
| Tentar deixar `tmServer` stateless/horizontal | médio | alto | `tmServer` é **shard stateful**; escalar por **canais** (§3.5), não por replicar o game-loop |
| Modernizar comms internas atrasar paridade | baixa | médio | comms internas são invisíveis ao cliente; `dbServer`/`binServer` já serão reescritos → gRPC não custa a mais (§3.5) |

---

## 6. Critérios de "pronto para corte" (Definition of Done)

O big-bang só corta quando:
1. **Transporte:** todos os testes da Fase 8 §3 verdes (header/transform/checksum/framing/initcode).
2. **Handlers críticos:** golden cases de Fase 8 §2.1–§2.7 verdes (login, char, movimento, combate,
   drop/get, trade, combine).
3. **Dados:** conversor importa 100% das contas de amostra sem perda; dump round-trip confere.
4. **Compat real:** o `WYD.exe` 7662 (+ ClientPatch) **conecta, loga, joga, refina e desloga** contra
   o `tmServer` novo numa sessão de QA manual.
5. **Segurança mínima:** senhas hasheadas; mTLS nos links internos; sem regressão de dup em teste de
   concorrência.
6. **Microservices:** os 3 serviços sobem em containers Linux; `tm↔db`/`tm↔bin` via gRPC; um canal
   ponta-a-ponta funcionando.
7. **War/Castle/Billing:** validados por captura (podem ir num corte faseado pós-v1 se isolados).

> **Status da Fase 9: COMPLETO (revisado).** Recomendação atualizada para **Go** (alinhada ao time,
> a Linux/Docker e a microservices), com Rust e C# como alternativas; topologia de microservices
> (§3.5), comms internas (gRPC+mTLS, NATS futuro), sequência, riscos e DoD atualizados. A escolha
> final é do time — os trade-offs estão explícitos.
