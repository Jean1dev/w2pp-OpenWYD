# GamePatch.dll

Pinta as linhas do tooltip de montaria no cliente WYD 7662: **dano em verde,
magia em azul, absorção em vermelho**. Os outros itens não mudam.

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
`UI\strdef.bin` e este DLL, prontos para o launcher.

## Segurança

Antes de gravar qualquer coisa na memória, o DLL confere os bytes da função do
tooltip. Se não forem os da build 7662, ele não faz nada: tooltip sem cor, em
vez de jogo caindo.
