# Plano: Editor de stats de mob/NPC (equivalente ao legado EDITAPPMOB)

> Status: **IMPLEMENTADO** (backend + estruturas de dados; front-end Next.js fora de escopo,
> mesma decisão de `npc-editing-plan.md`). Origem: issue #167 — "existe esse projeto na source
> legado, identifique como ou se eh utilizado no nosso projeto migrado, e precisamos criar uma
> ferramenta equivalente no nosso portal web". Guia de integração pro front-end (contrato gRPC,
> exemplos de payload, semântica de UI): [`mob-template-editing-frontend.md`](./mob-template-editing-frontend.md).

## 1. O que é o "EditaPPMob" da issue

O nome na issue é uma leitura corrida de **`EDITAPPMOB`** (`Source/Code/EDITAPPMOB/`, binário
em `Release/{TMsrv,DBsrv}/run/EDITAPPMOB.exe`) — "Edita-App-Mob": um editor Win32 (`Main.cpp` +
`File.cpp`) que lê **todos** os arquivos de `./npc/*` (cada um um `STRUCT_MOB` cru de 816 bytes,
o mesmo formato compartilhado por players e mobs) e permite editar e salvar de volta:

- Identidade/raça/classe: `MobName`, `Clan`, `Merchant`, `Class`.
- Economia: `Coin`, `Exp`.
- Posição embutida no template: `SPX`/`SPY`.
- Combate: `BaseScore.{Level, Ac, Damage, ChaosRate, Str, Int, Dex, Con, Special[4], Hp, MaxHp,
  Mp, MaxMp}`, direção/velocidade (`AttackRun`/`Direction`), com o save espelhando
  `CurrentScore = BaseScore`.
- Skills: `SkillBar[4]`, `LearnedSkill`, `ScoreBonus`, `RegenHP`/`RegenMP`, `Resist[4]`.
- Equipamento: `Equip[16]`.
- Loja: `Carry[64]` (estoque do merchant).
- Botões `Save`/`Reload`/`Clear` (zera `Carry[]`)/`Delete` (apaga o arquivo `npc/<nome>`).

Não existe conceito de "PP" no código-fonte, na documentação ou nos comentários — é apenas a
substring de "editA-APPmob". `EDITAPSHOP` (`Source/Code/EDITAPPSHOP/`) é um fork quase idêntico,
mesma struct, focado em edição de loja.

**Não havia port anterior desta ferramenta** no projeto migrado (`grep -rin "ppmob"` em
`tmserver/`, `dbserver/`, `binserver/`, `webserver/`, `internal/` → zero ocorrências). O que já
existia era uma cobertura **parcial** por sobreposição de responsabilidade:
[`npc-editing-plan.md`](./npc-editing-plan.md) (status IMPLEMENTADO) entrega um painel de
moderação para o subconjunto **posição de spawn / visibilidade / loja** dos NPCs merchant
(tabelas `npc_definition` + `npc_shop_item` + `item_price`, serviço gRPC `NpcAdminService`,
overlay em `tmserver/internal/handler/npcconfig.go`) — mas **não** cobre os campos de stats de
combate (Level/HP/MP/atributos/EXP/skills/equip) que são o grosso do que o `EDITAPPMOB` edita.
Esta é a lacuna que este plano fecha.

## 2. Decisões de escopo

- **Backend apenas.** O portal Next.js (BFF) ainda não existe neste repositório — só o
  `webserver` (gRPC) existe hoje. Mesma decisão de `npc-editing-plan.md`: o front-end consumirá
  a mesma borda `web-api` quando for construído; fora de escopo aqui.
- **Cobertura completa de campos**, paridade real com o `EDITAPPMOB`, **exceto**:
  - `Carry[]` (loja) — já coberto por `npc_shop_item` (não duplicado).
  - Posição de spawn do subconjunto DB-managed — já coberto por `npc_definition.pos_x/pos_y`.
  - Renomear/apagar o arquivo de template — `Release/` é montado **read-only** em produção
    (docker-compose); "deletar" no novo tool remove a linha de override (volta ao default do
    arquivo), nunca o arquivo em si.
  - `SpecialBonus`, `SkillBonus`, `Critical`, `GuildLevel`, `Magic` — o `EDITAPPMOB` legado
    também não edita esses campos.

