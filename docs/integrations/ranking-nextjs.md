# Integração Next.js ↔ web-api: ranking de personagens

> Guia para o **front-end Next.js** consumir a feature de **ranking Top EXP**.
> Fonte da verdade do contrato: `api/web/v1/web.proto`, serviço
> `web.v1.RankingWebService`.

## 1. Topologia

```text
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │
                        │ gRPC + mTLS, server-side only
                        ▼
                     web-api (:7600) ──> Postgres (`character`)
```

Regras:

- O browser **não** chama gRPC diretamente.
- O Next.js atua como BFF: expõe uma rota HTTP/JSON para o browser e chama o
  `web-api` via gRPC + mTLS no server-side.
- O ranking é **público** nesta versão: não precisa de sessão, `account_id` ou
  `moderator_id`.

## 2. RPC

```proto
service RankingWebService {
  rpc ListExpRanking(ListExpRankingRequest) returns (ListExpRankingResponse);
}

message ListExpRankingRequest {
  int32 limit = 1;  // default 50, max 100
  int32 offset = 2; // negative values are treated as 0
}

message RankingEntry {
  int32 rank = 1; // 1-based rank in the full ordered list
  string name = 2;
  int32 class = 3;
  int32 clan = 4;
  uint32 guild_id = 5;
  int32 level = 6;
  int64 exp = 7;
  int32 class_master = 8;
}

message ListExpRankingResponse {
  repeated RankingEntry entries = 1;
  int32 total_count = 2;
}
```

## 3. Paginação

- `limit <= 0`: o backend usa `50`.
- `limit > 100`: o backend limita para `100`.
- `offset < 0`: o backend usa `0`.
- `rank` já vem calculado pelo backend como posição global `1-based`.
- `total_count` é a quantidade total de personagens elegíveis no ranking, útil
  para paginação.

Exemplos:

| Página | Request sugerido |
|--------|------------------|
| primeira página com 50 | `{ limit: 50, offset: 0 }` |
| segunda página com 50 | `{ limit: 50, offset: 50 }` |
| top 10 | `{ limit: 10, offset: 0 }` |

## 4. Ordenação e filtros

O ranking é gerado a partir da tabela persistida `character`.

Filtro:

- personagens com `level >= 1000` não entram no ranking, seguindo o filtro do
  ranking legado.

Ordenação:

1. `class_master` normalizado, descendente.
2. `exp`, descendente.
3. `level`, descendente.
4. `name`, ascendente, para desempate determinístico.

Normalização legada de `class_master`:

| Valor salvo | Significado legado | Valor usado na ordenação |
|------------:|--------------------|--------------------------:|
| `2` | Mortal | `1` |
| `1` | Arch | `2` |
| outros | tiers superiores/outros | o próprio valor |

O campo retornado `class_master` é o valor salvo original, não o normalizado.

## 5. Payload HTTP sugerido no BFF

O contrato externo do Next.js pode ser simples:

```http
GET /api/ranking?limit=50&offset=0
```

Resposta JSON sugerida:

```json
{
  "entries": [
    {
      "rank": 1,
      "name": "PlayerName",
      "class": 0,
      "clan": 1,
      "guildId": 123,
      "level": 400,
      "exp": "1234567890123",
      "classMaster": 3
    }
  ],
  "totalCount": 250
}
```

Observação importante para JavaScript/TypeScript:

- `exp` é `int64`. Para evitar perda de precisão no browser, serialize como
  `string` no JSON público do BFF, mesmo que no gRPC ele chegue como inteiro /
  `Long` / `bigint`, dependendo da lib usada.

## 6. Exemplo de handler Next.js

Exemplo ilustrativo. Ajuste nomes/imports conforme a lib gRPC usada no projeto
front-end.

```ts
import { NextRequest, NextResponse } from "next/server";
import { rankingClient } from "@/server/webApiClient";

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const limit = Number(url.searchParams.get("limit") ?? 50);
  const offset = Number(url.searchParams.get("offset") ?? 0);

  const resp = await rankingClient.listExpRanking({ limit, offset });

  return NextResponse.json({
    entries: resp.entries.map((entry) => ({
      rank: entry.rank,
      name: entry.name,
      class: entry.class,
      clan: entry.clan,
      guildId: entry.guildId,
      level: entry.level,
      exp: entry.exp.toString(),
      classMaster: entry.classMaster,
    })),
    totalCount: resp.totalCount,
  });
}
```

## 7. Estados de UI

Estados recomendados:

- **Carregando:** enquanto o BFF consulta o `web-api`.
- **Vazio:** `entries.length === 0`.
- **Erro temporário:** falha gRPC/HTTP, mostrar mensagem genérica e permitir
  tentar novamente.
- **Paginação:** desabilitar "anterior" quando `offset === 0`; desabilitar
  "próxima" quando `offset + entries.length >= totalCount`.

## 8. Campos para exibição

Campos mínimos para uma tabela:

- posição: `rank`
- personagem: `name`
- nível: `level`
- experiência: `exp`
- classe: `class`
- evolução/tier: `class_master`
- guilda: `guild_id` quando `guild_id != 0`
- reino/clan: `clan`

Mapeamento visual de `class`, `clan` e `class_master` fica do lado do front-end.
O backend retorna os valores numéricos do jogo.
