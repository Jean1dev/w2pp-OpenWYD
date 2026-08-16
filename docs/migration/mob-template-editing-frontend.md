# Editor de stats de mob/NPC — Guia de Integração Front-end (issue #167)

> Público: quem constrói o **BFF Next.js** (Route Handlers / Server Actions) e a tela de **painel
> admin** (moderador) que edita stats de combate de templates `npc/<template_name>` (equivalente
> web do legado Win32 `EDITAPPMOB`). Este doc descreve o contrato gRPC exposto pela `web-api`
> (serviço Go `webserver`). Fonte da arquitetura: [`mob-template-editing-plan.md`](./mob-template-editing-plan.md)
> e [`web-platform-plan.md`](./web-platform-plan.md). Irmão de [`npc-editing-plan.md`](./npc-editing-plan.md)
> (esse edita posição/visibilidade/loja; este edita Level/HP/MP/atributos/EXP/skills/equip).

## 1. Como o front fala com o backend

```
Browser ──HTTPS──> Next.js (Route Handlers / Server Actions = BFF)  ──gRPC + mTLS──> web-api (:7600)
         cookie de sessão httpOnly            (só server-side)                     └─ Postgres
```

Regras que **não** mudam (mesmas de qualquer outro serviço admin do `web-api`):

- **O browser nunca fala gRPC** nem vê o certificado mTLS. Toda chamada sai do lado servidor do
  Next.js (Route Handler / Server Action), com os stubs gerados de `api/web/v1/web.proto`.
- O BFF guarda em cookie `httpOnly` de sessão o `account_id` e o `role` do usuário (obtidos no login
  via `AccountWebService.VerifyCredentials`). **Nunca** aceite `account_id`/`moderator_id` vindos do
  browser — use sempre o valor da sessão. Ver seção 7.
- **Autorização é reconferida no servidor.** O `role` no cookie serve só para mostrar/esconder a UI
  de moderador. Toda RPC de `MobTemplateAdminService` revalida `account.role` no backend; um cookie
  adulterado recebe `ADMIN_RESULT_FORBIDDEN`.

Endereço do serviço: `web-api` em `:7600` (flag `-addr`, mTLS via `-tls-*`). Pacote proto:
`web.v1`. Gere os stubs a partir de `api/web/v1/web.proto`.

**Sem overlay ativo no servidor de jogo, a edição não vale.** O tmServer só aplica essas
overrides se subir com `-mob-stat-editing` (e `-dbserver`/`-content`) — e **só no boot** (sem
hot-reload, paridade com o `EDITAPPMOB` legado, que também exigia restart). Se o painel salvar uma
edição e o jogo não refletir, o primeiro suspeito é essa flag, não o BFF. Vale mostrar um aviso
fixo na tela do editor: *"a mudança só vale após o próximo restart do servidor de jogo"*.

## 2. Convenção de resultados (importante)

As RPCs **não** usam códigos de erro gRPC para regras de negócio — só para falha de infra
(indisponibilidade, timeout). O resultado de negócio vem **no corpo**, no enum `AdminResult`
(o mesmo já usado por `NpcAdminService`/`DonateAdminService`):

| valor | quando | UI sugerida |
|---|---|---|
| `ADMIN_RESULT_OK` (1) | sucesso | seguir |
| `ADMIN_RESULT_FORBIDDEN` (2) | não é moderador/admin | 403 / esconder |
| `ADMIN_RESULT_INVALID` (3) | validação falhou (slot fora de 0-15, `item_index` ≤ 0, slot duplicado, `template_name` vazio…) | erro de formulário |
| `ADMIN_RESULT_NOT_FOUND` (4) | template sem override salvo (e sem leitura through possível) — ou `SetMobTemplateEquip`/`Delete` num template sem override ainda | 404 |

`AdminAck` = `{ AdminResult result }` (resposta de `SetMobTemplateEquip`/`DeleteMobTemplateStat`).

## 3. Modelo de dados

### 3.1 `MobTemplateFile` — item do picker (`ListMobTemplates`)