## 3. Achado que simplificou a implementação: `internal/savefmt` já modela o `STRUCT_MOB` inteiro

`internal/savefmt/structs.go` + `codec.go` já tinham um **codec completo e testado** do
`STRUCT_MOB` (`type Mob struct{...}`, `DecodeMob`), reaproveitado por `dbserver import-npcs` e
por `webserver/internal/npctemplates.Scan`. Só faltava o `encodeMob` **exportado** — adicionado
como `savefmt.EncodeMob(m Mob) []byte`, um wrapper trivial sobre o encoder privado já existente.
Isso eliminou a necessidade de reimplementar o mapa de offsets do `STRUCT_MOB`: o trabalho novo
foi modelagem de persistência (Postgres) + CRUD gRPC + o ponto de aplicação no tmServer.

## 4. Arquitetura: overlay nos bytes do template, aplicado só no boot

Generaliza o padrão já usado em `tmserver/internal/handler/npcconfig.go:
npcTemplateWithDisplayName` (copiar o template cru e sobrescrever campos antes de
`SpawnMobAt`): `savefmt.DecodeMob(template)` → aplica os campos do override na cópia → espelha o
`Score` editado em **ambos** `BaseScore` e `CurrentScore` (replica o `mob->CurrentScore =
mob->BaseScore` do `EDITAPPMOB` no save) → `savefmt.EncodeMob(mob)` → bytes prontos pra
`SpawnMobAt`. Implementado em `tmserver/internal/mobstat.Apply`.

**Aplicação só no boot, sem hot-reload** — decisão deliberada, não uma lacuna: o `EDITAPPMOB`
original também exigia restart do servidor pra uma edição valer (ele escreve o arquivo no disco;
o server só lê `npc/<nome>` na inicialização). Replicar esse mesmo comportamento é paridade real,
e evita todo o risco de um hot-reload tocando milhares de mobs vivos em memória.

**Aplicação universal, não só nos NPCs geridos pelo banco** — diferente do overlay de
posição/loja (só vale pro subconjunto merchant em `npc_definition`), o override de stats vale
pra **qualquer** template `npc/<nome>`, incluindo monstros comuns que nascem só via
`NPCGener.txt` — esse é o caso de uso principal de rebalanceamento que o `EDITAPPMOB` servia. Por
isso o overlay é ligado por uma **flag própria**, independente de `-npc-editing`
(`-mob-stat-editing` / `W2PP_MOB_STAT_EDITING`), e o ponto de aplicação fica nos dois lugares que
chamam `content.LoadNPCTemplate` em `tmserver/cmd/tmserver/main.go`: o closure `load` dentro de
`spawnNPCs` (cobre todo spawn vindo de `NPCGener.txt`) e o `TemplateLoader` passado a
`dbclient.NewNpcConfig` (cobre os NPCs geridos pelo banco). Os dois sistemas
(`npc_definition`/posição-loja vs. `mob_template_stat`/atributos) permanecem ortogonais — nenhum
exige o outro.

## 5. Modelo de dados

Migration `internal/migrations/0013_mob_template_stats.up.sql`, seguindo o estilo de
`0005_npc_editing.up.sql`:

- **`mob_template_stat`** — override completo de atributos, chaveado por `template_name`
  (aponta pro arquivo em `Release/TMsrv/run/npc/`). Uma linha = override ativo; ausência de
  linha = usa os bytes crus do arquivo (comportamento de hoje, sem mudança). Cobre
  `clan/merchant/class/coin/exp/spx/spy`, o bloco de combate (`level/ac/damage/chaos_rate/
  attack_run/direction/str/intel/dex/con/special1..4/max_hp/hp/max_mp/mp`), skills
  (`learned_skill/score_bonus/skill_bar1..4/regen_hp/regen_mp/resist1..4`) e `display_name`
  (override cosmético do nome, mesmo mecanismo de `npcTemplateWithDisplayName`).
- **`mob_template_equip`** — override de `Equip[16]` por slot, mesmo padrão de `npc_shop_item`
  (FK pra `mob_template_stat`, `ON DELETE CASCADE`).

