# Integração Next.js ↔ web-api: painel de faturamento

> Guia para o **front-end Next.js** consumir o **painel de faturamento** (receita
> de donate, pedidos, compradores e extrato de créditos). Fonte da verdade do
> contrato: `api/web/v1/web.proto`, serviço `web.v1.DonateRevenueAdminService`.
>
> ⚠️ **Isto não é o `binServer`.** `bin.v1.BillingService.CheckBilling` é o portão
> de *entitlement* de login; `docs/migration/web-platform-plan.md` diz
> explicitamente que ele **não** é a carteira de cash. Faturamento no sentido de
> receita vive aqui: `donate_topup_order` + `donate_shop_audit` + `account`.

## 1. Topologia

```text
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │  só server-side; cookie de sessão httpOnly
                        │ gRPC + mTLS
                        ▼
                     web-api (:7600) ──leitura──> Postgres
                                                   ├── donate_topup_order   (dinheiro real)
                                                   ├── donate_shop_audit    (créditos donate)
                                                   ├── account              (identidade + carteira)
                                                   └── donate_payer_profile (nome + CPF)
```

Regras:

- O browser nunca chama gRPC nem recebe certificados mTLS.
- O Next.js deriva `moderator_id` do cookie de sessão; **nunca** aceita esse campo
  do browser.
- O `web-api` revalida `account.role in ('moderator','admin')` em toda chamada.
  Um cookie adulterado recebe `ADMIN_RESULT_FORBIDDEN`.
- **Este serviço não escreve nada** — nem linha de auditoria. Por isso não
  interage com o loop single-owner do `tmServer` e pode ser chamado à vontade.
- Os dados vêm do Postgres (armazenamento frio). Um pedido confirmado agora
  aparece imediatamente; estado de personagem online não é relevante aqui.

## 2. RPCs

```proto
service DonateRevenueAdminService {
  rpc GetRevenueSummary(GetRevenueSummaryRequest) returns (GetRevenueSummaryResponse);
  rpc ListTopupOrders(ListTopupOrdersRequest)     returns (ListTopupOrdersResponse);
  rpc ListTopBuyers(ListTopBuyersRequest)         returns (ListTopBuyersResponse);
  rpc ListDonateSpend(ListDonateSpendRequest)     returns (ListDonateSpendResponse);
  rpc SearchAccounts(SearchAccountsRequest)       returns (SearchAccountsResponse);
}

message RevenueWindow {
  int64 from_unix = 1; // inclusive, Unix seconds; 0 = to_unix menos 30 dias
  int64 to_unix   = 2; // exclusive, Unix seconds; 0 = agora
}

enum RevenueBucket {
  REVENUE_BUCKET_UNSPECIFIED = 0; // não retorna série
  REVENUE_BUCKET_DAY = 1;
  REVENUE_BUCKET_WEEK = 2;        // semana ISO, começa SEGUNDA
  REVENUE_BUCKET_MONTH = 3;
}

enum DonateLedgerAction {
  DONATE_LEDGER_ACTION_UNSPECIFIED = 0; // ambos os tipos
  DONATE_LEDGER_ACTION_PURCHASE = 1;
  DONATE_LEDGER_ACTION_CREDIT   = 2;
}

message RevenueTotals {
  int64 paid_orders     = 1;
  int64 gross_cents     = 2;
  int64 credits_sold    = 3;
  int64 distinct_buyers = 4;

  int64 created_orders  = 5;
  int64 pending_orders  = 6;
  int64 pending_cents   = 7;

  int64 shop_purchases  = 8;
  int64 credits_spent   = 9;
  int64 manual_credits  = 10;
  int64 credits_granted = 11;
}

message RevenueByMethod {
  PaymentMethod payment_method = 1;
  int64 paid_orders = 2;
  int64 gross_cents = 3;
}

message RevenuePoint {
  int64 bucket_start_unix = 1;
  int64 paid_orders       = 2;
  int64 gross_cents       = 3;
  int64 credits_sold      = 4;
  int64 distinct_buyers   = 5;
}

message TopupOrderRow {
  int64  id = 1;
  string external_reference = 2;
  int64  account_id = 3;
  string account_name = 4;
  string account_email = 5;
  string payer_name = 6;
  string payer_cpf_masked = 7;
  int32  credits = 8;
  int64  amount_cents = 9;
  PaymentMethod payment_method = 10;
  string provider = 11;
  TopupStatus status = 12;
  int64  created_at_unix = 13;
  int64  confirmed_at_unix = 14;
}

message TopBuyerRow {
  int64  account_id = 1;
  string account_name = 2;
  string account_email = 3;
  int64  window_paid_orders   = 4;
  int64  window_gross_cents   = 5;
  int64  lifetime_paid_orders = 6;
  int64  lifetime_gross_cents = 7;
  int64  lifetime_credits     = 8;
  int64  first_paid_at_unix   = 9;
  int64  last_paid_at_unix    = 10;
  int32  donate_balance       = 11;
}

message DonateLedgerRow {
  int64  id = 1;
  DonateLedgerAction action = 2;
  int64  created_at_unix = 3;
  int64  subject_account_id = 4;
  string subject_account_name = 5;
  int64  actor_account_id = 6;
  string actor_account_name = 7;
  int64  credits_delta = 8;
  int64  balance_after = 9;
  int64  shop_item_id = 10;
  string shop_item_title = 11;
  string reason = 12;
}

message AccountSummary {
  int64  id = 1;
  string name = 2;
  string email = 3;
  int32  donate_balance = 4;
  string role = 5;
  bool   is_blocked = 6;
}
```

