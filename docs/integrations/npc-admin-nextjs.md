# Integração Next.js ↔ web-api: edição de NPCs (moderação)

> Guia de integração para o **projeto Next.js** consumir a funcionalidade de **edição de NPCs por
> moderadores** (visibilidade, posição e loja/preço). Fonte da verdade do contrato:
> `api/web/v1/web.proto` (serviço `NpcAdminService`). Contexto de arquitetura: `docs/migration/npc-editing-plan.md`
> e `docs/migration/web-platform-plan.md`.

## 1. Topologia (o que o Next.js fala com o quê)

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │  só server-side; cookie de sessão httpOnly
                        │ gRPC + mTLS
                        ▼
                     web-api (serviço Go, :7600)  ──> Postgres (escreve config fria de NPC)
                                                   ──> (o tmServer lê e materializa no jogo)
```

Regras que **não** mudam:

- **O browser nunca fala gRPC nem vê certificado mTLS.** Todas as chamadas ao `web-api` saem do
  server-side do Next.js (Route Handlers / Server Actions).
- O Next.js é **BFF puro**: guarda apenas um cookie de sessão `httpOnly` e traduz REST/HTTP ⇄ gRPC.
- O `web-api` é a **autoridade** de autorização e validação. O front pode espelhar validações para UX,
  mas nunca confiar nelas para segurança.

## 2. Pré-requisitos

1. **web-api no ar** (`webserver`, porta `:7600`) com mTLS — mesmos certs dos links internos
   (`internal/secure`). O Next.js precisa do par cliente (cert/key) + CA para conectar.
2. **Migrations aplicadas** (`0005_npc_editing`) — sobem automaticamente quando o `web-api`/`dbserver`
   iniciam.
3. **Conta de moderador**: a coluna `account.role` precisa ser `'moderator'` ou `'admin'`. **Não há RPC
   para promover** — é operação de DBA/seed (ex.: `UPDATE account SET role='moderator' WHERE name=...`).
   Contas normais têm `role='player'` e recebem `ADMIN_RESULT_FORBIDDEN`.
4. **Seed dos NPCs** (uma vez): `dbserver import-npcs -content <Release/> -dsn <dsn>` popula os NPCs
   merchant a partir do `NPCGener.txt`. Antes disso, `ListNpcs` volta vazio.
5. **Overlay ligado no tmServer**: `W2PP_NPC_EDITING=true` (senão as edições ficam só no banco e não
   aparecem no jogo). Ver §7.

## 3. Autenticação e sessão do moderador

Reaproveita o **mesmo `AccountWebService`** que o Next.js já usa para login web (ver
`web-platform-plan.md`):

- `VerifyCredentials(name, password)` → `{ ok, account_id, blocked, role }`.
  - `role` é `account.role`: `"player"` | `"moderator"` | `"admin"`.
- Em caso de `ok`, o BFF cria o **cookie de sessão `httpOnly`** guardando (ou referenciando) o
  `account_id` **e** o `role`.

**`account_id` da sessão = `moderator_id`** de toda RPC do `NpcAdminService`. Regras:

- O BFF **sempre deriva `moderator_id` do cookie de sessão**, nunca do corpo enviado pelo browser.
- **Gate da página de edição de NPC:** `role ∈ { "moderator", "admin" }`. Esse flag é **só para UX**
  (mostrar/esconder o menu e a rota). **Não é a decisão de autorização** — o `NpcAdminService` revalida
  o papel server-side em **toda** chamada, então uma sessão adulterada ainda recebe
  `ADMIN_RESULT_FORBIDDEN`. Trate o `role` da sessão como uma dica de UI, nunca como permissão.
- **Setar quem é moderador (fase inicial):** direto no banco — não há RPC de promoção. Ex.:
  `UPDATE account SET role = 'moderator' WHERE name = 'fulano';`. O `role` só passa a valer no próximo
  login (é lido no `VerifyCredentials`); se precisar refletir na hora, invalide a sessão do usuário.
- Não reaproveitar o login CPSock do jogo — é outro mundo (`web-platform-plan.md §Autenticação`).

## 4. Contrato — `web.v1.NpcAdminService`

Todas as RPCs recebem `moderator_id` (int64). Resultados de negócio viajam **no corpo** via enum
`AdminResult`; só falha de infraestrutura vira erro gRPC.

### 4.1 Enum de resultado

| `AdminResult`               | Significado                                   | HTTP sugerido no BFF |
|-----------------------------|-----------------------------------------------|----------------------|
| `ADMIN_RESULT_OK` (1)       | sucesso                                        | 200                  |
| `ADMIN_RESULT_FORBIDDEN` (2)| chamador não é `moderator`/`admin`             | 403                  |
| `ADMIN_RESULT_INVALID` (3)  | validação falhou (slot, merchant, campos…)     | 400 / 422            |
| `ADMIN_RESULT_NOT_FOUND` (4)| NPC alvo não existe                            | 404                  |
| `ADMIN_RESULT_CONTENT_OWNED` (5)| NPC é conteúdo versionado (`origin = "content"`) — deve ser **escondido**, não apagado (§5.7) | 409 |
| `ADMIN_RESULT_UNSPECIFIED`(0)| não deve ocorrer                              | 500                  |

Erros gRPC (ex.: `UNAVAILABLE`, `INTERNAL`) = falha de infraestrutura → 502/500 no BFF.

> **Não trate um valor desconhecido como erro genérico.** `ADMIN_RESULT_CONTENT_OWNED` é um resultado
> *esperado* de `DeleteNpc` na esmagadora maioria dos NPCs; um mapa `AdminResult → HTTP` que não o
> conheça o joga em 500 e o moderador vê "erro interno" sem saber o que fazer — foi exatamente o bug
> da issue #257. Ver §5.7.

### 4.2 RPCs

| RPC | Request (campos além de `moderator_id`) | Response | Uso |
|-----|------------------------------------------|----------|-----|
| `ListNpcs` | — | `{ result, npcs: AdminNpc[] }` | tabela de moderação |
| `GetNpc` | `npc_id` | `{ result, npc: AdminNpc }` | detalhe (com loja) |
| `UpsertNpc` | `slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant` | `{ result, npc_id }` | criar/editar definição |
| `SetNpcVisibility` | `npc_id, enabled` | `AdminAck{ result }` | atalho "aparece ou não" |
| `SetNpcShop` | `npc_id, items: AdminNpcShopItem[]` | `AdminAck{ result }` | substituir a loja inteira **e também remover item** (não há RPC separada de remoção — ver §5.2) |
| `SetItemPrice` | `item_index, price` | `AdminAck{ result }` | preço **global** do item |
| `DeleteNpc` | `npc_id` | `AdminAck{ result }` | remover definição |
| `ListMerchantTemplates` | — | `{ result, templates: MerchantTemplate[] }` | combobox de `template_name` no formulário |
| `ListItemCatalog` | — | `{ result, items: ItemCatalogEntry[] }` | combobox de `item_index` na loja/preço |
| `ListItemPrices` | — | `{ result, prices: ItemPrice[] }` | overrides de preço global atuais, para popular a coluna/edição de preço |
| `ListMapZones` | — | `{ result, zones: MapZone[] }` | combobox de `map_id` no formulário |

### 4.3 Mensagens

```proto
message AdminNpc {
  int64  id            = 1;  // id numérico da definição (use em npc_id)
  string slug          = 2;  // chave humana estável (ex. "Karkarian-42")
  string template_name = 3;  // arquivo do template em Release/TMsrv/run/npc/
  string display_name  = 4;
  bool   enabled       = 5;  // "aparece ou não"
  int32  map_id        = 6;  // carregado, mas o jogo posiciona só por x/y (grid único)
  int32  pos_x         = 7;  // "onde fica" (ponto de spawn)
  int32  pos_y         = 8;
  int32  route_type    = 9;  // 0 = parado
  int32  merchant      = 10; // ver §5.1
  repeated AdminNpcShopItem shop = 11; // preenchido em GetNpc/ListNpcs
  string origin          = 12; // "content" (veio do NPCGener.txt) | "custom" (criado por moderador) — ver §5.7
  int32  generator_index = 13; // linha de origem no NPCGener.txt; -1 quando não aplicável
}

