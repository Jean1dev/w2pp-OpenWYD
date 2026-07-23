# Integração Next.js ↔ web-api: eventos de mundo

> Guia para o **front-end Next.js** consumir o controle de **eventos globais de
> mundo** do portal. Fonte da verdade do contrato: `api/web/v1/web.proto`,
> serviço `web.v1.WorldEventAdminService`.

## 1. Topologia

```text
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │  só server-side; cookie de sessão httpOnly
                        │ gRPC + mTLS
                        ▼
                     web-api (:7600) ──> Postgres (`world_event_config`)
                                                │
                                                ▼
                     dbServer  <── polling ── tmServer
```

Regras:

- O browser nunca chama gRPC nem recebe certificados mTLS.
- O Next.js deriva `moderator_id` do cookie de sessão; nunca aceita esse campo do
  browser.
- O `web-api` revalida `account.role in ('moderator','admin')` em toda chamada.
- O portal escreve configuração fria em Postgres. O `tmServer` carrega no boot e
  recarrega por polling; uma alteração salva no portal não é um push em tempo real.

## 2. RPCs

```proto
service WorldEventAdminService {
  rpc GetWorldEventConfig(GetWorldEventConfigRequest)
      returns (GetWorldEventConfigResponse);
  rpc SetWorldEventConfig(SetWorldEventConfigRequest)
      returns (AdminAck);
}

message WorldEventConfig {
  bool enabled = 1;
  int32 item_index = 2;
  int32 rate = 3;
  int32 start_index = 4;
  int32 current_index = 5;
  int32 end_index = 6;
  bool indexed = 7;
  bool notice_enabled = 8;
  bool double_exp_enabled = 9;
  bool newbie_event_enabled = 10;
}

message GetWorldEventConfigRequest {
  int64 moderator_id = 1;
}

message GetWorldEventConfigResponse {
  AdminResult result = 1;
  int64 version = 2;
  WorldEventConfig config = 3;
}

message SetWorldEventConfigRequest {
  int64 moderator_id = 1;
  WorldEventConfig config = 2;
}
```

`SetWorldEventConfig` substitui a configuração inteira. Não existe patch parcial
nem `expected_version` no contrato web; o BFF deve buscar o estado mais recente
antes de salvar para evitar sobrescrever dados de uma aba antiga.

## 3. Campos

| Campo | Tipo | Uso |
|-------|------|-----|
| `enabled` | `bool` | liga/desliga o drop global de item |
| `item_index` | `int32` | índice do item entregue no evento |
| `rate` | `int32` | divisor do roll: `rand() % rate == 0`; `1` significa sempre que elegível |
| `start_index` | `int32` | primeiro serial/progresso válido |
| `current_index` | `int32` | progresso atual; o `tmServer` avança após drop entregue |
| `end_index` | `int32` | limite superior; o evento para quando `current_index >= end_index` |
| `indexed` | `bool` | serializa o item com efeitos `62/63` e aleatório `59` |
| `notice_enabled` | `bool` | anuncia o drop para jogadores online |
| `double_exp_enabled` | `bool` | liga flag global de EXP dobrada no `tmServer` |
| `newbie_event_enabled` | `bool` | liga flag global de evento newbie no `tmServer` |

`version` vem apenas no `GetWorldEventConfigResponse`. Ele é incrementado por
edições de moderador e não muda quando o `tmServer` persiste progresso de
`current_index`.

## 4. Validação do Backend

Valores negativos são inválidos para `item_index`, `rate`, `start_index`,
`current_index` e `end_index`.

Limites:

- `item_index <= 32767`;
- se `enabled = false`, os campos numéricos podem ficar zerados;
- se `enabled = true`, então:
  - `item_index > 0`;
  - `rate > 0`;
  - `start_index > 0`;
  - `end_index > start_index`;
  - `current_index >= start_index`;
  - `current_index <= end_index`.

Em runtime, o drop só está ativo enquanto `current_index < end_index`. Portanto
`current_index == end_index` é uma configuração válida, mas representa evento
esgotado.

## 5. Resultado e Erros

As RPCs usam `AdminResult` no corpo. Erro gRPC representa falha de infraestrutura.

