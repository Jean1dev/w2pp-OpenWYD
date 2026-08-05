# Progresso da frente de skills

Atualizado em 2026-07-11.

Este arquivo rastreia a execucao de `docs/migration/skills/implementation-plan.md`.
O status aqui prevalece sobre as tabelas por classe enquanto elas nao forem recontadas apos cada
fase de codigo.

## Status por fase

| Fase | Status | Evidencia | Pendencias |
|------|--------|-----------|------------|
| 1 - Core comum | CONCLUIDA | `combat/critical.go`, `handler/combat.go`, `handler/hpmp.go`, `protocol/score.go`, testes de combat/protocol/handler | Captura viva do cliente segue nao bloqueante para bytes/visual; `CurrentWeather` fica 0 porque nenhum ciclo/valor foi capturado. |
| 2 - Transknight | CONCLUIDA | Matriz TK `0..23` recontada: dano, buffs/debuffs, passivas, ticks e especiais portados/testados | Captura visual do cliente para os pacotes de movimento segue nao bloqueante. |
| 3 - Foema | CONCLUIDA | Matriz FM `24..47` recontada: cura, dano, ticks, buffs e utilitarios portados/testados | Captura visual do cliente e PvP automatico de Trovão seguem nao bloqueantes ate existir PK-mode/arena no modelo Go. |
| 4 - BeastMaster | CONCLUIDA | Matriz BM `48..71` recontada: Chamas, Aura, summons, party UI e transformacoes portados/testados | Glow `EF_SANC` da transformacao segue visual/deferido; PvP automatico da Aura depende de PK-mode/arena. |
| 5 - Huntress | CONCLUIDA | Matriz HT `72..95` recontada: Ilusao, dano, buffs, passivas, invisibilidade, combine e Extracao portados/testados; `MobExtra.Soul` agora e importado/persistido | Marcador visual secundario de Lamina Aerea e receitas exatas de Alquimia seguem documentados como nao bloqueantes. |
| 6 - Sephira/shared | CONCLUIDA | Matriz `96+` recontada: gate vivo `LearnedSkill % 24`, skills 96/97/98/99/102, effects 5/6/12/39/42, summon value 9, `SecLearnedSkill` morto/reservado e effects 40+ icon-only/no-op | Captura ao vivo segue opcional para UI/bytes; troca Celestial/Sub-Celestial por `SaveCelestial[]` fica para fase propria. |
| 7 - Fechamento | EM ANDAMENTO | Validacao automatizada da Fase 7 concluida: gaps do `validation-plan.md` fechados por teste e e2e smoke verde (ver secao Fase 7) | Golden cases completos por classe e validacao manual com cliente real seguem pendentes. |

## Fase 1 - Core comum

Status: CONCLUIDA.

Itens fechados:
- `GetParryRate` portado como `combat.ParryRate` e usado no dano fisico e em skill hit positivo.
- `BASE_GetDoubleCritical` portado como `combat.DoubleCritical`; o byte do cliente nao e autoridade.
- `ReqHp`/`ReqMp` modelados em `world.Session`, inicializados no login/restart e usados em cast/heal.
- `MSG_SetHpMp` (`0x0181`, 28 bytes) codificado e enviado quando falta MP ou quando skills especiais
  alteram HP/MP fora do corpo principal de `_MSG_Attack`.
- Cadencia server-side de 800 ms preservada por `ClientTick`; `Delay` do CSV nao e cooldown de reject.
- Validacao comum de skill limitada ao caminho `Dam=-1`: `MaxTarget`, `bParty`, range por
  `SkillData.Range` e self-only seguro para `TargetType=0`. Melee continua tolerante a encodings
  nao capturados, preservando a regressao B12.
- `Aggressive` segue aplicado na aterrissagem de affect/tick contra aliados/guild/leader; a fonte
  local nao usa `TargetType` para outras decisoes server-side em `_MSG_Attack`.
- `CurrentWeather` deixou de ser 0 fixo na issue #116: o roll do minuto e o broadcast
  `_MSG_UpdateWeather` estao em `worldevents/weather.go` + `handler/weather.go`, e
  `handler/combat.go` alimenta `combat.SkillBaseDamage` com o valor vivo. O que cada valor 0/1/2
  renderiza no cliente segue UNVERIFIED (nao ha captura).

Testes de fechamento:
- `go test ./tmserver/internal/combat ./tmserver/internal/protocol ./tmserver/internal/world ./tmserver/internal/handler`
- `make test`
- `make build`
- `make vet`

## Fase 2 - Transknight

Status: CONCLUIDA.

Itens fechados:
- `Furia Divina` (`skillnum==6`) aplica deslocamento via `MSG_Action`, respeita imunidades de merchant,
  clan/geradores e aciona batalha de grupo para mobs.
- `Exterminar` (`skillnum==22`) consome todo MP/`ReqMp`, soma MP anterior + INT viva ao dano e aplica o
  reposicionamento visual.