message AdminNpcShopItem {
  int32 slot       = 1;  // índice de loja 0..26 (ver §5.2)
  int32 item_index = 2;  // índice no ItemList (> 0)
  int32 eff1 = 3; int32 effv1 = 4;  // efeitos opcionais (0 = sem efeito)
  int32 eff2 = 5; int32 effv2 = 6;
  int32 eff3 = 7; int32 effv3 = 8;
  int32 quantity = 9; // quantidade vendida no slot; 0 é tratado como 1
}

// Um template merchant encontrado em Release/TMsrv/run/npc/ (CurrentScore.Merchant
// != 0). Alimenta o combobox de template_name do formulário (ver §5.5).
message MerchantTemplate {
  string template_name = 1;  // valor exato para UpsertNpc.template_name (sem .txt)
  string display_name  = 2;  // mob.Name do template, para exibir na UI
  int32  merchant      = 3;  // ver §5.1 (1, 2, 19, 100…)
}
message ListMerchantTemplatesRequest { int64 moderator_id = 1; }
message ListMerchantTemplatesResponse {
  AdminResult result = 1;
  repeated MerchantTemplate templates = 2;
}

// Uma linha de Release/Common/ItemList.csv, só o que o picker de item_index e o
// ícone precisam (o resto da linha — preço, efeitos — não viaja aqui).
// A chave real vem de itemicon.bin; os demais campos dirigem busca e fallback.
message ItemCatalogEntry {
  int32 item_index = 1;
  string name = 2;           // nome cru, com "_"
  string icon_key = 3;       // "iNNNN"; vazio = fallback
  string display_name = 4;   // name com "_" virando espaço — é o que se exibe
  int32 slot_mask = 5;       // nPos: bitmask sobre Equip[16]; 0 = não equipável
  repeated string slots = 6; // slot_mask decodificado ("boots", "weapon", …)
  int32 grade = 7;           // 1=Normal 2=Místico 3=Arcano 4=Lendário
  int32 mesh = 8;
  int32 texture = 9;
  string icon_url = 10;      // URL pública HTTPS; vazio = fallback
}
message ListItemCatalogRequest { int64 moderator_id = 1; }
message ListItemCatalogResponse {
  AdminResult result = 1;
  repeated ItemCatalogEntry items = 2;
  string catalog_version = 3;
  string icon_pack_version = 4;
}

