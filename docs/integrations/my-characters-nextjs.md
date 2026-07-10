# Integração Next.js ↔ web-api: meus personagens

> Contrato: `api/web/v1/web.proto`, serviço `CharacterWebService`.

## Fluxo

O browser chama uma rota REST do próprio Next.js. Essa rota roda server-side, lê o cookie `httpOnly`
da sessão criada por `AccountWebService.VerifyCredentials`, e chama o `web-api` por gRPC+mTLS.

```
Browser ──HTTPS──> Next.js BFF ──gRPC+mTLS──> web-api ──> Postgres
```

O browser nunca envia `account_id`. A rota usa sempre `session.accountId`.

## RPC

```proto
service CharacterWebService {
  rpc ListMyCharacters(ListMyCharactersRequest) returns (ListMyCharactersResponse);
}

message ListMyCharactersRequest {
  int64 account_id = 1;
}
```

`ListMyCharacters` retorna um resumo seguro de cada personagem da conta: `slot`, `name`, `class`,
`level`, `exp`, `coin`, `hp/mp` e atributos básicos. Inventário, equipamentos e buffs ficam fora
desta API.

## Rota BFF sugerida

| Método + rota | RPC |
|---------------|-----|
| `GET /api/me/characters` | `CharacterWebService.ListMyCharacters` |

```ts
// app/api/me/characters/route.ts
import { NextResponse } from "next/server";
import { characterWeb } from "@/lib/characterWebClient";
import { getSession } from "@/lib/session";

export async function GET() {
  const session = await getSession();
  if (!session) return NextResponse.json({ error: "unauthenticated" }, { status: 401 });

  const resp = await new Promise<any>((resolve, reject) =>
    characterWeb.listMyCharacters({ accountId: session.accountId }, (err: unknown, r: unknown) =>
      err ? reject(err) : resolve(r)),
  ).catch(() => null);

  if (!resp) return NextResponse.json({ error: "upstream" }, { status: 502 });
  return NextResponse.json({ characters: resp.characters ?? [] });
}
```

## Observações

- Os dados vêm do Postgres e podem estar levemente atrasados enquanto o personagem está online; o
  estado vivo continua pertencendo ao tmServer.
- Essa API é somente leitura. Qualquer escrita futura em personagem deve respeitar a regra do
  single-owner game loop e passar pelo tmServer ou por uma fila de entrega.
