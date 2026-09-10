# GamePatch.dll

O visual do tooltip de montaria no cliente WYD 7662. Os outros itens não mudam.

- **Paleta das linhas:** título `#E0F7FF`, level necessário `#6E7C8C`, dano
  `#FF9E64`, imunidade e evasão `#FDE68A`, ataque mágico `#A78BFA`, absorção
  PvP/PvE `#5EEAD4`, vitalidade/HP/level/ração `#CBD5E1`, preço `#7DD3FC`.
  Aviso vermelho do cliente (level insuficiente etc.) continua vermelho.
- **Fundo** rgb(12,18,25) com **cantos redondos** e **borda de 5 px** na cor da
  raridade, com um reflexo que dá a volta a cada 2,4 s.
- **Linha "Montaria nível X"** no fim do tooltip, na cor da raridade.

## Raridade: `GamePatch.txt`

Na pasta do cliente, uma montaria por linha (texto em Windows-1252, `#` comenta):

```
Andaluz B = Divina, dourado
```

Nome como aparece no título do tooltip; depois do `=`, o nome do nível e a
família de cor da borda: `cinza`, `verde`, `azul`, `roxo`, `dourado`, `laranja`
ou `vermelho`. Montaria fora do arquivo fica com borda cinza e sem linha de nível.

## Raridade dos equipamentos: `GamePatchItens.bin`

Armas e armaduras ganham o mesmo fundo e borda, com a linha "Item nível X". O
nível de cada item vem de `GamePatchItens.bin`, que o gerador escreve a partir
do ItemList (regra em `webserver/internal/clientrarity`):

| Nível | Borda | Itens |
|---|---|---|
| Divino | dourado | todo Ancient (grade 5–8) |
| Mítico | vermelho | Celestial (`EF_MOBTYPE 3`) |
| Lendário | laranja | (Le) e Arch (`EF_MOBTYPE 1`) |
| Épico | roxo | armadura (A); arma sem letra lv 200+ |
| Raro | azul | armadura (M); arma lv 150–199 |
| Incomum | verde | arma lv 100–149 |
| Comum | cinza | armadura (N); arma até lv 99 |

A refinação ergue o nível, nunca o baixa: +11 e +12 no mínimo Épico, +13
Lendário, +14 Mítico, +15 Divino. Os pisos vão no cabeçalho do arquivo, e o DLL
lê a refinação do item sob o mouse como `BASE_GetItemSanc`.

As cores das linhas continuam as do cliente nos equipamentos; a paleta é só
das montarias.

## Como o desenho funciona

O tooltip é o painel `0x102` da janela; o cliente o desenha como um nó de cor
sólida. O DLL troca o `Render` da classe do painel e, no lugar do retângulo do
cliente, encadeia cópias desse nó: primeiro um arredondado cheio com as cores da
borda, depois o arredondado de dentro com o fundo, por cima. Cada pedaço passa
1 px do vizinho, sempre para dentro, então não há junta que abra.

O nó tem **0x16C bytes** (construtor em `0x40BF80`). A cópia precisa ser inteira:
o desenho lê o índice de textura em `+0x160`, e uma cópia curta faz o último
pedaço ser tratado como textura 0 e não aparecer.

## Como o cliente carrega

O `WYD.exe` já tem um carregador no ponto de entrada (`0x5F3C66`) que chama
`LoadLibraryA` para `GamePatch.dll`, `Shield.dll` e `ClientPatch.dll`, nessa
ordem, antes de entrar no jogo. Nenhum dos dois primeiros existia, então basta
este arquivo estar na pasta do cliente: o exe e o `ClientPatch.dll` ficam como
estão. (O código do `ClientPatch` em `Source/Code/ClientPatch_v7662` não é
exatamente o que está compilado no cliente, por isso ele não é recompilado.)

## Como compilar

Precisa do Visual Studio 2022 Build Tools com C++:

```
client\gamepatch\build.bat
```

Sai em `client\gamepatch\out\GamePatch.dll`: 32 bits, runtime estático, sem
dependência além do `KERNEL32.dll`.

## Como publicar

Junto com os arquivos das montarias, pelo gerador:

```
go run ./webserver/cmd/montariacliente -tabela montarias-cliente.txt ^
    -cliente "<pasta do cliente original>" -gamepatch client\gamepatch\out\GamePatch.dll
```

O gerador põe na pasta de saída o `WYD.exe`, o `ItemList.bin`, o
`UI\strdef.bin` e este DLL, prontos para o launcher. O `GamePatch.txt` ainda é
escrito à mão; a raridade ainda não está no painel.

## Segurança

Antes de gravar qualquer coisa na memória, o DLL confere os bytes da função do
tooltip. Se não forem os da build 7662, ele não faz nada: tooltip sem cor, em
vez de jogo caindo.