// Override de preço global de um item (tabela item_price). Item ausente desta
// lista = sem override, vale o preço-base do catálogo do jogo.
message ItemPrice {
  int32 item_index = 1;
  int64 price = 2;
}
message ListItemPricesRequest { int64 moderator_id = 1; }
message ListItemPricesResponse {
  AdminResult result = 1;
  repeated ItemPrice prices = 2;
}

// Uma das 5 zonas fixas de cidade (mesma ordem da tabela `cities` do tmServer).
// Só rótulo: map_id não tem efeito de jogo hoje (mundo roda num grid único).
message MapZone {
  int32 id = 1;
  string name = 2;
}
message ListMapZonesRequest { int64 moderator_id = 1; }
message ListMapZonesResponse {
  AdminResult result = 1;
  repeated MapZone zones = 2;
}
```

> `UpsertNpc` **não** edita a loja — use `SetNpcShop`. `SetItemPrice` é **global por item**, não por NPC.
> Para vender packs (ex.: 120 entradas), envie `quantity: 120`; não envie `EF_AMOUNT`/efeito `61`
> manualmente nos campos `eff*`, porque o servidor materializa esse efeito no item legado.

## 5. Semântica de domínio (o que a UI precisa saber)

### 5.1 `merchant` (tipo do NPC)

Conjunto aceito (outros → `ADMIN_RESULT_INVALID`):

| valor | significado |
|------:|-------------|
| `0`   | não-merchant (sem loja) |
| `1`   | loja normal |
| `2`   | guarda-carga (abre o armazém, não uma lista de compra) |
| `19`  | loja tipo 3 |
| `100` | NPC de quest |

A **loja** (`SetNpcShop`) só faz sentido para NPCs merchant (tipicamente `1`/`19`).

### 5.2 Loja: slots 0..26 = **3 abas de 9**

O `slot` é o **índice de exibição** do `MSG_ShopList`, de `0` a `26`, organizado em **3 abas de 9 itens**:

- **Aba 1:** slots `0..8`
- **Aba 2:** slots `9..17`
- **Aba 3:** slots `18..26`

A UI deve apresentar exatamente assim. O backend cuida do mapeamento interno para o inventário do NPC;
o front trabalha só com `0..26`. Regras de validação (espelhe para UX):

- `slot` único e em `[0, 26]`;
- `item_index > 0`;
- `quantity` ausente/`0` vira `1`; valores válidos explícitos ficam em `[1, 255]`;
- não envie `EF_AMOUNT`/efeito `61` em `eff1..eff3`; ele é derivado de `quantity`;
- `SetNpcShop` **substitui a loja inteira** — envie **todos** os itens que devem existir; slots omitidos
  ficam vazios (loja vazia = NPC não vende nada).

**Removendo item(ns):** não existe RPC separada de remoção — é a **mesma** `SetNpcShop`:

- **Remover um item específico:** monte `items` a partir do estado atual da loja (de `GetNpc`/`ListNpcs`,
  campo `shop`) **excluindo** o slot que o moderador removeu, e chame `SetNpcShop` com esse array. Não
  basta enviar só o item removido nem só os itens "novos" — o array precisa refletir o estado final
  completo da loja.
- **Remover todos os itens (esvaziar a loja):** chame `SetNpcShop` com `items: []`.

### 5.3 Preço é **global por item**

Não existe preço por-NPC. `SetItemPrice(item_index, price)`:

- `price >= 0` → define o preço global daquele item (vale em **todos** os NPCs que o vendem);
- `price < 0` → **limpa** o override e o item volta ao preço do catálogo do jogo.

`ListItemPrices()` retorna a lista atual de overrides (`item_index, price`) para a UI popular a
coluna/edição de preço na tabela de itens sem precisar rastrear cada `SetItemPrice` chamado
anteriormente (mesma semântica de ausência/limpeza acima). Assim como `ListItemCatalog`, essa lista
não depende do NPC — junte com `ListItemCatalog`/`GetNpc` no cliente por `item_index` para montar a
coluna de preço da tabela de itens.

### 5.4 Propagação para o jogo (não é instantâneo)

A escrita vai para o Postgres na hora, mas o mundo do jogo só reflete quando o **tmServer recarrega**:

- **no boot**, e
- por **poll periódico (~15s)** enquanto roda.

Ou seja: comunique ao moderador algo como *"a alteração aparece no jogo em alguns segundos"*. Requer o
overlay ligado (`W2PP_NPC_EDITING=true`). Sem isso, a edição fica só no banco.

### 5.5 Combobox de `template_name` (`ListMerchantTemplates`)

`template_name` é texto livre no `UpsertNpc`, mas precisa bater **exatamente** com um arquivo em
`Release/TMsrv/run/npc/` (sem `.txt`). Um valor errado não dá erro nenhum na hora — o Postgres aceita a
definição, mas o tmServer não spawna o NPC (`npc template resolve failed` / `npc definition has no
template — skipped` só nos logs do servidor). `ListMerchantTemplates` existe para eliminar essa classe de
erro:

- Retorna só os templates com `merchant != 0` (mesmo filtro que `dbserver import-npcs` usa para decidir
  o que é merchant), escaneados **uma vez no boot** do web-api a partir de `-content`/`W2PP_CONTENT` — não
  há leitura de disco por request.
- **Vazio não é erro:** se o operador não configurou `-content` no web-api, a resposta vem `result = OK`
  com `templates = []`. A UI deve tratar isso como "picker indisponível" e cair no campo de texto manual
  (com o aviso de validação), não como falha.
- UX sugerida: combobox pesquisável por `template_name` ou `display_name`; ao selecionar, preencher
  `template_name` (exato) e sugerir `merchant` (o form ainda permite o moderador trocar, já que o mesmo
  template pode ser reaproveitado com outro `merchant` intencionalmente). Manter a opção "digitar
  manualmente" colapsada para o caso raro de um template ainda não coberto pelo scan.

### 5.6 Combobox de `item_index` (`ListItemCatalog`) e de `map_id` (`ListMapZones`)

Os mesmos dois problemas de digitação existem em outros dois campos numéricos do formulário:

- **`item_index`** (usado em `SetNpcShop.items[].item_index` e em `SetItemPrice.item_index`): um índice
  errado não é validado contra o catálogo — o backend aceita qualquer `item_index > 0` (§5.2/§5.3). Um
  valor que não existe em `Release/Common/ItemList.csv` faz o NPC "vender" um item inexistente/errado no
  jogo, sem aviso nenhum na hora de salvar.
- **`map_id`** (usado em `UpsertNpc.map_id`): hoje é só um rótulo — o mundo roda num **grid único**, então
  o spawn efetivo depende só de `pos_x`/`pos_y` (§9.2 de `npc-editing-plan.md`). Ainda assim vale
  padronizar o valor (evita um `map_id` aleatório/inconsistente entre NPCs da mesma cidade).

Ambas as RPCs seguem exatamente o mesmo contrato de `ListMerchantTemplates`:

- `ListItemCatalog`: escaneado **uma vez no boot** do web-api a partir do mesmo `-content`/`W2PP_CONTENT`
  (não é uma flag nova — reaproveita a mesma configuração). `items = []` quando `-content` não foi setado;
  UI cai no campo `item_index` numérico manual. Catálogo é grande (~3200 entradas) — carregar uma vez no
  client e filtrar localmente (combobox pesquisável por nome), sem round-trip a cada tecla.
  Cada entrada também carrega `icon_key`/`display_name`/`slots`/`grade`, então o combobox pode mostrar
  ícone e raridade em vez de só o índice — ver `docs/integrations/item-icons-nextjs.md`. Filtre por
  `name` (cru), exiba `display_name`.
- `ListMapZones`: **não depende de `-content`** — é uma tabela fixa de 5 zonas (`0 Armia … 4 Noatum`),
  sempre retornada (nunca vazia). Simples `<select>` de 5 opções, sem necessidade de busca.

### 5.7 `origin` — NPC de conteúdo **não pode ser apagado**, só escondido

Cada definição carrega um `origin` (§4.3, campo 12):

| `origin` | de onde veio | `DeleteNpc` |
|----------|--------------|-------------|
| `"content"` | importado do `NPCGener.txt` por `dbserver import-npcs` — é conteúdo versionado do jogo | **sempre recusado** com `ADMIN_RESULT_CONTENT_OWNED` |
| `"custom"`  | criado pelo moderador no painel (`UpsertNpc`) | permitido |

A regra é intencional: apagar um NPC do `NPCGener.txt` no banco não o remove do conteúdo — o próximo
`import-npcs` o traria de volta, e nesse meio-tempo o banco e o `Release/` ficariam divergentes. A
saída pretendida é **desabilitar**: `SetNpcVisibility(npc_id, enabled=false)` tira o NPC do mundo
mantendo a definição.

**Isso não é um caso de canto.** Na base atual são **567 de 572 definições** com `origin = "content"`
(~99%). Um painel que mostra o botão de excluir para todos convida a uma ação que falha em quase
todos os cliques.

Duas coisas a fazer na UI:

1. **Não oferecer a ação impossível.** `origin` já vem no payload de `ListNpcs`/`GetNpc` — esconda ou
   desabilite o botão de excluir quando `npc.origin === "content"`, deixando só o toggle de
   visibilidade. É decisão puramente de UI, sem round-trip extra.
2. **Explicar quando acontecer mesmo assim** (link direto, chamada por script, corrida com outro
   moderador): traduza `ADMIN_RESULT_CONTENT_OWNED` (5) numa mensagem que aponte a saída, não num
   erro genérico. Texto sugerido:

   > Este NPC faz parte do conteúdo do jogo e não pode ser excluído. Desabilite-o para removê-lo do mundo.

Do lado do servidor, uma recusa dessas é registrada como
`delete npc refused: content-owned npc_id=… moderator_id=…` no log do `webserver` — útil para
confirmar que a rejeição chegou ao backend (ela não deixa linha de auditoria, porque nada foi
escrito).

## 6. Camada BFF (Next.js) — como implementar

O browser fala com **rotas REST do próprio Next.js**; essas rotas (server-side) chamam o `web-api` por
gRPC+mTLS. Sugestão de mapeamento REST → RPC:

| Método + rota (exemplo) | RPC |
|-------------------------|-----|
| `GET /api/admin/npcs` | `ListNpcs` |
| `GET /api/admin/npcs/:id` | `GetNpc` |
| `POST /api/admin/npcs` / `PUT /api/admin/npcs/:id` | `UpsertNpc` |
| `PATCH /api/admin/npcs/:id/visibility` | `SetNpcVisibility` |
| `PUT /api/admin/npcs/:id/shop` | `SetNpcShop` |
| `PUT /api/admin/items/:index/price` | `SetItemPrice` |
| `GET /api/admin/item-prices` | `ListItemPrices` |
| `DELETE /api/admin/npcs/:id` | `DeleteNpc` |
| `GET /api/admin/npc-templates` | `ListMerchantTemplates` |
| `GET /api/admin/items` | `ListItemCatalog` |
| `GET /api/admin/map-zones` | `ListMapZones` |

Esqueleto de um Route Handler (TypeScript, server-side; `@grpc/grpc-js` + stubs gerados do `.proto`):

```ts
// lib/npcAdminClient.ts  (SERVER-ONLY — nunca importar no client)
import { credentials } from "@grpc/grpc-js";
import fs from "node:fs";
import { NpcAdminServiceClient } from "@/gen/web/v1/web_grpc_pb"; // gerado do web.proto