- `Toque Sagrado`/affect 1 aplica slow de movimento, attack-speed e penalidade de INT para robe.
- `Lamina Congelada`/affect 7 aplica reducao de attack-speed e penalidade de INT para robe.
- `Assalto`/affect 13 aplica +15% de dano, `DAMAGEMULTI += Level/10+Value` e -10% de MaxHP.
- `Armadura Critica`/affect 36 ativa `RSV_DRAIN`.
- `Mestre das Armas` (`bit 9`) faz a mao secundaria contar 100% no `WeaponDamage`.
- `Nocao de Combate` (`bit 14`) injeta mastery de mitigacao `Special[2]/20`, clamp `0..15`, no cast.
- `TickType 20` de veneno reduz HP em 1000 por tick, respeita floor 1 e move `ReqHp` junto com o delta.
- Tabela `transknight.md` recontada para 24 `IMPLEMENTED`, 0 `PARTIAL`, 0 `MISSING`.

Testes de fechamento:
- `go test ./tmserver/internal/handler ./tmserver/internal/world ./tmserver/internal/combat ./tmserver/internal/protocol`

## Fase 3 - Foema

Status: CONCLUIDA.

Itens fechados:
- Heal Foema (`InstanceType 6`) aplica formula especial da skill 27, caps 1100/2200 por tier, bloqueio de clan 4, reducao por fada nos slots legados e EXP por cura fora de vila.
- `Flash` (`InstanceType 7`) limpa estado de combate do alvo.
- `Desintoxicar`/`Renascimento` (`InstanceType 8`) removem debuffs legados; Renascimento zera MP do caster e revive com chance/fator de HP da fonte.
- `Julgamento Divino` soma HP atual do caster ao dano e reduz o HP do caster para `HP/6+1`.
- `Trovão`/tick 22 dispara um ataque sintetico de `Relâmpago` contra mobs proximos, com custo de mana e limite de alvos derivados da fonte.
- `Velocidade` (`InstanceType 9`) move player alvo vivo para a coordenada de destino livre.
- Buffs 41/43/44/45 aplicam os affects 2/11/9/15; `Arma Mágica` preserva o bonus Foema bit 19 e o limite multi-alvo `Special/25+2`.
- `Controle de Mana`/affect 18 drena MP e reduz dano recebido pelo divisor legado.
- `Cancelamento` remove type 19 antes do affect generico.
- Tabela `foema.md` recontada para 24 `IMPLEMENTED`, 0 `PARTIAL`, 0 `MISSING`.

Testes de fechamento:
- `go test ./tmserver/internal/handler`
- `go test ./tmserver/internal/handler ./tmserver/internal/world ./tmserver/internal/combat ./tmserver/internal/protocol`
- `make test`
- `make build`
- `make vet`

## Fase 4 - BeastMaster

Status: CONCLUIDA.

Itens fechados:
- `Chamas Etereas` (`InstanceType 12`) queima MP do alvo player ou remove affects 14/16/18/19.
- `Aura Bestial`/tick 23 dispara ataque sintetico de `Furia de Gaia` (skill 52), com custo de mana e limite de alvos da fonte.
- Summons 56..63 respeitam `Special[2]` para quantidade, `pSummonBonus`, `PartyList`, vida Type 24, follow/assist/cleanup e `MSG_CNFAddParty`.
- Recast de summon conta pets existentes e nao duplica acima do limite calculado.
- `freeCellNear` segue a expansao 1..4 de `GetEmptyMobGrid`.
- Transformacoes 64/66/68/70/71 aplicam mesh, dano/AC/HP, resists, critical e attack/run speed por `pTransBonus`.
- Passivas 65/67/69 alteram apenas os bonuses gated de Lobo/Urso/Astaroth.
- Tabela `beastmaster.md` recontada para 24 `IMPLEMENTED`, 0 `PARTIAL`, 0 `MISSING`.

Testes de fechamento:
- `go test ./tmserver/internal/protocol ./tmserver/internal/handler`
- `go test ./tmserver/internal/handler`
- `go test ./tmserver/internal/handler ./tmserver/internal/world ./tmserver/internal/combat ./tmserver/internal/protocol`
- `make test`
- `make build`
- `make vet`

## Fase 5 - Huntress

Status: CONCLUIDA.

Itens fechados:
- `Ilusao` (`MSG_Action3`) consome MP, exige classe/learned bit e respeita cadencia de 900 ms.
- `Tempestade de Raios` (`skillnum==79`) usa o branch legado com `Damage*180/100`, `BASE_GetDamage` e divisao por 2.
- `Explosao Eterea` (`skillnum==85`) cobra gold `100*Level` e aplica affect 31; `Escudo Dourado` (`86`) nao cobra gold especial.
- Affects HT 21/27/29/30/31/36/37/38 alteram score, RSV ou dano conforme Buff Loop.
- Frost/Drain por arma (`EF_WTYPE` 101/41) aplicam os efeitos on-hit legados via skills 36/40.
- Passivas HT portadas: bonus por arma, off-hand 100%, +200 damage, critical, proc de Lamina Aerea e AC de Protecao das Sombras.
- Invisibilidade (`A28`) aplica `RSV_HIDE` e ja e respeitada por aggro/ticks.
- `_MSG_CombineItemAlquimia` passa a exigir HT; `_MSG_CombineItemExtracao` consome catalisador 1774 e transforma item refinado conforme fonte local.
- Tabela `huntress.md` recontada para 24 `IMPLEMENTED`, 0 `PARTIAL`, 0 `MISSING`.

