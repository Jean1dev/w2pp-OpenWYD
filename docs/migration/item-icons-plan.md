# Plano de viabilidade — Imagens dos itens para o front-end

> Status: **FASE 1 IMPLEMENTADA** (§8). Pergunta que este doc responde: *dá para mostrar a imagem de
> cada item no portal web, e por qual caminho?* Resposta curta: **dá, mas a imagem não vem do
> servidor** — o repositório não tem nenhum pixel de item. O que o servidor tem, e que ninguém
> estava usando, é a **chave** que identifica a imagem. O plano é publicar essa chave num contrato
> estável agora e plugar o pacote de imagens depois, como conteúdo.
>
> O contrato já está publicado: `ItemCatalogService.ListItems` (`api/web/v1/web.proto`) e os campos
> `icon_key`/`display_name`/`slot_mask`/`slots`/`grade`/`mesh`/`texture` em `ItemCatalogEntry`.
> Guia de consumo pelo front: [../integrations/item-icons-nextjs.md](../integrations/item-icons-nextjs.md).

---

## 1. Por que isso importa

O front-end (Next.js BFF, `docs/migration/web-platform-plan.md`) já manipula item em várias telas:
loja de doação (`donate-shop-frontend.md`), recompensa diária (`daily-reward-frontend.md`), DropTool
(`docs/integrations/drop-tool-nextjs.md`), editor de loja de NPC
(`docs/integrations/npc-admin-nextjs.md`), templates de mob (`mob-template-editing-frontend.md`) e
world events (`docs/integrations/world-events-nextjs.md`). Hoje todas mostram `item_index` + `name`
cru (`Botas_Douradas(N)`). Ícone é o que transforma essas telas em algo usável.

> Correção: `my-characters` **não** entra nessa lista — `docs/integrations/my-characters-nextjs.md`
> é explícito que "inventário, equipamentos e buffs ficam fora desta API". Mostrar inventário com
> ícone exige antes uma RPC de inventário, que não existe.

---

## 2. Levantamento: o que existe no repositório

| Recurso | Onde | Serve para imagem? |
|---|---|---|
| `Release/Common/ItemList.csv` | catálogo editável, 6439 linhas / **3220 itens nomeados** | **sim — é a chave**, não a imagem |
| `Release/DBsrv/run/ItemList.bin` | forma compilada, 910 004 B | idem (mesmos campos) |
| `images/sv.bmp` | 3 MB, splash da lista de servidores | não |
| `Release/**` restante | mapas, drops, NPCs, rates | não |

