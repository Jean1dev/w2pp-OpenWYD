# Fase 4 — Regras de Jogo e Fórmulas (w2pp-OpenWYD)

> **Objetivo:** extrair a lógica de negócio hardcoded para pseudocódigo/tabelas determinísticas,
> para reimplementar com **paridade**. Fonte: `MobKilled.cpp`, `_MSG_UseItem.cpp`,
> `_MSG_CombineItem*.cpp`, `CMob.cpp`, `CItem.cpp`, configs em `Release/.../Settings/` e `Rates.txt`.
>
> **Aviso de paridade:** muitas fórmulas usam `rand()` (libc do MSVC). Para reproduzir *exatamente*
> os mesmos números seria preciso o mesmo gerador/sequência — inviável entre stacks. A estratégia é
> reproduzir a **distribuição** e validar por amostragem (ver Fase 8 sobre seed/RNG). As constantes
> mágicas abaixo **devem** ser idênticas.

---

## 1. Curva de EXP e distribuição em party (PvE)

Fonte: `TMSrv/MobKilled.cpp:397-1425` (`#pragma region PvE`). Disparado quando um mob (`target >=
MAX_USER`) morre por um jogador (`conn < MAX_USER`).

> **Correção (issue #43):** a primeira versão desta seção transcreveu o branch dos mapas de
> Pesadelo (`MobKilled.cpp:443-590`). O branch que governa mapas normais (incl. campo de treino)
> é o **branch geral** em `MobKilled.cpp:1272-1425`, que difere no fator `450/(30+myLevel)`, nas
> tabelas de divisores e no cap `eMob`. As §1.3–1.5 abaixo documentam o branch geral.

> **Atualização — os sete ramos.** Os seis branches por mapa foram portados
> (`internal/level/expzone.go`) e agora são escolhidos pelo **bloco de 128 tiles** onde a morte
> acontece, como o legado faz: `(tx/128, ty/128)` do corpo e de quem matou têm de coincidir, e
> qualquer outro lugar cai no campo geral. Antes disso toda masmorra pagava a tabela do campo.
> Os blocos: Pesadelo Arcano `(9,1)` `:443`, Pesadelo Místico `(8,2)` `:592`, Pesadelo Normal
> `(10,2)` `:737`, Água Arcano `(10,27)` `:851`, Água Místico `(9,28)` `:1001`, Água Normal
> `(8,27)` `:1150`, campo geral `:1272`.
>
> Quatro diferenças entre os ramos, além dos divisores:
>
> - **Escala base.** Os três Pesadelos escrevem `(UNK_1 + myLevel) * isExp / (UNK_1 + myLevel)`,
>   que é a identidade; os outros quatro escrevem `450 * isExp / (30 + myLevel)`. Como
>   `450/(30+nível) > 1` abaixo do nível 420, o campo *multiplica* onde o Pesadelo não mexe.
> - **Cap `eMob`.** Só Água e campo. No Pesadelo Arcano ele está comentado (`:531`) e nos outros
>   dois nem existe.
> - **`g_pFairyContent[0]`.** A linha de bônus é `ExpBonus + g_pFairyContent[0]` na Água e no
>   campo e só `ExpBonus` no Pesadelo — então a Fada Suprema (3913) vale 46% em campo e 16%
>   dentro do Pesadelo.
> - **Tabelas ausentes.** Pesadelo Normal não tem tabela de Mortal nem de Arch (`:747-790`), e
>   Água Normal não tem a de Arch (`:1188`): nesses casos o legado simplesmente não divide. E o
>   Pesadelo Normal repete o bloco celestial duas vezes (`:752` e `:773`), dividindo a EXP
>   celestial em dobro — portado como está.
>
> `expzone_test.go` relê cada divisor do próprio `MobKilled.cpp` e compara com a tabela portada,
> então uma edição à mão que se afaste do legado quebra o teste em vez de passar calada.

> **Mesa de XP.** Sobre os sete ramos existe uma camada de configuração
> (`internal/level/xpconfig.go`, tabela `xp_rule` da migração 0030, tela `/auditoria/xp`):
> por (zona, evolução) dá para substituir a tabela de quebras inteira e aplicar uma taxa
> percentual, que multiplica o valor final **depois** de toda a conta do legado. Sem linha
> gravada o comportamento é exatamente o legado. O tmServer lê no boot e depois repolla a cada
> 15 ticks (`handler/xpconfig.go`), como o ritmo de nascimento e a configuração de evento —
> trocar as tabelas com gente jogando paga valores diferentes para a mesma morte na virada, que
> é o mesmo degrau que ligar um evento de XP já dava, e em troca um valor errado se desfaz sem
> derrubar o servidor.

> **CONSERTADO em 10/09/2026, como divergência deliberada do legado.** A base identidade dos três
> Pesadelos passou a ser a identidade de verdade (`exp = isExp`, em `level.ExpReward`), em vez da
> conta em int32 que o original fazia e que estourava. `level.ExpOverflow` foi removida junto,
> porque o que ela diagnosticava deixou de existir, e com ela o aviso do simulador da Mesa de XP
> que mandava "BAIXAR a XP do monstro" — que era exatamente o conserto pelo dado, recusado porque
> enfraqueceria os 88 no Campo também e deixaria a armadilha armada para o próximo aumento.
>
> Medido depois do conserto, celestial, dos 406 monstros reais:
>
> | zona | nível | pagam | razão | estouro | cauda |
> |---|---:|---:|---:|---:|---:|
> | Pesadelo Arcano | 1 | 389 | 0 | **0** | 17 |
> | Pesadelo Arcano | 100 | 359 | 43 | **0** | 4 |
> | Pesadelo Arcano | 199 | 321 | 82 | **0** | 3 |
> | Pesadelo Místico | 1 / 100 / 199 | 389 / 359 / 321 | 0 / 43 / 82 | **0** | 17 / 4 / 3 |
> | Pesadelo Normal | 1 / 100 / 199 | 218 / 218 / 216 | 0 / 43 / 82 | **0** | 188 / 145 / 108 |
> | Campo | 1 / 100 / 199 | 390 / 359 / 319 | 0 / 43 / 82 | 0 | 16 / 4 / 5 |
>
> O estouro foi a zero nas três versões, os 88 que estouravam passaram a pagar, e **razão e cauda
> não se mexeram** — nem podiam: abaixo do teto a conta em int32 já dava a identidade exata, então
> só os valores que estouravam mudaram de resultado.
>
> **O que isso expõe no Pesadelo Normal:** a cauda dele é enorme (188 no nível 1) e não foi causada
> por este conserto — é o `celestialTwice`, que aplica a tabela celestial duas vezes, ÷320 sobre
> ÷320. Tudo que não é grande some na divisão inteira. Antes ficava escondido atrás do estouro;
> agora é a coisa dominante na zona, e entra na mesma conversa das tabelas de corte.
>
> O resto deste bloco é o histórico do defeito — o porquê, os números de antes, e as armadilhas
> que a análise encontrou no caminho. Fica porque quem mexer nesta conta de novo precisa dele.
>
> ---
>
> **(Histórico) Celestial no Pesadelo recebia ZERO dos monstros bons, e a tabela de cortes não tinha culpa.**
>
> Os três Pesadelos usam base identidade (`identityBase`), e ali a conta
> `(30+myLevel) * isExp / (30+myLevel)` passa por um `int32`. Como um celestial soma 400 ao nível,
> o divisor fica grande e o teto de `MobExp` que cabe no int32 fica BAIXO: 2.491.280 no nível 1,
> caindo para 1.707.061 no 199 (`level.ExpOverflow` calcula esse teto). Passou do teto, o resultado
> escalado sai da janela e o prêmio é **zero**.
>
> Medido, celestial, mob de nível 399:
>
> | mob vale | Pesadelo (os três) | Campo (controle) |
> |---|---:|---:|
> | 2.000.000 | 3.188 até o nível ~112, depois **0** | 2.988 no nível 50 |
> | 2.990.849 (o valor da curva no 399) | **0 em qualquer nível** | 4.469 no nível 50 |
>
> **NÃO é o filtro de 10M.** Ele nunca dispara aqui: a maior base possível para um celestial é
> `450 × 5.981.698 / 629 = 4.279.434`, onde 5.981.698 é o teto de 200% do `ExpApply` e 629 é
> `30 + 599`. Sobra mais que o dobro de folga até os 10 milhões.
>
> **São três causas distintas, e só uma é defeito.** Dos 406 monstros reais que o boot carrega,
> para um celestial no Pesadelo Arcano:
>
> | nível | pagam | razão de nível | **estouro** | cauda |
> |---:|---:|---:|---:|---:|
> | 1 | 301 | 0 | **88** | 17 |
> | 100 | 271 | 43 | **88** | 4 |
> | 199 | 233 | 82 | **88** | 3 |
>
> - **Razão de nível** é o `ExpApply` devolvendo 0 porque o matador é forte demais para o alvo —
>   um celestial 199 entra como 599, e contra mob de nível 50 ou 100 o retorno é 0. É a régua
>   funcionando, acontece igual no Campo (82 lá também no nível 199), e não se conserta.
> - **Estouro** é o defeito, e é **fixo em 88 monstros, em qualquer nível**. No Campo esse número
>   é ZERO. Ele cai justamente sobre os monstros fortes — onde deveria pagar mais. Os 88 são
>   homogêneos: todos nível 399 com `Exp` 2.990.849, cujo `ExpApply` bate o teto de 200%
>   (5.981.698) e passa do limite de 3.414.123 que cabe no int32 no nível 199.
>
>   **CUIDADO: existem DOIS tetos, e eles diferem por um fator de dois.** O que `ExpOverflow`
>   devolve como `limit` está em **MobExp** (1.707.061 no nível 199); o que ele compara por dentro
>   está em **isExp**, a saída do `ExpApply` (3.414.123 = `MaxInt32 / 629`). Um é o dobro do outro
>   porque o `ExpApply` bate no teto de 200%. Comparar um valor de MobExp contra o teto de isExp,
>   ou o contrário, dá a resposta errada — e foi o que aconteceu duas vezes na análise deste
>   parágrafo.
>
>   **Eram 56 até 09/09/2026, e o conserto de nível os levou a 88.** A correção que baixou 43
>   monstros de 400+ para 399 (commit `a1180829`) está certa pelo que ela conserta — acima de 400 o
>   `ExpApply` desiste de escalar e a recompensa inverte. Mas 32 daqueles monstros estavam em
>   **401 ou mais**, e por não serem escalados o `ExpApply` devolvia os 2.990.849 crus: como isExp,
>   isso fica abaixo do teto de isExp (3.414.123), não estoura, e pagava **2.383 por morte**. Em
>   399 eles voltam a ser escalados até os 200% (5.981.698), passam do teto, e zeram. Os outros 11,
>   que estavam exatamente em 400, já eram escalados e já estouravam.
>
>   Medido, celestial 199 no Pesadelo Arcano, mob com `Exp` 2.990.849:
>
>   | nível do mob | ExpApply | estoura | prêmio |
>   |---:|---:|---|---:|
>   | 399 | 5.981.698 | sim | **0** |
>   | 400 | 5.981.698 | sim | **0** |
>   | 401 e acima | 2.990.849 | não | 2.383 |
>
>   Medido nas duas árvores, celestial 199 no Pesadelo Arcano:
>
>   | árvore | pagam | razão | estouro | cauda |
>   |---|---:|---:|---:|---:|
>   | antes (`a1180829^`) | 265 | 82 | 56 | 3 |
>   | hoje | 233 | 82 | **88** | 3 |
>
>   Não é motivo para desfazer o conserto de nível: a inversão de recompensa que ele fecha vale
>   para todo mundo, e o estouro só atinge celestial no Pesadelo. Mas sobe a prioridade de tratar
>   o estouro, porque o raio dele cresceu 57%.
>
>   **E o conserto é um número só, não caso a caso.** Os 88 são homogêneos: mesma faixa (399),
>   mesmo `Exp` (2.990.849), o valor único que a curva do `exptool` carimba no topo. Some junto
>   quando a conta parar de estourar.
>
>   **Ordem sugerida:** tratar o estouro ANTES de ligar as tabelas de corte. O cronograma põe o
>   celestial como a fase mais longa do jogo; sem isso o Pesadelo segue morto para celestial
>   exatamente quando ele vira a maior parte da vida do jogador, e as tabelas que forem desenhadas
>   nesse meio-tempo serão desenhadas sobre uma zona que não paga.
> - **Cauda** é prêmio pequeno demais sobrevivendo à divisão inteira do ÷320 e virando zero. No
>   Campo são 5 no nível 199 (Orc_Medico, Troll_Zumbi, Rei_Taurus e variantes), todos com o
>   `ExpApply` já reduzido a algumas centenas.
>
> **Consequência prática:** mexer na tabela de cortes celestial do Pesadelo não muda nada hoje —
> o zero acontece ANTES de a tabela ser consultada. A tabela gravada em Pesadelo Arcano
> (119/149/169/179/189, inalcançável pelo deslocamento de 400) foi deixada como está por isso, e
> não por estar certa.
>
> Quando o estouro for tratado, a tabela passa a importar **muito**, e aí ela precisa ser desenhada
> junto com a do Campo: sem isso o Pesadelo fica cerca de 48x mais rápido e derruba o cronograma
> de progressão inteiro. É uma tarefa só, e maior.

**Gate de clã:** toda a distribuição está dentro de `if (pMob[target].MOB.Clan != 4)`
(`MobKilled.cpp:402`) — mob de clã 4 **nunca** dá EXP (gold/drop ficam fora do gate).

### 1.1. Base

```text
MobExp = GetExpApply(killer.extra, target.MOB.Exp, killer.Level, target.Level)   # :405
UNK_1 = 30                                   # constante base da fórmula  (:409)
UNK_3 = killer.extra.ClassMaster             # tier do personagem (party class)
```

> **Nível de mob acima de 400 INVERTE a recompensa.** `GetExpApply` escala pela razão de níveis
> entre matador e alvo, mas desiste quando o alvo passa do teto do jogador
> (`internal/level/level.go`, `if target > MaxLevel+1 { return exp }`) e devolve o valor cru.
> Medido: um jogador de nível 200 recebe **zero** de um mob 399 e **1.271.111** de um mob 599.
> O monstro que parece mais forte é o que paga.
>
> O conteúdo desta árvore trazia **55 templates referenciados pelo NPCGener fora de 1..399** — o
> `599` nunca foi nível, era alguém escrevendo "mais forte que 400". **43 foram corrigidos para
> 399** e **12 continuam fora, de propósito**:
>
> - **8 objetivos de evento** (Torre, Torre_Runica, Torre_de_Thor, Torre_Guardia, Torre_Guardia_,
>   Torre_Real, Arvore_de_Natal, Cristal), todos no nível 500. Baixar uma torre para 399 **não é
>   neutro**: em 500 a escala de nível está desligada e a torre paga um valor fixo; em 399 a escala
>   religa e ela passa a pagar XP cheia de fim de jogo para quem a derruba. Isso é pior que o estado
>   atual, então é decisão de produto e não varredura de dado.
> - **4 lojistas com estoque real e atributo de chefe**: Zakum_Inf (19 itens), Zakum_Inf_ (18),
>   Imp_Inferno (10) e Sulrang (2). Mexer no nível deles mexe também no que vendem.
>
> Torre_Real está nos dois grupos: é torre e carrega byte de mercador.
>
> O boot avisa enquanto sobrar alguém fora: procure `monster template level outside 1..399` no log.
> Hoje ele diz `templates=12 unscaled_above_400=12 with_merchant=5 highest_level=600`. O aviso
> existe para que a correção não se desfaça calada na próxima importação de conteúdo.

### 1.2. Bônus de party (número de membros)

```text
if 0 < ClassMaster <= MAX_PARTY (12):                 # :419
    NumMob = g_EmptyMob + ClassMaster                 # :421
    if ClassMaster > 1:
        NumMob += PARTYBONUS - 100                     # bônus por party >1  (:424)
    eMob  = MobExp
    isExp = NumMob * MobExp / 100                       # exp escalada pelo tamanho (:427)
```

> `g_EmptyMob` e `PARTYBONUS` são constantes globais (verificar valores em `Server.cpp`). UNK_3 ser
> "ClassMaster" como proxy de tamanho de party é peculiar — **UNVERIFIED** se intencional; preservar.

### 1.3. Loop de distribuição por membro (branch geral)

Para cada membro `party` da `PartyList` do líder (`MAX_PARTY+1` iterações, `:434`), exigindo o
membro vivo e dentro de `±HALFGRIDX/±HALFGRIDY` do kill (`:1272`):

```text
isExp = GetExpApply(party.extra, target.MOB.Exp, party.Level, target.Level)   # :1276
myLevel = party.Level
if ClassMaster not in {MORTAL, ARCH}:  myLevel += MAX_LEVEL + 1                # :1281

exp = 450 * isExp / (UNK_1 + myLevel)                  # fator 450/(30+myLevel)  (:1283)
gate: só premia se 0 < exp <= 10_000_000               # NÃO clampa — pula o prêmio  (:1284)
```

### 1.4. Divisores por tier e faixa de nível (tabela determinística)

`MobKilled.cpp:1286-1356`. Aplicar conforme `ClassMaster` sobre `myLevel` (divisão int/float do C,
trunca):

**MORTAL** (`:1286-1308`):
| Nível ≤ | divide exp por |
|--------:|---------------:|
| 200 | 1 |
| 300 | 1.07f |
| 356 | 1.25f |
| 370 | 1.70 |
| 380 | 2.10f |
| 390 | 2.60 |
| 399 | 4 |

**ARCH** (`:1310-1335`):
| Nível ≤ | divide por |
|--------:|-----------:|
| 200 | 1 |
| 300 | 0.85f |
| 356 | 0.90f |
| 360 | 4.50f |
| 370 | 5.90f |
| 380 | 11 |
| 390 | 17 |
| 400 | 35 |

**Outros tiers (Celestial/SD/SP/DK/CS — não MORTAL nem ARCH)** (`:1337-1356`):
| Nível < | divide por |
|--------:|-----------:|
| 120 | 10 |
| 150 | 20 |
| 170 | 40 |
| 180 | 80 |
| 190 | 160 |
| ≥190 | 320 |

### 1.5. Ajustes finais e eventos

```text
exp = 6 * exp / 10                                    # corte fixo de 40%  (:1358)
if exp > eMob:  exp = eMob                            # cap no GetExpApply do killer  (:1360)
if 0 < killer.ExpBonus < 500:
    exp += exp * (ExpBonus + fairy) / 100             # bônus de item + fada  (:1363)
if g_pRvrWar.Bonus == party.Clan:  exp += exp*5/100   # RvR +5%  (:1366-1370)
if NewbieEventServer and party.Level < 100 and tier not Celestial*:
    exp += exp / 4                                    # +25% newbie  (:1372)
if DOUBLEMODE:   exp *= 2                              # evento exp dobrada  (:1375)
if KefraLive == 0:  exp /= 2                           # Kefra derrubado penaliza  (:1378)
if NewbieEventServer:  exp += exp*15/100  else  exp -= exp*15/100   # ±15%  (:1381-1384)

# Log diário de exp (reset por dia)  (:1386-1391)
# "Hold" de exp (trava de ganho — consome o ganho até quitar o Hold)  (:1393-1408)
# Clamp ao máximo:
if party.MOB.Exp + exp > g_pNextLevel[MAX_LEVEL+1]:
    party.MOB.Exp = g_pNextLevel[MAX_LEVEL+1]          # :1410-1417
```

> **Bônus de XP em grupo = o de quem matou.** Nas mesmas linhas, o bônus sai de `conn` (quem deu
> o golpe final; num summon, o dono) e todo o resto sai de `party` (o membro sendo pago): nível,
> evolução, zona, clã da guerra de reino, evento de novato. Vale nos sete ramos — os três do
> Pesadelo (`:534/679/794`, só `ExpBonus`) e os quatro de Água e campo (`:943/1092/1214/1363`,
> com `g_pFairyContent[0]`); o teto `< 500` também olha quem matou. Então um grupo que mata com
> um personagem de +100% recebe +100% cada um, e um membro de +100% que não matou recebe sem
> bônus. O rewrite chegou a ler o bônus de cada membro; voltou ao do legado (fidelidade
> restaurada, `doMatador` em `tmserver/internal/handler/mobkilled.go`).
>
> **O teto `eMob` também é de quem matou.** `eMob` é o `GetExpApply` de `conn` (nível de quem
> matou contra o do mob), calculado uma vez antes do laço (`:405`, `:426`) e aplicado a todo
> membro em Água e campo (`:940/1089/1211/1360`); os Desertos do rewrite copiam o campo e têm o
> mesmo teto. No Pesadelo o teto está comentado (`:531-532`) e continua sem existir. Efeito: quem
> está muito acima do mob limita o grupo inteiro ao número pequeno dele, e com uns 2x o nível do
> mob esse número é 0 — ninguém ganha. Carregar personagem fraco só rende se o fraco der o golpe
> final. O rewrite chegou a limitar cada membro pelo próprio `GetExpApply`; voltou ao do legado
> (fidelidade restaurada, `KillingBlow` em `internal/level/expreward.go`). Sozinho não muda nada:
> quem mata e quem recebe são o mesmo personagem.

> **Dados (issue #43):** os templates originais de `Release/TMsrv/run/npc/` traziam `Exp` zerado
> ou absurdo em centenas de monstros. O campo é regravado offline por `tmserver/cmd/exptool`
> usando a curva `level.MobExpForLevel` (pacing clássico em kills-por-level, invertendo o
> pipeline acima com flags default). Rode a ferramenta após adicionar/editar templates.

`g_pNextLevel[]` é a tabela de XP por nível (curva de level-up). Flags globais `DOUBLEMODE`,
`NewbieEventServer`, `KefraLive` controlam eventos. A `Rates.txt` (não-código, é descritiva ao
jogador) lista faixas de exp por área — **não** é parseada pelo servidor; serve de referência.

---

## 2. Drop

Fonte: `MobKilled.cpp:2693-2900+`. Ocorre na morte do mob.

### 2.1. Drop de gold (`:2693-2722`) — determinístico exceto `rand()`

```text
UNKGOLD = 18
if target.Level < 10: UNKGOLD = 2
elif target.Level < 20: UNKGOLD = 4
elif target.Level < 30: UNKGOLD = 6
elif target.Level < 50: UNKGOLD = 9
UNKGOLD = rand() % (UNKGOLD + 1)          # chance: ~1/(UNKGOLD+1) de dropar

if MobCoin != 0 and UNKGOLD == 0:         # dropou
    MobCoin = 4 * (rand()%(((MobCoin+1)/4)+1) + (MobCoin+1)/4 + MobCoin)
    if MobCoin > 2000: MobCoin = 2000     # teto por kill
    killer.Coin += MobCoin                # com clamp de 2_000_000_000
```

### 2.2. Drop de item comum (`:2758-2800+`) — tabela = `Carry[]` do mob

A **loot table de cada mob é o próprio inventário** `MOB.Carry[MAX_CARRY=64]`. Para cada slot
ocupado:

```text
for i in [0, MAX_CARRY):
    if target.Carry[i].sIndex == 0: continue
    droprate  = g_pDropRate[i]                         # taxa-base por slot (:2764)
    dropbonus = g_pDropBonus[i] + killer.DropBonus     # bônus do matador, handler/drop_bonus.go (:2765)
    if dropbonus != 100:
        dropbonus = 10000 / (dropbonus + 1)
        droprate  = dropbonus * droprate / 100         # bônus reduz o divisor (:2769-2770)
    pos = i / 8
    if i < 60 and pos in {0,1,2}:                      # ajuste por nível do alvo
        if target.Level < 10: droprate = 4*droprate/100
        elif target.Level < 20: ...                    # (:2779+)
    # roll final compara droprate com rand() (continua após :2800)
```

`g_pDropRate[]`/`g_pDropBonus[]` são arrays globais **por slot** (64 posições do `Carry`, não por
item) — **origem confirmada:** valores estáticos em `Basedef.cpp:222-238`, ajustáveis em runtime via
`gameconfig.txt` (carregado em `Server.cpp:1302-1342`) e por comando GM (`imple.cpp:1095-1109`).

Valores-base reais (`Basedef.cpp`):
```text
g_pDropBonus[64] = todos 100               # 100 = sem bônus (neutro)
g_pDropRate[64]  = {                        # quanto MAIOR, mais raro (é divisor/odds)
  slots  0-7  : 900            (equip comum)
  slots  8-11 : 4              (muito comum — provável gold/poção)
  slots 12-15 : 900
  slots 16-23 : 20000          (raríssimo)
  slots 24-47 : 2000
  slots 48-55 : 3000
  slot  56    : 1              (sempre dropa)
  slots 57-63 : 35,500,2500,5000,5000,10000,20000
}
```
> O slot do `Carry` do mob determina a raridade — não o item em si. `ItemDropList.txt` no `Release/`
> (formato `Item: N: Mobs:<count>`) é **gerado** pelo `DropTool.exe` (relatório inverso), **não**
> consumido em runtime. Na migração, modelar drop como `(mob_slot → item, rate)` e tornar
> `g_pDropRate`/`Bonus` configuráveis.

### 2.3. Drop de evento global (`:2725-2755`)

```text
if evOn and evItem and evRate and evCurrentIndex < evEndIndex and rand()%evRate == 0:
    item.sIndex = evItem
    if evIndex:   # item serializado/numerado
        item.eff[0]=62/idx_hi; eff[1]=63/idx_lo; eff[2]=59/rand()
    SetItemBonus(item, target.Level, 0, 0)
    PutItem(conn, item); broadcast notice
    evCurrentIndex++
```

Controlado por variáveis globais de evento (`evOn/evRate/evItem/evStartIndex/evEndIndex`) — viram
config/feature-flags na stack nova.

### 2.4. Bônus de drop — `SetItemBonus` (`Server.cpp:1777-2717`)

Todo equipamento que um mob derruba passa por aqui antes de chegar ao inventário. É o que faz duas
cópias do mesmo item saírem diferentes: o catálogo dá os números base, esta função dá a **uma cópia**
os seus três pares de efeito. Portado em `tmserver/internal/refine/dropbonus.go`, chamado de
`handler.rolarBonusDrop` no drop comum e no drop de evento.

**Entradas.** `Level` (nível do mob), `a3` (modo cristal), `DropBonus` (fada/gema/Grade 5, dividido
por 8 e limitado a 2, `CMob.cpp:701-864`), e um marcador `EF_GRADE0..5` (100-105) que o chamador
pode deixar em `stEffect[0]`: o id vira a distância de nível e o valor vira o teto de refino.

**Distância de nível** (`lvdif`), o número que escolhe todas as tabelas:

```text
if not a3 and Level >= 210: Level -= 47
lvdif = (Level - ReqLvl + 1) / 25         # marcador de grade sobrescreve
pForc = lvdif >= 4                        # piso garantido, medido ANTES do corte
clamp lvdif to 0..3;  if a3: clamp to 0..2
```

**Portão.** Só entra se `nPos & 0xFE`, `stEffect[0]` vazio e `nPos != 128`. Rosto (bit 0), escudo
puro (128) e tudo de acessório pra cima (256+) ficam de fora. O `case 128` da segunda tabela de
efeitos é código morto no original.

**Seis sorteios, nesta ordem** (a ordem é o que a paridade de RNG compara):

| # | Sorteio | Papel |
|---|---------|-------|
| 1 | `rand()%101 % div` | qual efeito no espaço 2. `div` = 8−bônus (lvdif 0), 6−bônus (1 e 2), 4 (3), 3 (cristal) |
| 2 | `rand()%100 % div` | qual efeito no espaço 3. `div` = 8/6/6/4, **sem** o bônus de drop |
| 3 | `rand()%100` | magnitude do espaço 2 (escada por lvdif); +`rand()%128` se sair vazio |
| 4 | `rand()%100` | magnitude do espaço 3; +`rand()%128` se sair vazio |
| 5 | `rand()%100` | espaço 1: refino, bônus especial ou nada |
| 6 | `rand()%100` | só quando 5 deu +2 e o marcador de grade tinha teto > 2 |

Armadura, calça, luva e bota **não consultam** o sorteio 1: sempre recebem `EF_CRITICAL2`,
`EF_CRITICAL2`, `EF_ACADD2` e `EF_DAMAGE2` respectivamente. Elmo e arma é que escolhem entre duas ou
três opções. Bota com magnitude ≤ 0 grava `EF_DAMAGE2` com valor 0 em vez do marcador de nada — é a
única peça que faz isso.

Faixas do sorteio 5: `6/22/75/90` para lvdif 0 e 1, `6/35/85/100` para 2 e 3 — refino +2 sai em 6%
sempre, e na faixa alta o resultado "nada" desaparece. **Item sem marcador de grade nunca passa de
+2**, o que é o freio do sistema inteiro.

**Cauda.** Depois de tudo, os 12 efeitos do catálogo são varridos e três deles sobrescrevem
`stEffect[0]`: `EF_SANC` (refino fixo), `EF_AMOUNT` (quantidade) e `EF_INCUBATE` (valor + `rand()%4`,
teto 9). Em seguida, treze índices de material (412/413/419/420/753, 447-450, 692-695) recebem
`EF_UNIQUE` com bytes aleatórios em todo espaço vazio — um carimbo de identidade por item que
**nada no original lê de volta**.

#### Três defeitos do original, corrigidos no port

1. **Troca de variável no segundo bônus** (`:2420-2436`). No ramo `lvdif == 0` o teste externo lê o
   sorteio do *primeiro* bônus e o `else` escreve na magnitude do *primeiro* bônus, que já foi
   gravada. 2% dos drops de nível compatível perdiam o segundo bônus. O port usa o próprio sorteio.
2. **Leitura fora da tabela** (`:2531-2534`). `g_pBonusValue` é `[10][2][2]` e é indexado por `lvdif`,
   que chega a 3: em lvdif 2 e 3 lê as linhas do tipo seguinte, e no tipo 9 lê para fora do array,
   em cima de `g_pBonusType`. O port limita o índice às linhas que existem.
3. **Ramo morto** (`:2390-2415` e gêmeo). As duas escadas têm uma tabela completa para `lvdif >= 4`
   que nunca roda, porque `lvdif` já foi cortado em 3. Não foi portada.

Nenhuma das três muda a **contagem** de chamadas de `rand()`, então a paridade de sequência
se mantém.

**Configurável desde 0037_drop_bonus:** as duas escadas (a magnitude e as faixas do refino) saem do
banco, uma linha por distância de nível, lidas **só** no boot (ao contrário da Mesa de XP, que
repolla). O painel edita em
`/rates/bonus-drop`, e o mesmo lugar tem o interruptor que devolve o servidor ao comportamento sem
sorteio. Qual efeito cada peça recebe NÃO é editável: é conteúdo, e mexer nele mudaria o que o jogo é.

**Portado** em `handler/drop_bonus.go` (`CMob.cpp:700-870`): `pMob[conn].DropBonus` é a soma da fada
(Azul +32; Vermelha +16, e só essas duas), mais 8 por peça Grade 5 e 8 por peça com gema 0, nos 16
espaços de equipamento. Fica em `Entity.EquipDropBonus`, recalculado no mesmo `refreshScore` que já
cuidava do bônus de experiência.

Ele é lido em **dois** lugares, e são sorteios diferentes: a chance do item cair
(`MobKilled.cpp:2765`) e o bônus rolado nele quando cai (`:2865`). O drop de **evento** fica de fora
dos dois de propósito — o legado passa um zero literal ali (`:2752`), então o prêmio é o mesmo
prêmio para todo mundo.

Não existe fonte de afeto, aqui nem no legado: `BASE_GetCurrentScore` preenche `ExpBonus` e nunca
encosta em `DropBonus`.

---

## 2.9. Loja de NPC — a vitrine NÃO são os 27 primeiros slots

O estoque de um lojista é o `Carry[]` do próprio template, mas a vitrine tem **27 linhas em 3 abas
de 9**, e as abas não são contíguas: `Carry[0..8]`, `[27..35]`, `[54..62]`
(`protocol.ShopSlot`, `MAX_SHOPLIST` = 27 em `Basedef.h:142`). O `dbserver import-npcs` semeia o
`npc_shop_item` pelo mesmo mapa, e a tabela só aceita `slot BETWEEN 0 AND 26`.

Ler "os 27 primeiros" dá uma resposta errada nos dois sentidos: inclui `Carry[9..26]`, que não é
vitrine, e exclui `[27..35]` e `[54..62]`, que são. Uma auditoria feita assim conclui que itens
inteiros não estão à venda quando estão.

**Preço 0 não quer dizer "não dá para comprar": quer dizer DE GRAÇA.** `handler.buy` aceita
`Price == 0` e só recusa preço negativo ou ouro insuficiente — é fiel ao legado
(`shop.go:83`). Então um item a zero na vitrine é entregue sem custo.

Índice **ausente do catálogo** é outra coisa: `buy` sai no `!ok` do `itemPrices`, então não é item
grátis, é vitrine suja. Aparece na lista que o cliente recebe e não pode ser comprado.

**A compra alcança mais espaços do que a vitrine mostra.** `buy` valida só `npcPos < MaxCarry`,
ou seja 0..63 — como o legado, que checa `TargetInvenPos >= MAX_CARRY` (`_MSG_Buy.cpp:49`) e indexa
`Carry[TargetInvenPos]` direto, porque é o cliente que manda o slot cru. Consequência: um item a
preço zero guardado FORA das três abas não aparece para ninguém e continua comprável por pacote
montado. É por isso que o aviso do boot conta duas linhas, vitrine e escondido — contar só a
vitrine deixaria passar justamente o caso invisível.

O boot conta isso em **três linhas**, e a terceira existe para as outras duas significarem algo:

```
shop stock priced at zero — the buyer pays nothing for it        items=27 slots=66
stock priced at zero OUTSIDE the shop window — invisible…        items=3  slots=24
class-master skill menu (not merchandise…)                       items=96 slots=96
```

As 96 são os livros de habilidade (`handler.SkillItemFirst..SkillItemLast`, 5000-5095, a faixa que
o próprio `learnSkill` valida). Elas ficam no `Carry` do mestre de classe como mercadoria fica, mas
não são: aprender custa **ponto de habilidade** e `learnSkill` não encosta no `Carry` — "comprar"
uma põe uma linha inútil na mochila e não ensina nada. Contadas juntas, seriam 96 falsos fixos que
impediriam o número de chegar a zero, e alarme que nunca zera vira paisagem.

Os 3 de fora da janela não são exclusivos dela: aparecem também em vitrine. Ou seja, hoje nenhum
item grátis mora **só** escondido — a linha existe para o dia em que morar.

### O template NÃO é o que o jogador vê

Com `-npc-editing` ligado — que é o estado de produção — os blocos de lojista do `NPCGener.txt` são
**pulados** (`merchant_blocks_skipped=548` no log) e quem manda são as definições do
`npc_definition`. `applyShop` **zera o Carry inteiro** e escreve os slots vindos do `npc_shop_item`.
Consequência: **editar o `Carry` do arquivo de template não muda nada para um lojista.**

Isso custou caro uma vez: cinco itens denominados em crédito de doação (as Cosmo Energia, `EF_DONATE`
200 a 10000) ficaram **de graça** numa loja viva por dois dias depois de terem sido "removidos" — a
remoção foi no arquivo, e a loja lia o banco. Foram apagados pelo painel em 09/09/2026, nos dois
registros de `DonatesBars` (NPC 17 e 627), deixando cada loja só com o índice 5751, que não está no
catálogo e por isso não é comprável.

Por isso a contagem de estoque grátis existe **duas vezes, e nunca somada**:

| Onde | O que responde | Quando roda |
|---|---|---|
| `main.spawnNPCs` | o que a IMAGEM traz no `Carry` dos templates | boot |
| `handler.auditaEstoqueGratis` | o que está À VENDA agora, pelo `npc_shop_item` | boot e a cada salvamento do painel |

Somar as duas esconderia justamente a discordância entre elas, que é o sintoma. A segunda usa o
preço **efetivo** (`d.itemPrices`: catálogo com as sobreposições por item escritas por cima), então
um item que o catálogo dá como zero mas o painel precificou não conta.

### Três lojas do conteúdo não existem em produção

`MileageTrader`, `Galaxy_Store*` e `Nordic_Store*` existem como arquivo em `Release/TMsrv/run/npc`,
**não são referenciados pelo `NPCGener.txt` e não estão no `npc_definition`** (conferido no painel
em 09/09/2026: a busca por Mileage, Galaxy e Nordic não devolve NPC nenhum). Ou seja: não nascem por
caminho nenhum e nada que esteja no estoque delas está à venda.

Isso vale para o **Selo Contratual (3444)**, o **Mandado de Exílio (5602)** e os quatro **Cartão de
Classe (4021-4024)**: eles constam do estoque dessas lojas e portanto **nunca estiveram à venda de
verdade**. Decisões tomadas sobre eles foram tomadas sobre item morto. Fica escrito para não
voltar — e para lembrar que estoque de template só significa alguma coisa quando o NPC existe.

Preços fechados até aqui, sempre e só para o que está de fato numa vitrine: Cristal de Extração
20.000.000 (5 dos 6 — o de Arma não está em loja), a série de Jóias 100 (o bloco vizinho 3206-3208
é a mesma coisa com outro nome e custa isso), Montarias copiando o homônimo já precificado
(150.000 / 1.500.000 / 2.000.000), Medalhas coloridas 1.000.000 (3 das 6), Esfera da Sorte N/M/A
200.000, e Entrada do Território **1.100.000 / 1.100.000 / 2.200.000** — este último não veio de
âncora e sim do irmão funcional: é bilhete de teleporte descartável (`useEntradaTerritorio`,
`EF_VOLATILE 188`), e o `Pedido_de_Caça` (`EF_VOLATILE 195`) já traz a escada pronta, 1.100.000 em
Armia/Dung/SubM/Kult, 2.200.000 no Nipple e 3.300.000 no Kefra.

Estado hoje, medido sobre os templates de `Release/TMsrv/run/npc`: **as 5 Cosmo Energia, os 4
Cartão de Classe, o Selo Contratual (3444) e o Mandado de Exílio (5602) foram REMOVIDOS** do
estoque de `DonatesBars`, `Galaxy_Store__`, `MileageTrader` e `Nordic_Store___`. Eles não podiam
ser precificados em ouro: são a categoria que vai para a loja de doação do site, e um NPC não tem
como cobrar em moeda de doação — **o tmServer não tem uma linha sequer sobre saldo de doação**,
que existe só no adminServer, no webServer e no banco. Preço em ouro faria o NPC concorrer com a
loja paga; zero fazia o NPC dar de graça.

## 3. Refino / Combine (Anct e variantes)

São ~10 handlers `_MSG_CombineItem*` (Fase 1 §3.1). Compartilham o padrão: validar combinação →
rolar sucesso → aplicar/consumir. **Consolidar numa engine de "receitas" parametrizada.**

### 3.1. Engine base — `_MSG_CombineItem.cpp` (Anct)

```text
combine = GetMatchCombine(m->Item)        # taxa da receita casada (0 = inválida)  (:46)
if combine == 0: erro "Wrong_Combination"; return                                  (:48-53)

# consome os itens de entrada
for i in MAX_COMBINE: if Item[i].sIndex: clear Carry[InvenPos[i]]; SendItem(...)    (:55-62)

# ROLL DE SUCESSO (chave para paridade):
_rand = rand() % 115                                                                # :80
if _rand >= 100: _rand -= 15        # "achata" 100..114 -> 85..99                    (:81-82)
success = (_rand <= combine) or LOCALSERVER                                          (:84)

if success:
    itemindex = Item[0].sIndex
    extra = g_pItemList[itemindex].Extra            # item resultante
    joia  = Item[1].sIndex - 2441                    # tipo da joia (0..3)            (:92)
    if 0 <= joia <= 3:
        Carry[ipos] = Item[0]; Carry[ipos].sIndex = joia + extra
        BASE_SetItemSanc(Carry[ipos], 7, 0)          # define refino/sanc = 7         (:100)
        _MSG_CombineComplete parm=1 (sucesso)
else:
    _MSG_CombineComplete parm=2 (falha)                                              (:139)
```

> **Constante de paridade crítica:** o roll é `rand()%115` com o "achatamento" `>=100 ⇒ -15`. Isso
> torna os valores 85..99 **duas vezes mais prováveis**. Reproduzir essa distribuição exatamente,
> não substituir por `rand()%100`. O sucesso é `_rand <= combine`, onde `combine` é a taxa (0..100)
> da receita.

### 3.2. Tabelas de taxa (`Release/Common/Settings/CompRate.txt`)

Carregadas por `CReadFiles::ReadCompRate()` (`CReadFiles.h:32`). Formato `Família Chave Valor`:

| Família | Chave | Taxa |
|---------|-------|-----:|
| Tiny | ChanceBase | 15 |
| Shany | ChanceBase | 30 |
| Ailyn | ChanceBase | 10 |
| Agatha | ChanceBase | 15 |
| Compositor | Item_+7 / +8 / +9 | 2 / 4 / 10 |
| Odin | Item_12_Ref_0..9 | 2,3,4,5,6,7,8,9,12,15 |
| Odin | Item_12_Minus_12..15 | 2,3,4,5 |
| Odin | Item_Celestial / Secreta | 5 / 1 |
| Ehre | Espiritual / Amunra | 40 / 10 |
| Ehre | (demais) | 100 |

(Lista completa no arquivo; cada handler `_MSG_CombineItem<Família>` usa `GetMatchCombine<Família>`.)

### 3.3. Tabela de refino por anvil/sanctificação (`Settings/SancRate.txt`)

Carregada por `ReadSancRate()` (`CReadFiles.cpp:77`). **PO = Poeira de Ori (412, `EF_VOLATILE 4`)**,
**PL = Poeira de Lac (413, `EF_VOLATILE 5`)** — o índice da linha é o `OriLacto = Vol-4`.

O arquivo **só sobrescreve os índices que lista**; o resto mantém o default compilado em
`Basedef.cpp:68`. As linhas do arquivo:

```text
PO      0..2 = 100%, 3=85, 4=70, 5=40
PL      0..5 = 100%, 6=80, 7..8=70, 9=10
Âmago   0=100,1=80,2=60,3=40,4=20,5..7=10,8..11=5
```

> ⚠️ **`BASE_GetSuccessRate` indexa a tabela em `sanc+1`**, não em `sanc` (`Basedef.cpp:2261`): o
> slot 0 de cada linha é morto e um refino **a partir de +N** lê o slot **N+1**. Então "PO 3 = 85"
> é a chance do **+2→+3**, não do +3→+4.

#### Bug do Âmago no legado — divergência intencional (issue #103)

`CReadFiles.cpp:157` grava as linhas `ÂMAGO` em `g_pSancRate[0]` — **a mesma linha do `PO`**
(`:123`), um copy-paste. Como o Âmago vem por último no arquivo, ele **sobrescrevia todas as taxas
do Ori**, e as linhas `PO` do arquivo eram letra morta. A taxa efetiva do Ori no servidor legado era
a do Âmago: `{100,80,60,40,20,10,...}`.

> **Decisão (issue #103): corrigido.** O Âmago vai para a linha 2 e as linhas `PO` do arquivo passam
> a valer — o `SancRate.txt` vira genuinamente autoritativo. Consequência: o arquivo não lista `PO 6`,
> então o índice 6 fica com o default do `Basedef.cpp` (**100**) e o **+5→+6 com Ori é garantido**.
> Para fechar isso basta acrescentar uma linha `PO 6 <taxa>` ao arquivo. A outra leitora da linha do
> Âmago é `BASE_GetGrowthRate` (`Basedef.cpp:2275`, crescimento de montaria), ainda não portada.

O arquivo é **cp1252**, não UTF-8 (`Âmago` = bytes `C2 6D 61 67 6F`). O legado também não decodifica:
`_strupr` só mexe em `a-z` no locale C, então o `0xC2` sobrevive e o `strcmp` casa byte a byte — o
port faz o mesmo (`content/rates.go`).

#### Pity ("success") e a codificação empacotada do sanc

O `cValue` do par `EF_SANC` **não guarda o nível puro** — ele empacota nível + um contador de
tentativas falhas (`BASE_SetItemSanc`, `Basedef.cpp:2282`):

```text
níveis 0..9    cValue = nível + 10*pity     (pity 0..20)
níveis 10..15  cValue = base + gem          (base 230/234/238/242/246/250, gem 0..3)
```

Cada ponto de pity soma `g_pSuccessRate[nível+1]` = `{5,5,5,5,4,4,3,3,2,1,0}` à taxa da próxima
tentativa. Uma falha acumula pity com chance 3/4 (`rand()%4 <= 2`): Lac sempre, Ori só até +5. Um
sucesso zera o pity.

> ⚠️ Ler o `cValue` cru reporta um nível errado para qualquer item que já falhou um refino — um +5
> com pity 2 guarda `25`. Ver o pacote `tmserver/internal/refine`, que é a única implementação.

### 3.4. Cooldown anti-spam de refino — **DESATIVADO no código**

`_MSG_UseItem.cpp:209-221`: o bloco que impedia refinar mais de 1×/segundo
(`if GetTickCount()-UseItemTime < 1000`) está **comentado**. Ou seja, **não há rate-limit** de
refino hoje.

> **Decisão de migração:** reativar um cooldown server-side (anti-macro) é recomendável, mas muda
> comportamento — registrar como divergência intencional vs. paridade pura (Fase 8/9).

### 3.5. Limites de refino
- Itens "tipo 5" (selados) não refinam além de `sanc >= 9` (`_MSG_UseItem.cpp:227`); outros não
  passam de `sanc >= 6 && Vol == 4` (`:203`). Mensagem `_NN_Cant_Refine_More`.

### 3.5.1. Quanto o refino vale em status (issue #282)

`BASE_GetItemAbility` (`Basedef.cpp:1687-1867`) devolve, para **um** efeito, a soma
catálogo + instância e só então aplica o multiplicador `(sanc+10)/10`. Duas regras
acompanham:

- **Promoção de acessório** (`:1851-1852`): item com `nPos & 0xF00` (os quatro slots
  `Equip[8..11]`) e `sanc == 9` passa a contar como `sanc = 10`, ou seja um **×2 exato**.
- **Lista de isenção** (`:1854`): `EF_GRID, EF_CLASS, EF_POS, EF_WTYPE, EF_RANGE, EF_LEVEL,
  EF_REQ_*, EF_VOLATILE, EF_INCUBATE, EF_INCUDELAY, EF_MOBTYPE, EF_ITEMTYPE, EF_ITEMLEVEL,
  EF_NOTRADE, EF_NOSANC, EF_DONATE` não escalam — são metadados de identidade/requisito.
  `EF_RUNSPEED` escala e depois passa por um clamp próprio (`:1860-1867`): nunca ultrapassa 2,
  e um item em `sanc == 9` **promovido não** ganha o ponto extra.

Exemplo canônico: `Pedra_Amunra` (**3464**, `ItemList.csv:5289`) tem `EF_STR/INT/DEX/CON 100`.
Em +9, acessório, cada pedra vale **200** — e um personagem usa quatro, logo **+800** por atributo.
O tooltip do cliente já implementa isso; o servidor precisa bater com ele.

> Os thresholds `+25 AC` (defesa, `Basedef.cpp:4601-4622`) e `+40 dano` (arma,
> `CMob::GetCurrentScore`) são somas **separadas**, aplicadas **por cima** do valor já
> multiplicado — não substituem o multiplicador. Ver `captura-wyd-affect-divina.md` §E.

No port: `handler.itemAbilityRefined` é a única implementação; `equipBonus`/`weaponDamage` a
consomem uma vez **por efeito distinto** de cada item, porque a divisão inteira precisa truncar a
soma uma única vez.

### 3.6. Refino com poeiras — onde ele realmente mora (issue #103)

**O refino com poeiras NÃO fica no handler de combine.** Ele vive dentro de `_MSG_UseItem` (0x0373):
o cliente arrasta a poeira sobre o item e o servidor classifica a ação pelo `EF_VOLATILE` da
poeira-origem, como qualquer outro uso de item. O `MsgUseItem` já carrega `DestType`/`DestPos`.

São **cinco** caminhos distintos no mesmo handler. Só o primeiro está portado:

| caminho | fonte | roll | falha |
|---|---|---|---|
| **equipamento padrão** ✅ | `:802-976` | `rand()%100` | só gasta a poeira; item intacto, pity sobe |
| selados no inventário ⏳ | `:224` | `rand()%115` + fold `-15` | **item destruído** |
| celestial/HC (+10~+15) ⏳ | `:385` | `rand()%115` + fold | sanc → 0 |
| pedras arcanas (1752-1759) ⏳ | `:514` | `rand()%115` + fold | sanc → 0 |
| brincos +10~+14 (Lac, slot 8) ⏳ | `:716` | `rand()%100` | **item destruído** |

> ⚠️ O roll do caminho principal é `rand()%100` puro — **não** é o `rand()%115` com o fold `>= 100 →
> -15` do combine (§3.1). As duas distribuições são diferentes e não são intercambiáveis.

Itens relevantes: `Lactolerium_100` (**4141**) tem `EF_VOLATILE 5` como uma Lac comum mas força a
linha do Âmago → 100% garantido. `EF_NOSANC` (126) bloqueia o refino de vez. Ori trava o item em +6,
Lac em +9; +10 vai a +11 preservando o gem e para.

**Ovo de montaria (2300-2329):** o bloco de incubação está inline no sucesso (`:890`). O gate de
choco lê `BASE_GetBonusItemAbility(dest, EF_INCUBATE)`, que é **instance-only** (`Basedef.cpp:2048`)
— e o `EF_INCUBATE` do ovo está no **catálogo**. Ou seja: lê 0 e **o ovo choca no primeiro refino
bem-sucedido**, independente do tier do catálogo. É o comportamento do legado, não um atalho do port.
Uma falha carimba `EF_INCUDELAY` (0..3), que só é decrementado pelo `RegenMob` **com o ovo
equipado** (`Server.cpp:4884`) — é o cooldown entre tentativas.

`MountProcess(conn, 0)` (`:914`) é **no-op**: com `Mount == NULL` o `IsEqual` fica 1 e a função
retorna de cara (`Server.cpp:4639-4643`). Não há nada para portar ali.

#### 3.6.1. Tinturas → Feijão Mágico (issue #130) — **UNVERIFIED, decisão de migração**

`Tintura_*` (**3397-3406**, 10 cores) não têm `EF_VOLATILE` nem `EF_NOSANC` — sem o fix, caíam no
caminho de "equipamento padrão" acima e ganhavam um `EF_SANC` falso via `refine.Bootstrap`. O
comportamento esperado (relatado na issue): arrastar uma poeira de Ori **ou** de Lac sobre uma
tintura consome a poeira e transforma o item no `Feijão_Mágico` da mesma cor (**3407-3416** =
`tintura + 10`, mesma ordem de cores que `useMagicBean` já usa para o caminho inverso, `item.go:1072`).

> ⚠️ **Nenhuma evidência no legado**: os dois branches de poeira (`_MSG_UseItem.cpp:140-980`) e uma
> busca no `Source/` inteiro por `3397`-`3417`/`Tintura`/`Feijão` não encontram nenhum caso especial
> para essa transformação — o único hit em `3407` é o branch, já portado, de *usar* um Feijão Mágico
> sobre um equipamento (`:3767-3861`, `useMagicBean`). A conversão é portanto uma decisão de
> migração (não um port): determinística, sem roll de sucesso, já que a tintura não carrega
> `EF_SANC` para rolar contra. Implementado em `refineTintura` (`refine.go`), curto-circuitado
> **antes** de `refine.Bootstrap`.

---

## 4. Combate (dano) — **fórmulas verificadas**

> Correção vs. versão anterior: as funções `BASE_*` **têm fonte** em `Basedef.cpp` (não são lib
> opaca). As fórmulas abaixo são o código real. RNG via `rand()` (validar por distribuição, Fase 8).

### 4.1. Fórmula de mitigação de dano — `BASE_GetDamage(dam, ac, combat)` (`Basedef.cpp:1265`)

```text
tdam  = dam - ac/2                       # AC mitiga metade do seu valor
combat = min(combat/2, 7)                # "combat" = nível de maestria da arma, teto 7
delta = 12 - combat
rnd   = rand() % delta + combat + 99     # fator % ∈ [combat+99 , 110]  (variância da arma)
tdam  = rnd * tdam / 100                 # aplica o fator percentual

# "piso" não-linear quando o dano fica baixo/negativo:
if tdam < -50:           tdam = 0
elif -50 <= tdam < 0:    tdam = (tdam+50)/7
elif 0 <= tdam <= 50:    tdam = 5*tdam/4 + 7
if tdam <= 0:            tdam = 1         # dano mínimo sempre 1
```

> Reproduzir **exatamente** o `rnd` (faixa depende de `combat`) e a escada de pisos. Quanto maior a
> maestria (`combat`), menor a `delta` → menos variância e piso de dano mais alto.

### 4.2. Dano de skill — `BASE_GetSkillDamage(dam, ac, combat)` (`Basedef.cpp:1486`)

```text
tdam  = dam - ac/2
combat = min(combat, 15)                 # teto 15 (skills usam mais maestria que melee)
delta = 21 - combat
rnd   = rand() % delta + combat + 90      # fator % ∈ [combat+90 , 110]
tdam  = tdam * rnd / 100
if tdam < -50:          tdam = 0
elif -50 < tdam < 0:    tdam = (tdam+50)/10
elif 0 <= tdam <= 45:   tdam = 5*tdam/4 + 5
if tdam <= 0:           tdam = 1
```

### 4.3. Pipeline de um golpe — `_MSG_Attack.cpp` (por alvo em `Dam[i]`)

```text
dam = atacante.CurrentScore.Damage                              # :442

# Crítico duplo (BASE_GetDoubleCritical decide bits em DoubleCritical):       :440
if DoubleCritical & 2:   # "critical parcial"
    dam = ((rand()%2 + 13) * dam)/10   se alvo é player (×1.3–1.4)            # :447
    dam = ((rand()%2 + 15) * dam)/10   se alvo é mob    (×1.5–1.6)            # :449

Ac = alvo.CurrentScore.Ac
if alvo é player:  Ac *= 3              # players têm AC 3× mais eficaz em PvP  :454-455
dam = BASE_GetDamage(dam, Ac, master)  # master = maestria da arma (§4.1)      :457

# (FM/Class 3 com skill 0x200000: 25% de chance de golpe em área extra)       :459-478
if DoubleCritical & 1:   dam *= 2       # crítico total dobra                  :480

# ... resolução de acerto/parry (§4.4), reflect (§4.5), clamps ...
if alvo é mob e dam>=1:  dam += atacante.ForceMobDamage                        :1470
if dam >= MAX_DAMAGE:    dam = MAX_DAMAGE                                      :1473
```

### 4.4. Acerto / esquiva / parry — `_MSG_Attack.cpp:1415-1440`

```text
attackdex = ... (+500 se Rsv & 0x40)
parryretn = GetParryRate(alvo.MOB, alvo.Parry, attackdex, atacante.Rsv)
if skill ∈ {79,22}:  parryretn = 30*parryretn/100      # certas skills reduzem parry
rd = rand() % 1000 + 1
if rd < parryretn:                # ESQUIVA/PARRY
    dam = -3                      # código de "miss/block"
    if (alvo.Rsv & 0x200) and rd < 100:  dam = -4
```

> Esquiva é um roll em **mil** (`rand()%1000+1`) contra `parryretn` (de `GetParryRate`). `dam<0` é o
> sinal de "errou/bloqueou" propagado ao cliente.

### 4.5. Reflect / absorção (PvP) — `_MSG_Attack.cpp:1496-1508`

```text
if alvo é player e dam>0:
    dam -= alvo.ReflectDamage                 # reflect plano; min 1
    dam -= dam/100 * alvo.ReflectPvP          # reflect percentual; min 1
```
Efeitos de status no golpe: `RSV_FROST` (50% → affect 36) e `RSV_DRAIN` (50% → affect 40) com
`Special[1]` (`:1511+`).

### 4.6. Conversão atributo→dano (no `BASE_GetCurrentScore`, `Basedef.cpp:3014+`)

`CurrentScore.Damage` parte do equip e recebe `EF_DAMAGE` (`:3028`); o **balanceamento por classe**
soma `Dex*kd + Str*ks` conforme a arma. Ex.: TK (`Class==0`) com skill "Confiança" (`:3259+`):

| Arma (`nUnique`) | Fórmula de bônus de dano |
|------------------|--------------------------|
| 43 Garra | `Dex*0.38 + Str*0.42` |
| 42 Arco | `Dex*0.55 + Str*0.60` |
| 46 Hermai | `Dex*0.36 + Str*0.40` |
| 48 Espada 2 mãos | `Dex*0.56 + Str*0.60` |

`Critical = BASE_GetMobAbility(EF_CRITICAL)/4` (`:3209`). Cada classe tem seu bloco de coeficientes
(continuar lendo `Basedef.cpp:3245+` por classe) — **tabelar todos na implementação**.

> **Ainda UNVERIFIED (menor):** os coeficientes Dex/Str de **todas** as classes/armas (só TK
> exemplificado aqui) e a árvore completa de `BASE_GetDoubleCritical`/`GetParryRate`. O esqueleto e as
> fórmulas-núcleo (§4.1–4.5) estão fechados; complementar com golden cases (Fase 8) para validar.

---

## 5. Skills / efeitos

- Modelo de efeito de item/skill: `ItemEffect.h` (constantes `EF_*`) + `STRUCT_SPELL`
  (`Basedef.h:1110+`, ver Fase 2 §3.2). Cada skill tem `Delay`, `Range`, `ManaSpent`,
  `Affect*`, `Tick*`, `Instance*`.
- **SkillDelay /4 no cliente:** o `ClientPatch` divide o `Delay` tabelado por 4 em 104 entradas
  (`ClientPatch_v7662/Hook.cpp:230-231`). Efeito prático: cooldowns de skill 4× mais rápidos do que
  a `SkillData.csv` indica. O **servidor** valida `LastAttackTick`/anti-flood (`CUser`), então a
  stack nova deve usar o mesmo delay efetivo (Delay/4) para não rejeitar ações legítimas do cliente.
- Efeitos de affect persistidos: `STRUCT_AFFECT[MAX_AFFECT=32]` com `Time` (expiração).

### 5.1. Soul permanente do Mortal — Kibita

O terceiro ramo do `case KIBITA` (`_MSG_Quest.cpp:2518-2558`) é uma progressão permanente, não um
cast nem o campo elemental `MobExtra.Soul`. O parâmetro `confirm` é ignorado.

```text
requer ClassMaster == MORTAL
requer CurrentScore.Level >= 369
requer LearnedSkill bit 30 ainda zerado
pedra_por_classe = [5334 Água, 5336 Sol, 5335 Terra, 5337 Vento]

limpa integralmente o primeiro slot com pedra_por_classe[MOB.Class]
LearnedSkill |= 1 << 30
Equip[15] = 3194 se Clan==7, 3195 se Clan==8, senão 3196
CharLogOut(); SendArchEffect(slot do personagem)
```

O `memset` do legado substitui qualquer capa anterior e apaga todos os efeitos do item consumido;
não é consumo de uma unidade de stack. O logout é parte da transição: salva antes de o cliente
reler a skill e a capa na seleção.

Esse bit 30 também não é o gate de `Limite_da_Alma` (skill 102 usa `102%24 == 6`). O affect 29 lê
`MobExtra.Soul`, que permanece inalterado neste fluxo.

---

## 6. Eventos e timers

Orquestrados por `ProcessSecMinTimer.cpp` (tick de segundo/minuto) e classes dedicadas:

| Evento | Fonte | Estado/constantes |
|--------|-------|-------------------|
| Guild War / Torre | `CWarTower.*` | `GTorreHour`, `TowerCount`, `TowerStage`, `GuildTower` (`Server.h:73-75`) |
| Castle / Zakum | `CCastleZakum.*` | `KeyDrop` (`MobKilled.cpp:2870`), `Settings/CastleQuest.txt` |
| RvR | regiões `Regions.txt` (`RvR`) | `BrState` |
| Quests diárias | `_MSG_Quest`, `QuestDiaria.txt`, `Settings/QuestsRate.txt` | `STRUCT_QUEST` por char |
| Eventos de exp/drop | flags `DOUBLEMODE`, `NewbieEventServer`, `evOn` | globais (viram feature-flags) |

`CastleQuest.txt` e `QuestsRate.txt` (em `Settings/`) parametrizam recompensas/taxas — carregar
como config (Fase 7).

### 6.1. O timer "de minuto" é de 12 segundos

Apesar do nome, `TIMER_MIN` dispara a cada **12000 ms** (`Server.cpp:4087`, ao lado do `TIMER_SEC`
de 500 ms), e o `ProcessMinTimer` conta as próprias passagens (`ProcessSecMinTimer.cpp:2523`).
Todo contador do legado que anda nesse timer está em unidades de 12 s. O caso que mais pesa é o
`MinuteGenerate` do `NPCGener.txt`: um bloco com 10 repõe o grupo a cada 120 s, não a cada 10
minutos (`:2723-2733`).

No rewrite a unidade vive em `handler/mintimer.go` (`minTimerTicks` = 12 tiques de 1 s) e em
`spawnrate.MinTimerPass` (12 s), e o painel mostra o tempo de relógio, não o número cru.
Fidelidade restaurada em 11/09/2026. Até então o gerador e o clima rodavam num "minuto" de 60 s, e
todo bloco com timer repunha **5x mais devagar** que o legado: Combatente em 10 min em vez de 2,
Morlock em 50 em vez de 10.

**Auditoria pendente** — rotinas que ainda andam num minuto de relógio (`minutoTicks` = 60) e
podem ter o mesmo defeito. A pergunta que decide cada uma: *ela conta passagens do timer do legado
ou olha a hora do relógio?*

| Rotina | Onde | O que o legado faz |
|---|---|---|
| Castelo Zakum | `castle.go` `tickCastle` → `events.castle.TickMinute` | `CCastleZakum::ProcessMinTimer`, chamado a cada passagem (`ProcessSecMinTimer.cpp:2620`) |
| Salas do trono do reino | `kingdom.go` `tickKingdomRvR` | limpeza em dois passos do `ProcessMinTimer` (`:2621-2643`) |
| Guerra de torre | `towerwar.go` `tickTowerWar` | `Step` olha a hora do relógio; provavelmente só a frequência da consulta muda |

**A lição que vale uma varredura.** Duas vezes o porte deu a um argumento do legado o significado que
o NOME sugeria, e as duas vezes o efeito foi grande: o `MinuteGenerate`, que nunca foi minuto (a
unidade é a passagem de 12 s), e o segundo argumento do `AddCrackError(conn, val, Type)`, que é um
peso somado e foi lido como "grupo", com um limite de 10 inventado por cima do 2.000.000.000 do
legado. Fica pendente uma varredura própria procurando outros parâmetros do legado que o rewrite
leu pelo nome e não pelo uso.

---

## 7. Constantes mágicas a preservar (resumo)

| Constante | Valor | Onde | Papel |
|-----------|------:|------|-------|
| Corte de exp | `6*exp/10` | `MobKilled.cpp:529` | -40% fixo |
| Newbie/evento exp | `±15%`, `+25%` | `:537,546-549` | ajustes de evento |
| Roll de combine | `rand()%115`, `>=100⇒-15` | `_MSG_CombineItem.cpp:80-82` | distribuição de sucesso |
| Joia base | `sIndex - 2441` | `:92` | offset do tipo de joia |
| Sanc pós-combine | `7` | `:100` | refino do item resultante |
| Teto de gold/kill | `2000` | `MobKilled.cpp:2713` | clamp |
| Teto de gold total | `2_000_000_000` | `:2715` | overflow guard |
| Cooldown refino | `1000ms` (DESATIVADO) | `_MSG_UseItem.cpp:214` | anti-spam comentado |
| SkillDelay client | `/4` | `Hook.cpp:230` | cooldown efetivo |
| Bônus de dano por sanc | `(Grade==6?80:40)*isanc` | `CMob.cpp:867` | force damage |
| Dano: mitigação AC | `dam - ac/2` | `Basedef.cpp:1267` | melee/skill |
| Dano: fator melee | `rand()%(12-min(combat/2,7)) + min(combat/2,7)+99` | `Basedef.cpp:1273` | variância por maestria |
| Dano: fator skill | `rand()%(21-min(combat,15)) + min(combat,15)+90` | `Basedef.cpp:1493` | variância skill |
| AC de player em PvP | `Ac *= 3` | `_MSG_Attack.cpp:455` | players resistem 3× |
| Crítico parcial | `×1.3–1.4` (player) / `×1.5–1.6` (mob) | `_MSG_Attack.cpp:447-449` | `DoubleCritical&2` |
| Crítico total | `dam *= 2` | `_MSG_Attack.cpp:480` | `DoubleCritical&1` |
| Esquiva/parry | `rand()%1000+1 < parryretn` → `dam=-3` | `_MSG_Attack.cpp:1423-1428` | miss/block |
| Dano mínimo | `1` | `Basedef.cpp:1294/1515` | piso |

> **Status da Fase 4: COMPLETO (núcleo).** EXP/party, drop (gold/comum/evento, com valores reais de
> `g_pDropRate`), refino/combine (rolls + tabelas), e **combate** (fórmulas reais de
> `BASE_GetDamage`/`BASE_GetSkillDamage` + pipeline do `_MSG_Attack`, acerto/parry, reflect) estão
> documentados com fórmula + file:line. UNVERIFIED **menor** (a complementar por golden cases, Fase 8):
> os coeficientes Dex/Str por **classe×arma** (só TK exemplificado), a árvore de
> `BASE_GetDoubleCritical`/`GetParryRate`, e o parsing fino de `NPCGener`/AI de mob.
