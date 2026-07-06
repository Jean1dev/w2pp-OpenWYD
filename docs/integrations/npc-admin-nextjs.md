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
| `ADMIN_RESULT_UNSPECIFIED`(0)| não deve ocorrer                              | 500                  |

Erros gRPC (ex.: `UNAVAILABLE`, `INTERNAL`) = falha de infraestrutura → 502/500 no BFF.

### 4.2 RPCs

| RPC | Request (campos além de `moderator_id`) | Response | Uso |
|-----|------------------------------------------|----------|-----|
| `ListNpcs` | — | `{ result, npcs: AdminNpc[] }` | tabela de moderação |
| `GetNpc` | `npc_id` | `{ result, npc: AdminNpc }` | detalhe (com loja) |
| `UpsertNpc` | `slug, template_name, display_name, enabled, map_id, pos_x, pos_y, route_type, merchant` | `{ result, npc_id }` | criar/editar definição |
| `SetNpcVisibility` | `npc_id, enabled` | `AdminAck{ result }` | atalho "aparece ou não" |
| `SetNpcShop` | `npc_id, items: AdminNpcShopItem[]` | `AdminAck{ result }` | substituir a loja inteira |
| `SetItemPrice` | `item_index, price` | `AdminAck{ result }` | preço **global** do item |
| `DeleteNpc` | `npc_id` | `AdminAck{ result }` | remover definição |

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
}

message AdminNpcShopItem {
  int32 slot       = 1;  // índice de loja 0..26 (ver §5.2)
  int32 item_index = 2;  // índice no ItemList (> 0)
  int32 eff1 = 3; int32 effv1 = 4;  // efeitos opcionais (0 = sem efeito)
  int32 eff2 = 5; int32 effv2 = 6;
  int32 eff3 = 7; int32 effv3 = 8;
}
```

> `UpsertNpc` **não** edita a loja — use `SetNpcShop`. `SetItemPrice` é **global por item**, não por NPC.

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
- `SetNpcShop` **substitui a loja inteira** — envie **todos** os itens que devem existir; slots omitidos
  ficam vazios (loja vazia = NPC não vende nada).

### 5.3 Preço é **global por item**

Não existe preço por-NPC. `SetItemPrice(item_index, price)`:

- `price >= 0` → define o preço global daquele item (vale em **todos** os NPCs que o vendem);
- `price < 0` → **limpa** o override e o item volta ao preço do catálogo do jogo.

### 5.4 Propagação para o jogo (não é instantâneo)

A escrita vai para o Postgres na hora, mas o mundo do jogo só reflete quando o **tmServer recarrega**:

- **no boot**, e
- por **poll periódico (~15s)** enquanto roda.

Ou seja: comunique ao moderador algo como *"a alteração aparece no jogo em alguns segundos"*. Requer o
overlay ligado (`W2PP_NPC_EDITING=true`). Sem isso, a edição fica só no banco.

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
| `DELETE /api/admin/npcs/:id` | `DeleteNpc` |

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

const httpFor = (r: number) => ({ 1: 200, 2: 403, 3: 422, 4: 404 } as const)[r] ?? 500;

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

Pontos importantes do BFF:

- **`moderatorId` vem SEMPRE da sessão** (`session.accountId`), nunca do corpo do request.
- Mapear `AdminResult` → HTTP (tabela §4.1). Rejeição de gRPC (promise reject) = falha de infra → 502.
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

## 9. Checklist de integração

- [ ] Gerar stubs de `web.proto` (`web.v1.NpcAdminService`).
- [ ] Cliente gRPC+mTLS server-side (singleton), certs via env.
- [ ] Login via `AccountWebService.VerifyCredentials` → cookie `httpOnly` guardando `account_id` **e** `role`.
- [ ] Gate da página/menu de NPC por `role ∈ { moderator, admin }` (só UX — o web-api reautoriza).
- [ ] Toda rota admin deriva `moderator_id` da sessão (nunca do corpo).
- [ ] Mapear `AdminResult` → HTTP; tratar reject de gRPC como 502.
- [ ] UI da loja em 3 abas de 9 (slots 0..8 / 9..17 / 18..26); `SetNpcShop` envia a loja inteira.
- [ ] Preço via `SetItemPrice` (global; `price < 0` limpa).
- [ ] Avisar que a mudança aparece no jogo em ~alguns segundos (poll do tmServer).
- [ ] Garantir que o operador setou `account.role` do moderador e rodou `import-npcs`.
