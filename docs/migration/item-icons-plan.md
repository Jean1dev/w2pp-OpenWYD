# Ícones reais de itens no portal web

> Status: **IMPLEMENTADO NO BACKEND E NO PIPELINE**. A geração do pacote final
> depende somente de executar o extrator contra a cópia do cliente 7662 usada
> pelo projeto. Pixels derivados do cliente não são versionados neste repo.

## Fonte correta

O levantamento inicial supunha que a imagem fosse derivável de
`(IndexMesh, IndexTexture, nPos)`. Essa hipótese está errada: diferentes itens
com a mesma tripla podem apontar para ícones distintos.

O cliente clássico usa duas fontes próprias:

- `itemicon.bin`: vetor little-endian indexado por `item_index`; cada valor
  positivo é o número da célula mais um, e zero significa “sem ícone”;
- `UI/itemiconNN.wyt`: atlases TGA encapsulados em WYT, com células 35×35,
  dez colunas e cem células endereçáveis por atlas.

Por isso `icon_key` é agora `iNNNN`, derivado exclusivamente dessa tabela.
`mesh`, `texture`, `slot_mask`, `slots` e `grade` continuam no contrato para
busca, diagnóstico e fallback, não para localizar pixels.

## Geração

Requisitos: Go e uma pasta do cliente contendo `itemicon.bin` e
`UI/itemicon01.wyt` em diante.

```bash
make item-icons CLIENT_DIR=/caminho/para/WYD-7662
# ou
go run ./webserver/cmd/itemiconpack \
  -client /caminho/para/WYD-7662 \
  -out dist/item-icons
```

O gerador:

- aceita tabelas curtas, reproduzindo a cauda zerada do cliente até 6.500 itens;
- descobre quantos atlases são necessários pelo maior índice referenciado;
- decodifica TGA 24/32 bits, cru ou RLE, nos wrappers WYT conhecidos;
- aplica o color key preto como transparência;
- emite somente células utilizadas, como PNG 35×35;
- calcula `pack_version` por SHA-256 de `itemicon.bin`, nomes e bytes dos
  atlases, tornando a saída imutável e reproduzível.

Saída:

```text
dist/item-icons/
├── manifest.json
└── <pack_version>/
    ├── i0000.png
    ├── i0001.png
    └── ...
```

`dist/item-icons/` é ignorado pelo Git. O manifesto registra cobertura total,
quantidade de células distintas e o mapa completo `item_index → célula`.

## Publicação pelo storage-manager-server

O publicador usa `Jean1dev/storage-manager-server`, envia cada PNG ao endpoint
multipart `POST /v1/s3` e registra exatamente a URL pública retornada:

```bash
export W2PP_STORAGE_MANAGER_URL=https://storage-manager-svc.herokuapp.com
export W2PP_STORAGE_MANAGER_BUCKET=jeanluca-teste
make item-icons-publish
```

O serviço atual gera nomes S3 aleatórios, portanto o portal não monta uma URL a
partir do nome local. O publicador cria `published-manifest.json` com o mapa
`icon_key → URL` e envia esse manifesto por último. O processo é retomável por
hash: uma segunda execução reutiliza URLs já auditadas e não duplica os ícones.

A implementação atual também não preserva o nome nem o `Content-Type` original:
os objetos terminam em `file` e são servidos como `application/octet-stream`.
Isso funciona em `<img>`, pois o conteúdo continua sendo PNG, mas deve ser
revisto no storage-manager caso consumidores passem a exigir metadados corretos.

O bucket padrão é `jeanluca-teste`: ele já permite a ACL `PublicRead` que o
serviço aplica após cada `PutObject`. Buckets novos com `BlockPublicAcls` ativo
fazem a API falhar depois da gravação e não devem ser usados até o
storage-manager migrar de ACL por objeto para bucket policy/presigned URL.

Todas as URLs, hashes, horários e estados ficam em
`docs/audits/item-icons-upload-<pack_version>.json`. Token opcional pode ser
fornecido em `W2PP_STORAGE_MANAGER_TOKEN`, mas nunca entra na auditoria.

## Web-api e implantação

O web-api lê o mesmo manifesto local que foi publicado:

```bash
webserver \
  -content /Release \
  -item-icons-manifest /item-icons/manifest.json
# equivalente: W2PP_ITEM_ICONS_MANIFEST=/item-icons/manifest.json
```

- Caminho ausente/não configurado: modo fallback, `icon_key` e
  `icon_pack_version` vazios.
- Caminho configurado, mas inválido: falha de boot; isso evita servir URLs
  silenciosamente incorretas.
- Manifesto válido: `ItemCatalogService.ListItems` e
  `NpcAdminService.ListItemCatalog` publicam as mesmas chaves e versões.

O deploy deve montar `published-manifest.json` no web-api. Os PNGs permanecem no
bucket/CDN e não transformam o serviço gRPC em servidor de arquivos.

## Riscos e fallback

Os ícones originais pertencem aos titulares do WYD. A decisão adotada é
versionar somente código e formato; cada operador gera os derivados a partir da
sua própria cópia do cliente e decide conscientemente onde publicá-los.

`icon_key == ""`, falha HTTP ou item novo deve cair no fallback procedural do
portal, guiado por `slots[0]` e `grade`. Refino, quantidade, Ancient/selado e
raridade continuam como overlays do frontend porque pertencem à instância do
item, não ao bitmap do catálogo.
