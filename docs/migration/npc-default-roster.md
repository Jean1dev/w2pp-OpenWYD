# NPC padrão — o catálogo curado de NPCs default (issue #29)

> Status: **IMPLEMENTADO**. Origem: issue #29 ("identificar quais são os npc padrão do jogo e
> cadastrar eles em nossa nova estrutura de npc's"). Este doc é a fonte da verdade da curadoria —
> lista **quais** NPCs entram no roster default e **por quê**, complementando
> [npc-editing-plan.md](./npc-editing-plan.md) (a arquitetura da estrutura) e
> [handlers/npc-map.md](./handlers/npc-map.md) (o mapa completo de tipos `Merchant`).

## 1. O que já existia vs. o que faltava

A "nova estrutura de NPC" (`npc_definition`, `dbserver import-npcs`) já sabia **identificar** NPCs
merchant — qualquer bloco de `NPCGener.txt` cujo template líder tenha `CurrentScore.Merchant != 0`.
Rodando contra o conteúdo real deste repo (`go run ./dbserver/cmd/dbserver import-npcs -content
./Release`, dry run): **544 blocos de spawn merchant**, de **205 templates únicos**, cobrindo
**muito mais códigos `Merchant`** do que os 4 que o `NpcAdminService` sabe validar
(`npcadmin.validMerchant = {0,1,2,19,100}`).

O que faltava não era identificação — era **curadoria**: decidir qual subconjunto conta como o
roster "padrão" (estável, documentado, com identidade), em vez de importar cegamente tudo que o
conteúdo bruto contém, incluindo códigos `Merchant` que o painel de moderação nem consegue validar
hoje (64, 96, 100+ variantes de raid/quest ainda não implementadas — ver `npc-map.md`).

## 2. Critério de curadoria

1. **Só os 4 códigos editáveis hoje**: `Merchant ∈ {1 (loja), 2 (guarda de carga/banco), 19 (loja
   tipo 3), 100 (quest)}` — os mesmos que `npcadmin.validMerchant` aceita. Qualquer coisa fora disso
   (raids do Zakum, chefes com `Merchant` usado como gatilho de quest não implementada, etc.) fica de
   fora do roster.
2. **Um representante por arquétipo**, não por variante regional/tier. Vários templates são cópias
   do mesmo NPC para outro continente ou versão: `Acessorios`/`Acessorios2..6`/`AcessoriosErion`
   (mesma loja, cópias diferentes) colapsam para `Acessorios`; `Set_BM`/`Set_BM_Erion` (mesmo
   `DisplayName`, cópia por continente) colapsam para `Set_BM`; mesma lógica para `Arma_*`,
   `Ferreiro*`, `Rainy*`, `Rapein*`, `Torcedor*`, `Martin*`, `Montarias*`.
3. **NPCs que legitimamente repetem por cidade continuam completos.** O filtro casa por
   `TemplateName`, não por bloco/slug — `Guarda_Carga` está no roster **uma vez**, mas isso mantém
   **todos os 8 blocos** que usam esse líder (uma guarda por cidade). Nenhuma cidade perde seu guarda
   de carga por causa da curadoria.
4. **`Merchant == 100` (quest) — só o que já funciona.** De ~20 templates com esse código, só a
   família `Perzen`/`Perzen_Normal`/`Perzen_Mistico`/`Perzen_Arcano` (troca esfera→montaria, grades
   7/8/9) tem handler implementado (`npc-map.md`). O resto (`Treinador1`, `Guarda_da_Sorte`,
   `Coveiro`, etc.) fica fora até ganhar implementação — registrar um NPC de quest sem handler não
   ajuda o moderador a fazer nada além de posicioná-lo.

Resultado: **63 templates únicos**, que resolvem para **80 blocos de spawn** reais (contando as
repetições legítimas por cidade) — contra os 544 blocos merchant que existem no conteúdo bruto.

## 3. O roster

A lista completa e exata é a própria migration (§4) — os 80 `INSERT INTO npc_definition` em
`internal/migrations/0006_default_npc_seed.up.sql`. Este doc resume por categoria:

| Categoria | Merchant | Nº de templates | Exemplos |
|---|---:|---:|---|
| Loja (`shop`) | 1 | 53 | `Acessorios`, `Ferreiro`, `Runas_Joias`, `Set_BM/FM/HT/TK`, `Trajes` |
| Guarda de carga/banco (`cargo_guard`) | 2 | 2 | `Guarda_Carga`, `Angela` |
| Loja especial (`shop_type3`) | 19 | 4 | `Foema_Ancian`, `Mestre_Archi`, `ForeLearner`, `Cap.Cavaleiros` |
| Quest (`quest`) | 100 | 4 | `Perzen`, `Perzen_Normal`, `Perzen_Mistico`, `Perzen_Arcano` |

## 4. Como fica populado — migration, não passo manual

O roster curado (80 blocos / 907 itens de loja) está **gravado como dado estático na migration**
`internal/migrations/0006_default_npc_seed.up.sql` (`INSERT ... ON CONFLICT DO NOTHING`). Os valores
foram gerados **uma vez**, rodando a mesma decodificação que `buildNPCDefinitions`/
`savefmt.DecodeMob` usam contra o `Release/` real deste repo e filtrando à mão para o critério do
§2 — não existe mais um pacote de código com essa lista; a migration **é** a curadoria, já congelada
em SQL.

Como `store.Migrate` roda automaticamente no boot de **qualquer** serviço (dbServer, webServer, ...)
— `CLAUDE.md` "Shared packages" — o roster **já está no banco assim que o primeiro serviço sobe**,
sem precisar rodar `dbserver import-npcs` manualmente. É exatamente o "100% integrado, editável pelo
portal web" que a issue #29 pediu: um moderador pode abrir o painel e mexer nesses NPCs no primeiro
boot.

`internal/migrations/0006_default_npc_seed.down.sql` reverte por `slug` exato (não `TRUNCATE`), então
NPCs que um moderador tenha criado à parte via o portal não são afetados.

`dbserver import-npcs` **não muda** — continua importando todo bloco merchant do conteúdo
(`Merchant != 0`), exatamente como antes desta issue, e continua útil para importar o que ficou fora
do roster curado ou para reimportar depois de uma mudança no conteúdo `Release/`. Como todo insert é
`ON CONFLICT (slug) DO NOTHING`, rodá-lo depois da migration é seguro: só adiciona o que faltar, não
duplica nem sobrescreve os 80 já seedados.

## 5. Risco aceito — leia antes de ligar `-npc-editing`

`tmserver/cmd/tmserver/main.go` pula **todo** bloco de `NPCGener.txt` cujo líder é merchant quando
`-npc-editing` está ativo — **não só os que estão no banco**. Isso significa:

> Se você ligar `-npc-editing` sem antes rodar `dbserver import-npcs` (que traz **todos** os 544
> blocos merchant), todo merchant que **não** está no roster seedado pela migration desaparece do
> jogo — inclusive as lojas regionais colapsadas (ex.: `Acessorios2`..`6`, `Set_BM_Erion`) e
> qualquer coisa fora de `{1,2,19,100}`.

Isso é aceito porque `-npc-editing` já é **opt-in, off por padrão**, e nada em produção liga essa
flag hoje. Mas se algum dia for necessário ligá-la com paridade completa, rode
`dbserver import-npcs -content <Release/> -dsn <dsn>` antes — ver
[npc-editing-plan.md §"Feature flag"](./npc-editing-plan.md).
