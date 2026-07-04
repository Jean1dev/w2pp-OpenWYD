# Plano: Sistema de Edição de NPCs (painel de moderação)

> Status: PLANO (não implementado). Origem: necessidade de **moderadores de jogo** editarem NPCs pela
> web — se o NPC **aparece ou não**, **onde** ele fica, e **quais itens vende e por qual preço**. Este
> doc é a fonte da verdade dessa decisão de arquitetura. Escopo pedido: **apenas estruturas de dados e
> backend** (o front-end Next.js não faz parte deste plano; ele apenas consome a mesma borda `web-api`
> descrita em `web-platform-plan.md`).

## 1. O que existe hoje (ponto de partida)

Os NPCs **não têm identidade estável nem configuração editável**. Eles nascem da **árvore de conteúdo
read-only** montada no boot:

- **`Release/TMsrv/run/NPCGener.txt`** — blocos de *spawn* (líder, seguidor, waypoints, `MinuteGenerate`,
  `MaxNumMob`, `RouteType`). Parseado por `content.LoadNPCGenerators` (`tmserver/internal/content/npc.go`).
- **`Release/TMsrv/run/npc/<nome>`** — o *template* cru de 816 bytes (`STRUCT_MOB`) de cada NPC/mob,
  carregado por `content.LoadNPCTemplate`. É daqui que sai o **flag `Merchant`** e o **`Carry[]`** (a
  vitrine da loja: `world/api.go:154-165`, `protocol.MobCarry`).
- **`itemPrices`** — o preço de compra vem do **catálogo de itens** (`ItemList`), não do NPC:
  `handler/shop.go:83` (`d.itemPrices[index]`). O NPC só decide **quais** itens aparecem (o `Carry[]`);
  o **preço** é global por índice de item.
- **Spawn no boot**, dentro do loop, por `spawnNPCs` (`tmserver/cmd/tmserver/main.go:222`): cada bloco
  vira um `world.Generator` e dispara um `GenerateMob`.

Limitações para o que os moderadores precisam:

1. **Aparecer/não aparecer** — hoje só editando `NPCGener.txt` e reiniciando o servidor.
2. **Onde fica** — idem: coordenadas estão no `.txt`; mudar exige editar arquivo + restart.
3. **O que vende / por qual preço** — o `Carry[]` está *dentro* do template binário de 816 bytes
   (nada amigável de editar); o preço é global no catálogo, não por-NPC.
4. **Sem ID estável** — blocos são posicionais (índice no arquivo). Não dá pra referenciar "o NPC X"
   de forma durável a partir de um banco.

## 2. A restrição que decide tudo (a mesma do resto da plataforma web)

Vale **integralmente** a regra de `web-platform-plan.md`:

> O tmServer tem **um único goroutine dono de todo o estado do mundo**, em memória, sem locks
> (`world/world.go`, `CLAUDE.md` §"single-owner game loop"). As **entidades NPC são estado vivo do
> loop** (`w.entities[...]`, `pMob[MaxUser..MaxMob)`).

Consequência direta:

> ❌ A web **nunca** muta entidade NPC viva. Nem escreve direto num Postgres que o tmServer também
> escreve para essas linhas. Isso reintroduziria exatamente o race que a arquitetura single-owner
> existe para evitar.

Mas há uma diferença importante em relação a inventário de personagem: **configuração de NPC é
conteúdo/definição, não estado de jogador**. O tmServer **não persiste** definição de NPC de volta — ele
só a **lê** e **materializa** entidades a partir dela. Isso torna o Postgres o **dono legítimo da
definição** (o tmServer é dono só da *instância viva*), e elimina o risco de clobber que existe para
`donate_balance`/inventário. O fluxo é **uma via**: `web-api → Postgres → dbServer → tmServer`.

## 3. Topologia decidida

```
Moderador ──HTTPS──> Next.js BFF ──gRPC+mTLS──> web-api ──> Postgres  (ESCRITA da definição + auditoria)
                                                              │
                                                              ▼
                                              dbServer (gRPC, api/db/v1)  ← tmServer LÊ a definição
                                                              │                (boot + reload periódico)
                                                              ▼
                                                        tmServer (loop single-owner)
                                                     aplica diff: spawn/despawn/mover/atualizar Carry
```

Decisões de posicionamento (todas seguindo os precedentes do repo):

