# Documentação de Migração — w2pp-OpenWYD

> **Propósito:** documentação de engenharia reversa para uma **reescrita TOTAL (big-bang)** do
> servidor WYD, mantendo o cliente `WYD.exe` 7662 (+ `ClientPatch_v7662.dll`) sem alterações. Toda a
> documentação é baseada em evidência do código (`arquivo:linha`); pontos não confirmados estão
> marcados **UNVERIFIED**.
>
> Gerado seguindo `DOCUMENTATION-PROMPT.md`. Complementa (não repete) os deep-dives em
> `docs/agents/` — referencie aqueles para análise de risco por componente.

---

## Índice de artefatos e status

| Fase | Artefato | Status | Resumo |
|-----:|----------|--------|--------|
| 1 | [protocol-spec.md](protocol-spec.md) | **COMPLETO** (transporte+catálogo) / PARCIAL (`_AUTH_GAME`) | HEADER, framing, INITCODE, keyword transform, checksum, catálogo dos 198 Types, structs críticas, `pKeyWord` completa |
| 2 | [data-formats.md](data-formats.md) | **PARCIAL** (structs validadas) | conta/char (`STRUCT_ACCOUNTFILE`=**7952 B**), `STRUCT_MOB`=816/`MOBEXTRA`=552/`QUEST`=56/`ITEM`=8/`SCORE`=48; alinhamento natural vs `pack(1)` (§0.1); mapas (4096²/1024²), CSVs de conteúdo, modelo de dados alvo |
| 3 | [domain-model.md](domain-model.md) | **COMPLETO** | índice `conn↔player↔mob`, estado global, máquinas de estado, concorrência |
| 4 | [game-rules.md](game-rules.md) | **PARCIAL** | EXP/party, drop (gold/item/evento), refino/combine, combate, skills, timers |
| 5 | [handlers/](handlers/) | **COMPLETO** (58/58) | contratos por handler ([índice](handlers/README.md)); lote 1 (8 críticos, 1 arquivo cada) + lote 2 (50, agrupados por domínio). `Quest`/`MessageWhisper` com UNVERIFIED interno (subsistemas) |
| 6 | [flows.md](flows.md) | **COMPLETO** (gameplay) / PARCIAL (war/castle/billing) | diagramas de sequência ASCII |
| 7 | [config-ops.md](config-ops.md) | **COMPLETO** | catálogo de config, topologia (8281/7514/3000), hardcodes, env/secrets |
| 8 | [parity-tests.md](parity-tests.md) | **COMPLETO** (metodologia+casos) | formato de golden case, casos por subsistema, captura, RNG |
| 9 | [migration-plan.md](migration-plan.md) | **COMPLETO** | NFRs, comparação de stacks + recomendação, sequência, riscos, DoD |
| 10 | [glossary.md](glossary.md) | **COMPLETO** | termos WYD/PT-BR e do código |

---

## Por onde começar (ordem recomendada de leitura)

1. **Fase 1 (protocol-spec)** — destrava tudo; sem compat de fio o cliente não conecta.
2. **Fase 2 (data-formats)** — para o conversor de dados existentes.
3. **Fase 3 (domain-model)** — entender o estado antes de mexer em handlers.
4. **Fases 4, 5, 8** — garantem paridade comportamental no big-bang.
5. **Fase 9 (migration-plan)** — decisão de stack e plano de corte.

## Achados de maior impacto (resumo executivo)

- **Versão de protocolo = 7640**, não 7662 (7662 é o nome do build/patch). `MSG_AccountLogin.
  ClientVersion` deve ser `7640` (`Basedef.h:102`). — Fase 1 §4.1.
- **Sem criptografia real:** ofuscação por tabela estática pública (`pKeyWord`) + checksum
  **não-rejeitante**; o cliente tem a verificação **desligada** pelo ClientPatch. — Fase 1 §1.4-1.5.
- **Senha e PIN em texto plano** no arquivo de conta e no fio. Dívida crítica a corrigir na
  migração. — Fase 2 §1.3, Fase 9 §5.
- **Players e mobs compartilham `STRUCT_MOB` e o array `pMob[]`**, particionados pelo índice
  (`<1000` = player). Toda a numeração de fio depende disso. — Fase 3 §1.
- **Persistência = dumps crus de struct C** (layout MSVC x86) em `account/<Key>/<NOME>`, com
  **múltiplas versões por tamanho de arquivo** (4294 legado / 7500–7600 / **7952 atual**). As structs
  de save usam **alinhamento natural** (não `pack(1)`, diferente das mensagens de rede). — Fase 2 §0.1, §1.
- **Gameplay é single-thread** (reactor WinSock). Preservar na v1 como **1 goroutine dona do estado
  + channels** (Go) para paridade e evitar dup de item. — Fase 3 §5, Fase 9 §3.5.
- **Stack recomendada: Go** (stack do time; encaixa em Linux/Docker/microservices; concorrência
  idiomática preserva o single-thread). Trade-off: codecs binários por offset explícito. Rust e C#
  como alternativas. — Fase 9 §3.
- **Arquitetura alvo: microservices** `tmServer` (CPSock↔cliente, stateful, 1 por canal) /
  `dbServer` (gRPC, PostgreSQL) / `binServer` (gRPC, novo). Só a borda cliente↔tmServer é presa ao
  protocolo legado; links internos via **gRPC+mTLS** (NATS futuro p/ cross-channel). — Fase 9 §3.5.

## Pendências UNVERIFIED (a fechar por captura/build)

- Layout interno de `_AUTH_GAME` (billing, 196 bytes) — Fase 1 §4.3 / Fase 6 §9.
- `sizeof` das structs de save calculados (MOB=816, MOBEXTRA=552, QUEST=56, ACCOUNTFILE=7952);
  **a confirmar no build:** largura de `time_t` (premissa =8) via `static_assert`, e `BASE_GetFirstKey`. — Fase 2 §0.1.
- Fórmula completa de combate (funções `BASE_*` sem fonte) — Fase 4 §4 / Fase 8 §2.4.
- Semântica bit-a-bit do `AttributeMap` e origem de `g_pDropRate[]` — Fase 2 §2 / Fase 4 §2.2.
- Passo-a-passo fino de alguns NPCs de quest longos (`_MSG_Quest`) — despacho e propósito dos 38
  NPCs já mapeados; Fase 5. (`_MSG_MessageWhisper`: 55 comandos enumerados.)

## Convenções

- Evidência sempre como `Source/Code/.../arquivo.cpp:linha`.
- Valores de fio em decimal **e** hex; little-endian.
- **UNVERIFIED** = não confirmado pela fonte; segue o que falta para confirmar.