| campo | tipo | descrição |
|---|---|---|
| `template_name` | string | nome exato do arquivo em `Release/TMsrv/run/npc/` — valor a enviar em `GetMobTemplateStat`/`Upsert...`/`SetMobTemplateEquip`/`Delete...` |
| `display_name` | string | `mob.Name` do arquivo, só para exibir na lista/busca |
| `merchant` | int32 | `CurrentScore.Merchant` cru do arquivo — **informativo apenas** (ver nota abaixo sobre o campo `merchant` duplicado) |

**Diferente de `NpcAdminService.ListMerchantTemplates`: esta lista NÃO é filtrada por merchant.**
Ela traz **todo** arquivo em `npc/` — monstros comuns são o caso de uso principal deste editor
(rebalanceamento), não só lojistas.

### 3.2 `AdminMobTemplateStat` — o formulário completo (`Get`/`Upsert`)

Cobertura completa de campos do `EDITAPPMOB` legado, **exceto** `Carry[]` (estoque de loja — já
editado por `NpcAdminService.SetNpcShop`) e a posição de spawn do subconjunto gerido pelo banco
(já em `NpcAdminService.UpsertNpc`).

| campo proto | tipo | grupo de UI sugerido | observação |
|---|---|---|---|
| `template_name` | string | — | chave; preencher a partir do picker (3.1), nunca digitar à mão |
| `overridden` | bool | — | **só em `GetMobTemplateStat`**; `false` = não há override salvo, os valores vieram do arquivo cru (leitura through) — ver seção 6.2 |
| `display_name` | string | Identidade | `""` mantém o nome original do arquivo |
| `clan` | int32 | Identidade | raça/facção (`Clan`) |
| `merchant` | int32 | Identidade | `STRUCT_MOB.Merchant` cru — **distinto** de `NpcAdminService`'s campo de merchant (posição/loja); editar aqui não afeta a classificação de loja do NPC gerido pelo banco |
| `class` | int32 | Identidade | classe do mob |
| `coin` | int32 | Economia | ouro dropado |
| `exp` | int64 | Economia | EXP dropada — **atenção**: 0 ou > 10.000.000 num monstro real (`level >= 1`, `merchant == 0`) é sinal de template mal calibrado (ver `cmd/exptool`); vale um aviso na UI se o moderador digitar um valor fora dessa faixa |
| `spx`, `spy` | int32 | Posição | posição embutida no template (não confundir com `pos_x`/`pos_y` de `npc_definition`) |
| `level`, `ac`, `damage` | int32 | Combate | nível, armor class, dano |
| `chaos_rate` | int32 | Combate | agressividade/PK rate |
| `attack_run` | int32 | Combate | velocidade de ataque/corrida |
| `direction` | int32 | Combate | direção inicial (0-7) |
| `str`, `intel`, `dex`, `con` | int32 | Atributos | atributos base (campo é `intel`, não `int` — palavra reservada) |
| `special1..4` | int32 | Atributos | atributos especiais (Special[4]) |
| `max_hp`, `hp`, `max_mp`, `mp` | int32 | Combate | vida/mana |
| `learned_skill` | int32 | Skills | id da skill aprendida |
| `score_bonus` | int32 | Skills | bônus de pontos de atributo |
| `skill_bar1..4` | int32 | Skills | barra de skill (4 slots) |
| `regen_hp`, `regen_mp` | int32 | Skills | regeneração passiva |
| `resist1..4` | int32 | Skills | resistências elementais (assinado: valores negativos são válidos) |
| `equip` | `AdminMobTemplateEquipItem[]` | Equipamento | ver 3.3; sub-recurso com seu próprio RPC de escrita |

> `BaseScore`/`CurrentScore` não existem como campos separados aqui: o tmServer espelha o mesmo
> valor editado em ambos ao aplicar o override (paridade com `mob->CurrentScore = mob->BaseScore`
> do `EDITAPPMOB` no save). Não peça esse detalhe ao moderador.

### 3.3 `AdminMobTemplateEquipItem` — um slot de `Equip[16]`

| campo | tipo | descrição |
|---|---|---|
| `slot` | int32 | 0 a 15 (`MAX_EQUIP`); um slot por item, sem duplicar |
| `item_index` | int32 | id do item no jogo. Deve ser > 0 |
| `eff1`,`effv1`,`eff2`,`effv2`,`eff3`,`effv3` | int32 | três pares efeito/valor (encantamento/refino), mesmo formato de `AdminNpcShopItem`. 0 = sem efeito |

