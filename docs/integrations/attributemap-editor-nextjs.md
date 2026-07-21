# Integração Next.js ↔ web-api: AttributeMap Editor

> Guia de integração para o portal consumir a ferramenta web que substitui o
> `Source/Code/AttributeMap_Editor`. Fonte da verdade do contrato:
> `api/web/v1/web.proto` (`AttributeMapAdminService`).

## Topologia

```
Browser ──HTTPS──> Next.js BFF
                        │ gRPC + mTLS
                        ▼
                     web-api ── lê Release/TMsrv/run/AttributeMap.dat
```

- O browser nunca fala gRPC nem recebe certificados mTLS.
- O BFF deriva `moderator_id` do cookie `httpOnly`; não aceite esse campo do browser.
- O `web-api` revalida `account.role` em toda chamada. Só `moderator` e `admin` passam.
- A RPC não sobrescreve `AttributeMap.dat`; ela retorna bytes para baixar como
  `AttributeMap_New.dat`.
- Para o jogo usar o novo mapa, o operador substitui o arquivo de conteúdo e reinicia o `tmserver`.

## RPCs

### `GetAttributeMapInfo`

Request:

```proto
message GetAttributeMapInfoRequest {
  int64 moderator_id = 1;
}
```

Response:

- `result`: `ADMIN_RESULT_OK`, `ADMIN_RESULT_FORBIDDEN` ou `ADMIN_RESULT_INVALID`.
- `info.dim`: sempre `1024`.
- `info.world_scale`: sempre `4`; cada célula cobre um bloco `4x4` do mundo `4096x4096`.
- `info.sha256`: hash do arquivo atual.
- `info.histogram`: 256 buckets (`value=0..255`).
- `info.meanings`: significados conhecidos dos valores/bits.

`ADMIN_RESULT_INVALID` significa que `W2PP_CONTENT/-content` não está configurado, o arquivo não
existe, ou o tamanho não é exatamente `1024*1024`.

### `TransformAttributeMap`

Request:

```proto
message TransformAttributeMapRequest {
  int64 moderator_id = 1;
  AttributeMapTransformOperation operation = 2;
  int32 operand = 3;
  AttributeMapRect rect = 4;
  AttributeMapTransformFilter filter = 5;
}
```

Operações:

| operação | uso |
|---|---|
| `LEGACY_MARK_PVP_EXP_LOSS` | regra do editor legado: `if value < 64 && value != 1 { value |= 64 }` |
| `ASSIGN_VALUE` | define o byte como `operand` (`0..255`) |
| `SET_BITS` | aplica `value |= operand`; `operand` deve ser `1..255` |
| `CLEAR_BITS` | aplica `value &^= operand`; `operand` deve ser `1..255` |
| `TOGGLE_BITS` | aplica `value ^= operand`; `operand` deve ser `1..255` |

`rect` é opcional. Quando enviado, usa coordenadas do mundo, inclusivas, `0..4095`; o backend converte
por `x/4,y/4`. Ex.: `min_x=4,max_x=8` cobre as células de atributo `1..2`.

`filter` é opcional:

- `enabled=false`: não filtra.
- `enabled=true, mask=0`: só altera células com `value == exact_value`.
- `enabled=true, mask!=0`: só altera células em que `(value & mask) == (match_value & mask)`.

Response:

- `changed_count`: células alteradas.
- `before_histogram` / `after_histogram`: 256 buckets.
- `original_sha256` / `new_sha256`: para revisão operacional.
- `filename`: `AttributeMap_New.dat`.
- `data`: payload binário completo de 1 MiB para download.

## Significados Conhecidos

| valor/bit | significado |
|---:|---|
| `0` | PvE sem flag PvP |
| `1` | cidade / área segura |
| `2` | bloqueio de pathfinding quando aplicado ao `HeightMap.dat` |
| `4` | não permite marcar Gema |
| `64` | flag de área PvP/perda de XP usada por regras de combate |
| `128` | gate de newbie zone |

Outros bits/combinações devem ser tratados como conteúdo legado preservado. A UI deve permitir ver o
valor bruto e evitar transformar células fora do filtro/retângulo escolhido.
