# Loja de Donate — Guia de Integração Front-end (issue #34)

> Público: quem constrói o **BFF Next.js** (Route Handlers / Server Actions) e as telas de
> **painel admin** (moderador) e **loja** (jogador). Este doc descreve o contrato gRPC exposto pela
> `web-api` (serviço Go `webserver`). Fonte da arquitetura: [`web-platform-plan.md`](./web-platform-plan.md).

## 1. Como o front fala com o backend

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)  ──gRPC + mTLS──> web-api (:7600)
         cookie de sessão httpOnly            (só server-side)                     └─ Postgres
```

Regras que **não** mudam:

- **O browser nunca fala gRPC** nem vê o certificado mTLS. Toda chamada sai do lado servidor do
  Next.js (Route Handler / Server Action), com os stubs gerados de `api/web/v1/web.proto`.
- O BFF guarda em cookie `httpOnly` de sessão o `account_id` e o `role` do usuário (obtidos no login
  via `AccountWebService.VerifyCredentials`). **Nunca** aceite `account_id`/`moderator_id` vindos do
  browser — use sempre o valor da sessão.
- **Autorização é reconferida no servidor.** O `role` no cookie serve só para mostrar/esconder a UI
  de moderador. Toda RPC de admin revalida `account.role` no backend; um cookie adulterado recebe
  `ADMIN_RESULT_FORBIDDEN`.

Endereço do serviço: `web-api` em `:7600` (flag `-addr`, mTLS via `-tls-*`). Pacote proto:
`web.v1`. Gere os stubs a partir de `api/web/v1/web.proto`.

## 2. Convenção de resultados (importante)

As RPCs **não** usam códigos de erro gRPC para regras de negócio — só para falha de infra
(indisponibilidade, timeout). O resultado de negócio vem **no corpo**, num enum:

`AdminResult` (operações de moderador):

| valor | quando | UI sugerida |
|---|---|---|
| `ADMIN_RESULT_OK` (1) | sucesso | seguir |
| `ADMIN_RESULT_FORBIDDEN` (2) | não é moderador/admin | 403 / esconder |
| `ADMIN_RESULT_INVALID` (3) | validação falhou (item_index ≤ 0, price ≤ 0, expires_days < 0, amount ≤ 0…) | erro de formulário |
| `ADMIN_RESULT_NOT_FOUND` (4) | alvo (oferta/conta) não existe | 404 |

`BuyResult` (compra do jogador):

| valor | significado | UI sugerida |
|---|---|---|
| `BUY_RESULT_OK` (1) | debitado + item enfileirado para entrega | sucesso; mostrar novo saldo |
| `BUY_RESULT_INSUFFICIENT_FUNDS` (2) | saldo < preço | "saldo insuficiente" |
| `BUY_RESULT_NOT_FOUND` (3) | oferta ou conta inexistente | 404 |
| `BUY_RESULT_DISABLED` (4) | oferta existe mas não está à venda | recarregar vitrine |

## 3. Modelo de dados: `DonateShopItem`

Uma oferta da loja = um item entregável por um preço em donate.

| campo | tipo | descrição |
|---|---|---|
| `id` | int64 | id da oferta. **0 em `UpsertShopItem` cria**; ≠0 atualiza a oferta existente |
| `item_index` | int32 | id do item no jogo (catálogo `ItemList.csv`). Deve ser > 0 |
| `eff1`,`effv1`,`eff2`,`effv2`,`eff3`,`effv3` | int32 | três pares efeito/valor (encantamento/refino). 0 = sem efeito |
| `price` | int32 | custo em donate. Deve ser > 0 |
| `title` | string | nome exibido na loja |
| `description` | string | descrição/observação |
| `enabled` | bool | `true` aparece na vitrine do jogador |
| `expires_days` | int32 | > 0 entrega item temporário (expira em N dias); 0 = permanente |

> Picker de itens: o catálogo (item_index → nome) é servido pelo `NpcAdminService.ListItemCatalog`
> (já existente) quando o `web-api` roda com `-content`. Reutilize-o no editor da oferta em vez de
> pedir `item_index` cru.

## 4. Serviço `DonateAdminService` (painel do moderador)

Todo request tem `moderator_id` como **primeiro campo** — preencha com o `account_id` da sessão do
moderador; o backend revalida o `role`.

### `ListShopItems(moderator_id) → { result, DonateShopItem[] }`
Lista **todas** as ofertas (habilitadas ou não) — a tabela de moderação.

### `UpsertShopItem(moderator_id, DonateShopItem item) → { result, item_id }`
Cria (`item.id == 0`) ou atualiza (`item.id != 0`). `item_id` é o id resultante. Validação →
`ADMIN_RESULT_INVALID`; atualizar id inexistente → `ADMIN_RESULT_NOT_FOUND`.

### `SetShopItemEnabled(moderator_id, item_id, enabled) → AdminAck`
Liga/desliga a oferta na vitrine.

### `DeleteShopItem(moderator_id, item_id) → AdminAck`
Remove a oferta.

### `CreditDonateBalance(moderator_id, account_id, amount, reason) → { result, new_balance }`
**Adiciona `amount` de donate à conta `account_id`** (crédito manual/administrativo — é a mesma porta
que o gateway de pagamento vai usar no futuro). `amount` deve ser > 0. `reason` fica no audit trail.
`new_balance` só vem em `OK`. Conta inexistente → `ADMIN_RESULT_NOT_FOUND`.

`AdminAck` = `{ AdminResult result }`.

## 5. Serviço `DonateShopService` (loja do jogador)

Passe o `account_id` da sessão httpOnly (nunca do browser).

### `ListShopItems() → { DonateShopItem[] }`
A **vitrine**: só ofertas `enabled = true`. Sem `moderator_id`, sem `result` (leitura pública).

### `GetBalance(account_id) → { balance }`
Saldo de donate da conta. Conta inexistente retorna `balance = 0`.

### `Buy(account_id, shop_item_id) → { BuyResult result, new_balance }`
Compra a oferta: debita o saldo e **enfileira o item para entrega**. `new_balance` só vem em
`BUY_RESULT_OK`. Ver semântica de entrega abaixo.

## 6. Semântica de entrega (o que dizer ao jogador)

A compra **não** coloca o item no jogo na hora — o servidor de jogo é o único que escreve inventário.
A `web-api` grava a compra numa fila (`delivery_queue`); o servidor de jogo **drena a fila no próximo
login do jogador** e coloca o item no **próximo espaço livre do armazém (cargo) da conta**.

Consequências para a UI:

- Após `BUY_RESULT_OK`, mostre algo como **"item será entregue no seu armazém no próximo login"**.
- Se o jogador comprar enquanto está **online**, o item chega **no próximo login** (não instantâneo —
  MVP com dreno só no login).
- **Se o armazém estiver cheio (128 espaços), o item é perdido** — vale avisar o jogador a manter
  espaço livre antes de comprar.
- O saldo (`donate_balance`) é debitado na hora da compra, independente de o jogador estar online.

## 7. Fluxo de sessão (login web)

O login web é separado do login do jogo (CPSock). O BFF autentica via
`AccountWebService.VerifyCredentials(name, password)`, que retorna `{ ok, account_id, blocked, role }`.
Grave `account_id` e `role` no cookie de sessão httpOnly e use-os nas chamadas acima. `role ∈
{player, moderator, admin}` — só `moderator`/`admin` enxergam o painel de admin (e o backend reconfere).

## 8. Exemplo (pseudocódigo BFF, server-side)

```ts
// Server Action (Next.js) — comprar uma oferta
'use server'
import { getSession } from '@/lib/session'          // lê o cookie httpOnly
import { donateShopClient } from '@/lib/grpc'        // stub gRPC+mTLS, só server-side