> Picker de itens: reutilize `NpcAdminService.ListItemCatalog` (`item_index` → nome) no editor de
> equipamento, em vez de pedir `item_index` cru — mesma recomendação já vale pro editor de loja.
>
> Ícone do item: as entradas do catálogo trazem `icon_key`, `display_name`, `slots` e `grade` — ver
> [item-icons-nextjs.md](../integrations/item-icons-nextjs.md).
>
> **Atenção à colisão de nomes:** o `slot` da tabela acima é o índice de `Equip[]` **sendo editado**
> (0..15). O `slot_mask`/`slots` do catálogo é onde o item **pode** ser equipado. Dá para usar o
> segundo para validar o primeiro no formulário (avisar quando o item escolhido não pertence àquele
> slot), mas o backend não faz essa checagem.

**Semântica "autoritativa" (importante para o formulário):** `SetMobTemplateEquip` **substitui a
lista inteira** — slots não enviados voltam a ficar vazios (não ficam com o valor antigo do
template). Se o moderador só quer trocar um slot, o front precisa mandar a lista completa
(os 16 slots atuais + a alteração), não um diff. Um editor de "grade de 16 slots, salva tudo de
uma vez" é o modelo mental certo, não um PATCH incremental por slot.

## 4. Serviço `MobTemplateAdminService` (painel do moderador)

Todo request tem `moderator_id` como **primeiro campo** — preencha com o `account_id` da sessão do
moderador; o backend revalida o `role`.

### `ListMobTemplates(moderator_id) → { result, MobTemplateFile[] }`
Lista **todo** arquivo `npc/<nome>` do content tree (não filtrado), pro picker de `template_name`.
Vem vazio se o `web-api` não tiver `-content`/`W2PP_CONTENT` configurado — trate como "picker
indisponível", não como erro.

### `GetMobTemplateStat(moderator_id, template_name) → { result, AdminMobTemplateStat stat }`
Abre o editor de um template. **Leitura through**: se não há override salvo ainda, o backend lê o
arquivo cru do content tree e devolve os valores atuais dele com `stat.overridden = false` — a UI
já abre mostrando os valores reais do template, não zeros. Salvar (`Upsert`) a partir daí passa a
criar o primeiro override. `template_name` inexistente (nem override, nem arquivo) →
`ADMIN_RESULT_NOT_FOUND`.

### `UpsertMobTemplateStat(moderator_id, AdminMobTemplateStat stat) → { result }`
Cria (se não havia override) ou substitui por completo o override do `stat.template_name`. Não é
parcial — mande o objeto inteiro (o formulário deve ler `Get` antes de editar, não montar do zero).
Validação (`template_name` vazio) → `ADMIN_RESULT_INVALID`.

### `SetMobTemplateEquip(moderator_id, template_name, AdminMobTemplateEquipItem[] items) → AdminAck`
Substitui o `Equip[]` do override (ver semântica autoritativa em 3.3). **Requer que já exista um
override** para `template_name` (chame `Upsert` primeiro se `GetMobTemplateStat` voltou
`overridden = false`) — sem isso, `ADMIN_RESULT_NOT_FOUND`. Slot fora de 0-15, `item_index` ≤ 0 ou
slot duplicado na lista → `ADMIN_RESULT_INVALID`.

### `DeleteMobTemplateStat(moderator_id, template_name) → AdminAck`
Remove o override — o template volta a usar os valores crus do arquivo (o `EDITAPPMOB` nunca fez
isso via UI, ele reescrevia o arquivo; aqui "deletar" é reverter pro default, não apagar nada em
disco). **Nunca** apaga o arquivo `npc/<nome>` em si — `Release/` é montado read-only em produção.

## 5. Fluxo de sessão (login web)

O login web é separado do login do jogo (CPSock). O BFF autentica via
`AccountWebService.VerifyCredentials(name, password)`, que retorna `{ ok, account_id, blocked, role }`.
Grave `account_id` e `role` no cookie de sessão httpOnly e use-os nas chamadas acima. `role ∈
{player, moderator, admin}` — só `moderator`/`admin` enxergam este painel (e o backend reconfere).