const ssl = credentials.createSsl(
  fs.readFileSync(process.env.WEB_API_CA!),      // CA
  fs.readFileSync(process.env.WEB_API_CLIENT_KEY!),
  fs.readFileSync(process.env.WEB_API_CLIENT_CERT!),
);
export const npcAdmin = new NpcAdminServiceClient(process.env.WEB_API_ADDR!, ssl); // ex. "web-api:7600"
```

```ts
// app/api/login/route.ts  — grava account_id + role na sessão
import { NextResponse } from "next/server";
import { accountWeb } from "@/lib/accountWebClient"; // AccountWebServiceClient (mesmo padrão mTLS)
import { createSession } from "@/lib/session";

export async function POST(req: Request) {
  const { name, password } = await req.json();
  const r = await new Promise<any>((resolve, reject) =>
    accountWeb.verifyCredentials({ name, password }, (e: unknown, x: unknown) => (e ? reject(e) : resolve(x))),
  ).catch(() => null);

  if (!r) return NextResponse.json({ error: "upstream" }, { status: 502 });
  if (!r.ok) return NextResponse.json({ error: "invalid_credentials" }, { status: 401 });
  if (r.blocked) return NextResponse.json({ error: "blocked" }, { status: 403 });

  await createSession({ accountId: r.accountId, role: r.role }); // cookie httpOnly
  const isModerator = r.role === "moderator" || r.role === "admin";
  return NextResponse.json({ accountId: r.accountId, role: r.role, isModerator });
}
```

```ts
// app/api/admin/npcs/route.ts
import { NextResponse } from "next/server";
import { npcAdmin } from "@/lib/npcAdminClient";
import { getSession } from "@/lib/session"; // lê o cookie httpOnly → { accountId }