Auditoria: reaproveita as tabelas **já existentes** `npc_audit`/`npc_config_meta` (`internal/
store/npc.go:auditAndBump`) em vez de uma trilha paralela — bumpar `npc_config_meta.version` por
uma escrita de stat é inofensivo (o poll do tmServer pra `npc_definition` só faz um reconcile
no-op). Ações novas: `create_template_stat`, `update_template_stat`, `set_template_equip`,
`delete_template_stat`.

## 6. Contratos gRPC

- **`api/db/v1` (leitura pelo tmServer)** — novo RPC no `NpcConfigService` existente:
  `ListMobTemplateStats` → `repeated MobTemplateStat overrides`. Deliberadamente **sem** campo de
  versão (não há hot-reload nesta entrega) e **sem** acoplamento a `ListNpcDefinitions`.
- **`api/web/v1` (CRUD do moderador)** — novo serviço `MobTemplateAdminService`, irmão do
  `NpcAdminService`: `ListMobTemplates` (todo arquivo em `npc/`, **sem** o filtro de merchant que
  `NpcAdminService.ListMerchantTemplates` aplica — monstros são o caso de uso principal),
  `GetMobTemplateStat` (com **leitura through**: sem override salvo, lê o arquivo cru via
  `savefmt.DecodeMob` e devolve os valores atuais com `overridden=false`, replicando o
  "abre e já mostra os valores atuais" do `EDITAPPMOB` sem precisar de importador de seed em
  massa), `UpsertMobTemplateStat`, `SetMobTemplateEquip`, `DeleteMobTemplateStat`.

## 7. Mapa de arquivos (implementação)

- **Codec:** `internal/savefmt/codec.go` (`EncodeMob`, novo export).
- **Modelo/migration:** `internal/migrations/0013_mob_template_stats.{up,down}.sql`, tipos em
  `internal/domain/domain.go` (`MobTemplateStat`, `MobTemplateEquipItem`).
- **Store:** `internal/store/mobtemplate.go` (CRUD + auditoria/versão via `auditAndBump`);
  integração em `internal/store/mobtemplate_integration_test.go` (`-tags=integration`).
- **Protos:** `api/db/v1/db.proto` (`NpcConfigService.ListMobTemplateStats`); `api/web/v1/
  web.proto` (`MobTemplateAdminService`).
- **dbServer:** `dbserver/internal/grpcsrv/npcconfig.go` (serve `ListMobTemplateStats`).
- **web-api:** `webserver/internal/mobtemplates` (scan **sem filtro** do conteúdo, ao contrário
  de `npctemplates`), `webserver/internal/mobtemplateadmin` (papel + validação + leitura
  through, testes em `service_test.go`), `webserver/internal/grpcsrv/mobtemplateadmin.go`,
  wiring em `webserver/cmd/webserver/main.go`.
- **tmServer:** `tmserver/internal/mobstat` (tipo `Override` + `Apply`, testes em
  `mobstat_test.go`), `tmserver/internal/dbclient/mobstatconfig.go` (`MobStatSource.Fetch`),
  wiring da flag `-mob-stat-editing` + os dois pontos de aplicação (`spawnNPCs`'s `load` e o
  `TemplateLoader` de `dbclient.NewNpcConfig`) em `tmserver/cmd/tmserver/main.go`.

### Flag (opt-in), independente de `-npc-editing`

`-mob-stat-editing` / `W2PP_MOB_STAT_EDITING`, off por padrão. Ligada, exige `-dbserver` (fonte
dos overrides) **e** `-content` (pra resolver os templates); faltando qualquer um, o tmServer
falha no boot com erro claro — mesmo padrão de validação fail-fast de `-npc-editing`. Sem a
flag, o comportamento é idêntico ao de hoje (bytes crus, sem overlay).

## 8. Relação com o resto da plataforma

Este plano é **irmão** de `npc-editing-plan.md`: reusa toda a infraestrutura de auditoria/versão
já construída, mas modela um domínio de dados ortogonal (atributos de combate do template vs.
posição/visibilidade/loja da instância gerida pelo banco). A mesma razão de segurança se aplica:
**definição de template não é estado de jogador** — o fluxo é de via única (web escreve
config fria; tmServer materializa no boot), sem risco de clobber.
