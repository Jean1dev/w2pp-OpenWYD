# Recompensa Diária — Guia de Integração Front-end (issue #35)

> Público: quem constrói o **BFF Next.js** (Route Handlers / Server Actions) e as telas de
> **painel admin** (moderador) e **resgate** (jogador). Este doc descreve o contrato gRPC exposto pela
> `web-api` (serviço Go `webserver`). Fonte da arquitetura: [`web-platform-plan.md`](./web-platform-plan.md).
> O fluxo é irmão da [loja de donate](./donate-shop-frontend.md) (issue #34) — mesma sessão, mesmo
> transporte, mesma semântica de entrega — só muda o preço (aqui não existe: é grátis) e o limite
> (uma vez por dia).

## 1. Como o front fala com o backend

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)  ──gRPC + mTLS──> web-api (:7600)
         cookie de sessão httpOnly            (só server-side)                     └─ Postgres
```

Regras que **não** mudam (idênticas ao donate):

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

`AdminResult` (operações de moderador — reutilizado do donate, mesmo enum):

| valor | quando | UI sugerida |
|---|---|---|
| `ADMIN_RESULT_OK` (1) | sucesso | seguir |
| `ADMIN_RESULT_FORBIDDEN` (2) | não é moderador/admin | 403 / esconder |
| `ADMIN_RESULT_INVALID` (3) | validação falhou (item_index ≤ 0, expires_days < 0…) | erro de formulário |
| `ADMIN_RESULT_NOT_FOUND` (4) | alvo (oferta) não existe | 404 |

`ClaimResult` (resgate do jogador):

| valor | significado | UI sugerida |
|---|---|---|
| `CLAIM_RESULT_OK` (1) | resgate registrado + item enfileirado para entrega | sucesso; mostrar confirmação |
| `CLAIM_RESULT_ALREADY_CLAIMED` (2) | a conta já resgatou uma recompensa hoje (UTC) | "você já resgatou sua recompensa de hoje" |
| `CLAIM_RESULT_NOT_FOUND` (3) | oferta ou conta inexistente | 404 |
| `CLAIM_RESULT_DISABLED` (4) | oferta existe mas não está mais disponível | recarregar vitrine |

## 3. Modelo de dados: `DailyRewardItem`

Uma oferta de recompensa = um item entregável, **grátis**, resgatável no máximo uma vez por conta por
dia. Comparado ao `DonateShopItem`, não existe campo `price`.

| campo | tipo | descrição |
|---|---|---|
| `id` | int64 | id da oferta. **0 em `UpsertRewardItem` cria**; ≠0 atualiza a oferta existente |
| `item_index` | int32 | id do item no jogo (catálogo `ItemList.csv`). Deve ser > 0 |
| `eff1`,`effv1`,`eff2`,`effv2`,`eff3`,`effv3` | int32 | três pares efeito/valor (encantamento/refino). 0 = sem efeito |
| `title` | string | nome exibido na tela de resgate |
| `description` | string | descrição/observação |
| `enabled` | bool | `true` aparece na vitrine do jogador |
| `expires_days` | int32 | > 0 entrega item temporário (expira em N dias); 0 = permanente |

> Picker de itens: o catálogo (item_index → nome) é servido pelo `NpcAdminService.ListItemCatalog`
> (já existente, reutilizado do donate) quando o `web-api` roda com `-content`. Reutilize-o no editor
> da oferta em vez de pedir `item_index` cru.

## 4. Serviço `DailyRewardAdminService` (painel do moderador)

Todo request tem `moderator_id` como **primeiro campo** — preencha com o `account_id` da sessão do
moderador; o backend revalida o `role`. Diferente do `DonateAdminService`, **não existe** RPC de
crédito de saldo aqui — recompensas diárias não têm carteira, só o catálogo.

### `ListRewardItems(moderator_id) → { result, DailyRewardItem[] }`
Lista **todas** as ofertas (habilitadas ou não) — a tabela de moderação.

### `UpsertRewardItem(moderator_id, DailyRewardItem item) → { result, item_id }`
Cria (`item.id == 0`) ou atualiza (`item.id != 0`). `item_id` é o id resultante. Validação →
`ADMIN_RESULT_INVALID`; atualizar id inexistente → `ADMIN_RESULT_NOT_FOUND`.

### `SetRewardItemEnabled(moderator_id, item_id, enabled) → AdminAck`
Liga/desliga a oferta na vitrine de resgate.

### `DeleteRewardItem(moderator_id, item_id) → AdminAck`
Remove a oferta.

`AdminAck` = `{ AdminResult result }`.

## 5. Serviço `DailyRewardService` (resgate do jogador)

Passe o `account_id` da sessão httpOnly (nunca do browser).

### `ListRewards() → { DailyRewardItem[] }`
A **vitrine**: só ofertas `enabled = true`. Sem `moderator_id`, sem `result` (leitura pública).

### `GetClaimStatus(account_id) → { claimed_today, claimed_item_id, claimed_item_title }`
Consulta se a conta **já resgatou hoje (UTC)**. Use isto para desenhar a tela: se `claimed_today ==
true`, desabilite o botão de resgate e mostre qual item foi resgatado (`claimed_item_title` — vazio
se a oferta foi apagada depois do resgate).

### `Claim(account_id, reward_item_id) → { ClaimResult result }`
Resgata a oferta escolhida: registra o resgate do dia e **enfileira o item para entrega**. O jogador
escolhe **uma** oferta entre as habilitadas; depois disso `CLAIM_RESULT_ALREADY_CLAIMED` bloqueia
qualquer outro resgate até o próximo dia UTC, **mesmo que ele tente uma oferta diferente**. Ver
semântica de entrega abaixo.

## 6. Semântica de entrega (o que dizer ao jogador)

Idêntica ao donate — mesma fila, mesmo dreno:

O resgate **não** coloca o item no jogo na hora — o servidor de jogo é o único que escreve inventário.
A `web-api` grava o resgate numa fila (`delivery_queue`); o servidor de jogo **drena a fila no próximo
login do jogador** e coloca o item no **próximo espaço livre do armazém (cargo) da conta**.

Consequências para a UI:

- Após `CLAIM_RESULT_OK`, mostre algo como **"item será entregue no seu armazém no próximo login"**.
- Se o jogador resgatar enquanto está **online**, o item chega **no próximo login** (não instantâneo —
  MVP com dreno só no login).
- **Se o armazém estiver cheio (128 espaços), o item é perdido** — vale avisar o jogador a manter
  espaço livre antes de resgatar.
- O limite diário reseta à **meia-noite UTC**, não 24h corridas desde o último resgate — deixe isso
  claro na UI (ex.: "resgates renovam às 00:00 UTC") em vez de mostrar uma contagem regressiva de 24h.

## 7. Fluxo de sessão (login web)

O login web é separado do login do jogo (CPSock). O BFF autentica via
`AccountWebService.VerifyCredentials(name, password)`, que retorna `{ ok, account_id, blocked, role }`.
Grave `account_id` e `role` no cookie de sessão httpOnly e use-os nas chamadas acima. `role ∈
{player, moderator, admin}` — só `moderator`/`admin` enxergam o painel de admin (e o backend reconfere).

## 8. Exemplo (pseudocódigo BFF, server-side)

```ts
// Server Action (Next.js) — resgatar a recompensa do dia
'use server'
import { getSession } from '@/lib/session'            // lê o cookie httpOnly
import { dailyRewardClient } from '@/lib/grpc'         // stub gRPC+mTLS, só server-side

