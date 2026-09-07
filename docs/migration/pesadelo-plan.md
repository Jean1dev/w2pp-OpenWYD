# Pesadelo N/M/A — a masmorra instanciada por horário

> Status: **FECHADO** — entrada, agendamento, limite, limpeza da instância (jogadores **e**
> monstros), persistência das entradas do Arcano, a recompensa de EXP própria de cada tier (§6.4) e
> a porta dos fundos que o legado deixava aberta no Normal (§4).
>
> Origem: os três tiers existiam no conteúdo (regiões, itens, mapas) mas **nenhum handler no
> servidor Go** — usar o pergaminho caía no `default` de `useItem` e devolvia "não pode usar aqui".
> Fonte legada: `_MSG_UseItem.cpp:2548` (N), `:2644` (M), `:2748` (A), `:3291` (Escritura);
> `ProcessSecMinTimer.cpp:1006-1035` (limpeza); `Server.cpp:381/398/415/687/754` (tabelas e config).

## 1. O que existia antes

| Peça | Onde | Estado |
|---|---|---|
| Regiões `Pesadelo_N/M/A` | `Release/TMsrv/run/Regions.txt:4-6` | ✅ no conteúdo |
| Pergaminhos de entrada | `ItemList.csv` — 3324/3390 (N), 3325/3391 (M), 3326/3392 (A) | ✅ no conteúdo |
| `Escritura_do_Pesadelo` | `ItemList.csv` 5137, `EF_VOLATILE 212` | ✅ no conteúdo |
| Handler de `EF_VOLATILE` 173/174/175 | — | ❌ **não existia** |
| `maxNightmare`, `PartyPesa[3]`, tabelas de posição | — | ❌ não portados |
| `inPesadelo()` | `handler/item.go` | ⚠️ existia, mas só para bloquear Gema Estelar/Portal — e só cobre M e A |

## 2. O formato que o legado repete três vezes

O legado escreve o mesmo bloco uma vez por tier, com números diferentes. Reduzido a uma regra:

- Cada tier abre por **4 minutos, 3 vezes por hora**, e os três são **escalonados de 5 em 5** para
  nunca se sobreporem.
- **Um minuto antes de cada abertura** a instância é limpa: quem está dentro é recolhido de volta
  (`ClearMapa` → `DoRecall`) e o contador de runs zera.
- Só o **líder de party** usa o pergaminho, e só na área de espera daquele tier.
- Entrada é uma **escada de classe e nível** — ver §2.1.
- No máximo **`maxNightmare` runs por janela por tier**, global do servidor.

### 2.1 A escada de entrada

O legado gateia **só por classe**; nível ele não checa em lugar nenhum (a única menção é um
`Level >= 180` **comentado** no laço de party do Arcano). Os tetos abaixo são **regra deste
servidor**, deliberadamente a mesma escada que o Pergaminho da Água já usa — um tier que você supera
te entrega ao próximo:

| Tier | Classe | Nível |
|---|---|---|
| **N** | Mortal | até 400 (sem teto na prática: `MaxLevel` é 399) |
| **M** | Arch | até 400 |
| **M** | Celestial | **até 40** — mesmo número do `waterMCelestialMaxLevel` |
| **A** | Celestial / CelestialCS / SCelestial | **até 150** — passou disso, só **Água A** |

Classe e nível são **portões separados** no código, para a recusa dizer qual fechou: "masmorra
errada" e "você passou deste tier" pedem ações diferentes do jogador. Um teste amarra os dois
extremos da escada — que o teto do Místico é o mesmo nos dois calabouços, e que um Celestial acima
do teto do Arcano **é aceito na Água A**, ou seja, não fica sem para onde ir.

O Arcano cobra ainda **uma entrada** (`extra.NT`) de cada personagem admitido, líder incluído.

| | Janelas | Limpeza | Entra por | Área de espera (seg./128) | Instância (seg./128) | Chegada |
|---|---|---|---|---|---|---|
| **N** | :00 :20 :40 | :19 :39 :59 | **Erion** | (19,15) | (10,2) | 1304, 335 |
| **M** | :05 :25 :45 | :04 :24 :44 | **Armia** | (16,16) | (8,2) | 1083, 308 |
| **A** | :10 :30 :50 | :09 :29 :49 | **Azran** | (19,13) | (9,1) | 13 pontos distintos |

Cada área de espera é o segmento do centro daquela cidade — Armia (2086,2093) cai em (16,16),
Erion (2453,2000) em (19,15) e Azran (2494,1707) em (19,13).

A `Escritura do Pesadelo` dá 13 entradas. O cooldown de 20h dela está **comentado na fonte legada**,
então não existe.

O cronômetro que o cliente recebe é **o que sobra da janela**, não 4 minutos novos: entrar às
:23:30 dá 30 segundos. Essa é a diferença que a implementação mais fácil erraria.

