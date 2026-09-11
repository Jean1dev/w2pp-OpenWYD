# Ícones de item

A arte dos ícones que criamos, no formato que o cliente usa: BMP 24 bits, sem
compressão. O cliente desenha ícone em célula de **35×35**; uma arte de 32×32
entra centrada, e o preto puro é o fundo (o cliente o trata como transparente),
então o contorno do desenho não pode ser preto puro.

| Arquivo | Item | Onde foi gravado no cliente |
|---|---|---|
| `chave-do-rei-orc.bmp` | 465 Chave do Rei Orc | célula 940 (`UI/itemicon10.wyt`), apontada por `itemicon.bin[465] = 941` |

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

Células livres são fáceis de achar no fim do último atlas: em 11/09/2026 o
maior ícone usado era o 939, e 940-999 estavam vazias.
