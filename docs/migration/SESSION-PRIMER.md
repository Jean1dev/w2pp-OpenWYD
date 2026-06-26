# Session primer — w2pp-OpenWYD (cole no início de uma nova sessão)

> Resumo de contexto + fluxo de trabalho para continuar a migração do WYD para Go.
> Leia também `CLAUDE.md` (raiz) e `docs/migration/ingame-bugs.md` (rastreador de bugs in-game).

## 1. O que é o projeto

Reescrita **big-bang em Go** do servidor do MMORPG **WYD (With Your Destiny)**, mirando o
**cliente Windows `WYD.exe` original e SEM modificações** ("Cavaleiros de Kersef").
- **ClientVersion = `12000`** (esse build manda 12000; o tmserver roda com `-client-version 12000`).
- Fonte C++ legada em `Source/Code/` (decompile **parcial** — algumas funções só têm declaração).
- Conteúdo de jogo + binários legados em `Release/` (montado read-only nos containers).
- Serviços novos em `tmserver/`, `dbserver/`, `binserver/`. Módulo Go: `github.com/jeanluca/w2pp-openwyd`.

### Status atual (o que JÁ funciona contra o cliente real)
Login → seleção/criação de personagem (com equipamento visual correto + **preview de atributos**:
score level/HP/MP/STR-INT-DEX-CON na própria tela de seleção, via `protocol/selchar.go`) → entrar no
mundo → andar → ver outros players → ver ~10.500 NPCs (nomes visíveis, aparência correta) → **lojas de
NPC** (abrir, comprar, vender, item aparece no inventário ao vivo, economia de gold) → **teleporte
entre cidades** (por tile, Armia↔Noatum↔Azran/Erion/Nippleheim) → **persistência** de inventário, gold
e **última cidade** (spawn na área default da última cidade) ao deslogar/relogar.

> Atenção (bug aberto B6): a **posição exata NÃO é persistida** — ao relogar o char nasce no spawn
> default da última cidade, não onde deslogou. Detalhe em `docs/migration/ingame-bugs.md`.

## 2. Arquitetura (o essencial)

Três microserviços; só a borda **cliente↔tmServer** fala o protocolo legado (CPSock). Links internos
são **gRPC (+mTLS opcional)**.
- **tmServer** (`tmserver/`): jogo. Porta **8281** (CPSock game) e **80** (HTTP status `serv00.htm`,
  listeners SEPARADOS — o cliente sonda o status no :80 antes de abrir o :8281). Dono de TODO o estado
  do mundo em **um único goroutine** (`world.World.Run`) — **sem locks**. Handlers rodam DENTRO do
  loop e mutam estado direto. Chamadas bloqueantes (dbServer/billing) vão para fora do loop via
  `World.Go(...)` e o resultado re-entra no loop por callback.
- **dbServer** (`dbserver/`, porta **7514**): persistência (PostgreSQL/pgx v5). Subcomandos:
  `serve`, `convert`, `seed-account`. Migrations embutidas em `dbserver/migrations/*.up.sql`
  (aplicadas em ordem no boot).
- **binServer** (`binserver/`, porta **3000**): billing (allow-all por padrão).

### Regras de fidelidade que importam
- **Sem criptografia real**: CPSock é obfuscação por tabela estática (`pKeyWord`) + checksum não
  rejeitante. Header CPSock = **12 bytes**.
- **Layout binário é offset-explícito**. Structs de save = alinhamento natural MSVC x86 (NÃO pack(1)).
  Não confie no alinhamento do Go — leia/escreva por offset. Tamanhos-chave: `STRUCT_MOB`=816,
  `STRUCT_ITEM`=8, `STRUCT_SELCHAR`=840.
- **Paridade de RNG** é meta (LCG do MSVC reimplementado em `tmserver/internal/rng`). Preserve a ordem
  das chamadas de rand em código de gameplay.

## 3. O AGENTE DO WINDOWS (como obter dados que não temos)

O usuário tem, **na máquina Windows**, o **servidor ORIGINAL que COMPILA e roda** (a fonte completa,
mais completa que nossa cópia parcial em `Source/Code/`) + uma instância do Claude Code lá, com um
**dumper compilado** (`_layout_probe/dump_layout.cpp`) que imprime `sizeof`/`offsetof` **verificados
pelo compilador** (MSVC x86).