| `AdminResult` | HTTP sugerido no BFF | Significado |
|---------------|----------------------|-------------|
| `ADMIN_RESULT_OK` | 200 | sucesso |
| `ADMIN_RESULT_FORBIDDEN` | 403 | usuário não é moderador/admin |
| `ADMIN_RESULT_INVALID` | 400 / 422 | request inválido ou falha de persistência validada pelo serviço |
| `ADMIN_RESULT_NOT_FOUND` | 404 | reservado; não esperado neste serviço |
| `ADMIN_RESULT_UNSPECIFIED` | 500 | estado inesperado |

O BFF deve transformar erro gRPC em `502` ou `500` e não repassar detalhes
internos para o browser.

## 6. Rotas BFF Sugeridas

| Rota HTTP | RPC | Observações |
|-----------|-----|-------------|
| `GET /api/admin/world-events` | `GetWorldEventConfig` | retorna `version` e `config` |
| `PUT /api/admin/world-events` | `SetWorldEventConfig` | envia a configuração completa |

Shape sugerido para `GET /api/admin/world-events`:

```json
{
  "version": "12",
  "config": {
    "enabled": true,
    "itemIndex": 777,
    "rate": 10,
    "startIndex": 100,
    "currentIndex": 104,
    "endIndex": 200,
    "indexed": true,
    "noticeEnabled": true,
    "doubleExpEnabled": false,
    "newbieEventEnabled": false
  }
}
```

`version` é `int64`; serialize como string no JSON público para evitar perda de
precisão em JavaScript.

Shape sugerido para `PUT /api/admin/world-events`:

```json
{
  "enabled": true,
  "itemIndex": 777,
  "rate": 10,
  "startIndex": 100,
  "currentIndex": 100,
  "endIndex": 200,
  "indexed": true,
  "noticeEnabled": true,
  "doubleExpEnabled": true,
  "newbieEventEnabled": false
}
```

O BFF deve:

- validar sessão e papel antes de chamar o gRPC;
- preencher `moderator_id` com `account_id` da sessão;
- mapear camelCase do JSON para snake_case do proto;
- enviar todos os campos de `WorldEventConfig` no `PUT`;
- mapear `AdminResult` para HTTP;
- transformar `int64` (`version`) em string no JSON.

## 7. Semântica no Jogo

Quando o evento de drop global está ativo e o roll passa:

- o item é entregue direto no carry do jogador que recebe a recompensa da kill;
- o item não vira ground item público;
- se `indexed = true`, o serial usa o `current_index` atual;
- o notice só é enviado depois de entrega bem-sucedida;
- `current_index` só avança depois de entrega bem-sucedida;
- se o carry acessível estiver cheio, o drop é perdido e `current_index` não
  avança.

O `tmServer` persiste o progresso de forma assíncrona. Se uma edição de moderador
acontecer enquanto um progresso antigo chega do `tmServer`, o backend rejeita o
progresso antigo pela versão da configuração.

## 8. UI Recomendada

Crie uma tela de moderação com:

- toggle geral do drop global (`enabled`);
- campo de item por índice (`itemIndex`);
- campo de divisor de chance (`rate`);
- controles numéricos para `startIndex`, `currentIndex` e `endIndex`;
- toggles para `indexed`, `noticeEnabled`, `doubleExpEnabled` e
  `newbieEventEnabled`;
- indicação de status: ativo, desligado ou esgotado;
- botão de salvar que envia a configuração completa;
- botão de recarregar para buscar o estado mais recente.

Estados obrigatórios:

- carregando;
- salvo com sucesso;
- alterações não salvas;
- `403` sem acesso;
- validação inválida;
- erro temporário de upstream;
- evento esgotado (`enabled = true` e `currentIndex >= endIndex`).

Para UX, mostre `rate` como divisor, não como porcentagem final. Exemplo:
`1 em N kills elegíveis`, sem prometer chance final exata, porque o servidor
preserva a ordem de RNG e outras regras de drop do jogo.

## 9. Checklist de Implementação no Portal

- [ ] Regenerar stubs a partir de `api/web/v1/web.proto`.
- [ ] Adicionar cliente server-side para `WorldEventAdminService`.
- [ ] Criar rota BFF `GET /api/admin/world-events`.
- [ ] Criar rota BFF `PUT /api/admin/world-events`.
- [ ] Derivar `moderator_id` da sessão em todas as chamadas.
- [ ] Mapear `AdminResult` para HTTP.
- [ ] Serializar `version` como string no JSON do BFF.
- [ ] Implementar validação client-side equivalente à validação do backend.
- [ ] Exibir estado esgotado quando `currentIndex >= endIndex`.
