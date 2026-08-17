# Integração Next.js ↔ web-api: ícones e catálogo de itens

> Guia de integração para o **projeto Next.js** exibir **imagem/ícone de item** em qualquer tela que
> mostre item (loja de doação, recompensa diária, DropTool, editor de loja de NPC, templates de mob,
> world events). Fonte da verdade do contrato: `api/web/v1/web.proto` (serviço `ItemCatalogService`).
> Contexto: `docs/migration/item-icons-plan.md` e `docs/migration/web-platform-plan.md`.

**Resumo em uma linha:** o servidor **não** tem as imagens dos itens e nunca vai ter — ele tem a
**chave** que identifica a imagem (`icon_key`). Integre contra a chave e o fallback agora; o pacote
de imagens pluga depois sem o front mudar nada.

## 1. Topologia

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)
                        │  só server-side; cookie de sessão httpOnly
                        │ gRPC + mTLS
                        ▼
                     web-api (serviço Go, :7600)  ──> Release/Common/ItemList.csv (mount read-only)
```

Regras que **não** mudam: o browser nunca fala gRPC nem vê certificado mTLS; o Next.js é BFF puro.

Diferente das demais telas, aqui **não há sessão envolvida**: o catálogo é conteúdo público e
imutável, lido do mount read-only no boot do `web-api`. `ItemCatalogService.ListItems` não recebe
`account_id` nem `moderator_id`.

## 2. Por que a imagem não vem do servidor

Não existe **nenhuma imagem de item no repositório**, e não vai existir: os assets gráficos vivem
dentro da pasta do cliente oficial (`WYD.exe` 7662), que não é open-source e não faz parte do repo
(`docs/agents/CLIENT-RESEARCH-2026-06-27.md` §1).

O que existe é a chave. Quando o **cliente** desenha um item ele lê o mesmo catálogo que o servidor
(`STRUCT_ITEMLIST`, `Source/Code/Basedef.h:1162-1185`), e os únicos campos visuais lá são
`IndexMesh`, `IndexTexture` e `nPos`. Ou seja:

> **A imagem do item é função de `(mesh, texture, nPos)`, não do `item_index`.**

Consequência prática: os **3220 itens nomeados colapsam em 1055 chaves visuais distintas** — armadura,
calça, luva e bota de um mesmo set compartilham o `mesh` e se diferenciam pelo `nPos`. Um pacote de
ícones precisa cobrir ~1k imagens, não 6,5k.

É por isso que o campo se chama `icon_key` e é **opaco**: se a regra de endereçamento do cliente for
outra (§7), o formato da chave muda no servidor e o front não é tocado.

> **UNVERIFIED:** a *fórmula exata* que o cliente usa para ir de `(mesh, texture, pos)` até o bitmap
> (atlas? arquivo por mesh? tabela intermediária?) não foi confirmada — exige o cliente em mãos.
> Ver `docs/migration/item-icons-plan.md` §7.

## 3. Contrato

```proto
service ItemCatalogService {
  rpc ListItems(ListItemsRequest) returns (ListItemsResponse);
}

message ListItemsRequest {}
message ListItemsResponse {
  repeated ItemCatalogEntry items = 1;
  string catalog_version = 2;
}

