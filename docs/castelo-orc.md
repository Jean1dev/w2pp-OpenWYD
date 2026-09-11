# Quest do Castelo Orc

Uma corrida pelo castelo orc de Erion (x 2438–2553, y 2053–2158), feita para
Mortais de 320 a 400 com set e armas de +6 a +9. Ela paga em gold e saque, e
**nunca em experiência**. É regra nova, não do legado. O design, a simulação de
party e as decisões ficam no artefato "Atlas de Quests W2PP".

## Estado

✅ Os oito monstros da quest (templates `COrc_*`) <br/>
✅ O saque e a chave na Mesa de Drops (migração `0053_castelo_orc_drops`) <br/>
✅ 0 XP para os monstros da quest <br/>
✅ Amuleto com add sorteado <br/>
✅ A corrida: a Chave Portão Orc Sul abre o castelo (no portão ou com o Xamã), um
grupo por vez, 15 min <br/>
✅ O Portão Orc Sul no arco: sempre trancado; a chave leva o grupo para dentro <br/>
⏳ Prêmio de conclusão e trava de nível/evolução <br/>

## A chave

É a **Chave Portão Orc Sul (465)**, a primeira das quatro chaves do castelo no
legado, item que o cliente já conhece. A migração tira a chave de todo monstro
(`*` a 0%). Ela volta só em três lugares:

| Onde | Como | Meta |
|---|---|---|
| Quest 256 das Hidras (nível 265–320) | na entrada paga, sorteada (`casteloOrcKeyOnEntry`) | 1 a cada 4 entradas |
| Quest 256 dos Elfos (nível 320–350) | na entrada paga, sorteada | 1 a cada 3 entradas |
| Deserto | drop dos monstros que só nascem lá (Adamant_Tauron, Aeon_Tauron, Aranha_Inferno, Arqueiro_Tauron, Cav._Lugefer, Ladrao_Tauron, Lugefer, Manticora, Taron_Assassino, Treant, Verme_, Tauron_Agmo, Verme_Agmo), 0,1% cada | 1 a cada 1.000 abates |

- **Por que o `*` a 0%:** hoje o Guarda_Orc_ do castelo aberto dá a 465 sempre (slot
  56), e ele renasce a cada 6 min.
- **Templates escolhidos:** só os que nascem apenas nesses lugares. O Tauron comum
  tem 1.648 dos seus 1.826 fora do deserto (Monster City e outros) e ficou de fora.
- **Nas arenas os monstros não dão a chave.** As arenas não têm relógio e renascem
  sozinhas: a chave no abate premiaria quem acampa lá dentro. Ela sai na entrada,
  quando o ticket (Mana do Batedor nas Hidras, Emblema do Guarda nos Elfos) é gasto
  no NPC ou usado da bolsa.
- **O Mestre Grifo leva de graça para as mesmas arenas e não dá chave**: senão,
  entrar e sair farmaria chaves.
- O sorteio usa o gerador dos eventos, não o dos drops, para não mexer na ordem
  que os testes de drop e refino fixam.
- **O Sentinela da quest** carrega a 466 (Portão Orc Leste), e não a 465: senão cada
  corrida pagaria a entrada da seguinte.

## A corrida (`handler/castelo_orc_run.go`)

- **Quem abre:** o líder do grupo (ou quem está sozinho), usando a chave no
  **Portão Orc Sul** do arco ou entregando ao **Xamã Orc** (template `COrc_Xama`,
  Merchant 100, grau 40), de pé no mesmo arco (2484,2129). A chave é consumida.
- **O Xamã nasce pelo código**, não pelo NPCGener. NPC com Merchant no NPCGener
  vira do overlay de NPCs quando `W2PP_NPC_EDITING` está ligado e só apareceria
  depois de um `dbserver import-npcs`. Assim ele fica de pé nos dois modos.
- **Um grupo por vez**, no servidor inteiro. Com o castelo ocupado, o Xamã diz
  quantos minutos faltam.