- **A escrita (CRUD de moderador) mora na `web-api`** (`api/web/v1`), um **novo serviço gRPC**
  `NpcAdminService` ao lado de `AccountWebService`. Motivo: moderadores são usuários **web**; a `web-api`
  já é a borda web sobre `internal/store` + `internal/secret`. Não estender `AccountService`
  (`api/db/v1`) — aquele espelha as mensagens legadas G↔DB.
- **A leitura pelo tmServer passa pelo dbServer** (`api/db/v1`, novo RPC `ListNpcDefinitions` /
  `NpcConfigVersion`). Motivo: o tmServer **só tem cliente gRPC do dbServer** (`main.go:135`), nunca fala
  Postgres direto, e degrada sem `-dbserver`. Manter isso: sem dbServer, o tmServer cai no fallback atual
  (NPCs do `NPCGener.txt`), o que preserva o bring-up local.
- **`internal/store`, `internal/migrations`, `internal/domain`** (repo-root) ganham o modelo de NPC,
  reaproveitados por web-api (escrita) e dbServer (leitura), como já fazem para `account`.

## 4. Como uma edição chega no loop que está rodando (o ponto central)

Espelha o padrão *mailbox* do `delivery_queue`, mas para **configuração** em vez de concessão. Duas
peças:

1. **Versão monotônica de configuração.** Uma linha `npc_config_meta(version bigint)` incrementada em
   **toda** escrita de moderador (na mesma transação). Barata de checar.
2. **Poll dentro do loop.** O tmServer já tem um ticker de simulação (`world.onTick`, `world/tick.go`).
   Adiciona-se um **tick lento de reconfiguração** (ex.: a cada N segundos) que:
   - **fora do loop** (`World.Go`, como toda I/O bloqueante) chama `dbServer.NpcConfigVersion`; se
     mudou, chama `ListNpcDefinitions`;
   - **de volta no loop**, aplica o **diff** entre a definição nova e as entidades vivas, tudo no
     goroutine dono — respeitando o single-owner:
     - NPC **desabilitado / removido** → `DespawnMob` da(s) instância(s) daquele NPC.
     - NPC **habilitado / novo** → spawn na posição configurada.
     - NPC **movido** → despawn + respawn na nova célula (ou reposicionamento direto se estático).
     - **Loja alterada** (itens/efeitos) → reescreve o `Carry[]` da entidade merchant viva.
     - **Preço alterado** → atualiza o override de preço consultado pelos handlers `buy`/`sell`.

Isso dá **hot-reload sem restart e sem race**: a web só escreve definição fria; o tmServer continua o
**único** a criar/mutar/destruir entidade viva.

> **Marco de corte:** o **milestone 1** pode aplicar a definição **só no boot** (ler a tabela em
> `spawnNPCs` como *overlay* sobre o `NPCGener.txt`). O **milestone 3** adiciona o tick de reload. Assim
> entrega-se valor cedo (moderar + restart) antes de construir o hot-reload.

### 4.1 Identidade estável e *seed* inicial

Cada NPC editável precisa de um **id durável**. Faz-se um **import one-shot** (subcomando novo no
dbServer, no espírito do `dbserver convert`/`seed-account`) que lê `NPCGener.txt` + os templates e
popula `npc_definition` — um id por bloco. A partir daí o `.txt` vira *bootstrap read-only* e o banco é a
fonte da verdade. Merchants (`Merchant != 0`) são o subconjunto que ganha linhas em `npc_shop_item`.

## 5. Modelo de dados (nova migration `internal/migrations/000N_npc_editing.up.sql`)

Esboço (nomes/colunas a refinar na implementação; `snake_case`, seguindo o estilo das migrations
existentes):

