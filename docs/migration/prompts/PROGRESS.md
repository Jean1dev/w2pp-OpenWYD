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
| 3 | Game-loop do tmServer (`tmserver/internal/world`) | **COMPLETO** | gRPC client p/ dbServer (adapter da port); sentinel do grid |
| 4 | Handlers por subsistema (8 lotes) | **COMPLETO** (núcleo testável) — 8/8 lotes ✅ | layout S→C; `_NN_*`; billing; parry/crit; EXP/party §1; recipe/quest tables; quest(38 NPCs)/cmds-whisper; auto-trade/loja/banco; ranking |
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

---

## Fase 4 — Handlers por subsistema (em lotes) — EM ANDAMENTO

Ordem dos lotes (migration-plan §4): login→char→movimento→combate→itens→trade→combine→party/guild.

### Lote 1/8 — login → criação/seleção de char ✅
`tmserver/internal/handler/` + `tmserver/internal/rng/`:
- `rng/` — **LCG do MSVC** (`state*214013+2531011; (state>>16)&0x7FFF`, seed default 1, seed
  injetável). Travado contra a sequência canônica (41,18467,6334,…; parity-tests §4.0). Base p/
  drop/combate/refino (lotes futuros).
- `handler/dispatch.go` — `Dispatcher` (rota `Type→handler`; roda no loop) instalado como
  `world.Handler`; contador de brute-force por conta (loop-only, sem lock).
- `handler/login.go` — `_MSG_AccountLogin`: valida versão (7640)/Mode(`USER_ACCEPT`)/brute-force →
  **relay assíncrono** ao dbServer via `world.Go` (off-loop) → resultado re-entra no loop
  (`USER_LOGIN`→`USER_SELCHAR`/`CNFAccountLogin`, ou notice). Mata head-of-line: I/O do DB fora do loop.
- `handler/character.go` — `_MSG_CreateCharacter`/`DeleteCharacter`/`CharacterLogin`/`CharacterLogout`/
  `Restart`: gates de `Mode` (SELCHAR/PLAY), relay ao dbServer, injeção do player no mundo no
  char-login (entity `MOB_USER`), revive HP=2 no restart.
- `handler/notice.go` — notificações `_NN_*` (placeholder `MessageBoxOk`+código).
- **Infra do world p/ handlers:** port `Persistence` estendida (AccountLogin/Create/Delete/Load),
  `World.Go` (async DB → callback no loop, com checagem de identidade de sessão), `World.Send/Close/
  Entity/Now`, flush-on-close (mensagem final entregue antes de fechar).

### Critério de pronto (lote)
- `go build ./...` ✅ · `go test -race ./...` ✅ · `go vet ./...` ✅ · `gofmt -l` limpo ✅.
- Casos de Fase 8 §2.1–§2.2 cobertos como **black-box S→C** (assert no Type/efeito, filosofia da
  Fase 8): login OK→`CNFAccountLogin`; versão errada→notice+close; modo errado→`LoginNow`; senha
  errada×3→`3WrongPass`; sem conta/bloqueado; create OK/nome inválido; delete; char login+logout;
  slot inválido. RNG: sequência canônica.

### UNVERIFIED / pendências do lote
- **Byte-exatidão S→C**: `SELCHAR` (`CNFAccountLogin`) e o snapshot de `CNFCharacterLogin` têm layout
  UNVERIFIED → payloads placeholder; **golden byte-a-byte pendente de captura** (Fase 8 §5). Testes
  asseguram o Type/efeito, não os bytes finais.
- **Notificações `_NN_*`**: formato de fio real não capturado (placeholder `MessageBoxOk`+código).
- **Billing/free-exp gate** do char-login (`Unk_*`,`BILLING`,`FREEEXP`,`g_Hour`): NÃO replicado —
  vira política explícita + captura (Fase 6).