Todo request tem `moderator_id` como **primeiro campo**. Todas as respostas
paginadas ecoam `from_unix`/`to_unix` **efetivamente aplicados** pelo servidor.

## 3. Campos

### `RevenueTotals`

| Campo | Tipo | Uso |
|-------|------|-----|
| `paid_orders` / `gross_cents` | int64 | **Receita reconhecida** no período (base `confirmed_at`). O número principal do painel. |
| `credits_sold` | int64 | Créditos donate emitidos pelos pedidos pagos. Unidade de jogo, **não** dinheiro. |
| `distinct_buyers` | int64 | Contas distintas que pagaram no período. |
| `created_orders` | int64 | Pedidos **criados** no período, qualquer status. Denominador da conversão. |
| `pending_orders` / `pending_cents` | int64 | Criados no período e ainda PENDING. **Não é receita e não é "a receber"** — ver §7. |
| `shop_purchases` / `credits_spent` | int64 | Compras na loja e créditos gastos. Saída de carteira, em créditos. |
| `manual_credits` / `credits_granted` | int64 | Créditos manuais de moderador (cortesia/compensação). Entrada de créditos **sem** entrada de dinheiro. |

### `TopupOrderRow`

| Campo | Tipo | Uso |
|-------|------|-----|
| `external_reference` | string | UUID do portal. **Chave de reconciliação** com o gateway e em chargeback. |
| `account_name` / `account_email` | string | Identidade do comprador, já resolvida por JOIN — não faça lookup por linha. |
| `payer_name` | string | Do `donate_payer_profile`; vazio se o pagador nunca preencheu. |
| `payer_cpf_masked` | string | **Sempre mascarado** (`***.456.789-**`). O CPF completo nunca sai da web-api; não existe variante sem máscara no contrato. Vazio quando não há perfil. |
| `amount_cents` | int64 | Dinheiro em centavos inteiros. |
| `credits` | int32 | Créditos donate do pedido. **Outra unidade** — nunca some com `amount_cents`. |
| `provider` | string | **Sempre vazio hoje**: nada escreve `donate_topup_order.provider`. Não construa UI em cima dele. |
| `confirmed_at_unix` | int64 | `0` enquanto PENDING. |

### `DonateLedgerRow`