## 3. Como ficou no Go

Tudo em [`tmserver/internal/handler/pesadelo.go`](../../tmserver/internal/handler/pesadelo.go), na
mesma forma da masmorra da Água (`waterscroll.go`), que já resolvia o padrão "pergaminho → sala
temporizada".

A tabela `pesaTierTable` carrega os números de cada tier e o resto é derivado dela, em vez de três
cópias de um bloco. O agendamento vira aritmética modular:

```go
elapsedMin := ((min - t.openMinute) + 60) % 20   // 20 = intervalo entre janelas
open := elapsedMin < 4                            // 4 = duração
left := 240 - (elapsedMin*60 + sec)               // o que resta
wipe := ((min+1) - t.openMinute + 60) % 20 == 0   // 1 minuto antes de abrir
```

Os portões rodam na ordem do legado — área, líder, classe, nível, entradas, horário, limite — e **toda
recusa devolve o pergaminho**: o cliente já o removeu otimisticamente, então sem o reenvio do slot
ele parece consumido até o próximo resync.

`d.clearArea` (já existente, `ClearArea` do legado) faz o papel do `ClearMapa`: revive quem está com
0 HP para 2 e recolhe todo mundo dentro do retângulo.

## 4. Desvios conscientes

- **`/nig` mostra o horário, não o relógio.** O legado imprime `!!HHMMSS` e deixa o cliente derivar
  as janelas. Como o escalonamento é conhecimento do servidor, o comando responde direto o que
  serve: qual tier está aberto e quanto falta para os outros.
- **Cinco avisos novos** (`NoticePesadelo*`). Só o limite de runs tem id legado
  (`_NN_Night_Limited`); os demais são strings literais que o legado manda direto pelo
  `SendClientMessage`, e o de nível não existe lá de forma alguma. Ficaram separados em vez de
  virarem um só porque "tier errado", "você passou deste tier", "horário errado" e "sem entradas"
  são quatro problemas com quatro soluções diferentes.
- **`rand()%1` não foi portado.** Aparece no bloco do A somando um jitter à posição de chegada, mas
  `%1` é sempre 0 — é um no-op do legado, não um espalhamento.
- **A 13ª linha das tabelas de posição foi descartada.** `MAX_PARTY` é 12, então o laço nunca lê
  `PesaAPosStandard[12]`.
- **`inPesadelo` cobre os três tiers, e o legado cobre dois.** `_MSG_UseItem.cpp:1365-1368` (Gema
  Estelar) e `:1424` (Portal) testam só (9,1) e (8,2) — o Normal, (10,2), fica de fora dos dois.
  Isso é uma porta dos fundos: o segmento do N não está dentro de nenhum retângulo de cidade, então
  a Gema salva ali sem obstáculo e o Pergaminho de Portal reentra na instância quando quiser,
  passando por cima da janela de 4 minutos, do líder de party, da escada de classe e do teto de
  runs — e o N é o tier Mortal, o mais cheio. `inPesadelo` passou a ler `pesaTierTable`, então cobre
  os três e não pode mais divergir de onde os tiers ficam. O lado da Gema não muda nada na prática
  (salvar no N já era permitido, por ausência de cidade; agora é permitido pelo carve-out).
- **O contador não expulsa ninguém — e isso é do legado.** `NigthTime` é o que resta da janela de
  entrada, não o tempo de permanência: quem entra às :00 vê 4 minutos, mas só é recolhido na
  limpeza, às :19. Na prática se fica de **15 a 19 minutos** dentro. O legado é idêntico
  (`_MSG_UseItem.cpp:2604-2614` calcula o contador; `ProcessSecMinTimer.cpp:1006-1035` é o único que
  tira alguém de lá). Fica como está até virar decisão de produto — o conserto é ou mandar o tempo
  até a limpeza no lugar do resto da janela, ou passar a expulsar no zero.

## 5. Configuração

`maxNightmare` (`Server.cpp:687`, default 3) virou flag e variável de ambiente:

```bash
go run ./tmserver/cmd/tmserver -max-nightmare 5      # ou W2PP_MAX_NIGHTMARE=5
```

Zero mantém o default legado de 3. No legado o valor vinha do bloco *Etc Settings* do portal
(`config-ops.md §2.1`); esse canal de config ainda não existe no port.

## 6. Persistência das entradas e limpeza dos monstros

As duas lacunas da primeira entrega foram fechadas.

### 6.1 O contador de entradas persiste

`MobExtra.NT` fica dentro do bloco de 552 bytes que o port trata como bruto, então seguiu o mesmo
caminho do resto da progressão desta reescrita: **coluna no Postgres**, não no blob legado.

`character.nightmare_tickets` (migration `0025`) → `domain.Character` → `api/db/v1` campo 48 →
`world.CharacterSave` → `world.Entity`. Dez pontos de toque, os mesmos que `celestial_arch_level`
percorre.