```sql
-- Definição editável de um NPC. Fonte da verdade da CONFIGURAÇÃO; o tmServer é
-- dono só da INSTÂNCIA viva materializada a partir daqui.
CREATE TABLE npc_definition (
  id            bigserial PRIMARY KEY,
  slug          text NOT NULL UNIQUE,        -- id humano estável (ex. "armia-merchant-1")
  template_name text NOT NULL,               -- nome do arquivo em Release/TMsrv/run/npc/
  display_name  text NOT NULL,
  enabled       boolean NOT NULL DEFAULT true,   -- "aparece ou não"
  map_id        integer NOT NULL,            -- mapa/cidade
  pos_x         integer NOT NULL,            -- "onde fica"
  pos_y         integer NOT NULL,
  route_type    smallint NOT NULL DEFAULT 0, -- parado / patrulha
  merchant      smallint NOT NULL DEFAULT 0, -- 0=não-merchant, 1=loja, 2=guarda-carga, 19=shop tipo 3 ...
  updated_by    bigint,                      -- account.id do moderador
  updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Estoque da loja de um NPC merchant. Sobrepõe o Carry[] do template binário.
-- price_override NULL = usa o preço global do catálogo (itemPrices) — comportamento atual.
CREATE TABLE npc_shop_item (
  npc_id         bigint NOT NULL REFERENCES npc_definition(id) ON DELETE CASCADE,
  slot           smallint NOT NULL,          -- 0..26 (MSG_ShopList tem 27 slots)
  item_index     integer NOT NULL,           -- índice no ItemList
  eff1_effect    smallint NOT NULL DEFAULT 0, eff1_value smallint NOT NULL DEFAULT 0,
  eff2_effect    smallint NOT NULL DEFAULT 0, eff2_value smallint NOT NULL DEFAULT 0,
  eff3_effect    smallint NOT NULL DEFAULT 0, eff3_value smallint NOT NULL DEFAULT 0,
  price_override bigint,                      -- NULL = catálogo
  PRIMARY KEY (npc_id, slot)
);

-- Sinal de hot-reload: incrementado em TODA escrita (mesma transação).
CREATE TABLE npc_config_meta (
  id      boolean PRIMARY KEY DEFAULT true CHECK (id),  -- single-row
  version bigint NOT NULL DEFAULT 0
);

-- Trilha de auditoria: quem mudou o quê (moderação = ação privilegiada).
CREATE TABLE npc_audit (
  id         bigserial PRIMARY KEY,
  npc_id     bigint,
  account_id bigint NOT NULL,     -- moderador
  action     text NOT NULL,       -- 'create' | 'update' | 'delete' | 'toggle'
  before     jsonb,
  after      jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
```

Além disso: **papel de moderador**. Adicionar `account.role text NOT NULL DEFAULT 'player'` (ou uma
tabela `account_role`) para a `web-api` autorizar as RPCs de admin. Ver §6.

Modelo relacional correspondente em `internal/domain` (`NpcDefinition`, `NpcShopItem`), no mesmo estilo
de `domain.Account`/`domain.Character`.

## 6. Autenticação e autorização dos moderadores

- **Sessão web** = o mesmo cookie `httpOnly` do `web-platform-plan.md` (login web via
  `VerifyCredentials`). Não reaproveitar o login CPSock do jogo.
- **Autorização** = `web-api` **exige `role in ('moderator','admin')`** em toda RPC de `NpcAdminService`.
  A verificação é **server-side no Go**, nunca só no BFF. Toda mutação grava `npc_audit` com o
  `account_id` do moderador na **mesma transação** que incrementa `npc_config_meta.version`.
- Ponto em aberto herdado: relação com `is_blocked`/binServer — ver §9 de `web-platform-plan.md`. Não
  bloqueia este plano.

## 7. Contratos gRPC (novos)

**`api/web/v1` — `NpcAdminService`** (borda do moderador; `make proto` regenera):

```proto
service NpcAdminService {
  rpc ListNpcs(ListNpcsRequest) returns (ListNpcsResponse);          // vitrine de moderação
  rpc GetNpc(GetNpcRequest) returns (NpcDefinition);
  rpc UpsertNpc(UpsertNpcRequest) returns (UpsertNpcResponse);       // criar/editar posição, enabled, merchant
  rpc SetNpcVisibility(SetNpcVisibilityRequest) returns (Ack);       // "aparece ou não" (atalho comum)
  rpc SetNpcShop(SetNpcShopRequest) returns (Ack);                   // itens + efeitos + price_override
  rpc DeleteNpc(DeleteNpcRequest) returns (Ack);
}
```

Resultados de negócio (slug já existe, item inválido, sem permissão) viajam **no corpo** como enums/flags
(padrão de `CreateResult` em `web.proto`), não como códigos de erro gRPC — só falha de infra vira `error`.
Todo request de admin carrega o `account_id`/token do moderador para a checagem de papel + auditoria.

**`api/db/v1` — leitura pelo tmServer** (dois RPCs enxutos):

