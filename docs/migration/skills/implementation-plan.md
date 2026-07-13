# Plano de implementacao faseada de skills

## Visao geral

O servidor Go ja possui catalogo `SkillData.csv`, validacao basica de cast, formulas puras de dano, learned-mask, gasto de mana, parte dos buffs, transformacoes BM, summons BM, hotbar e persistencia de learned/skillbar/affects. A proxima etapa deve fechar lacunas pequenas sem reabrir a regressao B12: `Dam=-2` e melee, `Dam=-1` e skill, `Dam=0` e slot vazio, e o servidor nunca deve confiar em dano calculado pelo cliente.

## Fase 1 - Core comum

Objetivo: consolidar regras compartilhadas antes de mexer por classe.

Mudancas esperadas:
- Portar `GetParryRate` local de `Source/Code/TMSrv/GetFunc.cpp:686-731` e trocar o placeholder de parry zero.
- Portar `BASE_GetDoubleCritical` de `Source/Code/Basedef.cpp:6122-6191`, mantendo ordem de RNG e deixando de confiar no byte do cliente.
- Modelar `ReqHp`/`ReqMp` como alvos server-side: inicializar em login, descontar cast de `CurrentScore.Mp` e `ReqMp`, aplicar clamp `Req>=atual`, e enviar `MSG_SetHpMp` (`0x0181`) quando faltar MP.
- Validar `TargetType`, `Range`, `MaxTarget`, party e agressividade sem rejeitar melee por campos nao verificados.
- Preservar cadencia server-side de 800 ms por `ClientTick`; nao usar `Delay` do CSV como cooldown de rejeicao no servidor.
- Implementar `CurrentWeather` se a captura confirmar ciclo/valores; ate la manter weather 0 documentado.

Arquivos provaveis: `tmserver/internal/combat`, `tmserver/internal/handler/combat.go`, `tmserver/internal/protocol`, testes de handler/combat.

Riscos: ordem de RNG, parry em mil, `SkillIndex` melee ainda nao capturado e anti-speed por `ClientTick`.

Testes obrigatorios: melee com catalogo ligado, skill learned/unlearned, mana insuficiente, parry deterministico com MSVC RNG, double-critical server-authoritative.

Dependencias Windows: nenhuma bloqueante para server-side; captura ao vivo segue opcional para documentar bytes reais do cliente.

## Fase 2 - Transknight

Objetivo: fechar TK `0..23`, priorizando danos e buffs que ja tem base Go.

Mudancas esperadas:
- Implementar Toque Sagrado/affect type 1 se confirmado no Buff Loop completo.
- Portar Furia Divina (`skillnum==6`) com deslocamento/`MSG_Action`, imunidades de merchant/clan/geradores e `SetBattle`.
- Completar Exterminar (`skillnum==22`): consumo total de MP, dano com MP/Int, reposicionamento e pacote visual.
- Revisar passivas Mestre das Armas e Nocao de Combate; hoje apenas parte dos bonus de weapon/AC esta modelada.
- Fechar side-effects de affects 7/36 e ticks de veneno sem quebrar dano principal ja implementado.

Arquivos provaveis: `handler/combat.go`, `handler/score_derive.go`, `handler/affect_score.go`, `world/affect.go`.

Riscos: skill 22 altera MP/ReqMp e posicao do alvo; skills com side affect nao podem virar hard-gate de dano.

Testes obrigatorios: uma skill de cada arvore, skill 6 movimento, skill 22 MP zero + dano, passivas de dano/AC, poison tick.

Dependencias Windows: captura visual dos pacotes de movimento de skill se `MSG_Action` nao for suficiente localmente.

## Fase 3 - Foema

Status: concluida em `progress.md`.

Objetivo: fechar cura, magias elementais, buffs e utilitarios FM.

Mudancas esperadas:
- Completar heal: formula especial da skill 27, caps 1100/2200 por tier, restricoes de clan 4, EXP por cura e `SetReqHp`.
- Portar Flash (`InstanceType 7`) e Desintoxicar/Renascimento (`InstanceType 8`), incluindo limpeza seletiva de affects.
- Portar Julgamento Divino (`skillnum==30`) com dano somando HP e auto-reducao de HP do caster.
- Implementar Trovão/tick 22 ou marcar comportamento como bloqueado por captura.
- Completar Teleporte/Velocidade/Cancelamento conforme branches locais `skillnum==41/44/47` e `InstanceType 9`.

Arquivos provaveis: `handler/combat.go`, `handler/affect_tick.go`, `handler/affect_score.go`, `protocol`.

Riscos: nomes do CSV podem nao bater com comentarios legados; usar indice numerico como autoridade.

Testes obrigatorios: heal self/outro, heal em clan 4 negado, Flash limpa aggro, Renascimento revive, Cancelamento remove block, buff FM bit 19 triplica affect 9.