**Quando usar:** sempre que precisar de um layout byte-exato de struct/pacote, de valores de tabela
hardcoded, ou da lógica de uma função que **não existe** na nossa cópia da fonte (decompile parcial).
Eu (assistente) **não falo com o agente direto** — eu **escrevo um prompt** e o usuário cola lá, roda,
e traz o resultado de volta (geralmente um `.md`).

**Como escrever um bom prompt para o agente:**
- Dê o contexto (migração WYD→Go, header CPSock=12B, o que já funciona).
- Peça **offsets + tipos** de cada campo, **tamanho total** (via o dumper), e o valor do **Type** do
  pacote (com `FLAG_GAME2CLIENT 0x0100` / `FLAG_CLIENT2GAME 0x0200`).
- Peça o **código** da função relevante (ex.: `SendFunc.cpp`, `_MSG_*.cpp`) quando a lógica importa.
- Peça para **salvar num arquivo** `captura-wyd-<assunto>.md`.
- Lembre que o agente tem a fonte COMPLETA: se uma função não está na nossa `Source/Code`, ele tem.

Exemplos já entregues: `serv00.htm`/sizes S→C, `STRUCT_MOB`/BaseMob, CreateMob/ShopList/SendItem,
ItemList preços + fórmulas buy/sell, tabela das 5 cidades, tabela de rotas de teleporte
(`GetTeleportPosition`/`DoTeleport`).

## 4. Rodar e testar localmente

Servidor roda nesta máquina Linux; o cliente real roda no **Windows do usuário** apontando o
`serverlist.bin` para o IP desta máquina (status `http://<ip>/serv00.htm` no :80, game no :8281).

```bash
docker compose up -d --build            # sobe tudo (db, dbserver, binserver, tmserver)
docker compose run --rm dbserver seed-account -name test -pass test123   # conta de teste
docker compose logs --since 2m tmserver # logs (todo pacote logado: "recv packet type=0x....")
```

**Teste de login HEADLESS (sem o cliente Windows) — use ANTES de culpar o código de jogo:**
```bash
W2PP_E2E_ADDR=127.0.0.1:8281 W2PP_E2E_ACCOUNT=test W2PP_E2E_PASSWORD=test123 \
W2PP_E2E_VERSION=12000 go test -tags=e2e -run TestE2ESmokeLogin ./tmserver/internal/world/ -v
```
`CNFAccountLogin (0x10a)` = chain tmServer→dbServer→Postgres OK. **Tem que mandar
`W2PP_E2E_VERSION=12000`** ou o servidor responde `0x102` (version mismatch).

Build/test padrão: `go build ./...`, `go test -race ./...`, `make lint`.

## 5. LIÇÕES DE DEBUG (aprendidas no sangue — leia antes de gastar horas)

1. **Servidor-correto ≠ cliente-correto.** O servidor pode calcular tudo certo e o cliente mostrar
   errado se o **pacote S→C** carrega o valor errado. Ex. real: a persistência da cidade funcionava
   (load `last_city` certo, spawn calculado certo), mas o `CNFCharacterLogin` mandava a **posição do
   template (Armia)** em vez do spawn calculado → cliente sempre desenhava Armia. **Sempre confirme o
   que VAI NO PACOTE**, não só o estado do servidor. Adicione logs dos dois lados (load no dbserver +
   spawn no tmserver) e compare.

2. **Imagem Docker stale é traiçoeira.** `docker compose up --build` às vezes **reusa um layer em
   cache** e NÃO recompila com o código novo — especialmente após `make proto` (o `db.pb.go`
   regenerado). Sintoma: o DB tem o valor certo mas o serviço retorna 0/antigo; ou o cliente ignora um
   campo novo do proto (desserializa sem o field). **Fix:** `docker compose build --no-cache <svc>` e
   reinicie. Quando um campo persistido "não carrega" mas o DB está certo, **suspeite da imagem antes
   do código**. Depois de mexer no `.proto`, rebuild `--no-cache` nos DOIS lados (tmserver+dbserver).

3. **gRPC stale / rede Docker.** "Login travado / conectando" quase sempre é o link gRPC
   tmServer↔dbServer, não o jogo: (a) redeploy só do tmserver deixa a conexão stale → reinicie os dois;
   (b) **IPAM do Docker esgotado** ("all predefined address pools have been fully subnetted") após
   muitos ciclos down/up/run → o compose tem um subnet fixo `172.28.0.0/24` em `networks.default.ipam`
   pra contornar; limpeza total = `sudo systemctl restart docker`.

