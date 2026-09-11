# Ícones de item

A arte dos ícones que criamos, no formato que o cliente usa: BMP 24 bits, sem
compressão. O cliente desenha ícone em célula de **35×35**; uma arte de 32×32
entra centrada, e o preto puro é o fundo (o cliente o trata como transparente),
então o contorno do desenho não pode ser preto puro.

| Arquivo | Item | Onde foi gravado no cliente |
|---|---|---|
| `chave-do-rei-orc.bmp` | 465 Chave do Rei Orc | célula 940 (`UI/itemicon10.wyt`), apontada por `itemicon.bin[465] = 941` |
| `chave-do-inferno.bmp` | 3222 Chave do Inferno | célula 941 (`UI/itemicon10.wyt`), apontada por `itemicon.bin[3222] = 942` |

O caminho até o cliente, hoje à mão (o gerador do launcher ainda não leva estes
arquivos):

1. gravar a arte na célula livre do atlas (`UI/itemiconNN.wyt`: TGA sem
   compressão, 350×350, 32 bits, origem embaixo; célula `n` em
   `x=(n%10)*35, y=(n/10)*35`);
2. apontar o item para a célula em `itemicon.bin` (vetor de int32 por índice de
   item; o valor é a célula + 1, e 0 é "sem ícone");
3. o nome sai do `ItemList.bin`, que o `webserver/internal/clientitemlist`
   escreve a partir do `Release/Common/ItemList.csv`;
4. a descrição do tooltip sai do `itemhelp.dat`: blocos de 10 linhas, a
   primeira com o índice do item e as outras nove com `AARRGGBB texto`, em
   Latin-1 e com `_` no lugar do espaço.

"Livre" é **célula que nenhum item aponta**, não célula em branco: os onze atlas
do cliente vêm pintados até a última célula, e a `itemicon.bin` para de apontar
bem antes do fim. A arte além da última apontada é órfã — o cliente nunca a
desenha — e é ela que se sobrescreve. Procurar célula em branco não acha
nenhuma, e pegar uma célula abaixo da última apontada apaga o ícone de algum
item.

Em 12/09/2026 a `itemicon.bin` apontava até a célula 939; a 940 foi para a Chave
do Rei Orc e a 941 para a Chave do Inferno, então a próxima livre é a 942.

O comando `webserver/cmd/itemnovocliente` faz os quatro passos de uma vez, numa
cópia do cliente (a pasta original é só lida):

	itemnovocliente -cliente "<pasta do cliente>" -item 3222 \
	                -icone client/icones/chave-do-inferno.bmp \
	                -catalogo Release/Common/ItemList.csv \
	                -linha "[Item_Premium]:premium" -linha "" \
	                -linha "Um lugar infernal, mas com grandes recompensas." \
	                -linha "Só venha se tiver coragem, NOOB!:vermelho"

Ele escolhe a célula sozinho pela regra acima e imprime qual usou — a tabela
deste arquivo é para quem for ler o cliente depois, não uma entrada do comando.