// Cobre TODOS os valores do enum (§4.1) — um valor faltando aqui vira 500 e a UI
// perde o motivo real da recusa (foi o bug da #257 com o 5).
const httpFor = (r: number) => ({ 1: 200, 2: 403, 3: 422, 4: 404, 5: 409 } as const)[r] ?? 500;

export async function GET() {
  const session = await getSession();
  if (!session) return NextResponse.json({ error: "unauthenticated" }, { status: 401 });

  const resp = await new Promise<any>((resolve, reject) =>
    npcAdmin.listNpcs({ moderatorId: session.accountId }, (err: unknown, r: unknown) =>
      err ? reject(err) : resolve(r)),
  ).catch(() => null);

  if (!resp) return NextResponse.json({ error: "upstream" }, { status: 502 }); // erro gRPC = infra
  const status = httpFor(resp.result);
  if (status !== 200) return NextResponse.json({ result: resp.result }, { status });
  return NextResponse.json({ npcs: resp.npcs });
}
```

```ts
// app/api/admin/npcs/[id]/route.ts — DELETE, com o caso content-owned (§5.7)
import { NextResponse } from "next/server";
import { npcAdmin } from "@/lib/npcAdminClient";
import { getSession } from "@/lib/session";

export async function DELETE(_req: Request, { params }: { params: { id: string } }) {
  const session = await getSession();
  if (!session) return NextResponse.json({ error: "unauthenticated" }, { status: 401 });

  const resp = await new Promise<any>((resolve, reject) =>
    npcAdmin.deleteNpc({ moderatorId: session.accountId, npcId: Number(params.id) }, (err: unknown, r: unknown) =>
      err ? reject(err) : resolve(r)),
  ).catch(() => null);

  if (!resp) return NextResponse.json({ error: "upstream" }, { status: 502 });
  if (resp.result === 5 /* ADMIN_RESULT_CONTENT_OWNED */) {
    // Não é falha do servidor: o NPC veio do NPCGener.txt. A UI deve mostrar a
    // mensagem e oferecer o toggle de visibilidade no lugar (§5.7).
    return NextResponse.json({ result: resp.result, error: "content_owned" }, { status: 409 });
  }
  const status = httpFor(resp.result);
  if (status !== 200) return NextResponse.json({ result: resp.result }, { status });
  return NextResponse.json({ ok: true });
}
```

```ts
// app/api/admin/npc-templates/route.ts — alimenta o combobox de template_name (§5.5)
import { NextResponse } from "next/server";
import { npcAdmin } from "@/lib/npcAdminClient";
import { getSession } from "@/lib/session";