4. **Não reinicie serviços no meio de um teste do usuário.** Restart no meio dispara save-on-shutdown /
   reconexão gRPC e gera falhas transitórias que parecem bug de código. Faça o deploy, ESPERE o usuário
   testar, e só então mexa.

5. **Persistência: save assíncrono vs reload.** Saves on logout/disconnect são `World.Go` (fora do
   loop). Para o logout-para-seleção (mesma conexão, reload rápido), use `World.SaveCharacterThen` (só
   confirma ao cliente DEPOIS do save commitar). `shutdown()` espera saves em voo via `saveWG`.

## 6. Fatos/constantes úteis

- ClientVersion **12000**. Contas teste **test/test123** e **test2/test123** (a segunda serve pra
  testar visão entre players — duas instâncias do cliente). Char inicial: 1.000.000 de gold, spawn Armia.
- **5 cidades** (`world/city.go`): Armia(0) (2086,2093), Azran(1) (2494,1707), Erion(2) (2453,2000),
  Nippleheim(3) (3652,3122), Noatum(4) (1050,1706). "Última cidade" salva em 2 bits (0–3; Noatum não é
  salvável → cai em Armia). Spawn = `CitySpawn(cidade) + rand%15`.
- **Teleporte** (`world/teleport.go`): por TILE; cliente pisa e manda `_MSG_ReqTeleport` (0x0290, só
  header); servidor resolve destino+custo pela posição (Go: `TeleportDest` em `world/teleport.go`). Noatum
  é hub (cidades pagam 700 p/ ir, voltam de graça). O teleporte = `MSG_Action` (0x036C) com `Effect=1`
  + grid reconcilia visão.
- Tipos de pacote ficam em `tmserver/internal/protocol/types.go`. Codecs S→C com testes byte-exatos em
  `protocol/*_test.go` (CreateMob, ShopList, SendItem, UpdateEtc, CNFCharacterLogin, SELCHAR).

## 7. Próximas frentes (roadmap)

Já feito: login, chars, NPCs (nomes/aparência), lojas (buy/sell + SendItem), teleporte, persistência
(itens/gold/cidade), **Banco/Cargo** (armazém compartilhado da conta: LoadCargo/SaveCargo no
dbServer + store; carregado no login, salvo no logout/shutdown; depósito 0x0388 / saque 0x0387 de
gold; NPC Guarda_Carga Merchant=2 abre o cofre), **mover itens** (drag-drop via 0x0376),
**equipar/desequipar** (0x0376 Carry↔Equip + _MSG_UpdateEquip 0x006B; equip carregado do DB no
login), **equip inicial** (semeado no login se o char está sem equip: equip da classe do template
— slot 0 = item de corpo que dá a aparência da classe — + montaria Shire item 342 no slot 14;
inventário inicial = poções/Esfera da Sorte/Baú de Exp do template), **NPC Perzen** (troca via
_MSG_Quest 0x028B: Merchant 100 + EF_GRADE0 7/8/9; consome npc.Carry[0] e dá npc.Carry[1] — ex.:
Esfera da Sorte A 4130 → montaria Thoroughbred 3987), **combate player→mob** (resolução de dano
server-authoritative já existia; agora ao matar: drop de gold/loot + **despawn** do mob com
`MSG_RemoveMob` type 1 + limpeza de grid/slot via `world.DespawnMob`), **EXP + level-up por kill**
(mob carrega seu reward em `STRUCT_MOB.Exp`@32; pacote `internal/level` = curva `g_pNextLevel[0..400]`
+ `ExpApply` scaling por nível + `ScoreBonus`/HP/MP por nível, **MORTAL solo**, da captura do agente;
ao matar: `killer.Exp += ExpApply`, level-up incrementa Level/MaxHP/MaxMP + cura full + pontos de
atributo, manda `MSG_UpdateScore` + efeito `MSG_Motion(14,3)`; exp/level/MaxHP/MaxMP persistem no DB;
exp entregue via `MSG_Attack.CurrentExp` no eco do ataque ao próprio atacante), **IA de mob iteração 1**
(tick periódico `world/tick.go` + `handler/mobai.go`: monstro agro por proximidade/retaliação, persegue
e ataca o player corpo-a-corpo — ver a seção do roadmap p/ detalhes e o que falta), **morte/respawn do
player** (`_MSG_Restart` 0x0289: reviver HP=2 + recall à última cidade + refresh; sem penalidade de exp).