Testes de fechamento:
- `go test ./tmserver/internal/handler ./tmserver/internal/content ./tmserver/internal/combat ./tmserver/internal/world`
- `make test`
- `make build`
- `make vet`

## Fase 6 - Sephira/shared

Status: CONCLUIDA.

Itens fechados:
- Livros Sephira `EF_VOLATILE 31..38` setam bits `24..31`, atualizam `UpdateEtc` e agora consomem apenas uma unidade quando o item esta stackado.
- Validação de cast shared/extra-class usa a regra viva do legado: `LearnedSkill & (1 << (skillnum % 24))` para `0..247`; `SecLearnedSkill` nao participa.
- `Poder Superior`/A39 aplica +100 `ExpBonus`; `Limite da Alma`/A29 aplica Soul no score quando `Entity.Soul` esta populado.
- `Canhão Guardião` exige item 746 no grid; `Muro de Espinhos` cria Vinha; `Ressureição` revive o personagem morto.
- Buff Loop recebeu os types 5, 6 e 12 alem dos types 39/42 ja usados por Sephira.
- `Invocação Final`/InstanceValue 9 foi triada para template de summon id 8 com `pSummonBonus` zero.
- `STRUCT_MOBEXTRA.SecLearnedSkill` e `Soul` agora sao decodificados por offset MSVC x86, importados para o modelo relacional, trafegam no contrato gRPC interno e sao preservados no save do tmServer; `SecLearnedSkill` fica reservado/morto por paridade.
- Affects/ticks `40/41/43/44/45/46/47/48` foram provados como icon-only/no-op no servidor original e cobertos por teste de score.
- `Concentração`/Accuracy fica no-op de gameplay porque o campo legado nao tem consumidor server-side neste build.
- Tabela `sephira-shared.md` recontada para 55 `IMPLEMENTED`, 0 `PARTIAL`, 0 `MISSING`.

Pendencias nao bloqueantes:
- Captura ao vivo de cliente para documentar UI/bytes reais.
- Implementar promocao/troca Celestial/Sub-Celestial em fase propria, usando `SaveCelestial[]` para trocar o conjunto ativo.

Testes de fechamento:
- `go test ./tmserver/internal/handler ./tmserver/internal/content ./tmserver/internal/combat ./tmserver/internal/world`
- `go test ./internal/savefmt ./dbserver/internal/convert ./internal/store ./dbserver/internal/grpcsrv ./tmserver/internal/dbclient ./tmserver/internal/world ./tmserver/internal/handler`
- `make test`
- `make build`
- `make vet`
- `git diff --check`

## Fase 7 - Fechamento de paridade

Status: EM ANDAMENTO (parte automatizada concluida em 2026-07-12).

A rodada de validacao automatizada do `validation-plan.md` foi executada e os bullets que ainda nao
tinham teste dedicado viraram regressao permanente. Itens fechados:

- `combat/skill_test.go`: `SkillResistScale` cobre types 1..5 (incluindo 3/4) e resist elevado por
  affect; `ManaSpent` cobre custo zero por base mana 0.
- `handler/skill_test.go`: cast de skill passiva e rejeitado sem broadcast (`validateCast` Passive).
- `handler/combat_special_test.go`: gates de alvo por-entrada de `attack()` - merchant imune a dano,
  alvo morto presente rejeita skill de dano, alvo inexistente zera o dano; e
  `applyCastAffect` agressivo pula same-leader/guild, respeita `RSV_BLOCK` e clan 6.
- `handler/affect_tick_test.go` (novo): `sweepAffects`/`processAffect` direto - HoT type 17 e DoT
  type 20 movem HP e emitem `MSG_SetHpDam`; expiracao zera o slot e refaz o snapshot `MSG_SendAffect`.
- `content/content_test.go`: `parseSkillData` divide `AffectTime` por 4 sem depender do CSV do Release.
- `world/affect_test.go`: `SetTick` clampa types 1/3/10 a 2 ticks e mantem HoT (17) com timer cheio.
- `handler/character_relogin_test.go`: `SkillBar`/`ShortSkill`/`BaseSpecial` sobrevivem save->reload e
  buffs derivados vivos (type 14 +CON, type 15 +Special) nao contaminam o score base persistido.
- E2E smoke de login (`-tags=e2e TestE2ESmokeLogin`) verde contra o stack compose: cadeia
  tmServer(CPSock) -> dbServer(gRPC/mTLS) -> PostgreSQL confirmada com `CNFAccountLogin (0x10a)`.

Suite de fechamento: `make test`, `make build`, `make vet` limpos; pacotes tocados verdes com `-race`.

Pendencias (nao automatizaveis nesta rodada):
- Golden cases completos por classe (estado fixo + RNG MSVC + pacotes esperados).
- Validacao manual com cliente real (7 cenarios do `validation-plan.md`); registrar logs/capturas em
  `docs/migration/captura-wyd-skills-*.md` quando houver cliente disponivel.