- **`BASE_CheckValidString`** (nome): conjunto de caracteres UNVERIFIED (placeholder alnum+`_`).
- **Restart**: teleporte por região/clan hardcoded não reproduzido (vira config + captura).
- **gRPC client p/ dbServer**: handlers usam a port; o adapter sobre `api/db/v1` (e `make proto`)
  ainda não foi escrito → `cmd/tmserver` usa `NopPersistence` (login reporta "no account").

### Lote 2/8 — movimento & visão ✅
`tmserver/internal/handler/movement.go`:
- `_MSG_Action` (+`Action2`/`Action3`): gates `USER_PLAY`/`Hp!=0`, **bounds** (PosX/Y, TargetX/Y no
  grid), **anti-speedhack** (janela de tick: `movetime > now+15000` ou `< now-120000` →
  `AddCrackError`, sem broadcast — lote2-movimento.md) → atualiza posição+grid e **multicast na visão**
  (mesma rota, `HEADER.ID = mover`).
- `_MSG_Motion`: gate + multicast. `_MSG_ChangeCity`: bit-packing de cidade em `Merchant` bits 6-7
  (layout documentado preservado). `_MSG_NoViewMob`: reconciliação de visão (Create/RemoveMob).
  `_MSG_ReqTeleport`: stub (economia UNVERIFIED).
- **Infra do world (batch 2):** `Send` (ID=você) vs `SendTo` (ID arbitrário p/ broadcast),
  `BroadcastInView` (Chebyshev ≤ `ViewRange`), `SetEntityPos` (sincroniza grid), `AddCrackError`
  (limite → drop). Refator `send`→`enqueue` (ID não mais forçado).

Casos Fase 8 §2.3 cobertos (black-box): `move_ok` (broadcast com mesma rota; mover não recebe a
própria), `move_speedhack` (tick futuro → dropado), `move_out_of_bounds` (alvo fora do grid →
dropado), motion broadcast. Clock injetável p/ determinismo do anti-speed.

UNVERIFIED do lote: `ViewRange`/geometria do `GridMulticast`; `BASE_GetVillage` (ChangeCity);
economia/zonas do `ReqTeleport`; snapshot de `CreateMob` (NoViewMob); illusion skill (`Action3`);
route-stepping autoritativo.

### Lote 3/8 — combate ✅
`tmserver/internal/combat/` + `tmserver/internal/handler/combat.go`:
- **`combat/` — fórmulas puras (game-rules.md §4)** com RNG **injetável** (interface `Rand`; ordem de
  chamadas preservada): `Damage` (§4.1 `BASE_GetDamage`), `SkillDamage` (§4.2), `ResolveHit` (§4.3-4.5:
  crítico parcial/total, AC×3 em PvP, parry roll em 1000, reflect plano+%, clamps). Testado **exato**
  por valores hand-computed (stub RNG) **e** amarrado ao LCG real (primeiro `rand()=41` → `Damage=99`).
- **`handler/combat.go` — `_MSG_Attack`** (+`AttackOne`/`Two`): gates TradeMode/`USER_PLAY`/vivo
  (skill 99 = ressurreição), **cadência anti-speed 800 ms** + sanidade de tick (int64, sem underflow),
  **dano server-authoritative** (recalcula via `combat`, sobrescreve `Dam[]` no payload), broadcast na
  visão. RNG do mundo é dono do loop (`World.Rand()`, seed 1).
- Codec `MsgAttackBody` (campos + `Dam[]` variável; `AttackOne`/`Two` por tamanho). Entity ganhou
  `Damage`/`AC`/`Master`; Session ganhou `LastAttackTick`/`TradeMode`.

Casos Fase 8 §2.4 (black-box): **`attack_hit` golden EXATO** (handler+combat+LCG → dano=145),
`attack_too_fast` (cadência → dropado), `attack_while_dead` (crack, sem efeito). Formas puras: 11
casos exatos.

UNVERIFIED do lote: `BASE_GetDoubleCritical` e `GetParryRate` (usados placeholders: DoubleCritical do
pacote, parry=0); coef. Dex/Str por classe×arma (`BASE_GetCurrentScore`); `MAX_DAMAGE`; `ESCENE_FIELD`
no broadcast; **`MobKilled` (EXP/party §1 + drop §2)** adiado p/ o lote de itens; `_MSG_SetHpMp` layout.

