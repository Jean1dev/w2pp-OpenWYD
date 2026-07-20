# Integração Next.js ↔ web-api: DropTool de drops

> Guia de integração para o **projeto Next.js** consumir a funcionalidade equivalente ao
> **DropTool.exe legado** no portal web. Fonte da verdade do contrato: `api/web/v1/web.proto`
> (serviço `NpcAdminService`, RPCs `ListDropItems` e `ListMobDrops`). Contexto de arquitetura:
> `docs/migration/game-rules.md` §2.2 e `Source/Code/DropTool/main.cpp`.

## 1. Topologia

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │  só server-side; cookie de sessão httpOnly
                        │ gRPC + mTLS
                        ▼
                     web-api (serviço Go, :7600)
                        │
                        └── lê catálogo em memória escaneado no boot:
                            Release/Common/ItemList.csv
                            Release/TMsrv/run/npc/*
```

Regras:

- O browser nunca fala gRPC nem vê certificado mTLS.
- O Next.js deriva `moderator_id` do cookie de sessão; nunca aceita esse campo do browser.
- O `web-api` revalida `account.role in ('moderator','admin')` em toda chamada.
- A ferramenta é **somente leitura**: não altera banco, arquivos de conteúdo, nem estado vivo do
  `tmServer`.

## 2. Pré-requisitos

1. `webserver` rodando com `-content <Release/>` ou `W2PP_CONTENT=<Release/>`.
2. Conta logada com `role = 'moderator'` ou `'admin'`.
3. Portal com stubs atualizados a partir de `api/web/v1/web.proto`.

Se `-content`/`W2PP_CONTENT` estiver vazio ou o scan falhar, as RPCs retornam
`ADMIN_RESULT_OK` com listas vazias. Isso deve ser tratado como catálogo indisponível, não como erro
de permissão.

## 3. O Que a Ferramenta Mostra

O DropTool legado fazia duas leituras:

- `item -> mobs que dropam`: relatório `ItemDropList.txt`.
- `mob -> itens no Carry[]`: relatório `MobDropList.txt`.

No servidor Go, a origem é a mesma estrutura legada:

- cada arquivo válido em `Release/TMsrv/run/npc/` deve ter 816 bytes (`STRUCT_MOB`);
- os drops de mob vêm dos 64 slots `STRUCT_MOB.Carry[]`;
- `slot` é importante: a raridade base vem de `g_pDropRate[slot]`, não do item.

`rate_divisor` exposto no contrato é o divisor base do slot. Quanto maior, mais raro. Ele **não** é a
chance final em runtime, porque a morte do mob ainda aplica bônus de drop do jogador/evento e ajustes
por nível em `tmServer`.

## 4. Contrato gRPC

As duas RPCs ficam em `web.v1.NpcAdminService` e usam `AdminResult` no corpo. Erro gRPC significa
falha de infraestrutura.

| `AdminResult` | HTTP sugerido no BFF | Significado |
|---------------|----------------------|-------------|
| `ADMIN_RESULT_OK` | 200 | sucesso |
| `ADMIN_RESULT_FORBIDDEN` | 403 | usuário não é moderador/admin |
| `ADMIN_RESULT_INVALID` | 400 / 422 | request inválido |
| `ADMIN_RESULT_NOT_FOUND` | 404 | reservado; não esperado nessas leituras |
| `ADMIN_RESULT_UNSPECIFIED` | 500 | estado inesperado |

### `ListDropItems`

Visão por item: retorna itens e a lista de mobs/slots que podem dropar cada item.

```proto
rpc ListDropItems(ListDropItemsRequest) returns (ListDropItemsResponse);

message ListDropItemsRequest {
  int64 moderator_id = 1;
  int32 item_index = 2;               // 0 = sem filtro exato
  string item_query = 3;              // substring em nome do item ou índice
  string mob_query = 4;               // substring em nome/template do mob
  bool include_zero_drop_items = 5;   // inclui itens sem mobs, estilo relatório legado
}

message ListDropItemsResponse {
  AdminResult result = 1;
  repeated DropItemEntry items = 2;
}

message DropItemEntry {
  int32 item_index = 1;
  string item_name = 2;
  repeated DropItemMob mobs = 3;
}

message DropItemMob {
  string template_name = 1;
  string mob_name = 2;
  int32 mob_level = 3;
  int32 slot = 4;
  int32 rate_divisor = 5;
}
```

Sem filtros, `ListDropItems` retorna apenas itens que têm pelo menos um drop. Para reproduzir o
comportamento do `ItemDropList.txt` legado, envie `include_zero_drop_items = true`.

### `ListMobDrops`

Visão por mob: retorna templates de mob e seus slots de drop.

```proto
rpc ListMobDrops(ListMobDropsRequest) returns (ListMobDropsResponse);

message ListMobDropsRequest {
  int64 moderator_id = 1;
  string mob_query = 2;   // substring em nome/template do mob
  int32 item_index = 3;   // 0 = sem filtro exato
  string item_query = 4;  // substring em nome do item ou índice
}

message ListMobDropsResponse {
  AdminResult result = 1;
  repeated MobDropEntry mobs = 2;
}

message MobDropEntry {
  string template_name = 1;
  string mob_name = 2;
  int32 mob_level = 3;
  repeated MobDropItem items = 4;
}

message MobDropItem {
  int32 slot = 1;
  int32 item_index = 2;
  string item_name = 3;
  int32 rate_divisor = 4;
}
```

Sem filtros de item, `ListMobDrops` pode retornar mobs sem drops (`items = []`). Com `item_index` ou
`item_query`, retorna apenas mobs que possuem pelo menos um drop correspondente.

## 5. Rotas BFF Sugeridas

| Rota HTTP | RPC | Query params |
|-----------|-----|--------------|
| `GET /api/admin/drops/items` | `ListDropItems` | `itemIndex`, `itemQuery`, `mobQuery`, `includeZero` |
| `GET /api/admin/drops/mobs` | `ListMobDrops` | `mobQuery`, `itemIndex`, `itemQuery` |

O BFF deve:

- validar sessão e papel antes de chamar o gRPC;
- preencher `moderator_id` com `account_id` da sessão;
- mapear `AdminResult` para HTTP conforme a tabela acima;
- transformar erro gRPC/reject em `502` ou `500`;
- nunca expor endereço/certificados do `web-api` ao browser.

Exemplo de shape HTTP para `GET /api/admin/drops/items?itemQuery=adaga`:

```json
{
  "items": [
    {
      "itemIndex": 1000,
      "itemName": "Adaga",
      "mobs": [
        {
          "templateName": "Alpha",
          "mobName": "Alpha",
          "mobLevel": 37,
          "slot": 0,
          "rateDivisor": 900
        }
      ]
    }
  ]
}
```

## 6. UI Recomendada

Crie uma tela de moderação com duas abas:

- **Por item**: busca por nome/índice do item, filtro opcional por mob, tabela de itens com contador de
  mobs e expansão mostrando `mob_name`, `template_name`, `mob_level`, `slot`, `rate_divisor`.
- **Por mob**: busca por nome/template do mob, filtro opcional por item, tabela de mobs com expansão
  mostrando `slot`, `item_index`, `item_name`, `rate_divisor`.

Estados obrigatórios:

- carregando;
- catálogo vazio/indisponível (`items=[]` ou `mobs=[]` com `200`);
- sem resultado para filtros;
- `403` sem acesso;
- erro temporário de upstream.

Para UX, explique `rate_divisor` como "divisor base do slot; maior = mais raro". Não apresente como
porcentagem final.

## 7. Notas Operacionais

- O scan acontece uma vez no boot do `webserver`; alterações nos arquivos de `Release/` exigem restart
  do `webserver` para aparecer no portal.
- `ItensExceptions.txt` é opcional em `<Release>/TMsrv/run/ItensExceptions.txt`. Se existir e for
  válido, seus ranges `from,to` são aplicados ao scan, como no DropTool legado. Se não existir, nenhum
  item é excluído.
- `ItemDropList.txt` existente no `Release/` continua sendo artefato gerado legado. O portal deve usar
  as RPCs novas, não ler esse arquivo.
- Essa tela não edita drops. Se futuramente houver edição de drops, ela deve virar outro contrato com
  validação e persistência explícitas.

## 8. Checklist de Implementação no Portal

- [ ] Regenerar stubs a partir de `api/web/v1/web.proto`.
- [ ] Adicionar cliente server-side para `NpcAdminService`.
- [ ] Criar rotas BFF `GET /api/admin/drops/items` e `GET /api/admin/drops/mobs`.
- [ ] Derivar `moderator_id` da sessão em todas as chamadas.
- [ ] Mapear `AdminResult` para HTTP.
- [ ] Implementar tela com abas "Por item" e "Por mob".
- [ ] Tratar catálogo vazio como estado esperado quando `W2PP_CONTENT` não estiver configurado.
- [ ] Exibir `rate_divisor` como divisor base, não chance final.