| Campo | Tipo | Uso |
|-------|------|-----|
| `subject_account_id` / `_name` | int64/string | **De quem é a carteira que se moveu.** É por aqui que se filtra e se agrupa. |
| `actor_account_id` / `_name` | int64/string | **Quem causou.** Igual ao subject numa compra; é o **moderador** num crédito manual. |
| `credits_delta` | int64 | **Assinado**: negativo em compra (débito), positivo em crédito. Soma da coluna = fluxo líquido. |
| `balance_after` | int64 | Saldo registrado no momento do movimento. |
| `shop_item_title` | string | Vazio se a oferta foi deletada depois (`shop_item_id` não é FK, de propósito). **Caia para `shop_item_id`** ou compras antigas parecem corrompidas. |
| `reason` | string | Só em `credit_balance`: a nota do moderador. |

> **A assimetria que importa.** Em `donate_shop_audit`, a coluna `account_id` é o
> comprador quando `action='purchase'`, mas é o **moderador** quando
> `action='credit_balance'` — a conta creditada só existe no JSON. O backend já
> normaliza isso em `subject`/`actor`; a UI só precisa usar os campos certos.
> Exemplo: moderador `admin1` credita 100 para `jogador7` → `subject=jogador7`,
> `actor=admin1`, `credits_delta=+100`.

## 4. Validação do Backend

- Janela: `from_unix=0` e `to_unix=0` → **últimos 30 dias**; só `to_unix=0` → até agora.
- `from >= to` → `INVALID`. Valores negativos → `INVALID`.
- Janela maior que **366 dias** → `INVALID`.
- `limit`: default **50**, máximo **100**. `offset` negativo vira `0`.
- `SearchAccounts`: prefixo mínimo **2 caracteres** (senão `INVALID`); é
  minusculizado e tem `%`/`_` escapados no servidor. `limit` default **20**, máx **50**.
- `account_id < 0` → `INVALID`. `account_id = 0` significa "todas as contas".
- `bucket` desconhecido → **não é erro**: degrada para "sem série", para um portal
  mais novo não quebrar contra um servidor mais velho.

## 5. Resultado e Erros

As RPCs usam `AdminResult` no corpo. Erro gRPC representa falha de infraestrutura.

| `AdminResult` | HTTP sugerido no BFF | Significado |
|---------------|----------------------|-------------|
| `ADMIN_RESULT_OK` | 200 | sucesso |
| `ADMIN_RESULT_FORBIDDEN` | 403 | usuário não é moderador/admin |
| `ADMIN_RESULT_INVALID` | 400 / 422 | janela inválida, prefixo curto, `account_id` negativo |
| `ADMIN_RESULT_NOT_FOUND` | 404 | reservado; **não retornado por este serviço** — uma janela sem dados é 200 com listas vazias |
| `ADMIN_RESULT_UNSPECIFIED` | 500 | estado inesperado |

O BFF deve transformar erro gRPC em `502` ou `500` e não repassar detalhes
internos para o browser.

## 6. Rotas BFF Sugeridas

| Rota HTTP | RPC | Observações |
|-----------|-----|-------------|
| `GET /api/admin/faturamento/resumo?from=&to=&bucket=&accountId=` | `GetRevenueSummary` | KPIs + série do gráfico |
| `GET /api/admin/faturamento/pedidos?from=&to=&status=&method=&accountId=&limit=&offset=` | `ListTopupOrders` | tabela de pedidos |
| `GET /api/admin/faturamento/top-compradores?from=&to=&limit=&offset=` | `ListTopBuyers` | ranking + LTV |
| `GET /api/admin/faturamento/extrato-donate?from=&to=&action=&accountId=&limit=&offset=` | `ListDonateSpend` | extrato de créditos |
| `GET /api/admin/faturamento/contas?q=` | `SearchAccounts` | autocomplete do filtro de conta |

Shape sugerido para `GET /api/admin/faturamento/resumo`:

```json
{
  "period": { "from": "2026-07-01T03:00:00Z", "to": "2026-08-01T03:00:00Z" },
  "totals": {
    "paidOrders": 128,
    "grossCents": "4560000",
    "creditsSold": "256000",
    "distinctBuyers": 97,
    "createdOrders": 190,
    "pendingOrders": 62,
    "pendingCents": "1880000",
    "shopPurchases": 340,
    "creditsSpent": "198000",
    "manualCredits": 4,
    "creditsGranted": "1200"
  },
  "byMethod": [
    { "paymentMethod": "PIX", "paidOrders": 120, "grossCents": "4300000" },
    { "paymentMethod": "CREDIT_CARD", "paidOrders": 8, "grossCents": "260000" }
  ],
  "series": [
    { "bucketStart": "2026-07-01T03:00:00Z", "paidOrders": 4, "grossCents": "150000", "creditsSold": "8000", "distinctBuyers": 4 }
  ]
}
```

Shape sugerido para `GET /api/admin/faturamento/pedidos`:

```json
{
  "period": { "from": "2026-07-01T03:00:00Z", "to": "2026-08-01T03:00:00Z" },
  "totalCount": 128,
  "orders": [
    {
      "id": "9012",
      "externalReference": "6f1c…-uuid",
      "accountId": "34",
      "accountName": "zarco",
      "accountEmail": "zarco@exemplo.com",
      "payerName": "Fulano de Tal",
      "payerCpfMasked": "***.456.789-**",
      "credits": 500,
      "amountCents": "2500",
      "paymentMethod": "PIX",
      "status": "PAID",
      "createdAt": "2026-07-10T15:00:00Z",
      "confirmedAt": "2026-07-10T15:02:11Z"
    }
  ]
}
```

Shape sugerido para `GET /api/admin/faturamento/extrato-donate`:

```json
{
  "totalCount": 344,
  "entries": [
    {
      "id": "771",
      "action": "CREDIT",
      "createdAt": "2026-07-12T18:30:00Z",
      "subject": { "accountId": "34", "accountName": "zarco" },
      "actor":   { "accountId": "2",  "accountName": "admin1" },
      "creditsDelta": "100",
      "balanceAfter": "600",
      "reason": "compensacao evento"
    }
  ]
}
```

O BFF deve:

- Derivar `moderator_id` **da sessão**, nunca do query string.
- Serializar todo `int64` como **string** no JSON público (`id`, `accountId`,
  `amountCents`, `grossCents`, `creditsDelta`, `shopItemId`, …) — acima de 2^53 o
  JavaScript perde precisão silenciosamente.
- Converter todo `*_unix` para ISO-8601 (`0` → `null`, não `1970-01-01`).
- Mapear `AdminResult` para os HTTP da §5.
- **Nunca** dividir centavos por 100 no cálculo — só na formatação final.

## 7. Semântica de Faturamento

Esta seção é o que impede um painel numericamente correto de contar a história errada.

1. **Receita é reconhecida em `confirmed_at`, não em `created_at`.** Um pedido
   criado em janeiro e pago em fevereiro é receita **de fevereiro**. Em janeiro
   ele aparece apenas em `created_orders`.
2. **Os buckets fecham em `America/Sao_Paulo`.** "Hoje" começa à 00h de Brasília e
   o mês fecha no último dia às 23h59 BRT — de propósito, para bater com o
   extrato bancário. Os instantes no fio continuam absolutos (Unix seconds); é só
   a fronteira do agrupamento que é BRT. Note que isso diverge do resto do
   servidor (a regra de dia da recompensa diária é UTC) — é intencional.
3. **Semana começa segunda-feira** (`date_trunc('week')` do Postgres).
4. **Não existe status de reembolso nem de expirado.** Um pedido PENDING fica
   PENDING para sempre; nada varre pedidos abandonados. Consequências:
   - `paid_orders / created_orders` é um **piso** da conversão, que decai com o tempo.
   - `pending_cents` cresce monotonicamente e **não é dinheiro a receber**.
     Rotule como "aguardando pagamento (histórico)".
   - Para uma conversão honesta, considere só pedidos criados nas últimas 24–48h.
5. **`credit_balance` é cortesia, não receita.** É valor entregue ao jogador sem
   entrada de dinheiro. Mostre separado dos KPIs de receita.