- **Na abertura:**
  - os orcs do castelo aberto (blocos 373–394 e 402–497) saem e não renascem até o
    fim;
  - quem não é do grupo e está dentro vai para a chegada do `/erion`;
  - nascem o boss, os 4 seguidores, os 3 guardiões e 60 de tropa;
  - o portão continua fechado;
  - o grupo cai sob o arco do Portão Sul (2494,2128), cada um numa casa livre,
    com o contador de 15 min. Até 11/09 caía em (2446,2134), que fica fora da
    muralha oeste.
- **Durante:**
  - quem não é do grupo não entra no castelo andando;
  - o Pedido de Caça não leva estranhos para dentro (os warps 2 e 3 caem lá);
  - os seguidores voltam a cada 30 s, para a última sala render saque.
- **Fim:** aos 15 min; 2 min depois que o Grão-Lorde cai (o tempo de saque); ou 1
  min depois que ninguém do grupo está mais no castelo. Os monstros da quest somem,
  quem estiver dentro volta para o `/erion` e os orcs do castelo aberto renascem
  pelos próprios timers.
- **Reinício do servidor** encerra a corrida (nada é persistido), como na Água e
  na Carta.

## O portão (`handler/castelo_orc_gate.go`)

O **Portão Orc Sul** (item 462, `EF_KEYID 5`, o mesmo da chave 465) está no
`InitItem.csv` do legado em (2487,2129), rotação 1. O servidor carregava os
portões do `InitItem`, mas nunca os mandava ao cliente. Agora manda este, e só
este: ligar todos mudaria portas pelo mapa inteiro sem ninguém ter pedido.

- **Nunca abre.** Aberto, deixaria um segundo grupo entrar atrás do primeiro
  (regra da equipe, 11/09). A chave no portão, ou com o Xamã, começa a corrida e
  teleporta o grupo para dentro; o portão fica trancado o tempo todo.
- O pacote é o do legado: `MSG_CreateItem` quando o portão entra na visão,
  `MSG_DecayItem` quando sai. Confirmado em jogo em 11/09 que o cliente desenha o
  portão.
- Clicar no portão com a chave é o mesmo que entregar a chave ao Xamã: mesmas
  regras, e a resposta sai no painel de mensagens.
- Quem impede a passagem pelo portão fechado é o cliente, que eleva o chão
  embaixo dele. O servidor não confere altura no movimento do jogador.
  **A confirmar em jogo** que o portão fechado bloqueia quem tenta atravessar.
- Os outros portões do castelo (463, 464, 468) não são mandados ao cliente: as
  passagens deles ficam abertas.

**O Ferreiro Penado** em (2515,2158), no cemitério ao sul, é um NPC do legado
(bloco 3439, Merchant 100, grau 26). O legado não tem quest para o grau 26:
clicar nele não faz nada, lá e aqui. Ele fica dentro da área da corrida, então
quem não é do grupo não chega até ele enquanto uma corrida acontece.
- **Contador: precisa do GamePatch.** O servidor manda o mesmo `MsgStartTime`
  da Água e do Pesadelo (900 s na abertura, 120 quando o boss cai, reenvio a cada
  minuto), mas o WYD.exe 7662 só desenha esse contador numa lista fixa de 15
  campos do mapa (blocos de 128×128; laço em 0x47DAA4) e o castelo fica no campo
  (19,16), fora dela. O `client/gamepatch/timerfields.cpp` acrescenta o campo
  (19,16); o cliente sem esse GamePatch.dll continua sem contador. O tempo também
  vai em texto a cada minuto ("Castelo Orc: N min"), para todo cliente.

## Os monstros