Expiração de item: server-side via `item.expires_at` (coluna TIMESTAMPTZ, migração 0003 + campo
proto `expires_at`); setada na entrega do Perzen (now+30d) e checada no login (`dropExpired` remove
vencidos de equip/carry). O cliente mostra "(30dias)" pelo nome do item.

Atributos (CurrentScore) — **separação Base↔Current FEITA** (`handler/item.go`). Modelo: a `Entity`
guarda `Base*` (score sem equipamento) e o live `Str/AC/Damage/MaxHP/MaxMP` (= base + equip). No login
`deriveBaseScore` deriva `base = current(carregado) − equipBonus`; ao equipar/desequipar (`refreshEquip`
→ `refreshScore`) e ao gastar ponto (`applyBonus` agora soma no `Base*`) recalcula `current = base +
equipBonus` (clampa HP/MP). `equipBonus` soma os efeitos-base do catálogo (`ItemList.BaseEffects`:
EF_AC/STR/INT/DEX/CON/HP/MP) + os refinos da instância — **agora AC/atributos/HP/MP de toda peça contam**,
no display E no combate (combat lê `e.AC`/`e.Damage` que já são current). **Sem double-count**: o valor
carregado vira o baseline (base = carregado − equip), então o delta de trocar item é exatamente o efeito
do item; persiste só o CurrentScore (a base é re-derivada a cada login, sem mudança de schema). **Dano
de arma** continua à parte (`weaponDamage`, EF_DAMAGE das slots 6/7, regra `max+min/2`) — é campo
separado no original, somado no hit; `computeScore`/combat somam por cima de `e.Damage` (EF_DAMAGE de
arma é EXCLUÍDO do `equipBonus` p/ não duplicar). `EncodeUpdateScore` no login/equip/applyBonus.
**UNVERIFIED/falta:** o `BASE_GetCurrentScore` exato (multiplicadores de classe, EF_ACADD/HPADD/MPADD,
caps de resist, almas) não está no Source → o baseline absoluto é o valor carregado, mas o **delta** por
equip é correto. Tiers/ADD-variants ficam p/ captura do agente.

**Requisitos de equip FEITO** (`meetsEquipReq`): `ItemList.Requirements()` parseia a 4ª coluna do CSV
`ReqLvl.ReqStr.ReqInt.ReqDex.ReqCon` (ordem = STRUCT_ITEMLIST, confirmada pelas armas: machados/espadas
põem o req de STR na 2ª posição; pos1 capa em 399 = nível). Equipar (useItem + tradingItem) checa
`e.Level/Str/Int/Dex/Con` (current) ≥ req; se não bate, `NoticeReqNotMet` e não equipa. Item sem entrada
no catálogo passa livre. (Validação de SLOT correto por `nPos` ainda não checada.)

### Bugs abertos conhecidos (rastreador: `docs/migration/ingame-bugs.md`)
- **B6** (P2): posição exata não persiste — `LoadCharacter` volta 0,0 e usamos o spawn do template;
  falta adicionar campos de posição ao proto `CharacterState` + dbServer salvar/carregar `SaveX/SaveY`
  (regerar proto). NÃO precisa do agente Windows.
- **B1** (P0, parcial): falta enviar `CreateMob`/`RemoveMob` quando players **cruzam a visão andando**
  (hoje só na entrada no mundo) + **equip visual** dos players (`BASE_VisualItemCode` — aparecem sem
  equipamento).
- **B5** (P3): level mostra +1 na seleção (provável quirk 1-indexado do cliente; confirmar campo/offset).