### Lote 4/8 — itens (drop/get/use) + drop de MobKilled ✅
`tmserver/internal/loot/` + `tmserver/internal/handler/{item,mobkilled}.go` + infra de itens no world:
- **`loot/` — fórmulas puras de drop (game-rules.md §2)** com RNG injetável: `GoldDrop` (§2.1, 2 rolls,
  teto 2000), tabela **real `g_pDropRate[64]`** (Basedef.cpp:222), `EffectiveDropRate` (bônus + ajuste
  por nível), `Drops`. Testado **exato** (stub + LCG: `41%19≠0` ⇒ sem gold).
- **Itens no world:** tipo `Item`/`Effect`, `Entity.Carry[64]`/`Equip[16]`/`Coin`/`Level`, store de
  itens no chão `pItem[]` (`CreateGroundItem`/`GroundItem`/`RemoveGroundItem`) + `itemGrid`,
  `AddToCarry`. **Claim atômico** no loop (mata dup).
- **`handler/item.go`:** `_MSG_DropItem` (gates vivo/play/trade, bounds de grid, **blacklist exata**
  {508,509,522,526-537,446,747,3993,3994}, cria no chão + limpa origem), `_MSG_GetItem`
  (`ItemID-10000`, distância ≤3, `DecayItem` se sumiu, claim atômico), `_MSG_UseItem` (equip
  CARRY→EQUIP).
- **`handler/mobkilled.go`:** morte de mob → **gold + drop por slot** (loot, exato); `Carry` do mob é a
  loot table. EXP (§1, party) **adiado** (UNVERIFIED).

Casos Fase 8 §2.5 (black-box): `drop_ok`+`get_ok` (round-trip), `drop_blacklisted`, `get_decayed`
(`DecayItem`), `get_too_far`, **`dup_race`** (2 gets → 1 `CNFGetItem` + 1 `DecayItem`), equip. Loot: 5
casos exatos.

UNVERIFIED do lote: **EXP/party (§1: `g_EmptyMob`/`PARTYBONUS`/divisores)**; valores de `ITEM_PLACE_*`
e layouts de `MSG_DropItem`/`GetItem` (best-effort, byte-exato pendente de captura); `MAX_ITEMLIST`;
roll/ajuste-por-nível final do drop (§2.2 truncado); `_MSG_CreateItem`/`CNFMobKill`/`RemoveMob`
broadcasts; requisitos de equip + `BASE_GetCurrentScore`; cooldown de refino (desativado no original);
**MobKilled precisa de spawn de mob (Fase 5) p/ teste de integração** (hoje só loot unit + wiring).

### Lote 5/8 — trade P2P direto ✅
`tmserver/internal/handler/trade.go` + codecs `MsgTradingItem`/`MsgTrade`/`WireItem` + `Session.Trade`:
- **`_MSG_Trade` (swap atômico):** valida (vivo/play, oponente em `USER_PLAY`, `0≤money≤Coin`, **memcmp
  do item ofertado vs slot real** — anti-troca-no-confirm), grava a oferta+confirmação; quando **ambos**
  confirmam e cruzam → **transação única** (`executeSwap`: tira itens dos dois, checa espaço, entrega +
  transfere gold, **rollback** se faltar slot). Qualquer falha → `removeTrade` (cancela nos dois).
- `_MSG_TradingItem` (abre/atualiza janela; reseta confirmações), `_MSG_QuitTrade` (cancela).
- **Anti-dup:** `removeTrade` é chamado ao **dropar/pegar item durante trade** (cancela nos dois lados).
- **Achado/correção:** `OpponentID ∈ (0, MAX_USER)` exclui conn 0 → **`allocConn` agora reserva conn 0**
  (fiel ao `conn > 0` do `_MSG_AccountLogin`); players começam em conn 1. Testes ajustados.