```proto
rpc NpcConfigVersion(NpcConfigVersionRequest) returns (NpcConfigVersionResponse); // só o int; poll barato
rpc ListNpcDefinitions(ListNpcDefinitionsRequest) returns (ListNpcDefinitionsResponse); // snapshot completo
```

## 8. Plano de execução por marcos

**Milestone 0 — Modelo & seed (sem gameplay novo).**
- Migration `000N_npc_editing` + tipos em `internal/domain`.
- Queries em `internal/store` (CRUD de `npc_definition`/`npc_shop_item`, bump de versão, auditoria) com
  testes `-tags=integration` no padrão de `store_integration_test.go`.
- Subcomando `dbserver import-npcs`: lê `NPCGener.txt` + templates → popula `npc_definition`
  (+`npc_shop_item` para merchants). Idempotente por `slug`.

**Milestone 1 — Overlay no boot (valor imediato).**
- `dbServer`: `ListNpcDefinitions`.
- `tmServer`: em `spawnNPCs`, quando `-dbserver` está setado, **preferir** a definição do banco sobre o
  `.txt` (posição, `enabled`, merchant Carry via `npc_shop_item`). Sem dbServer → comportamento atual.
- Handlers `buy`/`sell`: consultar `price_override` do NPC antes de cair no `itemPrices` global.
- Entrega: moderar exige restart, mas já é 100% via banco/web.

**Milestone 2 — CRUD do moderador (web-api).**
- `NpcAdminService` + autorização por papel + `npc_audit`.
- Validação: item existe no catálogo; `slot` 0..26; coords dentro do mapa; `merchant` num conjunto
  conhecido.

**Milestone 3 — Hot-reload (sem restart).**
- `dbServer`: `NpcConfigVersion`.
- `tmServer`: tick lento de reconfiguração (`World.Go` para o poll fora do loop; aplicação do **diff**
  dentro do loop: spawn/despawn/mover/atualizar `Carry`/preço). Testes de mundo (`-race`) cobrindo o
  diff e a invariância single-owner (nenhuma mutação de entidade fora do loop).

**Milestone 4 — Refinamentos.**
- Preço **por-NPC** default (não só override por item).
- Histórico/rollback via `npc_audit`.
- Métricas/observabilidade das reconfigurações (contagem de spawn/despawn por reload).

## 9. Pontos em aberto (decidir antes de construir)

1. **Preço: global vs por-NPC.** Hoje o preço é global por índice de item (`itemPrices`). O plano
   introduz `price_override` **por slot de NPC** — confirmar que é isso que os moderadores querem (vs.
   editar o catálogo global). Recomendação: manter override por-NPC (mais localizado e reversível).
2. **Granularidade de "onde fica".** `map_id + pos_x/pos_y` cobre NPC estático. Para NPCs **patrulhando**
   (waypoints do `NPCGener.txt`), decidir se os moderadores editam waypoints ou só o ponto de spawn.
   Recomendação: milestone inicial só ponto de spawn; waypoints depois.
3. **Instância única vs grupo.** Um bloco `NPCGener` pode spawnar líder + seguidores. Definir se a
   edição de moderador opera por **bloco** (grupo) ou por **entidade**. Recomendação: por definição
   (bloco), que é a unidade natural do conteúdo.
4. **Validação de `merchant` / tipos de loja.** O mapeamento `Merchant → ShopType` (1→1, 19→3, 2=guarda-
   carga) está **UNVERIFIED** no código (`handler/shop.go:23`). Fixar o conjunto permitido antes de expor
   na UI de moderação.
5. **Autoridade de spawn no boot.** O boot atual popula tudo do `.txt` (divergência deliberada,
   `main.go:216`). Com o overlay do banco, decidir a precedência exata quando um bloco existe no `.txt`
   mas não no banco (e vice-versa). Recomendação: banco vence quando presente; `.txt` é só o seed.

## 10. Relação com o resto da plataforma

Este plano é **irmão** de `web-platform-plan.md` e reusa toda a sua infraestrutura (web-api, mTLS,
`internal/store`, hashing/sessão, o padrão mailbox). A diferença conceitual — e por que é mais seguro que
a loja-web — é que **definição de NPC não é estado de jogador**: o fluxo é de via única
(web escreve config fria; tmServer materializa), sem o risco de clobber que exige o `delivery_queue` para
concessões de item/cash.