export async function GET() {
  const session = await getSession();
  if (!session) return NextResponse.json({ error: "unauthenticated" }, { status: 401 });

  const resp = await new Promise<any>((resolve, reject) =>
    npcAdmin.listMerchantTemplates({ moderatorId: session.accountId }, (err: unknown, r: unknown) =>
      err ? reject(err) : resolve(r)),
  ).catch(() => null);

  if (!resp) return NextResponse.json({ error: "upstream" }, { status: 502 });
  if (resp.result === 2 /* ADMIN_RESULT_FORBIDDEN */) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }
  // templates: [] é resposta válida (web-api sem -content configurado) — o form
  // deve cair no campo de texto manual, não tratar como erro.
  return NextResponse.json({ templates: resp.templates });
}
```

`GET /api/admin/items` (`ListItemCatalog`) e `GET /api/admin/map-zones` (`ListMapZones`) seguem o
**mesmo esqueleto** acima, só trocando `npcAdmin.listMerchantTemplates(...)` por
`npcAdmin.listItemCatalog(...)` / `npcAdmin.listMapZones(...)` e o campo da resposta (`items` / `zones`).
`zones` nunca vem vazio (tabela fixa, não depende de `-content`); `items` vazio tem o mesmo significado de
`templates` vazio — `-content` não configurado, caia no campo numérico manual.

Pontos importantes do BFF:

- **`moderatorId` vem SEMPRE da sessão** (`session.accountId`), nunca do corpo do request.
- Mapear `AdminResult` → HTTP (tabela §4.1), **incluindo o `5` (`CONTENT_OWNED` → 409)**, que a UI
  precisa traduzir numa mensagem específica e não num erro genérico (§5.7). Rejeição de gRPC
  (promise reject) = falha de infra → 502.
- Cliente gRPC é **singleton server-side**; nunca exposto ao browser.
- Cookie de sessão `httpOnly` + proteção CSRF nas rotas mutantes (POST/PUT/PATCH/DELETE).

## 7. Geração dos stubs

O time do Next.js gera os stubs a partir de **`api/web/v1/web.proto`** (peça o arquivo ou aponte para
este repo). Pacote proto: `web.v1`; serviços: `AccountWebService` (login) e `NpcAdminService` (esta
feature). Ferramentas usuais: `@grpc/grpc-js` + `grpc-tools`/`ts-proto`, ou `buf generate`.

## 8. Variáveis de ambiente (Next.js server-side)

| Var | Descrição |
|-----|-----------|
| `WEB_API_ADDR` | host:porta do web-api (ex. `web-api:7600`) |
| `WEB_API_CA` | caminho do CA (PEM) que valida o web-api |
| `WEB_API_CLIENT_CERT` / `WEB_API_CLIENT_KEY` | par cliente (PEM) para o mTLS |
| `SESSION_SECRET` | assinatura/criptografia do cookie de sessão |

> **Nota (lado backend, não Next.js):** `ListMerchantTemplates` **e** `ListItemCatalog` só retornam dados
> se o **próprio `webserver`** (web-api) tiver sido iniciado com `-content <Release/>` (ou
> `W2PP_CONTENT`), o mesmo diretório montado no `tmServer` — é a mesma flag para as duas RPCs, não precisa
> configurar duas vezes. Sem essa flag no `webserver`, ambas respondem `result = OK` com lista vazia — não
> é um erro de configuração do Next.js, é do serviço Go. `ListMapZones` **não** depende dessa flag (tabela
> fixa). Ver `docker-compose.yaml` (serviço `webserver`).

## 9. Checklist de integração

- [ ] Gerar stubs de `web.proto` (`web.v1.NpcAdminService`).
- [ ] Cliente gRPC+mTLS server-side (singleton), certs via env.
- [ ] Login via `AccountWebService.VerifyCredentials` → cookie `httpOnly` guardando `account_id` **e** `role`.
- [ ] Gate da página/menu de NPC por `role ∈ { moderator, admin }` (só UX — o web-api reautoriza).
- [ ] Toda rota admin deriva `moderator_id` da sessão (nunca do corpo).
- [ ] Mapear `AdminResult` → HTTP (os **6** valores, incluindo `ADMIN_RESULT_CONTENT_OWNED` = 5 → 409);
      tratar reject de gRPC como 502.
- [ ] Botão de excluir escondido/desabilitado quando `npc.origin === "content"` (§5.7) — sobra só o
      toggle de visibilidade, que é a ação suportada para ~99% dos NPCs.
- [ ] `ADMIN_RESULT_CONTENT_OWNED` traduzido em mensagem específica ("faz parte do conteúdo do jogo…
      desabilite-o", §5.7), nunca em "erro interno".
- [ ] UI da loja em 3 abas de 9 (slots 0..8 / 9..17 / 18..26); `SetNpcShop` envia a loja inteira.
- [ ] Cada item da loja tem um controle de remover (ex.: botão "x" por slot) que, ao salvar, tira aquele
      slot do array de `items` antes de chamar `SetNpcShop` (§5.2) — sem isso não há como excluir um item
      já adicionado.
- [ ] Preço via `SetItemPrice` (global; `price < 0` limpa); tabela/coluna de preço populada via
      `ListItemPrices` (item ausente = preço do catálogo, sem override).
- [ ] Avisar que a mudança aparece no jogo em ~alguns segundos (poll do tmServer).
- [ ] Garantir que o operador setou `account.role` do moderador e rodou `import-npcs`.
- [ ] Moderador vê um combobox pesquisável de templates merchant válidos (`ListMerchantTemplates`,
      §5.5) em vez de digitar `template_name` à mão; `templates = []` cai no campo manual, não é erro.
- [ ] Moderador vê um combobox pesquisável de itens (`ListItemCatalog`, §5.6) em vez de digitar
      `item_index` à mão na loja/preço; `items = []` cai no campo numérico manual, não é erro.
- [ ] Moderador vê um `<select>` com os nomes das 5 cidades (`ListMapZones`, §5.6) em vez de digitar
      `map_id` à mão.