Casos Fase 8 §2.7 (black-box): **`trade_ok`** (swap atômico — A recebe item de B e vice-versa,
verificado no result), `trade_cancel` (`QuitTrade` nos dois), **`trade_dup`** (drop durante trade →
cancela nos dois).

UNVERIFIED do lote: **auto-trade (`SendAutoTrade`/`ReqTradeList`/`ReqBuy`), loja NPC (`Buy`/`Sell`) e
banco (`Deposit`/`Withdraw`)** — sub-lote separado; layout real de `MSG_Trade` result/`SendItem`
(placeholder); `ADMIN_RESERV` (topo) ainda não reservado.

### Lote 6/8 — combine/refino (engine parametrizada) ✅
`tmserver/internal/combine/` + `tmserver/internal/handler/combine.go`:
- **`combine/Roll` — função pura do roll** (`rand()%115`; `≥100⇒-15`; `success = v ≤ rate`). Testado
  **exato** (stub + LCG: 1º `rand()=41`⇒v=41) **e por distribuição** (N=300k: 85–99 com peso ~2× e
  nada cai em 100–114).
- **Engine única parametrizada** (`CombineFamily{Rate, Apply}`) consolida as **9 variantes Item[]**
  (Anct/Ehre/Tiny/Shany/Ailyn/Agatha/Odin/Lindy/Alquimia) — todas roteiam pro mesmo `combineItem`,
  diferindo só na receita/taxa. Ordem fiel: **valida receita → consome insumos → rola** (falha
  consome mesmo assim). `Extracao` (MSG_STANDARDPARM2) é stub.
- Codec `MsgCombineItemBody` + `MsgCombineComplete` (parm 0/1/2). Famílias configuráveis via
  `handler.Config.CombineFamilies` (default = placeholder UNVERIFIED, rate 0).

Casos Fase 8 §2.6 (black-box, roll determinístico seed 1): **`refine_success`** (rate 50→v41→parm 1),
**`combine_consumes_on_fail`** (rate 30→v41→parm 2, insumos consumidos), **`combine_invalid`**
(rate 0→parm 0, **insumos NÃO consumidos**). Roll: exato + distribuição.

UNVERIFIED do lote: **tabelas de receita/taxa** (`GetMatchCombine<X>` + `CompRate.txt`/`SancRate.txt`)
e `ItemList[].Extra` → famílias são placeholders até Fase 5; encoding de **sanc** em `STRUCT_ITEM`;
`MAX_COMBINE` e layout de `MsgCombineItem`; `Ehre`/`Odin` (lógica própria extensa, `srand` no Odin);
extração; cooldown anti-spam (comentado no original).

### Lote 7/8 — party & guilda ✅
`tmserver/internal/handler/{party,guild}.go` + estado party/guild no `Entity` + codecs.
- **Party:** `_MSG_SendReqParty` (convite, grava `LastReqParty` no alvo), `_MSG_AcceptParty` (entra na
  party com **gate anti-forja `LastReqParty`** — bloqueia PARTYHACK; forma a party, adiciona membro,
  sincroniza lista via broadcast), `_MSG_RemoveParty` (líder→dissolve / membro→sai). Regra de nível
  `partyLevelOK` (simplificada).
- **Guilda:** `_MSG_InviteGuild` (alvo mesmo-clan/sem-guilda + oficial convidante; debita gold —
  4M/100M; muta `Guild`/`GuildLevel` do alvo; broadcast `CreateMob` da tag; boas-vindas).
  `_MSG_GuildAlly`/`_MSG_War` = relays ao dbServer (valida líder; propagação UNVERIFIED).
  `_MSG_Challange`/`ChallangeConfirm` = stubs (economia de zona UNVERIFIED).
- Entity ganhou `Clan`/`Guild`/`GuildLevel`/`ClassMaster`/`Leader`/`LastReqParty`/`PartyList[12]`;
  CharacterState idem.

Casos Fase 8 §2.8 (black-box): `party_invite`+`accept` (convite→sync nos dois), **anti-forja**
(accept sem convite → rejeitado), `guild_invite` (boas-vindas + refresh de tag).

