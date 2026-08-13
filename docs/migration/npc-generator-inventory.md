# Inventário dos generators com `Merchant != 0`

Fonte reproduzível: `dbserver import-npcs -content ./Release` usa o mesmo parser e
codec do seed de produção. O catálogo atual contém **548 blocos** em 6099 slots;
produção possuía apenas 84 antes da migration do catálogo integral.

O byte `Merchant` é sobrecarregado pelo legado: além de lojas, identifica quests,
montarias e atores de eventos. Por isso cada registro preserva a receita completa
do `NPCGener.txt` e o `generator_index`, em vez de virar um spawn simples.

Principais grupos que estavam integral ou parcialmente ausentes:

| Merchant | Blocos | Exemplos | Classificação |
|---:|---:|---|---|
| 1 | 88 | lojas regionais e equipamentos | loja |
| 12 | 4 | Jeffi, Torre_Real | quest pendente |
| 16 | 64 | montarias e criaturas associadas | montaria/evento |
| 23/24 | 2 | Mestre_Grifo, Alquimista_Odin | quest/combine |
| 32 | 65 | guardas e cavaleiros | evento/combate |
| 64 | 178 | Zakum, arqueiros, lanceiros | evento/combate |
| 74 | 2 | Kibita, Lindy | quest/combine parcial |
| 78 | 2 | BlueOracle, RedOracle | quest |
| 96 | 43 | guardas, soldados e Lanceiro | evento/combate |
| 100 | 26 | cadeia Quest 256 e tutoriais | quest parcial |
| 111 | 2 | Rei_Glantuar, Rei_Harabard | quest |

Casos não classificados com paridade comprovada permanecem `UNVERIFIED` e são
acompanhados pela issue guarda-chuva. A presença no mundo não implica que a
interação específica já esteja implementada.
