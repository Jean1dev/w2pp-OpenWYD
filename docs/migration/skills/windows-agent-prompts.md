# Prompts copiaveis para o agente Windows - skills

Status: os prompts `WIN-SKILL-001..008` ja foram respondidos. As conclusoes estao em
`windows-agent-findings.md`; mantenha este arquivo como historico/reexecucao futura.

Use estes prompts no agente Windows, preferencialmente um por vez. Cada resposta deve ser salva no arquivo indicado e trazida de volta para este repositorio Linux.

Regras para todos os prompts:
- Nao chute. Se uma informacao nao aparecer na fonte completa, dumper MSVC x86 ou captura real, escreva `NAO_ENCONTRADO` e explique onde procurou.
- Cite `arquivo:linha` sempre que a resposta vier da fonte.
- Para layouts, use MSVC x86/dumper, nao inferencia por alinhamento manual.
- Para captura de cliente, registre cliente/build, acao feita, pacote C->S/S->C, campos decodificados e resultado visual.
- Preserve valores decimais e hex quando possivel.

## Prompt mestre opcional

```text
Contexto: estamos migrando o servidor WYD para Go no projeto w2pp-OpenWYD. O alvo e o cliente original WYD.exe build/patch 7662, que envia ClientVersion 12000. O header CPSock tem 12 bytes. Flags relevantes: FLAG_GAME2CLIENT=0x0100 e FLAG_CLIENT2GAME=0x0200.

No Linux ja auditamos a frente de skills em docs/migration/skills/. A fonte local parcial ja tem alguns trechos: _MSG_Attack.cpp, Basedef.cpp, Server.cpp, GetFunc.cpp. Use a fonte Windows completa, o servidor original compilavel, o dumper MSVC x86 em _layout_probe/dump_layout.cpp e capturas reais do cliente quando solicitado.

Responda somente com evidencia. Se nao conseguir provar algo por fonte, layout compilado ou captura, marque NAO_ENCONTRADO. Salve a resposta no arquivo pedido pelo prompt.
```

## WIN-SKILL-001 - Encoding real de melee/skill no cliente 12000

Copie este prompt:

```text
Contexto: migracao WYD -> Go. Precisamos fechar o encoding real de _MSG_Attack enviado pelo cliente WYD.exe 12000/7662. No Go, por seguranca, o caminho de skill so e ativado quando algum Dam[i].Damage == -1. Uma regressao anterior bloqueou melee ao confiar em SkillIndex/Dam sem captura real.

Tarefa: capture e decodifique pacotes C->S _MSG_Attack do cliente real.

Faca exatamente estes cenarios:
1. Personagem sem usar skill, ataque melee comum contra um mob valido.
2. Personagem usando uma skill de dano simples contra um mob valido.
3. Se possivel, ataque melee contra player e skill contra player, marcando se nao der.

Para cada pacote, informe:
- Cliente/build e servidor original usado.
- Type do pacote em decimal e hex.
- HEADER: Size, Type, ID, ClientTick.
- Body offsets confirmados para: CurrentHp, CurrentMp, CurrentExp, SkillIndex, ReqMp, Motion, PosX/Y, TargetX/Y, DoubleCritical.
- Para todos os Dam[i] nao vazios: indice i, TargetID, Damage em decimal e hex.
- Se Dam[i].Damage usa -2, -1, 0 ou outro valor no melee real.
- Se SkillIndex no melee real vem -1, 0, skill anterior ou outro valor.
- Se o cliente envia dano calculado localmente em Dam[i].Damage.

Arquivos/funcoes para comparar:
- _MSG_Attack.cpp no servidor original.
- Definicao de MSG_Attack em Basedef.h ou equivalente.

Nao corrija codigo. Nao inferir. Se nao conseguir capturar algum cenario, escreva NAO_ENCONTRADO e explique o motivo.

Salve em: captura-wyd-skills-attack-encoding.md

Formato da resposta:
WIN-SKILL-001
Resumo:
Cenario melee mob:
Cenario skill mob:
Cenario melee player:
Cenario skill player:
Conclusao objetiva para o Go:
Evidencias arquivo:linha / captura:
```

## WIN-SKILL-002 - ReqMp/ReqHp, SetReqMp e SetReqHp

Copie este prompt:

```text
Contexto: migracao WYD -> Go. O Go hoje desconta MP no cast e ecoa ReqMp baseado no pacote recebido, mas o legado usa pUser[conn].ReqMp, pUser[conn].ReqHp, SetReqMp(conn) e SetReqHp(conn). Precisamos da regra exata.

Tarefa: na fonte Windows completa, localizar e copiar a logica real de ReqHp/ReqMp.

Procure:
- Declaracao de pUser[conn].ReqHp e pUser[conn].ReqMp.
- Inicializacao em login, CharacterLogin, respawn/restart, troca de personagem e qualquer reset.
- Codigo completo de SetReqHp.
- Codigo completo de SetReqMp.
- Chamadas de SetReqHp/SetReqMp dentro de _MSG_Attack, heal, cast sem MP, resurrection e regen.
- Qual pacote S->C e enviado por SetReqHp/SetReqMp ou SendSetHpMp, com Type e layout.

Depois capture no cliente real:
1. Cast com MP suficiente: CurrentMp e ReqMp antes/depois no pacote _MSG_Attack ou pacote S->C relacionado.
2. Cast sem MP suficiente: pacote enviado ao cliente e valores de MP/ReqMp.
3. Heal em alvo player, se facil: ReqHp antes/depois.

Obrigatorio:
- Cite arquivo:linha para toda fonte.
- Se o codigo estiver em macros/includes, inclua tambem o caminho do include.
- Se uma funcao nao existir com esse nome na fonte completa, escreva NAO_ENCONTRADO e liste termos pesquisados.

Salve em: captura-wyd-skills-reqhp-reqmp.md

Formato da resposta:
WIN-SKILL-002
Campos pUser:
Inicializacao ReqHp:
Inicializacao ReqMp:
SetReqHp codigo/resumo:
SetReqMp codigo/resumo:
SendSetHpMp/layout:
Captura cast ok:
Captura cast sem MP:
Captura heal:
Conclusao objetiva para o Go:
```

## WIN-SKILL-003 - Delay real de skill e anti-speed

Copie este prompt:

```text
Contexto: migracao WYD -> Go. Temos SkillData.csv com Delay e sabemos que o ClientPatch divide SkillDelay por 4 no cliente. O servidor legado tambem aplica checks em _MSG_Attack, incluindo limite de 800 ms. Precisamos saber o comportamento real do cliente 12000/7662 e a regra exata da fonte completa.

Tarefa A - fonte:
- Localize em _MSG_Attack.cpp e funcoes auxiliares toda validacao de delay/cadencia de skill.
- Informe como g_pSpell[skillnum].Delay vira tempo real.
- Informe se o servidor usa Delay do CSV, Delay/4, Delay-1, minimo 700ms, LastAttackTick, LastAttack ou outro campo.
- Cite arquivo:linha.

Tarefa B - captura:
Capture tentativas de cast consecutivo com:
1. Mesma skill de buff curto.
2. Mesma skill de dano.
3. Duas skills diferentes em sequencia.

Para cada tentativa, registre:
- SkillIndex, nome, Delay no CSV.
- Horario local ou contador, ClientTick enviado, intervalo desde pacote anterior.
- Se o cliente enviou ou nao pacote enquanto o botao parecia em cooldown.
- Se o servidor aceitou, rejeitou ou gerou crack/log.
- Resultado visual no cliente.

Nao inferir intervalos por sensacao; use logs/timestamps quando possivel.

Salve em: captura-wyd-skills-delay.md

Formato da resposta:
WIN-SKILL-003
Fonte - regra de delay:
Captura skill buff:
Captura skill dano:
Captura skills diferentes:
Conclusao objetiva para o Go:
Pontos ainda NAO_ENCONTRADO:
```

## WIN-SKILL-004 - Buff curto, timer real e pacotes visuais

Copie este prompt:

```text
Contexto: migracao WYD -> Go. O Go aplica SetAffect/SetTick com Time em ticks de 8s e envia MSG_UpdateScore e MSG_SendAffect. Precisamos confirmar com cliente real como icone/timer/expiracao aparecem.

Tarefa: capture um buff curto no cliente real e compare com a fonte completa.

Skills sugeridas, escolha a mais facil no servidor original:
- Foema 41, 43 ou 44.
- Alternativa: qualquer buff com AffectTime baixo/visivel.

Fonte:
- Localize SetAffect, SetTick, GetAffect, SendScore, SendAffect/SendEtc se existir.
- Confirme formula de Time: (AffectTime+1)*Delay/100, unidade real e expiracao.
- Confirme se MSG_UpdateScore carrega Affect[32] e se MSG_SendAffect tambem e enviado.
- Cite arquivo:linha.

Captura:
1. Castar buff em si.
2. Se possivel, castar buff em outro player em visao.
3. Cronometrar tempo real ate o icone sumir.
4. Capturar pacotes S->C logo apos cast e na expiracao.

Para cada pacote relevante, informe:
- Type decimal/hex.
- HEADER.ID.
- Offset/valor do Affect slot: Type, Value, Level, Time.
- Para MSG_UpdateScore, valor bruto de Affect[i] u16 e interpretacao `(Type<<8)|(Time&0xFF)`.
- Se outro player recebe pacote visual adicional.

Salve em: captura-wyd-skills-buff-timer.md

Formato da resposta:
WIN-SKILL-004
Skill usada:
Fonte - SetAffect/SetTick/GetAffect:
Pacotes apos cast:
Pacotes para outro player:
Tempo real ate expiracao:
Pacotes na expiracao:
Conclusao objetiva para o Go:
```

## WIN-SKILL-005 - Huntress 85/86 e Escudo Dourado

Copie este prompt:

```text
Contexto: migracao WYD -> Go. No SkillData.csv local, indice 85 aparece como Explosão_Etérea e 86 como Escudo_Dourado. Na fonte local parcial, _MSG_Attack.cpp comenta o bloco skillnum == 85 como Escudo_dourado e cobra 100*Level de gold. Precisamos resolver por fonte completa e cliente real, sem renomear por chute.

Tarefa A - fonte completa:
- Localize todos os tratamentos especiais para skillnum == 85 e skillnum == 86.
- Localize strings/nomes mostrados no cliente ou no SkillData usado pelo servidor original Windows.
- Verifique se alguma tabela desloca indices de Huntress.
- Cite arquivo:linha.

Tarefa B - captura cliente:
Com uma Huntress que tenha as duas skills disponiveis/aprendidas:
1. Acione o botao/nome que o cliente mostra como Explosão_Etérea.
2. Acione o botao/nome que o cliente mostra como Escudo_Dourado.

Para cada acao, informe:
- Nome exibido no cliente.
- SkillIndex enviado.
- Mana/MP antes/depois.
- Gold antes/depois.
- Dam[i].Damage, se houver.
- Pacotes S->C relevantes e resultado visual.

Se nao conseguir personagem com ambas, extraia da UI/dados do cliente e marque captura de cast como NAO_ENCONTRADO.

Salve em: captura-wyd-skills-ht-85-86.md

Formato da resposta:
WIN-SKILL-005
Fonte skill 85:
Fonte skill 86:
Cliente skill 85:
Cliente skill 86:
Gold/MP/Dano:
Conclusao objetiva para o Go:
```

## WIN-SKILL-006 - Layouts MSVC x86 sensiveis a skills

Copie este prompt:

```text
Contexto: migracao WYD -> Go. Precisamos confirmar byte-exato por MSVC x86 os layouts usados pela frente de skills. Use o dumper _layout_probe/dump_layout.cpp ou adapte-o. Nao inferir por leitura manual.

Tarefa: gerar sizeof e offsetof de cada campo listado.

Structs obrigatorias:
1. MSG_Attack
   - sizeof total.
   - offset absoluto e offset de body para: CurrentHp, CurrentMp, CurrentExp, SkillIndex, ReqMp, Motion, DoubleCritical, PosX, PosY, TargetX, TargetY, Dam[0].TargetID, Dam[0].Damage, stride de Dam[i].
2. MSG_UpdateScore
   - sizeof total.
   - offset absoluto/body para STRUCT_SCORE, Critical, SaveMana, Affect[0], Affect[31], Guild, GuildLevel, Resist[0], CurrHp, CurrMp, Magic, Special tail se existir.
3. MSG_SetHpDam
   - sizeof total.
   - offset de Hp e Dam.
4. MSG_SendAffect ou pacote equivalente de affect completo
   - sizeof total.
   - offset/stride de STRUCT_AFFECT[32], Type, Value, Level, Time.
5. MSG_SetShortSkill
   - sizeof total.
   - offset de Skill1[4] e Skill2[16].

Tambem informe:
- Valor de Type decimal/hex para cada pacote quando existir constante.
- Se a struct usa pack(1), alinhamento natural ou pragma local.
- Trecho do dumper usado ou comando executado.

Obrigatorio:
- Rodar em MSVC x86.
- Incluir output bruto do dumper.
- Se uma struct tem nome diferente na fonte completa, informar alias e arquivo:linha.

Salve em: captura-wyd-skills-layouts.md

Formato da resposta:
WIN-SKILL-006
Ambiente compilador:
Comando/dumper:
MSG_Attack:
MSG_UpdateScore:
MSG_SetHpDam:
MSG_SendAffect:
MSG_SetShortSkill:
Output bruto:
Conclusao objetiva para o Go:
```

