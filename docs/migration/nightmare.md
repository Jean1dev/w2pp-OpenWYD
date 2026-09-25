# Pesadelo — issue #346

O servidor Go trata os pergaminhos individuais 3390–3392 e de grupo 3324–3326
pelos efeitos voláteis 173–175. A Escritura 5137 (efeito 212) concede 13 créditos
por unidade; `/nt` informa o saldo e `/nig` envia `!!HHMMSS`. O cooldown da
Escritura permanece desativado, como no C++.

## Acesso e calendário

O calendário usa `time.Local` do processo (configure `TZ` no ambiente de
implantação); `/nig` permite conferir o horário efetivamente usado. Cada rodada
tem quatro minutos para entrada, quinze minutos de combate e um minuto vazio.
Os três mapas são compartilhados, não instâncias por grupo.

| Tipo | Classe | Origem (segmento x/y) | Destino padrão | Entrada | Combate | Limpeza |
| --- | --- | --- | --- | --- | --- | --- |
| N | Mortal | 19/15 | 1304,335 | 00–03; 20–23; 40–43 | 04; 24; 44 | 19; 39; 59 |
| M | Arch | 16/16 | 1083,308 | 05–08; 25–28; 45–48 | 09; 29; 49 | 24; 44; 04 |
| A | Celestial, CelestialCS, SCelestial | 19/13 | tabela `PesaAPosStandard` | 10–13; 30–33; 50–53 | 14; 34; 54 | 29; 49; 09 |

Os segmentos têm 128 tiles. M/A respeitam o ponto salvo dentro do próprio mapa.
O usuário precisa ser líder ou estar sozinho, mesmo usando pergaminho individual.
O pergaminho de grupo transporta os integrantes elegíveis sem exigir proximidade.
Cada participante de A gasta um crédito; integrantes sem crédito ficam no local.
Somente o usuário do pergaminho perde uma unidade de item.

`-max-nightmare` / `W2PP_MAX_NIGHTMARE` define o limite por mapa/rodada (padrão 3;
valores não positivos usam o padrão). N/M contabilizam apenas pergaminhos de
grupo; A contabiliza também individuais. Desconectar ou sair não restitui vaga.

## Ciclo e paridade

- `World` possui todo o estado transitório, sem locks ou IO bloqueante no loop.
  O tick identifica cada ocorrência pela abertura; atraso do tick não repete
  spawns e mantém o encerramento original. Retrocesso do relógio não reabre uma
  ocorrência já iniciada. Rodadas inteiramente perdidas são descartadas.
- O início envia `MsgStartTime`/IDScene com 900 segundos (ou o restante quando o
  tick atrasou), gerando monstros somente se houver jogadores no mapa.
- Os geradores 2368–2375, 2377–2384 e 2385–2394 são exclusivos do evento:
  não geram no bootstrap, timer genérico nem fila de respawn de 15 segundos.
- Mortes de monstros de Pesadelo por jogadores usam a ordem do C++: sorteio de
  `DieSay`, tentativa de `GenerateMob`, ouro e itens. A tentativa de geração ainda
  conta o monstro que está morrendo, preservando o limite de população e o sorteio
  de tamanho do grupo mesmo quando o gerador está cheio. Loot usa as tabelas
  existentes dos templates, sem recompensa extra inventada para o evento.
- Os ramos de EXP de N/M/A são próprios. Cada integrante vivo no mesmo mapa do
  atacante recebe seu `GetExpApply`, sem divisão por número de integrantes;
  o bônus de item é o do atacante. Preservam-se divisores float32, ordem dos
  bônus globais, offset de nível Celestial e a divisão Celestial duplicada de N.
  As limitações preexistentes de `DayLog`/`Hold` e dos drops gerais do port Go
  continuam descritas em `game-rules.md` e nos pacotes `level`/`loot`.
- A limpeza remove monstros e respawns dos geradores, zera admissões e retorna
  jogadores à cidade. Mortos recebem 2 HP antes do recall, como `ClearMapa`.

As validações adicionais do port recusam troca/autotrade, membros mortos,
conteúdo ausente, destino indisponível e overflow dos créditos. Recusas não
consomem pergaminho, crédito nem vaga. IDs inválidos, desconectados, summons e
duplicatas da lista de grupo não são admitidos nem recebem EXP.

Ao reiniciar, o processo aguarda a próxima janela completa. Login com posição
antiga dentro de qualquer Pesadelo retorna à cidade, mantendo o ponto salvo para
uso futuro de um pergaminho. Créditos consumidos não são reembolsados pelo
reinício; saldo e inventário seguem os saves periódicos/logout existentes.

## Persistência e implantação

Migração `0024_nightmare_entries`: `character.nightmare_entries` (INTEGER) e
`last_nightmare_use` (BIGINT), ambos com default zero. O importador lê NT em
448–451 e LastNT em 440–447 de STRUCT_MOBEXTRA, little-endian, time_t de 8 bytes.
Os campos internos protobuf são 50 e 51; não há alteração no protocolo CPSock.
Saldo, timestamp e inventário participam da mesma transação de save.

Aplicar a migração e atualizar dbserver/tmserver juntos em manutenção; versões
antigas de tmserver não transportam os novos campos e podem zerá-los ao salvar.
Antes de rollback, exportar os saldos: a migração down remove essas colunas.
Conferir nos logs `nightmare started`, `nightmare ended` e avisos de geração.

## Fontes e validação

Referências: `_MSG_UseItem.cpp:2548–2869,3291`,
`ProcessSecMinTimer.cpp:1005–1036,1430–1493`, `Server.cpp:381–430,9628–9670`,
`MobKilled.cpp:443–875,1430,1882–1889,2693` e `Basedef.h:709–712`.

Os testes cobrem os seis itens, horários, classes, liderança, limite assimétrico,
grupos, créditos, troca, consumo, calendário, atraso/repetição, limpeza, RNG,
conteúdo real, wire do contador, EXP, comandos, logout/relogin, importação,
protobuf e transações PostgreSQL. Executar `make build vet test lint` e
`go test -tags=integration ./internal/store/...` com `W2PP_TEST_DSN`.

Validação manual no cliente 7662: usar as seis entradas nos locais/horários da
tabela; conferir contador de entrada e de combate, monstros, EXP/drops, `/nt`,
consumo da Escritura, persistência após relog e retorno à cidade ao encerrar.