| Template | Nome no jogo | Nv | HP | Defesa | Dano | Resist. | Bloco |
|---|---|---|---|---|---|---|---|
| `COrc_GraoLorde` | Grão-Lorde Orc | 350 | 3.000.000 | 3.000 | 2.020 | 25 | 6099 |
| `COrc_Guarda` | Guarda do Lorde | 320 | 150.000 | 2.200 | 1.520 | 15 | 6100 (grupo de 4) |
| `COrc_Sentinela` | Sentinela Orc | 330 | 450.000 | 2.400 | 1.620 | 20 | 6101 · chave 466 |
| `COrc_Capitao` | Capitão Orc | 330 | 450.000 | 2.400 | 1.620 | 20 | 6102 · chave 467 |
| `COrc_Chefe` | Chefe Orc | 330 | 450.000 | 2.400 | 1.620 | 20 | 6103 · chave 469 |
| `COrc_Cavaleiro` | Cavaleiro Orc | 300 | 18.000 | 1.800 | 1.220 | 10 | 6104, 6107, 6110, 6113 |
| `COrc_Arqueiro` | Arqueiro Orc | 300 | 18.000 | 1.800 | 1.220 | 10 | 6105, 6108, 6111, 6114 |
| `COrc_MeioOrc` | Meio Orc | 300 | 18.000 | 1.800 | 1.220 | 10 | 6106, 6109, 6112, 6115 |

**Onde a tropa nasce:** 12 blocos de 5, raio 2, cada um num ponto de spawn do
miolo do castelo do legado (blocos 476–496 do `NPCGener`):

| Tropa | Pontos (bloco do legado) |
|---|---|
| Cavaleiro | (2493,2116) 477 · (2523,2112) 484 · (2514,2141) 492 · (2536,2143) 494 |
| Arqueiro | (2492,2134) 479 · (2513,2127) 486 · (2509,2143) 491 · (2524,2140) 495 |
| Meio Orc | (2512,2124) 485 · (2526,2121) 488 · (2531,2112) 489 · (2522,2153) 493 |

Até 11/09 eram 3 blocos de 20 com raio 8 em pontos escolhidos à mão, e parte da
tropa nascia do lado de fora das paredes. Os pontos 476 e 487 do legado ficaram
de fora por terem parede a 2 casas no mapa de altura. Boss e guardiões já
estavam nos pontos do legado: Orc_Lord (483), Orc_Guard_ (480), Guarda_Orc_ (490)
e Chefe_Orc_ (497).

O boss usa o corpo do Troll_Martelo (rosto 213), uma Espada Bastarda +11 em cada
mão e monta um Lobo. O HP dos bosses (Grão-Lorde e guardiões) caiu pela metade
depois do primeiro teste em jogo.

**Refino no visual do mob:** de +10 para cima o EF_SANC não é o número. O cliente
lê valores abaixo de 230 como o resto por 10 (um 11 aparece como +1); +10 é
230–233, +11 é 234–237, e assim de 4 em 4 (`protocol/visual.go`). O boss usa 234. A montaria e o brilho da arma são só aparência: o servidor não soma o
equipamento de mob no score.

**Por que esses números:**
- O golpe de mob no jogador faz `Dano − Defesa/2`, com sorteio de 99–110% e ×0,75
  montado, contra o HP do jogador em dobro, como no legado (`65346fe8`). Com set
  de +6 a +9, um Mortal 320 tem 1.150–1.600 de defesa. Com estes números, na
  simulação de party (TK, FM, BM e HT montados), cada golpe tira do TK: boss
  23–25% do HP, guardião 16–18%, seguidor 14–16%, tropa 8–11%. FM e BM levam
  quase o dobro disso em porcentagem.
- Até 11/09/2026 a defesa entrava ×3 também contra mob, e o Dano era 2.300–2.700.
  Na regra de hoje, aqueles números tiravam 35–73% do HP por golpe do boss.
- As resistências são positivas de propósito. O servidor lê a resistência como
  número sem sinal, e um −20 vira 100, o que corta pela metade o dano de skill.