message ItemCatalogEntry {
  int32 item_index = 1;
  string name = 2;           // nome cru do catálogo, ex. "Botas_Douradas(N)"
  string icon_key = 3;       // "m<mesh>_t<texture>_p<slot_mask>"
  string display_name = 4;   // name com "_" virando espaço
  int32 slot_mask = 5;       // nPos: bitmask sobre STRUCT_MOB.Equip[16]; 0 = não equipável
  repeated string slots = 6; // slot_mask decodificado ("boots", "weapon", …)
  int32 grade = 7;           // 1=Normal 2=Místico 3=Arcano 4=Lendário …
  int32 mesh = 8;
  int32 texture = 9;
}
```

| campo | tipo | descrição |
|---|---|---|
| `item_index` | `int32` | id do item no jogo — é o que as outras RPCs (`SetNpcShop`, `DonateShopItem`, `DailyRewardItem`) usam |
| `name` | `string` | nome cru, com `_`. Use para busca/match; **não** exiba |
| `icon_key` | `string` | chave do pacote de ícones. ~1055 valores distintos |
| `display_name` | `string` | é o que se exibe |
| `slot_mask` | `int32` | onde o item **pode** ser equipado (bitmask, §4) |
| `slots` | `string[]` | `slot_mask` já decodificado — use este, não reimplemente os bits |
| `grade` | `int32` | raridade; dirige a cor do fallback e da moldura |
| `mesh` / `texture` | `int32` | componentes da `icon_key`, expostos para depuração e agrupamento por set |
| `catalog_version` | `string` | fingerprint do `ItemList.csv` (§6) |

`NpcAdminService.ListItemCatalog` (moderação) devolve **a mesma** `ItemCatalogEntry` com os mesmos
campos — os dois caminhos compartilham o mapper em `webserver/internal/grpcsrv/itemcatalog.go`, então
não podem divergir.

### 3.1. Modo degradado

Sem `-content`/`W2PP_CONTENT` configurado no `web-api`, `ListItems` devolve **lista vazia e nenhum
erro** (mesma degradação dos pickers de moderação), com `catalog_version` também vazio (`""`). O
front tem que tratar catálogo vazio como "sem enfeite", nunca como falha — mas sem cacheá-lo para
sempre; ver §6.

## 4. `slot_mask`: o bitmask de `Equip[16]`

Bit *i* = `STRUCT_MOB.Equip[i]`.

| bit | valor | slot | bit | valor | slot |
|---|---|---|---|---|---|
| 0 | 1 | `face` | 8 | 256 | `accessory` |
| 1 | 2 | `helmet` | 9 | 512 | `amulet` |
| 2 | 4 | `armor` | 10 | 1024 | `orb` |
| 3 | 8 | `pants` | 11 | 2048 | `gem` |
| 4 | 16 | `gloves` | 12 | 4096 | `medal` |
| 5 | 32 | `boots` | 13 | 8192 | `fairy` |
| 6 | 64 | `weapon` | 14 | 16384 | `mount` |
| 7 | 128 | `shield` | 15 | 32768 | `cape` |

Dois casos que importam:

- **`slot_mask = 192`** (`64|128`) = **arma de duas mãos**, ocupa arma *e* escudo. `slots` vem
  `["weapon", "shield"]`. São 238 itens.
- **`slot_mask = 0`** = **não equipável** (poções, cupons, baús, pergaminhos). `slots` vem vazio.
  São 890 itens — o fallback precisa de um ícone genérico para eles, não pode assumir slot.

> Cuidado com a colisão de nome: em `mob-template-editing-frontend.md` o campo `slot` é o índice de
> `Equip[]` **sendo editado** (0..15). Aqui `slot_mask`/`slots` é onde o item **pode** ser equipado.
> São coisas diferentes.

## 5. Renderização: chave → imagem, com fallback obrigatório

```
icon_key ──> item-icons/<icon_key>.webp   (pacote de imagens, ainda não existe — Fase 2)
                  │ 404 / pacote ausente
                  └──> fallback SVG por slot, colorido por grade
```

**O fallback não é temporário.** Ele cobre: itens novos do servidor que nunca existiram no cliente
original, lacunas do pacote, e o modo degradado do §3.1. Trate-o como o caminho normal e a imagem
real como enfeite por cima.

Mapeamento sugerido (o front decide a arte; o que importa é a chave de decisão ser
`slots[0]` + `grade`):

| slot | ícone do fallback | | `grade` | rótulo | cor |
|---|---|---|---|---|---|
| `weapon` | espada | | 1 | Normal | cinza |
| `shield` | escudo | | 2 | Místico | azul |
| `helmet` | elmo | | 3 | Arcano | roxo |
| `armor` / `pants` | peça de armadura | | 4 | Lendário | dourado |
| `gloves` / `boots` | luva / bota | | 0 ou ausente | sem grade | cinza |
| `accessory` / `amulet` / `orb` / `gem` / `medal` | anel/gema | | | | |
| `fairy` / `mount` | fada / montaria | | | | |
| `cape` | manto | | | | |
| *(vazio)* | caixa genérica | | | | |

### 5.1. Decoração fica no front, não na imagem

Refino (`+1..+9`), aura/moldura por `grade`, quantidade e marca de "selado" são **overlay em
CSS/SVG** sobre o ícone. Assar isso nos arquivos multiplicaria o pacote por 10 — e o refino é estado
por instância do item, não do catálogo, então nem estaria disponível aqui.

## 6. Cache: use o `catalog_version`

A lista inteira são ~3220 entradas (~400 KB de proto). **Não chame por item.** O catálogo é imutável
em runtime — o `Release/` é um mount read-only e o `web-api` lê uma vez no boot — então
`catalog_version` só muda quando o servidor é redeployado com outro `ItemList.csv`.

Padrão recomendado: buscar a lista uma vez no server-side, indexar por `item_index`, e servir ao
browser um mapa já reduzido ao que a tela precisa. Use `catalog_version` como `ETag`/chave de
revalidação.

Uma exceção ao "buscar uma vez": quando o `web-api` sobe sem `-content`, a resposta vem vazia e
`catalog_version` vem `""` (§3.1). Esse resultado **não** deve ser cacheado — se o `web-api` for
redeployado com conteúdo, um processo que cacheou o vazio continuaria servindo mapa vazio (ícones
presos no fallback, silenciosamente) até reiniciar. `version === ""` é a checagem barata.

## 7. Exemplo (pseudocódigo BFF, server-side)

```ts
// lib/itemCatalogClient.ts  (SERVER-ONLY — nunca importar no client)
import { credentials } from "@grpc/grpc-js";
import fs from "node:fs";
import { ItemCatalogServiceClient } from "@/gen/web/v1/web_grpc_pb"; // gerado do web.proto