export async function claimReward(rewardItemId: string) {
  const { accountId } = await getSession()             // nunca do browser
  const res = await dailyRewardClient.claim({
    accountId: BigInt(accountId),
    rewardItemId: BigInt(rewardItemId),
  })
  switch (res.result) {
    case ClaimResult.CLAIM_RESULT_OK:
      return { ok: true, message: 'Item será entregue no seu armazém no próximo login.' }
    case ClaimResult.CLAIM_RESULT_ALREADY_CLAIMED:
      return { ok: false, error: 'Você já resgatou sua recompensa de hoje.' }
    case ClaimResult.CLAIM_RESULT_DISABLED:
      return { ok: false, error: 'Oferta indisponível.' }
    default:
      return { ok: false, error: 'Oferta não encontrada.' }
  }
}
```

```ts
// Server Component/Action — carregar a tela de resgate
export async function loadRewardScreen() {
  const { accountId } = await getSession()
  const [rewards, status] = await Promise.all([
    dailyRewardClient.listRewards({}),
    dailyRewardClient.getClaimStatus({ accountId: BigInt(accountId) }),
  ])
  return {
    offers: rewards.items,
    alreadyClaimedToday: status.claimedToday,
    claimedItemTitle: status.claimedItemTitle,
  }
}
```

```ts
// Server Action — moderador cria/edita uma oferta
export async function upsertReward(item: DailyRewardItemInput) {
  const { accountId, role } = await getSession()
  if (role !== 'moderator' && role !== 'admin') throw new Error('forbidden')  // UI gate; backend reconfere
  const res = await dailyRewardAdminClient.upsertRewardItem({
    moderatorId: BigInt(accountId),
    item: { ...item, id: BigInt(item.id ?? 0) },       // id 0 = criar
  })
  if (res.result !== AdminResult.ADMIN_RESULT_OK) throw mapAdminError(res.result)
  return res.itemId
}
```

## 9. Fora de escopo (não existe endpoint ainda)

- **Entrega instantânea para quem já está online** (só há dreno no login, igual ao donate).
- **Sequência/streak de dias consecutivos** (recompensa maior a cada dia seguido resgatado): o modelo
  atual é "uma oferta grátis por dia, sem memória de streak" — se isso for pedido no futuro, é uma
  extensão da tabela `daily_reward_claim`, não deste contrato.
- **Escolha automática/sorteio da oferta do dia**: hoje o jogador escolhe manualmente entre todas as
  ofertas habilitadas; não há conceito de "a recompensa de hoje é X para todo mundo".