Dependencias Windows: nenhuma bloqueante; usar `AffectTime/4`, tick de 8 s, `Time=(AffectTime+1)*Delay/100`, `MSG_UpdateScore` grid e `MSG_SendAffect` self.

## Fase 4 - BeastMaster

Status: concluida em `progress.md`.

Objetivo: estabilizar BM `48..71`.

Mudancas esperadas:
- Portar Chamas Etereas (`InstanceType 12`) com mana burn e dispel de affects 14/16/18/19.
- Implementar Aura Bestial/tick 23 se a fonte local ou Windows confirmar efeito real.
- Fechar summons: limite/mensagem de party full, `SendAddParty` se o cliente depender, ordem de celulas e desaparecimento.
- Completar transformacoes: critical, attack/run speed, glow/sanc visual, e comparar `pTransBonus` local com fonte completa.

Arquivos provaveis: `handler/combat.go`, `handler/summon.go`, `handler/transform.go`, `handler/affect_score.go`.

Riscos: summons ocupam slots compartilhados `[MaxUser, MaxMob)` e nao podem bloquear loop; transform nao deve persistir score derivado.

Testes obrigatorios: summon count por Special[2], summon expira/desloga, transform self/in-view/expiry, Chamas Etereas contra player.

Dependencias Windows: captura visual de transform/summon se pacote local nao reproduzir cliente.

## Fase 5 - Huntress

Status: concluida em `progress.md`.

Objetivo: fechar HT `72..95`, que hoje e a classe com maior numero de gaps.

Mudancas esperadas:
- Portar Ilusao/Action3 e suas regras de MP/learned bit.
- Implementar affects HT ausentes: 27, 29, 30, 31, 36, 37, 38 e utilitarios ligados a Soul/extra.
- Manter os nomes do CSV: `85 = Explosao Eterea` e `86 = Escudo Dourado`. A skill 85 aplica o affect 31 "Escudo Dourado" e custa `100*Level` de gold; a 86 nao tem custo especial de gold.
- Completar Tempestade de Raios (`skillnum==79`) com branch legado que usa `BASE_GetDamage` e divide o resultado.
- Fechar passivas de dano/AC/weapon, stealth e protecao.

Arquivos provaveis: `handler/combat.go`, `handler/movement.go`, `handler/score_derive.go`, `handler/affect_score.go`.

Riscos: muitos efeitos dependem de `STRUCT_MOBEXTRA`/Soul e de visual/retarget no cliente.

Testes obrigatorios: skill 79, Ilusao, Imunidade/Evasao, invisibilidade, skill 85 com ouro, skill 86 sem ouro, passivas HT.

Dependencias Windows: nenhuma bloqueante; captura real de 85/86 so documenta UI e valores antes/depois.

## Fase 6 - Sephira/shared

Status: concluida em `progress.md`; `WIN-SKILL-007/008` provaram que `SecLearnedSkill` e campo morto/reservado, `200..247` usam `LearnedSkill & (1 << (skillnum % 24))` e effects `40+` sao icon-only/no-op.

Objetivo: mapear e implementar `96+` sem misturar regras de base class.

Mudancas esperadas:
- Fechar livros Sephira: bits `Vol-7`, learned mask, UpdateEtc e feedback visual.
- Implementar skill 97 com pre-condicao do item 746 no grid e skill 98 criando Vinha.
- Implementar Resurrection 99 de verdade; hoje o indice so bypassa a checagem de morto.
- Triar skills 200..247: muitas sao passivas/affects sem consumidor server-side; implementar paridade como no-op/icon-only quando a fonte Windows provar ausencia de efeito.
- Adicionar suporte a summon value 9 ou documentar ausencia do template.

Arquivos provaveis: `handler/combat.go`, `handler/item.go`, `handler/affect_score.go`, `world`, `protocol`.

Riscos: learned bits `1<<(skillnum-72)` compartilham o campo `LearnedSkill`; nao colidir com bits 0..23 das classes.

Testes obrigatorios: livro Sephira, cast 96/97/99/216/226, learned bit errado, skill 97 sem item 746, Muro de Espinhos.

Dependencias Windows: resolvidas por `WIN-SKILL-007/008`; captura real segue opcional para documentar UI/bytes.

## Fase 7 - Fechamento de paridade

Objetivo: transformar lacunas em golden cases e regressao permanente.

Mudancas esperadas:
- Incorporar respostas Windows em docs e testes.
- Criar golden cases por classe com estado fixo, RNG MSVC e pacotes esperados.
- Validar manualmente com cliente real e registrar logs/capturas em `docs/migration/captura-wyd-skills-*.md`.
- Atualizar `docs/migration/prompts/PROGRESS.md` com status e pendencias.

Riscos: divergencia pequena em score/MP/visual pode parecer bug de skill; sempre comparar estado servidor e pacote S->C.

Testes obrigatorios: suite Go completa focada em `tmserver/internal/combat`, `handler`, `protocol`, `world`; validacao manual no cliente.