## WIN-SKILL-007 - SecLearnedSkill e skills 200..247

Copie este prompt:

```text
Contexto: migracao WYD -> Go. No Linux ja decodificamos e persistimos
STRUCT_MOBEXTRA.SecLearnedSkill no offset MSVC x86 4 do MobExtra, mas ainda nao ha
fonte local que prove como esse mask autoriza/aprende as skills 200..247.

Tarefa: na fonte Windows completa e, se necessario, com captura do servidor original:
1. Localize TODAS as leituras/escritas de SecLearnedSkill.
2. Localize handlers de livros/NPCs/quests que ensinam ou removem skills 200..247.
3. Localize a validacao de cast/aprendizado de skillnum 200..247 em _MSG_Attack ou equivalente.
4. Informe o mapeamento exato: qual skill usa qual bit, se ha mais de um campo, e como tratar 232..247.
5. Para ao menos uma skill 200+, capture ou logue o valor de SecLearnedSkill antes/depois de aprender e o SkillIndex enviado ao castar.

Obrigatorio:
- Cite arquivo:linha para cada leitura/escrita.
- Se a fonte tiver branch morto/inacessivel, marque explicitamente.
- Se o mapeamento nao existir na fonte, escreva NAO_ENCONTRADO e liste termos pesquisados.
- Nao inferir bit por formula sem evidencia.

Salve em: captura-wyd-skills-seclearnedskill.md

Formato da resposta:
WIN-SKILL-007
Leituras de SecLearnedSkill:
Escritas de SecLearnedSkill:
Regra de aprendizado:
Regra de cast:
Mapa skill -> bit/campo:
Captura/log de skill 200+:
Conclusao objetiva para o Go:
Pontos NAO_ENCONTRADO:
```

## WIN-SKILL-008 - Affects e ticks 40+

Copie este prompt:

```text
Contexto: migracao WYD -> Go. A matriz Sephira/shared usa affects/ticks
40/41/43/44/45/46/47/48, mas a Buff Loop local parcial so provou alguns tipos
ate 39 e o type 42. O Go nao deve implementar formulas por chute.

Tarefa: na fonte Windows completa:
1. Localize todos os consumidores de STRUCT_AFFECT.Type ou TickType para 40, 41, 43, 44, 45, 46, 47 e 48.
2. Para cada tipo, informe se altera score, dano, resist, regen, target selection, on-hit, item, invisibilidade, cooldown ou apenas icone.
3. Copie a formula exata, clamps e ordem em relacao a item/buff/base score.
4. Informe quais skills de SkillData.csv aplicam cada tipo e se dependem de SecLearnedSkill.
5. Se possivel, capture uma skill que aplica um type 40+ e registre score/HP/MP/dano antes/depois.

Obrigatorio:
- Cite arquivo:linha para toda formula.
- Se algum type nao aparecer na fonte completa, escreva NAO_ENCONTRADO.
- Diferencie affect permanente/passivo, timed buff e tick periodico.

Salve em: captura-wyd-skills-affects-40plus.md

Formato da resposta:
WIN-SKILL-008
Type 40:
Type 41:
Type 43:
Type 44:
Type 45:
Type 46:
Type 47:
Type 48:
Skills relacionadas:
Captura/log:
Conclusao objetiva para o Go:
Pontos NAO_ENCONTRADO:
```
