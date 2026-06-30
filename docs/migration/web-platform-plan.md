# Plano: Plataforma Web (contas, cash, recompensas diárias, loja-web, ranking)

> Status: PLANO (não implementado). Origem: decisão arquitetural de como uma aplicação **Next.js**
> (criar conta, login, comprar cash, resgatar recompensas diárias, anunciar itens na loja-web, ver
> ranking) deve se integrar ao stack do jogo. Este doc é a fonte da verdade dessa decisão para
> qualquer agente que for construir a plataforma.

## A restrição que decide tudo

O tmServer tem **um único goroutine dono de todo o estado do mundo**, em memória, sem locks
(`tmserver/internal/world/world.go`, doc do pacote; ver `CLAUDE.md` §"single-owner game loop"). O
Postgres (via dbServer) é **cold storage**: o personagem é carregado no login e só volta pro banco
via `SaveCharacterAsync`/`SaveCharacterThen` (periódico + no quit; `world/world.go:223,241`).

Consequência direta para a web:

> **Enquanto um personagem está online, o banco está desatualizado, e qualquer escrita da web na
> linha dele será sobrescrita no próximo save do tmServer** → item duplicado / escrita perdida. É
> exatamente o bug que a arquitetura single-owner existe para evitar.

Por isso, a primeira regra é o que **NÃO** fazer:

- ❌ Next.js conectando direto no Postgres — sofre o race acima e ainda duplicaria o hashing
  argon2id (que vive em `dbserver/internal/convert/hash.go`).
- ❌ Web escrevendo estado vivo de personagem (inventário, coin, stats) por qualquer caminho.

## Topologia decidida

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │  (só server-side; cookie de sessão httpOnly)
                        │ gRPC + mTLS (mesmo padrão dos links internos)
                        ▼
                  web-api  (NOVO serviço Go)
                   ├── Postgres   ← leituras frias (ranking, perfil) + escrita de conta/cash
                   └── tmServer   ← nada de escrita direta; concessões via delivery_queue
