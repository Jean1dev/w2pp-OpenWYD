# Fase 9 — Recomendação de Stack e Plano de Migração (w2pp-OpenWYD)

> Reescrita TOTAL (big-bang), cliente `WYD.exe` 7662 mantido. Baseado nas Fases 1–8.

---

## 1. Requisitos não-funcionais (derivados do código)

| # | Requisito | Origem (fase) | Implicação |
|---|-----------|---------------|------------|
| NF1 | Reator de I/O com alto fan-out (até `MAX_USER=1000` conexões) | Fase 3 §6, CPSock | epoll/IOCP-like; centenas de sockets simultâneos |
| NF2 | Parsing de **structs binárias** little-endian, layout MSVC x86 | Fase 1, Fase 2 | mapear bytes→struct com offsets/padding exatos |
| NF3 | Ofuscação/checksum por byte reproduzível | Fase 1 §1.4-1.5 | aritmética `u8` com wrap; `pKeyWord` idêntica |
| NF4 | Baixa latência (combate cadência 800 ms, ticks) | Fase 5 Attack | GC pauses devem ser controlados |
| NF5 | Estado de mundo **em memória**, autoritativo | Fase 3 | grandes arrays (`pMob[25000]`, grids 4096²) |
| NF6 | Persistência (arquivos→banco) | Fase 2 | migrar dumps de struct para schema relacional |
| NF7 | Modelo de concorrência que preserve semântica single-thread | Fase 3 §5 | gameplay serializado; I/O assíncrono |
| NF8 | Compat de fio byte-for-byte | Fase 1 | sem margem para erro de layout |

---

## 2. Comparação de candidatos

