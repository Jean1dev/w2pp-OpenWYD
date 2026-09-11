# Quest do Castelo Orc

Uma corrida pelo castelo orc de Erion (x 2438–2553, y 2053–2158), feita para
Mortais de 320 a 400 com set e armas de +6 a +9. Ela paga em gold e saque, e
**nunca em experiência**. É regra nova, não do legado. O design, a simulação de
party e as decisões ficam no artefato "Atlas de Quests W2PP".

## Estado

✅ Os oito monstros da quest (templates `COrc_*`) <br/>
✅ O saque na Mesa de Drops (migração `0051_castelo_orc_drops`) <br/>
✅ 0 XP para os monstros da quest <br/>
✅ Amuleto com add sorteado e montaria que cai viva <br/>
⏳ O motor da corrida: NPC de entrada, salas só da party, relógio de 14 min, boss e
seguidores nascendo na abertura, seguidores renascendo, portões que voltam a
trancar, Pedido de Caça bloqueado e limpeza <br/>

Até o motor existir, os monstros não nascem sozinhos: os blocos são "de evento"
(`world.IsCasteloOrcGenerator`). Um GM os levanta pelos comandos abaixo.

## Os monstros

| Template | Nome no jogo | Nv | HP | Defesa | Dano | Resist. | Bloco |
|---|---|---|---|---|---|---|---|
| `COrc_GraoLorde` | Grão-Lorde Orc | 350 | 6.000.000 | 3.000 | 2.700 | 25 | 6099 |
| `COrc_Guarda` | Guarda do Lorde | 320 | 150.000 | 2.200 | 2.450 | 15 | 6100 (grupo de 4) |
| `COrc_Sentinela` | Sentinela Orc | 330 | 900.000 | 2.400 | 2.500 | 20 | 6101 · chave 465 |
| `COrc_Capitao` | Capitão Orc | 330 | 900.000 | 2.400 | 2.500 | 20 | 6102 · chave 467 |
| `COrc_Chefe` | Chefe Orc | 330 | 900.000 | 2.400 | 2.500 | 20 | 6103 · chave 469 |
| `COrc_Cavaleiro` | Cavaleiro Orc | 300 | 18.000 | 1.800 | 2.300 | 10 | 6104 (grupos de 4–5) |
| `COrc_Arqueiro` | Arqueiro Orc | 300 | 18.000 | 1.800 | 2.300 | 10 | 6105 |
| `COrc_MeioOrc` | Meio Orc | 300 | 18.000 | 1.800 | 2.300 | 10 | 6106 |

O boss usa o corpo do Troll_Martelo (rosto 213), a Espada Bastarda +11 e monta um
Lobo. A montaria e o brilho da arma são só aparência: o servidor não soma o
equipamento de mob no score.

**Por que esses números:**
- O golpe de mob no jogador faz `Dano − 1,5 × Defesa` (a defesa entra ×3; ver a
  memória "mob bate com defesa ×3"). Com set de +6 a +9, um Mortal 320 tem
  1.300–1.600 de defesa. Por isso o Dano fica entre 2.300 e 2.700: abaixo disso,
  todo golpe tira 1.
- As resistências são positivas de propósito. O servidor lê a resistência como
  número sem sinal, e um −20 vira 100, o que corta pela metade o dano de skill.
- O 0 XP está no código (`handler/castelo_orc.go`) e não em Clan 4. Clan 4 é o clã
  das evocações: divide por 4 todo golpe que o monstro leva e o esconde da
  Tempestade de Raios. O Exp do template também é 0. O boot avisa ("unbalanced
  Exp"), e esse aviso é esperado para estes oito. **Não rode o `cmd/exptool` neles.**

## O saque

Os templates não têm drop próprio, só a chave de portão dos guardiões (slot 56,
cai sempre). O resto é da Mesa de Drops e se ajusta em `/drops` no painel.

| Quem | Item | Chance |
|---|---|---|
| Tropa e guardiões | Moeda de Prata (1Mi) 4026 | 5% |
| | Classe C 4018 · Classe D 4019 | 1% · 0,5% |
| | Âmago de Lobo 2392 · Urso 2394 · Dragão Menor 2393 | 1% cada |
| | Âmago de Dente de Sabre 2395 | 0,5% |
| | Resto de Ori 419 · Resto de Lac 420 | 20% · 10% |
| | Poeira de Ori 412 · Poeira de Lac 413 | 5% · 2% |
| Guarda do Lorde | Resto de Ori · Resto de Lac | 20% · 10% |
| | Âmago de Dragão Menor · Dente de Sabre | 3% · 2% |
| | Cavalo s/ Sela N 2366 · B 2371 | 0,5% · 0,25% |
| | Ovo de Cavalo s/ Sela N 2306 · B 2311 | 1% · 0,5% |
| Grão-Lorde | Amuleto de Prata 551–554 | 15% cada |
| | Poeira de Ori · Poeira de Lac | 30% · 15% |
| | Ovo de Cavalo s/ Sela N · B | 25% · 12,5% |

As chances de Moeda, Restos e Poeiras da tropa vieram do design. As outras são
proposta para o primeiro teste.

**O que o servidor acrescenta** (`casteloOrcFinish`, só em monstro da quest):
- O **amuleto** cai +0 e com **um** add sorteado: 4–10 de magia, 10–20 de dano,
  1–2 de crítico ou 50–70 de HP. Crítico e magia somam no equipamento inteiro e são
  divididos por 4 no personagem: 1–2 de crítico sozinho não aparece.
- O **anel** (501–506), se alguém der uma regra a ele, cai +0 com 1–3 de magia,
  5–7 de dano ou 10–20 de mana.
- A **montaria** cai viva, como a do Baú da Montaria (HP 26.652, vitalidade
  sorteada, ração 100). Pela Mesa sozinha, ela chegaria sem vida e não daria nada.

## Como testar

Em jogo, com conta de GM, perto do castelo:

```
/gm criar COrc_GraoLorde        um boss na sua frente (não renasce)
/gm gerar 6100 aqui             o grupo de 4 seguidores
/gm gerar 6104 aqui             um grupo de tropa (6105, 6106: as outras)
/gm criar COrc_Sentinela        um guardião (6102, 6103: os outros)
```

O painel mostra os templates novos e as regras depois do deploy. A Mesa relê as
regras a cada ~15 s.