UNVERIFIED do lote: **PARTY_DIF + ajustes de tier (`ClassMaster`/`MAX_CLEVEL`)**; bloqueio dominical do
convite; **propagação de aliança/guerra no dbServer**; **economia de imposto de zona** (`WeekMode`,
`GuildImpostoID`, Exp-como-cofre, item 4011, `zone` de `BaseScore.Level` vs `Merchant-6`); layout S→C
de party (reusa types C→S); Battle Royale no convite.

### Lote 8/8 — chat, bônus, quest/cash ✅
`tmserver/internal/handler/{chat,misc}.go`:
- **`_MSG_MessageChat`:** fala pública → **multicast na visão**; toggles `whisper`/`guildon`/`guildoff`.
- **`_MSG_MessageWhisper`:** sussurro a jogador online por nome (`SessionByName`); offline → notice,
  alvo bloqueado (`Whisper`) → "deny". **55 comandos** (incl. backdoors GM) **NÃO** tratados (UNVERIFIED).
- **`_MSG_ApplyBonus`:** distribuição de ponto de atributo (Score: Str/Int/Dex/Con; decrementa
  `ScoreBonus`, `UpdateScore`). Special/Skill UNVERIFIED.
- **Stubs UNVERIFIED:** `_MSG_Quest` (38 NPCs, 2753 linhas), `_MSG_ReqRanking` (duelo),
  `_MSG_CapsuleInfo`/`_MSG_AccountSecure` (relays DB), `_MSG_PutoutSeal`.

Casos Fase 8 (black-box): chat público (broadcast), whisper (entrega/offline/bloqueado), applybonus
(`UpdateScore`).

UNVERIFIED do lote: **engine de quest** (flags `MOBEXTRA.QuestInfo`, 38 NPCs, recompensas hardcoded →
data-driven); **command-bus** dos 55 cmds de whisper (com autorização, removendo backdoors); máquina de
duelo (`DoRanking`); relays DB (PIN/capsule); imposto de guilda no chat; bônus Special/Skill.

---

## Fase 4 — RESUMO (8/8 lotes, núcleo testável COMPLETO)

`tmserver/internal/`: **8 pacotes** — `protocol` (Fase 1), `world` (Fase 3, game-loop + estado),
`rng`/`combat`/`loot`/`combine` (fórmulas **puras** com LCG do MSVC), `handler` (~40 handlers em 8
lotes). **Pacotes de paridade exata** (golden cases via LCG): combate (`Damage=145` e2e), drop
(`g_pDropRate` real), combine (`rand()%115` + distribuição). Modelo: **1 goroutine dona + channels**,
mutação só no loop, DB assíncrono via `World.Go`, anti-dup atômico (itens/trade) por construção.

**Dívidas transversais (pós-Fase 4 / Fases 5-7):** layouts S→C byte-exatos (SELCHAR/CNFCharLogin/
SendItem/notices) pendentes de **captura** (Fase 8 §5); **loaders de conteúdo (Fase 5)** destravam
ItemList/SkillData/CompRate/SancRate/mapas/NPCGener → fecham combine/quest/drop UNVERIFIED;
**adapter gRPC dbServer** (login/char/save real, hoje `NopPersistence`); EXP/party de MobKilled; billing
(`_AUTH_GAME`); auto-trade/loja-NPC/banco; mob spawns (Fase 5) p/ testar MobKilled/quest in-game.

### Pendências de tooling (checklist §25)
- `golangci-lint` / `goimports` / `govulncheck` NÃO instalados localmente (binário `go` local 1.22.2
  baixa toolchain 1.25.x via `GOTOOLCHAIN` por causa do go.mod). Rodar lint/vuln no container
  `golang:1.26-alpine` (`make lint`, `make vuln`) ou via `go install` antes do corte. `go vet` +
  `gofmt` (disponíveis) cobrem Fases 1–2. `make proto` (protoc + plugins) ainda não rodado.