const ssl = credentials.createSsl(
  fs.readFileSync(process.env.WEB_API_CA!),
  fs.readFileSync(process.env.WEB_API_CLIENT_KEY!),
  fs.readFileSync(process.env.WEB_API_CLIENT_CERT!),
);
export const itemCatalog = new ItemCatalogServiceClient(process.env.WEB_API_ADDR!, ssl);
```

```ts
// lib/itemCatalog.ts — busca uma vez por processo e indexa por item_index
import { itemCatalog } from "@/lib/itemCatalogClient";

export type CatalogItem = {
  itemIndex: number;
  displayName: string;
  iconKey: string;
  slots: string[];
  slotMask: number;
  grade: number;
};

type Catalog = { version: string; byIndex: Map<number, CatalogItem> };

let cached: Catalog | null = null;
let inFlight: Promise<Catalog> | null = null;

export async function getCatalog(): Promise<Catalog> {
  if (cached) return cached;
  // Sem dedupe, N requests a frio puxam N × ~400 KB antes do primeiro cache.
  if (inFlight) return inFlight;
  inFlight = fetchCatalog().finally(() => {
    inFlight = null;
  });
  return inFlight;
}

async function fetchCatalog(): Promise<Catalog> {
  const r = await new Promise<any>((resolve, reject) =>
    itemCatalog.listItems({}, (e: unknown, x: unknown) => (e ? reject(e) : resolve(x))),
  );
  const byIndex = new Map<number, CatalogItem>();
  for (const it of r.items ?? []) {
    byIndex.set(it.itemIndex, {
      itemIndex: it.itemIndex,
      displayName: it.displayName,
      iconKey: it.iconKey,
      slots: it.slots ?? [],
      slotMask: it.slotMask,
      grade: it.grade,
    });
  }
  const catalog: Catalog = { version: r.catalogVersion ?? "", byIndex };
  // Catálogo vazio (version "") = web-api sem -content. Não é erro — as telas caem no
  // fallback —, mas NÃO cacheie: se a web-api for redeployada com conteúdo, este processo
  // continuaria servindo mapa vazio até reiniciar. Sem cache, a próxima chamada reconsulta.
  if (catalog.version !== "") cached = catalog;
  return catalog;
}
```

```tsx
// components/ItemIcon.tsx — a imagem real é opcional; o fallback é que é garantido
export function ItemIcon({ item }: { item: CatalogItem }) {
  const slot = item.slots[0] ?? "none";
  return (
    <img
      src={`/item-icons/${item.iconKey}.webp`}
      alt={item.displayName}
      title={item.displayName}
      data-grade={item.grade}
      onError={(e) => {
        // Na Fase 1 nem o pacote de ícones nem os SVGs de fallback existem (§9), então os
        // dois URLs dão 404. Sem esta guarda, reatribuir o src redispara onError em loop —
        // um stream de requests por item, num picker de ~3220. Só tente o fallback uma vez.
        if (e.currentTarget.dataset.fb) return;
        e.currentTarget.dataset.fb = "1";
        // pacote ausente ou item sem ícone: cai no SVG por slot+grade (§5)
        e.currentTarget.src = `/item-fallback/${slot}.svg`;
      }}
    />
  );
}
```

## 8. Como isso chega nas outras telas

`DropItemEntry`, `MobDropItem`, `DonateShopItem`, `DailyRewardItem` e os campos `item_index` de world
events **não** carregam os campos visuais — são mensagens distintas. Faça o **join por `item_index`**
contra o mapa do §7, em vez de pedir que cada RPC duplique `icon_key`/`grade`.

## 9. Fora de escopo (não existe ainda)

- **O pacote de imagens.** `item-icons/<icon_key>.webp` é convenção deste doc; nada o publica hoje.
  É a Fase 2 do `item-icons-plan.md`, e depende de (a) confirmar como o cliente endereça o ícone e
  (b) uma **decisão do dono do projeto sobre distribuir assets extraídos do cliente oficial**
  (HanbitSoft/JoyImpact) — ver §6 e §9.1 daquele plano. Até lá, 100% fallback.
- **Inventário/equipamento do personagem.** `ListMyCharacters` devolve só o resumo;
  `docs/integrations/my-characters-nextjs.md` é explícito que inventário e equipamento ficam fora da
  API. Mostrar o inventário com ícones exige uma RPC que ainda não existe.
- **Render 3D dos meshes** para alta resolução (Fase 3, opcional).