| Critério | Rust/tokio | C#/.NET 8 | Go | TypeScript/Node |
|----------|-----------|-----------|-----|-----------------|
| Mapear structs binárias | ★★★★★ (`#[repr(C)]`, `bytemuck`, `zerocopy`) | ★★★★ (`StructLayout`, `Span<byte>`, `MemoryMarshal`) | ★★★ (`encoding/binary`, sem repr garantido) | ★★ (`Buffer`/`DataView` manual) |
| Aritmética `u8` com wrap | ★★★★★ (`wrapping_*`, tipos sem sinal) | ★★★★ (`byte`/`unchecked`) | ★★★★ | ★★★ (cuidado com number) |
| Latência / GC | ★★★★★ (sem GC) | ★★★★ (GC server, baixo) | ★★★★ (GC, pausas curtas) | ★★ (GC + single-thread JS) |
| Rede / fan-out | ★★★★★ (tokio) | ★★★★★ (Kestrel/Socket async) | ★★★★★ (goroutines) | ★★★★ (libuv) |
| Modelo de concorrência p/ estado compartilhado | ★★★ (borrow checker exige actor/Arc<Mutex>) | ★★★★ (mais flexível) | ★★★ (channels, mas data races possíveis) | ★★★★ (single-thread natural ≈ modelo atual!) |
| Ecossistema / libs de jogo/rede | ★★★★ | ★★★★ | ★★★★ | ★★★ |
| **Reuso do que já existe** | ★★ | ★★★★★ (**Wyd2Client em C# já implementa o protocolo de fio**, Fase 0/comparação) | ★ | ★★ |
| Curva para o time | ★★ (borrow checker) | ★★★★ (familiar, OO) | ★★★★ | ★★★★ |
| Risco geral | médio | **baixo** | médio | médio-alto |

---

## 3. Recomendação

### Opção recomendada: **C# / .NET 8** (com trade-offs explícitos)

**Por quê:**
1. **Reuso direto:** o `Wyd2Client` (kevinkouketsu, C#) já implementa o protocolo WYD de fio
   (HEADER + keyword-table) — é documentação viva e código de partida para a camada de transporte e
   para gerar/validar golden cases (Fase 8). Reduz o maior risco do big-bang (NF8).
2. **Structs binárias:** `[StructLayout(LayoutKind.Sequential, ...)]` + `MemoryMarshal`/`Span<byte>`
   mapeiam os layouts MSVC sem cópia — **respeitando os dois regimes**: `Pack=1` para mensagens de
   rede (`MSG_*`), e **alinhamento natural** (`Pack=8`/offsets explícitos) para as structs de save
   (`STRUCT_ACCOUNTFILE`=7952 etc.). Ver Fase 2 §0.1.
3. **Concorrência:** permite começar com **um game-loop single-thread** (preservando a semântica
   atual, NF7) e isolar I/O/persistência em `async`. Migração de modelo de baixo risco.
4. **GC server-mode** mantém latência adequada para NF4 (não é um FPS competitivo; 800 ms de
   cadência dá folga).
5. Time provavelmente já conhece C# (linhagem Windows do projeto).

**Trade-offs aceitos:** não é "zero-GC" como Rust; exige disciplina para evitar alocações no
hot-path (usar `Span`/pooling). Se latência/escala virarem gargalo real, o hot-path de rede pode ser
otimizado depois sem trocar a stack.

### Alternativa forte: **Rust/tokio**
Escolher se a prioridade for performance/segurança de memória máxima e o time topar a curva. Melhor
em NF2/NF3/NF4; o custo é o borrow-checker forçar cedo um desenho actor-model para o estado global
compartilhado (Fase 3 §5) — o que é bom a longo prazo, mas mais lento para chegar à paridade.

### Descartados para a v1
- **Go:** ótimo em rede, mas sem `#[repr(C)]`/layout garantido o mapeamento binário é mais
  trabalhoso e arriscado (NF2); data races no estado global compartilhado.
- **TS/Node:** o modelo single-thread casa com o atual (NF7), mas parsing binário e CPU de combate
  em larga escala são pontos fracos; descartado por NF2/NF4.

---

## 4. Sequência do big-bang (ordem de reconstrução)

```
1. Transporte + protocolo  ──►  2. Persistência (DBSrv + conversor)
        │                              │
        ▼                              ▼
3. Loop/reactor do TMSrv  ──►  4. Handlers por subsistema  ──►  5. Conteúdo (loaders)
        │                                                            │
        ▼                                                            ▼
6. War / Castle / Billing  ────────────────────────────────►  7. Hardening (segurança)
```

1. **Transporte + protocolo (Fase 1):** HEADER, framing, INITCODE, keyword transform, checksum,
   catálogo de Types. Critério: testes de transporte da Fase 8 §3 verdes contra captura real.
2. **Persistência (Fase 2):** schema novo + **conversor one-shot** dos arquivos de conta (detectar
   versões por tamanho: 4294 legado / 7500–7600 intermediário / **7952 atual**). Mapear as structs
   de save em **alinhamento natural** (não `pack(1)`) — `STRUCT_ACCOUNTFILE`=7952 (MOB=816,
   MOBEXTRA=552, QUEST=56), travadas por `static_assert` (Fase 2 §0.1). Hash de senhas/PIN na importação.
3. **Loop do TMSrv (Fase 3):** reactor + estado global (entidades por índice, grids). Single-thread
   de gameplay.
4. **Handlers por subsistema (Fase 5):** login → char → movimento → combate → itens (drop/get/use) →
   trade → combine → party/guild. Cada lote valida seus golden cases (Fase 8 §2).
5. **Conteúdo (Fase 2/4):** loaders de `ItemList`, `SkillData`, `NPCGener`, mapas, rates.
6. **War/Castle/Billing (Fase 6):** após captura do protocolo de billing (`_AUTH_GAME`).
7. **Hardening:** corrigir as dívidas (senha hash, checksum rejeitante, keytable → sessão real).

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
| Concorrência reintroduzindo dup de item | médio | alto | manter gameplay single-thread na v1; testes de race (Fase 8 §2.5) |
| Lógica de billing hardcoded (`Unk_*`) | médio | médio | reescrever como política explícita + captura (Fase 5 CharacterLogin) |

---

## 6. Critérios de "pronto para corte" (Definition of Done)

O big-bang só corta quando:
1. **Transporte:** todos os testes da Fase 8 §3 verdes (header/transform/checksum/framing/initcode).
2. **Handlers críticos:** golden cases de Fase 8 §2.1–§2.7 verdes (login, char, movimento, combate,
   drop/get, trade, combine).
3. **Dados:** conversor importa 100% das contas de amostra sem perda; dump round-trip confere.
4. **Compat real:** o `WYD.exe` 7662 (+ ClientPatch) **conecta, loga, joga, refina e desloga** contra
   o servidor novo numa sessão de QA manual.
5. **Segurança mínima:** senhas hasheadas; sem regressão de dup em teste de concorrência.
6. **War/Castle/Billing:** validados por captura (podem ir num corte faseado pós-v1 se isolados).

> **Status da Fase 9: COMPLETO.** Recomendação fundamentada (C#/.NET com Rust como alternativa),
> sequência, riscos e DoD definidos. A escolha final de stack é uma decisão do time — os trade-offs
> estão explícitos.