- O 0 XP está no código (`handler/castelo_orc.go`) e não em Clan 4. Clan 4 é o clã
  das evocações: divide por 4 todo golpe que o monstro leva e o esconde da
  Tempestade de Raios. O Exp do template também é 0. O boot avisa ("unbalanced
  Exp"), e esse aviso é esperado para estes oito. **Não rode o `cmd/exptool` neles.**

## O saque

Os templates não têm drop próprio, só a chave de portão dos guardiões (slot 56,
cai sempre). O resto é da Mesa de Drops e se ajusta em `/drops` no painel.

As chances saem da meta por entrada do design. A conta supõe 60 de tropa, 3
guardiões, o boss e ~44 seguidores (4 no começo, mais 4 a cada 30 s em uns 5 min
na última sala).

| Meta por entrada | Quem dropa | Chance |
|---|---|---|
| 10 Moedas de Prata (1Mi) 4026 | tropa e guardiões | 16% |
| 10 Repletion: Classe C 4018 + Classe D 4019 | tropa e guardiões | 9,5% + 6,5% |
| 15 Âmagos de Lobo 2392 | tropa e guardiões | 24% |
| 12 Âmagos de Dragão Menor 2393 | tropa e guardiões · seguidores | 10% · 13% |
| 10 Âmagos de Dente de Sabre 2395 | tropa e guardiões · seguidores | 8% · 11% |
| 7 Âmagos de Cavalo s/ Sela: N 2396 + B 2401 | seguidores | 9% + 7% |
| ~0,6 Âmago de Urso 2394 | tropa e guardiões | 1% |
| ~32 Restos: Ori 419 + Lac 420 | tropa e guardiões · seguidores | 20% + 10% em cada |
| 7 Poeiras: Ori 412 + Lac 413 | tropa e guardiões · boss | 7,5% + 2,8% · 30% + 25% |
| ~1 Ovo de Cavalo s/ Sela: N 2306 + B 2311 | seguidores · boss | 1% + 0,5% · 25% + 12,5% |
| 0,6 Amuleto de Prata 551–554 | só o boss | 15% cada |

- **Moedas, Repletion e Âmago de Lobo** caem só da tropa e dos guardiões, que são
  sempre 63. Assim a meta não depende de quanto tempo o grupo fica na última sala.
- **Cavalo s/ Sela não cai**: só os ovos.

**Bolsa cheia perde o item.** Uma entrada rende uns 106 itens, uns 26 para cada
um de 4 jogadores. O drop de mob ocupa sempre um espaço novo, sem juntar na pilha
que já está na bolsa (`putMobDrop`), e com a bolsa cheia o item se perde.

**O que o servidor acrescenta** (`casteloOrcFinish`, só em monstro da quest):
- O **amuleto** cai +0 e com **um** add sorteado: 4–10 de magia, 10–20 de dano,
  1% ou 2% de crítico ou 50–70 de HP.
  - O crítico é gravado em décimos de ponto percentual: o tooltip mostra o valor
    ÷ 10, então 1% é 10 e 2% é 20, como no bônus de drop do legado, que dá
    crítico de 10 em 10. Até 11/09 o sorteio era 1–2 e o amuleto mostrava
    "Crítico: 0.2%"; os que já caíram assim ficam como estão.
  - Crítico e magia somam no equipamento inteiro e são divididos por 4 no
    personagem: 10 ou 20 de crítico viram 2 ou 5 pontos.
- O **anel** (501–506), se alguém der uma regra a ele, cai +0 com 1–3 de magia,
  5–7 de dano ou 10–20 de mana.

## Como testar

A corrida inteira, com conta de GM:

```
/gm item 465                    a Chave Portão Orc Sul na bolsa
vá até o arco do Portão Sul do castelo (2487,2129): o portão deve estar fechado
clique no portão como líder do grupo (ou no Xamã Orc, ao lado): o grupo vai
para dentro e o portão continua fechado
```

Um monstro solto, perto do castelo e sem corrida:

```
/gm criar COrc_GraoLorde        um boss na sua frente (não renasce)
/gm gerar 6100 aqui             o grupo de 4 seguidores
/gm gerar 6104 aqui             um grupo de tropa (6105, 6106: as outras)
/gm criar COrc_Sentinela        um guardião (Capitão e Chefe: os outros)
```

GM (moderador para cima) não é barrado nem varrido do castelo durante a corrida,
como no castelo do Zakum.

O painel mostra os templates novos e as regras depois do deploy. A Mesa relê as
regras a cada ~15 s.