```

- **Next.js é BFF puro.** O navegador nunca fala gRPC nem vê certificado mTLS. Todas as chamadas
  saem dos Route Handlers / Server Actions (server-side). O front guarda só um cookie de sessão
  `httpOnly`.
- **Novo serviço Go `web-api`** (proto novo em `api/web/v1`, mTLS como os demais links internos).
  **Não** estender o `AccountService` do dbServer — ele espelha as mensagens legadas G↔DB
  (`AccountLogin`, `LoadCharacter`…; `api/db/v1/db.proto`) e deve continuar focado nisso. O web-api
  reaproveita `dbserver/internal/store` e `dbserver/internal/convert/hash` (argon2id) por baixo.

## O padrão que resolve "online vs banco": entrega assíncrona (mailbox)

Qualquer coisa que **conceda** item/cash/coin (compra de cash, recompensa diária, item comprado na
loja-web) **não escreve no personagem**. O web-api faz INSERT numa fila de entrega; o tmServer drena.

```sql
CREATE TABLE delivery_queue (
  id           bigserial PRIMARY KEY,
  account_id   bigint NOT NULL,
  character_id bigint,          -- NULL = qualquer char da conta
  kind         text NOT NULL,   -- 'item' | 'cash' | 'coin'
  payload      jsonb NOT NULL,  -- p/ item: {index, eff1..effv3, expires_at}
  status       text NOT NULL DEFAULT 'pending',
  created_at   timestamptz NOT NULL DEFAULT now()
);
```

- **web-api** só faz INSERT aqui, **transacionalmente** junto com debitar o cash. Idempotência via
  chave da transação de pagamento (evita crédito duplicado em retry de webhook).
- **tmServer** drena a fila **dentro do loop** no login do personagem (e/ou num tick periódico para
  quem já está online) e aplica via um handler de concessão — respeitando o single-owner. O
  vocabulário já existe: `grantStarterCarry` (`handler/character.go:497`), `World.Go` para I/O fora
  do loop. O save normal persiste o resultado.

Isso elimina o race por construção: a web nunca toca estado vivo; o tmServer é o único escritor do
personagem.

## ⚠️ Loja-web é a feature mais perigosa

Anunciar um item significa **retirá-lo de um personagem que pode estar online**. Se a web remover do
banco, o tmServer reescreve no próximo save e o item volta (duplicação). Regra: **escrita de
inventário fica sempre no tmServer**.

Caminho preferido:

1. O jogador consigna o item num **NPC/baú dentro do jogo** (handler novo no tmServer) → grava em
   `market_listing`.
2. A web apenas **lê** os anúncios (vitrine) e processa a **compra**, que credita o comprador via
   `delivery_queue` e debita o cash.

(Alternativa inferior: anunciar pela web só item que esteja no **cargo de conta de char offline**, e
ainda sob lock lógico exigindo que o char não esteja logado. Preferir a opção 1.)

## Autenticação — dois mundos separados

- **Login do jogo**: CPSock, validado por tmServer/dbServer (fluxo legado).
- **Login da web**: cookie de sessão `httpOnly`.

São sessões independentes, mas compartilham a **mesma tabela `account` e o mesmo hash argon2id**. Não
reaproveitar o protocolo legado para a web.

## Onde cada feature cai

| Feature | Caminho | Observação |
|---|---|---|
| Criar conta | web-api → `store` + `convert/hash` (argon2id) | hashing no Go, nunca em Node |
| Login web | web-api valida hash → cookie de sessão | separado do login do jogo |
| Ranking | web-api → SELECT read-only no Postgres | dado de char online fica levemente atrasado — aceitável |
| Ver perfil/personagem | idem, read-only | |
| Comprar cash | gateway → webhook → web-api credita `donate_balance` + `delivery_queue` | `donate_balance` é coluna por-conta (`account`); ver gotcha abaixo |
| Recompensa diária | web-api → `delivery_queue` + tabela "resgatou hoje" | |
| Loja-web | consignação in-game + entrega via `delivery_queue` | ver seção ⚠️ acima |

## binServer / billing — papel na nova arquitetura

(Ver também `flows.md` §billing e o contrato `api/bin/v1/bin.proto`.)

binServer hoje = **gate de login apenas**: RPC `CheckBilling(account_name) → {allowed, status}`
chamada no character-login (`handler/character.go:138`) fora do loop. Estado atual: **no-op por
padrão** (`AllowAllBilling` sem `-binserver`), **sem persistência** (map em memória,
`binserver/internal/billing/billing.go`), **sem RPC de escrita**.

Decisão: **manter o binServer como o serviço de _entitlement/acesso_** (premium/assinatura/ban
administrativo). Ele **NÃO** é a carteira de cash — cash = `donate_balance` no Postgres.

Upgrades necessários antes de servir a web:
1. **Persistência** — tabela própria `billing_entitlement(account_id, status, expires_at)` (não
   reusar colunas da `account`).
2. **RPC de escrita** (`SetBilling`/`GrantPremium(account, days)`) que o **webhook de pagamento /
   web-api** chama para estender `expires_at`. O tmServer continua só lendo `CheckBilling` no login.

## Pontos em aberto (decidir antes de construir)

1. **Redundância de ban**: existem dois conceitos — `account.is_blocked` (Postgres, checado no
   *account*-login, `LOGIN_RESULT_BLOCKED` em `db.proto`) vs `billing.StatusBlocked` (binServer,
   checado no *character*-login). Decidir a fonte da verdade antes do painel admin, senão um botão
   "banir" escreve em dois lugares e eles divergem. Recomendação: ban administrativo no binServer;
   `account.is_blocked` só para lockout de credencial/segurança, ou eliminado.
2. **Crédito de `donate_balance`**: confirmar que o `SaveCharacter` do tmServer faz update **parcial**
   e **não reescreve a linha `account`** (hoje ele grava *character*, não account — `store.go`), para
   garantir que creditar cash por fora não seja clobberado. Validar antes de implementar a compra.