**Não há nenhuma imagem de item no repositório, e não haverá:** o cliente (`WYD.exe` 7662) não faz
parte do repo e não tem fonte aberta — ver `docs/agents/CLIENT-RESEARCH-2026-06-27.md` §1 ("não
existe cliente gráfico 3D de WYD com código-fonte aberto"). Os assets gráficos vivem só dentro da
pasta do cliente oficial.

### 2.1. O que o catálogo carrega de visual

`STRUCT_ITEMLIST` (`Source/Code/Basedef.h:1162-1185`, parse em `Source/Code/Basedef.cpp:5717-5772`):

| Campo | Origem no CSV | Papel visual |
|---|---|---|
| `IndexMesh` | col. 3 antes do ponto (`10.0` → 10) | modelo/aparência (o "set") |
| `IndexTexture` | col. 3 depois do ponto | variante de cor do modelo |
| `IndexVisualEffect` | — (o servidor zera; **0 em 100% do `.bin`**) | efeito/brilho |
| `nPos` | col. 7 | bitmask de `Equip[16]` — a peça (elmo, botas, arma…) |
| `Grade` | col. 9 | 1=Normal 2=Místico 3=Arcano 4=Lendário… |
| `EF_GRID` | par `EF_*` | tamanho na grade — **0 em todo o catálogo atual** |

Decodificação do `nPos` (bit *i* = `STRUCT_MOB.Equip[i]`), confirmada cruzando com os nomes reais:

```
bit0 face/corpo(233)  bit1 elmo(191)   bit2 armadura(191)  bit3 calça(189)
bit4 luvas(188)       bit5 botas(190)  bit6 arma(494)      bit7 escudo(19)
bit8 acessório(28)    bit9 amuleto(20) bit10 orb(15)       bit11 gema(33)
bit12 medalha(55)     bit13 fada(55)   bit14 montaria(146) bit15 manto(36)
64|128 = 192 → arma de duas mãos (238 itens)      nPos = 0 → não equipável (890 itens)
```
(bit13 = fada bate com `Source/Code/TMSrv/CMob.cpp:712` lendo `Equip[13]` para os ids 39xx;
bit14 = montaria bate com `mountEquipSlot = 14` em `tmserver/internal/handler/character.go:662`.)

### 2.2. A conclusão que destrava o plano

O cliente **lê o mesmo `ItemList.bin`** que o servidor. Logo, na hora de desenhar um item ele não
tem nada além desses campos — ou seja:

> **A imagem do item é função de `(IndexMesh, IndexTexture, nPos)`, não do `item_index`.**

Isso muda a escala do problema: os 3220 itens nomeados colapsam em **1055 chaves visuais
distintas** (617 meshes, 892 pares mesh.texture). Um pacote de ícones precisa cobrir ~1k imagens,
não 6,5k. É por isso que armadura/calça/luva/bota de um mesmo set compartilham `IndexMesh` (o set
"Malha" inteiro é mesh 4) e se diferenciam pelo `nPos`.

> **UNVERIFIED:** a *fórmula exata* que o cliente usa para ir de `(mesh, texture, pos)` ao bitmap
> (atlas? arquivo por mesh? tabela intermediária tipo `meshlist.bin`?) não foi confirmada — exige o
> cliente em mãos. §7 descreve o experimento que confirma ou derruba isso. Se cair, o plano não
> muda de forma: só a função que gera `icon_key` muda.

### 2.3. Bônus do levantamento: o `.bin` foi decodificado

`Release/DBsrv/run/ItemList.bin` = **6500 registros × 140 bytes, ofuscado com XOR 0x5A plano, sem
cabeçalho** (sobram 4 bytes no fim). Os comentários de offset no header (`Price // 92`) são de uma
versão antiga da struct com nome curto; com `ITEMNAME_LENGTH=64` (`Basedef.h:203`) o layout real é
`Name[64] + 8 shorts + 12×(short,short) + int Price + 4 shorts = 140`. **Já incorporado** em
`data-formats.md` §3.1, e reimplementado em Go como teste de regressão
(`webserver/internal/itemcatalog/bindrift_test.go`).

Cruzando `.csv` × `.bin`: **3193 de 3216 itens batem**; 23 divergem em mesh (Dríade Selado,
Sephirot, Pedra/Cupom da Imortalidade). O `.bin` está **desatualizado** em relação ao `.csv` — o
`.csv` é a fonte de verdade porque é ele que os serviços Go carregam
(`tmserver/internal/content/catalog.go`). Esse check **já roda no CI**: os 23 estão fixados como
baseline em `webserver/internal/itemcatalog/bindrift_test.go`, dentro do `go test` que já existia —
sem Python no pipeline. Um item entrando ou saindo do conjunto quebra o teste.

---

## 3. Onde estão as imagens de verdade (pesquisa externa)

| Fonte | O que é | Avaliação |
|---|---|---|
| Cliente oficial 7662 | ícones dentro dos recursos do cliente (`.wys` + `.msh/.msa`; interface na pasta `UI/`) | fidelidade máxima; exige o cliente e um unpacker |
| Pack comunitário de ícones | "praticamente todos os ícones", **PNG 35×35 com fundo preto** ([WebCheats](https://www.webcheats.com.br/topic/2271954-%C3%ADcones-itens-wyd/)) | caminho mais rápido; resolução baixa, fundo preto, procedência incerta |
| Sites de droplist (ex.: [wydmisc](https://wydmisc.raidhut.com.br/droplist/global/index.html?type=item)) | já servem ícone por item | só como *referência* de que o mapeamento existe — hotlink está fora de cogitação |
| Ferramentas da comunidade ([WYD-ItemList-Editor](https://github.com/Rechdan/WYD-ItemList-Editor), [WYD-Tools](https://github.com/davirs/WYD-Tools)) | editores de `ItemList.bin` e outros formatos | úteis para conferir o layout e talvez o formato dos recursos |

> Nota de ambiente: `webcheats.com.br` e `wydmisc.raidhut.com.br` estão **bloqueados pelo proxy de
> egress** desta sessão (403 no CONNECT). As duas primeiras linhas da tabela precisam ser
> verificadas manualmente por um humano antes da Fase 2.

---

## 4. Opções avaliadas

| # | Abordagem | Fidelidade | Esforço | Risco | Veredito |
|---|---|---|---|---|---|
| A | Extrair do cliente oficial (unpack dos recursos) | alta | médio-alto | jurídico (assets HanbitSoft/JoyImpact) + formato desconhecido | **fonte alvo** |
| B | Pack comunitário PNG 35×35 | média-baixa (fundo preto, 35 px) | baixo | licença/procedência incertas; precisa remapear as chaves | **atalho da Fase 2** |
| C | Renderizar os meshes 3D em qualquer resolução | alta | alto (parser `.msh` + pipeline de render) | depende do cliente do mesmo jeito | Fase 3 opcional |
| D | Fallback procedural (SVG/CSS por `slot` + `grade` + classe) | baixa (não é o ícone real) | muito baixo | nenhum | **entrega já, e é a rede de segurança permanente** |
| E | Gerar com IA | inconsistente entre itens | médio | visual incoerente com o jogo | descartado |
| F | Hotlink de sites de terceiros | — | baixo | quebra, latência, uso indevido | descartado |

**Recomendação: D agora, A (ou B) como camada de conteúdo por cima**, com o contrato de §5 no meio.
O front-end integra uma vez e não muda quando a fonte da imagem mudar.

---

## 5. Arquitetura proposta

```
ItemList.csv ──(build)──> items.json (manifesto)  ──> Next.js: item_index → icon_key
                                                            │
                          item-icons/ (pacote separado) ────┘ icon_key → <img src>
                                                            │ 404
                                                            └─> fallback SVG por slot+grade
```

1. **`icon_key` derivado do catálogo.** Formato `m<mesh>_t<texture>_p<pos>` (ex.: `m10_t0_p32` =
   Botas Douradas). Estável, ~1055 valores, calculável offline e no servidor.
2. **Manifesto `items.json`** gerado no build a partir do `.csv`: `index, name, display_name,
   icon_key, slots, grade, class_mask, req_level, price`. ~1 MB cru, ~120 KB gzip. O catálogo é
   **imutável em runtime** — pode ser servido como estático com cache longo, sem RPC por item.
3. **Pacote de imagens em repositório/bucket separado** (`item-icons/`), *não* neste repo:
   `icons/<icon_key>.webp` (64 e 128 px, fundo transparente) + `manifest.json` com hash. Manter
   fora do repo do servidor isola a questão de licença (§6) e não infla o clone.
4. **Fallback obrigatório no front:** `icon_key` sem arquivo → SVG por `slot` (arma, elmo, poção…)
   colorido por `grade`. Nenhuma tela quebra por ícone faltando; a Fase 1 entrega já com 100% de
   fallback.
5. **Decoração fica no front, não na imagem:** refino (`+1..+9`), aura por `grade`, quantidade
   (`EF_AMOUNT`), marca de "selado". Assar isso no PNG multiplicaria o pacote por 10.
6. **Contrato gRPC (FEITO, e foi este o caminho escolhido):** `ItemCatalogEntry`
   (`api/web/v1/web.proto`) ganhou `icon_key`, `display_name`, `slot_mask`, `slots`, `grade`,
   `mesh` e `texture`, e existe um `ItemCatalogService.ListItems` público (sem `moderator_id`) para
   as telas de jogador. `NpcAdminService.ListItemCatalog` continua para a moderação e compartilha o
   mesmo mapper.

   > Correção ao levantamento original: a implementação **não** tinha "tudo em mãos".
   > `webserver/internal/itemcatalog` lia apenas as duas primeiras colunas do CSV
   > (`Entry{Index, Name}`), então faltava o parser das colunas visuais — não só o mapper. A coluna
   > 2 (`mesh.texture`) não era parseada em Go em lugar nenhum; `content/catalog.go` já tinha
   > `Grades()` (col. 8) e `Positions()` (col. 6).

   O manifesto `items.json` estático foi **descartado** em favor da RPC: o webserver é gRPC puro
   (não serve HTTP), e uma RPC segue o padrão existente sem inventar uma convenção de asset
   estático que o repo não tem. O efeito de cache foi preservado via `catalog_version`.

### 5.1. O que já existe para apoiar

`scripts/item-icon-manifest.py` (neste commit) lê o `.csv`, emite o manifesto e opcionalmente faz o
diff contra o `.bin`:

```bash
./scripts/item-icon-manifest.py -o items.json --check-bin
# items: 3220  distinct icon_key: 1055
# bin rows: 3216  visual drift csv!=bin: 23  only in bin: 0
```

---

## 6. Riscos

| Risco | Impacto | Mitigação |
|---|---|---|
| **Jurídico** — ícones são assets da HanbitSoft/JoyImpact | distribuir num CDN público é exposição real | pacote de imagens fora deste repo; default é o fallback procedural (D); decisão consciente do dono do projeto antes da Fase 2 |
| Chave `(mesh, texture, pos)` pode não ser o que o cliente usa | refaz o gerador de `icon_key` | experimento §7 **antes** de produzir o pacote; o contrato (`icon_key` opaco) absorve a mudança |
| `.bin` divergente do `.csv` (23 itens) | ícone errado em itens específicos | `.csv` como fonte única + check no CI com `--check-bin` |
| Pack comunitário 35×35 com fundo preto | visual ruim em tema claro | remoção de fundo em lote + upscale, ou pular direto para A/C |
| Nomes em Latin-1 com `_` | `Botas_Douradas(N)` na UI | `display_name` já sai normalizado no manifesto |
| Itens novos do servidor (não-originais) | sem imagem no pack | fallback cobre; arte própria depois |

---

## 7. Experimento que decide a Fase 2 (meio dia)

1. Obter uma cópia do cliente 7662 e listar os arquivos de recurso (`.wys`, pasta `UI/`).
2. Extrair/abrir o container e localizar o conjunto de ícones de inventário.
3. Verificar como cada ícone é endereçado: por `IndexMesh`? por posição num atlas? por uma tabela
   auxiliar tipo `meshlist.bin`?
4. Conferir 20 itens conhecidos e diversos — `Botas_Douradas(N)` (m10/p32), `Garra` (m837/p64),
   `Cupom_da_Sorte` (m2711/p0), set Malha inteiro (m4, cinco `nPos`), um item com `texture=1`
   (Dríade Selado) e uma montaria (m943/p16384).
5. Saída: ou o gerador de `icon_key` está certo (segue Fase 2 direto), ou sai dali a regra correta.

---

## 8. Faseamento e esforço

| Fase | Entrega | Esforço | Depende de |
|---|---|---|---|
| 0 | Manifesto + relatório de cobertura (`scripts/item-icon-manifest.py`) | **feito** | — |
| 1 | `icon_key`/`slots`/`grade` no contrato + fallback SVG especificado | **feito** | nada |
| 2 | Pacote de ícones reais (A ou B), WebP 64/128, publicado | ~2-4 dias | experimento §7 + decisão de licença |
| 3 | Render 3D dos meshes p/ alta resolução, ou arte própria nas lacunas | ~1 semana | opcional |

Fase 1 já resolve a dor descrita no pedido (telas usáveis, item reconhecível por slot/raridade) sem
depender de nada externo. Fase 2 é o "bonito", e é a única que carrega risco jurídico.

**O que a Fase 1 entregou neste repositório** (o portal Next.js é um projeto externo — não há código
TypeScript aqui, então o fallback SVG está *especificado*, não implementado):

- `webserver/internal/itemcatalog`: parser das colunas visuais + `icon_key`, e `Catalog.Version`
  (hash do `ItemList.csv`) para o BFF cachear.
- `api/web/v1/web.proto`: campos 3..9 de `ItemCatalogEntry` (aditivos) e `ItemCatalogService`.
- `webserver/internal/grpcsrv/itemcatalog.go`: o serviço + o mapper compartilhado com a moderação.
- Testes: casos de parity do bitmask (manto `-32768`, duas mãos `192`, `nPos = 0`), os totais do
  catálogo real (3220 itens / 1055 chaves) e o baseline de drift `.csv` × `.bin`.
- `docs/integrations/item-icons-nextjs.md`: contrato, tabela bit→slot, regra do fallback e o
  Route Handler de exemplo.

---

## 9. Decisões que precisam de um humano

1. **Distribuir assets extraídos do cliente oficial?** (Fase 2 A/B). Se a resposta for não, o plano
   para na Fase 1 + Fase 3 com arte própria. **Ainda em aberto** — é o que bloqueia a Fase 2.
2. ~~**Manifesto estático ou RPC pública?**~~ **Decidido: RPC pública** (`ItemCatalogService`), com
   `catalog_version` para o BFF cachear. Ver §5.6.
3. **Onde hospedar o pacote de imagens** — repo separado + CDN, ou bucket do próprio portal.
   **Ainda em aberto**, e só importa a partir da Fase 2.