Tanto a concessão (Escritura, +13) quanto o gasto (cada admissão no Arcano) chamam
`SaveCharacterAsync` **na hora**, em vez de esperar o save periódico: entrada se compra com gold, e
um crash no meio devolveria a entrada de graça.

> Os stubs foram regerados com `protoc 3.21.12` + `protoc-gen-go v1.36.11` + `protoc-gen-go-grpc
> v1.6.2` — exatamente as versões gravadas nos arquivos existentes. Por isso o diff é cirúrgico: só
> `db.pb.go` mudou; os outros cinco gerados saíram idênticos.

### 6.2 Os monstros somem na virada

`despawnPesadeloMobs` é o `DeleteMobMapa` (`Server.cpp:9655`): na limpeza, os monstros que sobraram
no segmento são removidos com `DespawnMob(id, 1)` — o mesmo `removeType` do legado, que também
decrementa a população do generator para o timer de minuto repor a instância.

**Desvio consciente: NPCs são poupados.** O legado apaga tudo no segmento e deixa os generators
reporem. Aqui não dá: `DespawnMob` só enfileira respawn para monstro de combate
(`Merchant == 0 && !NonCombatNPC`), então despawnar um lojista o apagaria até o próximo restart. E
isso não é hipotético — ver §6.3.

### 6.3 As "vilas não identificadas" eram as masmorras

Ao portar a limpeza, os segmentos do Pesadelo bateram com dois clusters de loja que o
[painel de NPCs](./npc-city-panel.md) tinha rotulado como vilas anônimas:

| Cluster | Segmento | É, na verdade |
|---|---|---|
| "Vila do Noroeste" (1090, 315) | (8, 2) | **Pesadelo Místico** |
| "Vila do Norte" (1310, 330) | (10, 2) | **Pesadelo Normal** |

Os segmentos são exatos, não aproximados. Isso também explica o outro achado daquele painel: seis
das oito lojas que vinham com `Carry[]` **vazio** — `Irena_`, `Lainy`, `RoPerion`, `Balmers`,
`Naomi`, `Rubyen` — são exatamente as que ficam dentro dessas duas instâncias. Não são lojas de
cidade quebradas; são NPCs de masmorra.

`webserver/internal/mapzones` foi corrigido: as zonas 6 e 7 agora se chamam *Pesadelo Místico* e
*Pesadelo Normal*, marcadas como verificadas, e o Arcano entrou como zona 9. O teste amarra cada uma
ao **segmento**, não ao rótulo — é o segmento que prova a identificação.

### 6.4 A EXP de dentro da masmorra — fechada

Os três ramos de `MobKilled.cpp` (`:443` A, `:592` M, `:737` N) foram portados em
`internal/level/expzone.go` e são escolhidos pelo **mesmo segmento de 128 tiles** que esta tela usa
para a limpeza. Antes disso as três instâncias pagavam a tabela do campo aberto. Ver `game-rules.md`
§1 para as quatro diferenças (escala base, teto `eMob`, conteúdo da fada, tabelas ausentes).

`TestPesadeloSegmentsMatchExpZones` amarra as duas tabelas: `pesaTierTable` e `level.ZoneForTile`
guardam os mesmos seis números em pacotes diferentes, e mover um tier aqui sem mover lá faria as
mortes voltarem calado para a tabela do campo.

**Cuidado que sobra:** os ramos de Pesadelo usam a escala identidade em `int` de 32 bits, então um
monstro com `Exp` acima de ~5,2 milhões **estoura e paga zero**. É comportamento do legado, portado
como está — mas é o tipo de coisa que parece bug de servidor quando um chefe novo não dá nada.

### 6.5 O que continua fora de escopo

**O contador não expulsa ninguém.** Ver §4: é fiel ao legado, e é uma decisão de produto, não uma
lacuna de porte.
## 7. Testes

[`pesadelo_test.go`](../../tmserver/internal/handler/pesadelo_test.go) fixa o agendamento (o que
mais fácil regride): as três janelas de cada tier, os minutos de limpeza, o fato de que **nunca há
dois tiers abertos ao mesmo tempo** (varrendo a hora inteira) e que **toda limpeza cai no minuto
anterior a uma abertura** — assim uma edição que mova um sem o outro quebra o teste.

Além disso: o cronômetro é o resto da janela e não 4 minutos; cada ponto de chegada está dentro do
próprio segmento; a assimetria do `inPesadelo` (cobre M e A, não N) está fixada em vez de esperar
ser redescoberta; e o tick limpa uma vez por minuto, não a cada tick.

Ponta a ponta, com relógio congelado: recusa fora da área, fora do horário, na classe errada e sem
entradas — todas devolvendo o pergaminho — e a entrada bem-sucedida (teleporte para o ponto certo,
cronômetro com o resto correto, pergaminho consumido).
