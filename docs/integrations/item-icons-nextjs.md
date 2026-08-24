# Integração Next.js ↔ web-api: ícones e catálogo de itens

O browser não fala gRPC. O BFF Next.js busca o catálogo server-side, faz join
por `item_index` e monta URLs para o bucket/CDN.

## Contrato

```proto
message ListItemsResponse {
  repeated ItemCatalogEntry items = 1;
  string catalog_version = 2;
  string icon_pack_version = 3;
}

message ItemCatalogEntry {
  int32 item_index = 1;
  string name = 2;
  string icon_key = 3;       // "iNNNN"; vazio = fallback
  string display_name = 4;
  int32 slot_mask = 5;
  repeated string slots = 6;
  int32 grade = 7;
  int32 mesh = 8;
  int32 texture = 9;
  string icon_url = 10;      // URL pública retornada pelo storage-manager
}
```

`NpcAdminService.ListItemCatalog` devolve as mesmas entradas e também
`catalog_version`/`icon_pack_version`.

O ícone real vem de `itemicon.bin`, não de `mesh`, `texture` ou `slot_mask`.
Trate `icon_key` como opaco. Nunca reconstrua a chave no frontend.

## URL e renderização

O storage-manager gera nomes aleatórios no S3, então use `iconUrl` devolvida
pela RPC. Não concatene base, versão e chave no frontend:

```ts
function itemIconUrl(item: CatalogItem, iconPackVersion: string): string | null {
  if (!item.iconKey || !item.iconUrl || !iconPackVersion) return null;
  return item.iconUrl;
}
```

O web-api só aceita URLs HTTPS absolutas presentes no manifesto publicado.

O componente deve começar no fallback e sobrepor a imagem real somente quando
ela carregar. Isso evita loop de `onError` e mantém nome/slot visíveis:

```tsx
type ItemIconProps = {
  item: CatalogItem;
  iconPackVersion: string;
};

export function ItemIcon({ item, iconPackVersion }: ItemIconProps) {
  const src = itemIconUrl(item, iconPackVersion);
  const slot = item.slots[0] ?? "none";

  return (
    <span className="item-icon" data-slot={slot} data-grade={item.grade}>
      <FallbackItemIcon slot={slot} grade={item.grade} />
      {src ? (
        <img
          src={src}
          alt=""
          width={35}
          height={35}
          loading="lazy"
          onError={(event) => event.currentTarget.remove()}
        />
      ) : null}
      <span className="sr-only">{item.displayName}</span>
    </span>
  );
}
```

Use CSS `image-rendering: auto`; o arquivo original tem 35×35 e não deve ser
artificialmente “melhorado” no backend. Moldura por raridade, refino, quantidade
e selado são overlays separados.

## Cache e modo degradado

- Busque a lista inteira uma vez e indexe por `item_index`; não crie uma chamada
  gRPC por item.
- Use `catalog_version` para invalidar metadados e `icon_pack_version` para
  invalidar o mapa de URLs publicado.
- Não retenha indefinidamente uma resposta com `catalog_version == ""`; ela
  significa web-api sem conteúdo configurado.
- `icon_pack_version == ""`, `icon_key == ""` ou `icon_url == ""` não é erro:
  mostre fallback.
- Erro/404 no CDN também deve manter o fallback, sem repetição automática.

As telas de drop, loja, recompensa, NPC e eventos continuam carregando apenas
`item_index`; todas fazem join contra este catálogo central.