### IA / movimento / combate de mob (frente grande — iteração 1 FEITA)
O loop é event-driven; o **tick de IA** agora existe (`world/tick.go`: `SetTickHandler`+`runTicker`
emite `tickEvent` a cada `DefaultMobTick`=1s; o ticker **não muta estado**, só enfileira um evento que
`apply` roda **dentro** do loop — invariante de goroutine única preservado). A IA vive em
`handler/mobai.go` (`Dispatcher.Tick`): cada monstro (Merchant==0) vivo **agro por proximidade**
(`FindPlayerNear`, caixa Chebyshev 4) ou **retaliação** (ser atacado seta `mob.Target`+`MobCombat` no
handler de ataque), **persegue** 1 tile/tick (`SetEntityPos`+broadcast `MSG_Action`) e **ataca** corpo-a-corpo
na cadência (`combat.ResolveHit` `TargetIsPlayer`, broadcast `MSG_Action`/`MSG_Attack`); hesitação por
Int (BattleProcessor). Lógica fiel ao `CMob.cpp` (StandingBy/BattleProcessor/GetEnemyFromView), mas o
**loop orquestrador original NÃO está na fonte** → cadência/return-codes UNVERIFIED.
- **Cidade = safe zone + regen** (fix do "morre logo após respawn"): mob NÃO agro/ataca player dentro de
  um retângulo de cidade (`world.Village>=0` — checado no agro e no `validTarget`, então o mob também
  larga o alvo se ele entra na cidade); e todo player vivo **regenera HP/MP** por tick (`regenPlayers`/
  `regenStep` ≈5%+piso do max, manda `MSG_UpdateScore`; morto/HP=0 não regenera — precisa restart). Taxa
  do `RegenMob` real não está na fonte (Server.cpp) → UNVERIFIED.
- **Morte/respawn do player FEITO** (`handler/character.go` `restart`, `_MSG_Restart` 0x0289): mob leva
  o HP a 0 → cliente mostra a morte (pela `Dam` letal do `MSG_Attack`) → player aperta restart → reviver
  com **HP=2** + reset de crack-errors + **recall** à última cidade (`CitySpawn`+`doTeleport`) + refresh
  (`MSG_UpdateScore`/`MSG_UpdateEtc`); na cidade ele fica seguro e a vida volta pelo regen. Fiel ao
  `_MSG_Restart.cpp` (sem penalidade de exp). UNVERIFIED: `_MSG_SetHpMp` (0x0181, layout 129B desconhecido
  → HP vai no UpdateScore); destinos per-clan (7/8) e o `DoRecall` exato → usamos o spawn da última cidade.
- **Não loga morto** (`completeCharacterLogin`): como mob salva o player com HP=0 ao matá-lo, no login um
  char com HP≤0 é revivido pra full (senão logava travado/morto — o regen exclui HP=0). Espelha o respawn
  vivo. (Causa do bug "loga e morre/fica morto na cidade".)
- **Falta (iteração 2+):** tabela de hostilidade por clan (hoje todo monstro agro qualquer player);
  ataque ranged/`EF_RANGE`; pathfinding real (`BASE_GetRoute` — hoje passo Chebyshev simples);
  roaming/segmentos/rotas (`RouteType`), summons; reveal ao cruzar visão andando (compartilha
  com B1); respawn de mob (slot é liberado no kill, `SpawnMob` é init-only).
- **Level-up** (da frente anterior) **Falta:** tiers ARCH/CELESTIAL (curva `g_pNextLevel_2`, quest-gates)
  + AC++/skill/special bonus (Entity não modela base-score separado) + itens por nível (`DoItemLevel`)
  + `MSG_CreateMob` p/ refletir novo nível/visual aos outros; EXP de party (divisores não confiáveis —
  ver `captura-wyd-levelup.md`).

### Frentes menores subsequentes
- **Demais NPCs de quest/montaria** (mapa completo em `docs/migration/handlers/npc-map.md`): montarias
  (Merchant 16 captura / 58 cura / 23 grifo / 101-110 unicórnio), class masters (Merchant 3/31), Perzen
  grades 0-4 (cadeia de level); generalizar a troca **data-driven**; SendSay (diálogo do NPC, UNVERIFIED).
- **NPCs de combinação/refino** (Odin/Lindy/Shany).
- Persistência de stats/skills mais completa; mais rotas de teleporte (campos/dungeons já parciais).

Sempre: ler `docs/migration/` antes de mexer em wire/format/gameplay; comentar o **porquê** (paridade);
testes table-driven `-race`; o snapshot/golden de protocolo são os testes críticos de paridade.

ler tambem o `development-guidelines/Go-development-guidelines.md` para entender o padrao de desenvolvimento