6. **Créditos ≠ dinheiro.** `credits_sold`, `credits_spent` e `credits_granted`
   estão na moeda do jogo; `*_cents` está em BRL. Nunca some as duas, e não
   converta uma na outra — a taxa varia por pedido.
7. **Passivo em circulação está fora de escopo.** O total de donate comprado e
   ainda não gasto (`SUM(account.donate_balance)`) **não** é exposto por este
   serviço. `TopBuyerRow.donate_balance` é o saldo de *uma* conta, para contexto
   no drill-down — não agregue essa coluna para estimar o passivo, porque ela só
   vem das contas da página atual.
8. **`provider` está sempre vazio.** A coluna existe para reconciliação futura por
   gateway, mas `CreateTopupOrder` nunca a escreve. Não há breakdown por gateway
   além de `payment_method`.

## 8. UI Recomendada

- **Cabeçalho de KPIs**: receita bruta, pedidos pagos, ticket médio
  (`gross_cents / paid_orders`, calculado no front), compradores distintos.
  Separe visualmente o bloco de créditos (gasto/cortesia) do bloco de dinheiro.
- **Gráfico da série**: barras/linha sobre `series`, com seletor dia/semana/mês.
  Os buckets vazios já vêm zerados — não faça gap fill.
- **Tabela de pedidos**: filtros de período, status, método e conta. Coluna
  `external_reference` copiável (é a chave de reconciliação).
- **Ranking de compradores**: ordenado por `window_gross_cents`, com `lifetime_*`
  ao lado. O drill-down é a **mesma** rota de pedidos com `accountId`.
- **Extrato de donate**: `credits_delta` com sinal e cor, colunas separadas para
  *subject* e *actor*.

**Estados obrigatórios:**

- carregando;
- **janela sem dados** (200 com listas vazias) — visualmente distinto de erro;
- `403` sem acesso (esconder o item de menu, mas ainda tratar a resposta);
- janela inválida (`>366 dias` ou `from >= to`);
- página além do fim (lista vazia mas `totalCount > 0` — volte para a página 1);
- erro temporário de upstream (502/500) com retry;
- **aviso de fuso**: rotular o período como horário de Brasília;
- **aviso de que PENDING não expira**, junto de `pending_cents`;
- oferta deletada no extrato (`shopItemTitle` vazio → mostrar o id).

## 9. Checklist de Implementação no Portal

- [ ] Regenerar os stubs a partir de `api/web/v1/web.proto`.
- [ ] Cliente gRPC server-side com mTLS (reusar o singleton de `npc-admin-nextjs.md`).
- [ ] As 5 rotas BFF da §6.
- [ ] `moderator_id` sempre derivado da sessão.
- [ ] Mapear `AdminResult` → HTTP conforme §5.
- [ ] Serializar `int64` como string; converter `*_unix` para ISO (0 → `null`).
- [ ] Rotular o período como horário de Brasília.
- [ ] Não somar centavos com créditos em lugar nenhum.
- [ ] Não exibir `provider` (sempre vazio).
- [ ] Fallback de `shopItemTitle` vazio para `shopItemId`.
- [ ] Rotular `pending_cents` como histórico, não como "a receber".

## 10. Fora de escopo (não existe endpoint)

- **CPF completo.** Só a forma mascarada existe no contrato. Se um chargeback
  exigir o número, isso deve virar uma RPC admin-only separada que escreve a
  própria linha de auditoria — não um parâmetro nesta.
- **Passivo em circulação** (`SUM(account.donate_balance)`) — ver §7.7.
- **Exportação CSV/planilha** — o BFF pode montar a partir das rotas paginadas.
- **Estorno/cancelamento de pedido** — não há write path; o contrato é read-only.
- **Breakdown por gateway** além de `payment_method` — ver §7.8.
- **Guia do fluxo de compra** (`DonateTopupService`, o lado do jogador): ainda
  **não documentado**. O contrato está em `api/web/v1/web.proto`; falta um
  `docs/integrations/donate-topup-nextjs.md`.