export async function buyOffer(shopItemId: string) {
  const { accountId } = await getSession()           // nunca do browser
  const res = await donateShopClient.buy({
    accountId: BigInt(accountId),
    shopItemId: BigInt(shopItemId),
  })
  switch (res.result) {
    case BuyResult.BUY_RESULT_OK:
      return { ok: true, newBalance: res.newBalance,
               message: 'Item será entregue no seu armazém no próximo login.' }
    case BuyResult.BUY_RESULT_INSUFFICIENT_FUNDS: return { ok: false, error: 'Saldo insuficiente.' }
    case BuyResult.BUY_RESULT_DISABLED:           return { ok: false, error: 'Oferta indisponível.' }
    default:                                       return { ok: false, error: 'Oferta não encontrada.' }
  }
}
```

```ts
// Server Action — moderador cria/edita uma oferta
export async function upsertOffer(item: DonateShopItemInput) {
  const { accountId, role } = await getSession()
  if (role !== 'moderator' && role !== 'admin') throw new Error('forbidden')  // UI gate; backend reconfere
  const res = await donateAdminClient.upsertShopItem({
    moderatorId: BigInt(accountId),
    item: { ...item, id: BigInt(item.id ?? 0) },     // id 0 = criar
  })
  if (res.result !== AdminResult.ADMIN_RESULT_OK) throw mapAdminError(res.result)
  return res.itemId
}
```

## 9. Fora de escopo (não existe endpoint ainda)

- **Gateway de pagamento / compra real de donate**: por enquanto o saldo só é creditado por
  `CreditDonateBalance` (moderador). O webhook de pagamento futuro chamará a mesma lógica.
- **Entrega instantânea para quem já está online** (só há dreno no login).
- **Mercado P2P / consignação de item pelo jogador** (feature separada — ver `web-platform-plan.md`).