## 6. Detalhes que a UI precisa saber

### 6.1 Nada disso afeta mobs já vivos
Diferente de um CRUD comum, salvar aqui **não muda nada em memória agora**. É config fria em
Postgres que o tmServer materializa só no próximo boot. Não existe "aplicar e ver na hora" — deixe
isso explícito na UI (ex.: um banner "mudanças pendentes até o próximo restart do servidor").

### 6.2 `overridden = false` não é erro
É o estado normal de um template nunca editado. A UI deve tratar isso como "valores atuais do
arquivo, ainda sem customização" — não como um formulário vazio nem como falha.

### 6.3 `merchant` aparece em dois lugares com significados diferentes
Se o mesmo moderador também usa o painel de `NpcAdminService` (posição/loja), cuidado pra não
confundir os dois campos "merchant": o daqui é o valor cru do `STRUCT_MOB.Merchant` do arquivo
(qualquer template); o de lá é a classificação de loja de um NPC **gerido pelo banco**
(`npc_definition.merchant`). Editar um não muda o outro. Vale rotular os dois com nomes diferentes
na UI (ex.: "Tipo de merchant (template)" vs. "Tipo de merchant (definição)") pra evitar confusão.

## 7. Exemplo (pseudocódigo BFF, server-side)

```ts
// Server Action — moderador abre o editor de um template
'use server'
import { getSession } from '@/lib/session'
import { mobTemplateAdminClient } from '@/lib/grpc'

export async function getMobTemplateStat(templateName: string) {
  const { accountId, role } = await getSession()
  if (role !== 'moderator' && role !== 'admin') throw new Error('forbidden')

  const res = await mobTemplateAdminClient.getMobTemplateStat({
    moderatorId: BigInt(accountId),
    templateName,
  })
  if (res.result !== AdminResult.ADMIN_RESULT_OK) throw mapAdminError(res.result)
  return res.stat // stat.overridden === false => valores lidos direto do arquivo
}

// Server Action — salvar o formulário completo
export async function upsertMobTemplateStat(stat: AdminMobTemplateStatInput) {
  const { accountId, role } = await getSession()
  if (role !== 'moderator' && role !== 'admin') throw new Error('forbidden')

  const res = await mobTemplateAdminClient.upsertMobTemplateStat({
    moderatorId: BigInt(accountId),
    stat,
  })
  if (res.result !== AdminResult.ADMIN_RESULT_OK) throw mapAdminError(res.result)
}

// Server Action — salvar a grade de equipamento (sempre os 16 slots atuais)
export async function setMobTemplateEquip(templateName: string, items: AdminMobTemplateEquipItemInput[]) {
  const { accountId, role } = await getSession()
  if (role !== 'moderator' && role !== 'admin') throw new Error('forbidden')

  const res = await mobTemplateAdminClient.setMobTemplateEquip({
    moderatorId: BigInt(accountId),
    templateName,
    items, // lista COMPLETA — slots omitidos ficam vazios
  })
  if (res.result !== AdminResult.ADMIN_RESULT_OK) throw mapAdminError(res.result)
}
```

## 8. Fora de escopo (não existe endpoint ainda)

- **UI pronta** — este doc é só o contrato; a tela em si (Next.js) ainda não foi construída neste
  repositório (mesma decisão de escopo do `npc-editing-plan.md`).
- **Hot-reload** — uma edição só vale depois do próximo restart do tmServer com
  `-mob-stat-editing`. Não peça (nem prometa) "aplicar agora".
- **Editar `Carry[]` (loja)** — use `NpcAdminService.SetNpcShop`.
- **Editar posição de spawn do subconjunto gerido pelo banco** — use
  `NpcAdminService.UpsertNpc` (`pos_x`/`pos_y`).
- **Renomear ou apagar o arquivo de template** — `DeleteMobTemplateStat` só remove o override; o
  arquivo em `Release/` nunca é tocado.
- **`SpecialBonus`, `SkillBonus`, `Critical`, `GuildLevel`, `Magic`** — o `EDITAPPMOB` legado
  também não editava esses campos, então não há RPC pra eles.
