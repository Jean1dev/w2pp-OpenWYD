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
NPC** (abrir, comprar inclusive item com preço 0, vender, item aparece no inventário ao vivo, economia de gold) → **teleporte
entre cidades** (por tile, Armia↔Noatum↔Azran/Erion/Nippleheim) → **persistência** de inventário, gold
e **última cidade** (spawn na área default da última cidade) ao deslogar/relogar.

> Regra confirmada: a **posição exata NÃO é persistida por design** — ao relogar o char nasce no centro/
> spawn da última cidade visitada, não onde deslogou. Detalhe em `docs/migration/ingame-bugs.md`.

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

6. **Header `ESCENE_FIELD` é obrigatório p/ campos do PRÓPRIO atacante (barra de XP/HP).** O cliente
   aplica o `Dam[]` aos alvos **independente do `HEADER.ID`** (por isso o ataque do mob fere o player
   mesmo com `ID`=id do mob), MAS só aplica os campos do atacante no pacote — `CurrentExp`/`CurrentHp`/
   `CurrentMp`, i.e. a **barra de experiência** — quando o ataque chega como evento de cena, ou seja com
   `HEADER.ID = ESCENE_FIELD` (30000 = `protocol.IDScene`). O original faz `m->ID = ESCENE_FIELD`
   (`_MSG_Attack.cpp:25`) e multicasta via `GridMulticast` (que inclui o atacante). Sintoma quando errado:
   servidor conta o exp certo (logs), mato/loot/gold normais, mas a **barra de XP não anda e o char não
   upa**. Fix em `handler/combat.go` (broadcast do attack com `ID=protocol.IDScene` p/ atacante + in-view);
   teste `TestAttackHeaderIsSceneField`. **Regra geral:** todo pacote S→C autoritativo de cena
   (attack/score/etc.) vai com `HEADER.ID = ESCENE_FIELD`, não com o conn do dono.

7. **NUNCA use campo UNVERIFIED do cliente como GATE de ação server-authoritative.** Regressão real
   (B12): a frente de skills fez o `attack` derrubar o pacote quando `SkillIndex`/`Dam` não batiam
   com os sentinelas da fonte parcial (`-2` melee / `-1` skill) → **todo dano player→mob morreu**
   (mob→player seguia OK porque é gerado no servidor — assimetria é a assinatura desse tipo de bug).
   Regra: para campo cujo comportamento do cliente 12000 não foi capturado, **tolere + logue e siga
   pelo caminho conservador** (aqui: resolver como melee, que ignora o valor mesmo); só rejeite/craque
   depois de captura confirmando. Corolário de teste: **o harness deve espelhar o wiring de PROD** —
   o bug ficou invisível porque os testes de handler criavam o Dispatcher SEM `Spells` no Config,
   então o caminho novo nunca era exercitado. Ao adicionar um Config novo que muda comportamento,
   adicione/atualize um teste com ele LIGADO (ver `TestMeleeAlwaysDamagesMob`,
   `combat_regression_test.go` — as 4 codificações plausíveis de melee contra um MOB real).

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
**equipar/desequipar** (0x0376 Carry↔Equip + _MSG_UpdateEquip 0x036B; equip carregado do DB no
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

**SKILLS + BUFFS/AFFECTS (frente grande FEITA — jul/2026).** Catálogo `SkillData.csv` tipado
(`content/skilldata.go`: STRUCT_SPELL, índice pela coluna 0 — esparso até 247; parser espelha o
sscanf legado: 22 conversões, a 23ª coluna é IGNORADA, `AffectTime/=4`; nomes em Latin-1).
Fórmulas puras em `combat/skill.go` (`ManaSpent`, `SkillBaseDamage` per-class Basedef.cpp:6998,
`SkillResistScale (150-res)*dam/100`, resist de mob /2; skill 79 = 180% do Damage; 97 = 15*level).
**Cast entra pelo `_MSG_Attack`** com sentinelas por alvo: `Dam=-2` melee, `-1` skill (o switch
antigo `SkillIndex!=0` estava ERRADO — melee real vem com SkillIndex=-1); validação fiel
(`handler/combat.go validateCast`): Passive, gate de classe `skillnum/24`, learn-mask
(`1<<(skillnum%24)`; ≥96 usa `1<<(skillnum-72)`), mana `BASE_GetManaSpent` (aborta sem MP; eco
carrega `CurrentMp@40`+`ReqMp@46`), skill 85 cobra 100×Special de gold, master de skill só TK com
bit 14 (`Special[2]/20` cap 15). Aprendizado (`handler/skill.go` + `misc.go applyBonus`): BonusType
0 (quirk: lote de 100 pontos com ScoreBonus≥300; Int/Con dão +2×pontos em MaxMP/MaxHP), 1
(Special, caps 200/255-com-8ª-skill e `3*(level+1)/2`), 2 (custo SkillPoint, exclusividade da 8ª
skill pos 7/15/23 + 7 anteriores + 50M gold); **SkillBonus é DERIVADO no login** (level*3 −
Σcustos, `deriveSkillBonus` = BASE_GetBonusSkillPoint, ProcessDBMessage.cpp:816) — não persiste;
SpecialBonus é incremental (+2/level, CMob.cpp:1128) e persiste. Level-up agora soma na BASE
(BaseMaxHP/MP) + grants. Livros Sephira no useItem (Vol 31-38 → bit Vol-7). **Affects**: ports de
`SetAffect`/`SetTick` (`world/affect.go`; só PLAYER recebe SetAffect, mob aceita tick; slot-reuse
por tipo; `Time=(AffectTime+1)*delay/100` em TICKS DE 8s — timer 500ms × gate %16 do legado);
aplicação no cast (`applyCastAffect`: aliados pulam agressivo, roll `rand()%100 >
RegenMP+AffectResist+difLevel`, clan 6 imune a player); estágio de score (`handler/affect_score.go`,
port do Buff Loop.txt: types 2/3/4/9/10/11/13/14/15/19/21/24/25/26/28/42) com contribuições
CACHEADAS read-time (`Aff*`/`Rsv` na Entity — nunca bakeadas no score flat persistido, mesmo
padrão do Divine → sem double-count no relogin); expiração na sub-cadência de 8s do tick
(`handler/affect_tick.go sweepAffects`, stagger conn%8; HoT 17, DoT 20 UNVERIFIED −1000/tick,
`Time<32400000` decrementa, Divine nunca); `MSG_SetHpDam` 0x018A (20B) flutua o heal/dano.
**`MSG_UpdateScore` agora vai completo** (152B pack(1) CONFIRMADO na fonte: `Affect[32]` u16
@body50 = `(Type<<8)|(Time&0xFF)` clamp 2550000, Guild@114, Resist@118, Magic@132, tail
Special[4]=0xCC quirk byte-exato). Affects persistem (rows na tabela `affect`; Divine continua
como deadline wall-clock) e re-hidratam no login + `SendAffect`. Hotbar `_MSG_SetShortSkill`
0x0378 (Skill1[4]→MOB.SkillBar persiste, Skill2[16]→Session.ShortSkill → ecoa no
CNFCharacterLogin@1034). Persistência: migração 0004 (`special SMALLINT[]`), proto Character
campos 21-27, SaveCharacter ampliado (score_bonus/special_bonus/learned_skill/special/skill_bar/
short_skill — **rebuild --no-cache nos dois lados!**). Blob do login patcha LearnedSkill@780/
bonuses@788-793/SkillBar@796/Special@Score+40. `Entity.Resist[4]` agora vem do template (@806).
**UNVERIFIED (perguntas prontas em `docs/migration/prompts/agent-prompt-skills.md`):** GetParryRate/
BASE_GetDoubleCritical (ainda confiamos no cliente), init de ReqMp, origem de SaveMana/Magic/
RegenMP de player, weather, Soul (29), DoT exato, unidade 8s
validar com buff curto no cliente real. Deferidos: types 1/5/6/7/12/22/27/29/36, skills
especiais 6/22/30/31/41(multi-alvo)/44/47/97-mortar/98/99/102, sweep de affect em mobs
(exceto o lifespan Type 24 de summon, varrido pelo `summonTick`). **Issue #21 (jul/2026):
Type 16 (transformação BM) FEITO** — `handler/transform.go` (pTransBonus, mesh 22-25/32 via
override read-time em `equipVisual`, DAMAGEMULTI = `AffDamageMultiPct`), broadcast no cast/expiry/
`/buffs` via `refreshEquip`; **evocações BM (InstanceType 11) FEITAS** — `handler/summon.go`
(GenerateSummon: templates `content.LoadBaseSummons`, Clan 4, `Entity.Summoner`, pSummonBonus,
affect 24 = lifespan, follow/assist em `summonTick`/`commandSummons`, mob-vs-mob em
`validTarget`/`mobAttack`, kill do pet credita exp ao dono); e o **vazamento de affects entre
personagens da mesma conexão** corrigido (`Entity.ResetAffects()` no char-login e logout).

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

### Bugs conhecidos / rastreador (`docs/migration/ingame-bugs.md`)
- **B1**: não reproduzido no cliente real com 2 usuários; manter apenas como histórico/observação.
- **B6**: não é bug; spawn por centro da última cidade visitada é comportamento esperado.
- **B5**: resolvido; `SELCHAR` envia `level-1` só no preview para compensar o display one-based do cliente.

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
- **Iteração 2 parte 1 FEITA (jul/2026) — clan hostility + ranged.** (a) **Hostilidade por clan**:
  `ParseMobBasics` lê `Clan@16`; `world/clan.go` porta `g_pClanTable[9][9]` (Basedef.cpp:207-220,
  0=hostil/1=amigo); `FindEnemyFromView` (`world/tick.go`) porta a geometria exata do
  `GetEnemyFromView` (CMob.cpp:1308-1358: janela `[x-4,x+5)`, clan 7/8 `[x-6,x+10)` assimétrica,
  scan y-externo, pula `RsvHide`, clan≥9 aborta o scan — 8 templates reais têm clan 9). Templates
  reais: clan 1/5 = agressivo, 2/3/4/6/7/8 = passivo até retaliação (o `Lobo` starter é clan 2 e
  NÃO agride — fiel; retaliação segue sem filtro de clan, como `SetBattle`). **Cuidado em testes:
  template clan 0 é AMIGO de player** (`clanTable[0][0]==1`) — o harness usa clan 5. (b) **Ranged**:
  `Entity.Range` cacheado no `SpawnMob` = max de EF_RANGE sobre os 16 equips (catálogo
  `ItemList.Ranges()` via `Config.ItemRanges` + efeitos de instância; port de `BASE_GetMobAbility`
  Basedef.cpp:2415/2523; EF_RANGE fora do multiplicador de refino e NUNCA no `BaseEffects`/score);
  o alcance dos mobs vem do item-modelo do Equip[0] (ex.: `Ciclope_Arqueiro` 242 → EF_RANGE 4).
  `mobBattle` porta a decisão do BattleProcessor (CMob.cpp:308-327): métrica `mobDistance` =
  `BASE_GetDistance`/`g_pDistanceTable` (Euclidiana arredondada, NÃO Chebyshev); `dis<=Range` →
  roll %100 SEMPRE consumido (paridade de rand); `Range>=4 && dis<=4 && roll>Dex` → **recua**
  (`mobRetreat`, 1+rand%2 por eixo, GetTargetPosDistance CMob.cpp:892-954); senão ataca (mesma
  MSG_Attack, sem projétil); fora de alcance persegue. Testes: `TestClanHostile`, `TestMobAISeams`
  (geometria), `TestFriendlyClanMobDoesNotAttack`, `TestBattleCode`, `TestMobDistance`,
  `TestRangedMobAttacksFromDistance`, `TestSpawnMobRange`.
- **Iteração 2 parte 2 FEITA (jul/2026) — pathfinding real.** Pacote novo `tmserver/internal/route`:
  `Bake` (port de `BASE_ApplyAttribute` Basedef.cpp:2624 — cada byte do AttributeMap cobre bloco 4×4
  do HeightMap; `att&2` → height 127 = `route.Blocked`) e `Next` (port 1:1 do `BASE_GetRoute`
  Basedef.cpp:6194-6482: line-walk guloso 8-direções, ordem EXATA dos 25 ramos — primária → fallbacks
  diagonais → bailout adjacente (:6379) → desvios de linha reta; passável = |Δh| < `MH`=8 EXCLUSIVO;
  alturas são **int8 SIGNED**; máx `MaxRoute-1`=23 passos; margem de borda 1). Wiring: `loadContent`
  carrega HeightMap.dat (4096²)+AttributeMap.dat (1024²) e faz o Bake uma vez → `handler.Config.Heights`
  (read-only; **nil = fallback** no passo Chebyshev cego — testes/boot sem mapas funcionam igual).
  `mobStep` mira o vizinho NW do alvo (`tx-1,ty-1` — quirk do `GetTargetPos` CMob.cpp:1034) e anda
  **1 passo de rota por tick de 1s** (mesma trajetória do original que pula `speed*8/4` tiles por
  ciclo de 8s — cada passo aplica a mesma regra gulosa; cadência UNVERIFIED); preso (`Route[0]==0`)
  = segura posição, NUNCA atravessa parede. `mobRetreat` também roteia o recuo. **Gate de dormência**
  (`wakeRadius`=12, cobre a janela clan 7/8 de offset +9): snapshot das posições dos players 1×/tick
  (scratch no Dispatcher, sem alloc); mob idle sem player perto pula o scan de aggro —
  `BenchmarkTickIdle10k` ≈ **0,2ms/tick com 10k mobs**. Greedy NÃO é A*: contorna obstáculo de
  1 célula (desvio lateral) mas fica retido por muro comprido — fiel. Testes: `route/route_test.go`
  (ramos golden), `TestMobRoutesAroundObstacle`, `TestMobHeldByWall`.
- **Iteração 2 parte 3 FEITA (jul/2026) — roaming/patrulha (M4).** Parser NPCGener COMPLETO
  (`content/npc.go`: MinuteGenerate/MaxGroup/Follower ("0"=nenhum)/RouteType/Formation +
  waypoints com o mapeamento do ParseString: **Start→Seg[0], Segment1-3→Seg[1-3], Dest→Seg[4]**;
  slots não usados ficam 0 e o walker PULA — bloco Start/Dest patrulha 0→4→0). `world.MobSpawn` +
  `SpawnMobAt` (SpawnMob virou atalho sem rota); cada instância recebe waypoints RANDOMIZADOS
  `seg − Range + rand()%(Range+1)` (viés p/ −Range, fiel a GenerateMob Server.cpp:3536-3546;
  boot usa math/rand de propósito — não queimar o stream do LCG) e **nasce no waypoint 0**.
  Entity ganhou RouteType/SegListX-Y/SegWait/SegProgress/SegDir/WaitTicks/**SegmentX-Y**(=waypoint
  atual e ÂNCORA de aggro+leash, CMob.cpp:292 — validTarget e o gate de aggro usam ele, não mais
  SpawnX)/GenIndex. IA: `mobRoam` porta o ramo não-summon do StandingByProcessor (CMob.cpp:156-229:
  chegou→arma SegWait→conta→`setSegment`→anda 1 passo de rota; preso→pula waypoint) e `setSegment`
  porta CMob.cpp:494-620 (RT1 reinicia, RT2/3 vai-e-volta via SegDir, RT4 circular, RT6 reset;
  RT0 no fim = PARA de andar — NÃO tocamos Merchant como o legado, divergência documentada; os
  paths OOB de RT1/RT3 do decompile foram clampados). Mob SEM rota volta ao âncora se deslocado
  (= MOB_RETURN emergente). Respawn preserva a rota da instância (respawnEntry carrega MobSpawn).
  Gate: `roamRadius`=20 (>ViewRange 18 — player nunca vê patrulha congelada; mobs sem player a 20
  ficam dormentes, divergência de otimização). RouteType real: 5894×RT2, 66×RT3, 63×RT0, 42×RT6,
  38×RT1. WaitTicks ≈ segundos no nosso tick de 1s (legado `WaitSec -= 6`/ciclo, cadência
  UNVERIFIED). RT5/summon FEITO na issue #21 (`handler/summon.go`, `summonTick`); RT3 no fim
  vira idle em casa (legado retorna 0x10000, não modelado). Testes: `TestSetSegment`, `TestMobRoamPatrolAndWait`,
  `TestMobReturnsHomeAfterCombat`, `TestPatrolVisibleToPlayer` (e2e), parser em `npc_test.go`.
- **Iteração 2 parte 4 FEITA (jul/2026) — respawn por gerador + grupos (M5, ÚLTIMO).**
  `world/generator.go`: `Generator` (recipe do bloco NPCGener + `CurrentNumMob` vivo) +
  `RegisterGenerators` + **`GenerateMob(idx)` port de Server.cpp:3442-3810 no LCG** com a ordem
  de rand fiel: (1) roll do tamanho do grupo `MinGroup+rand()%qmob` ANTES do check de população
  (:3489); (2) 2 rolls por waypoint com Range (X depois Y, viés −Range); (3) roll `%10` só se
  `Clan==1` (demote p/ clan 2, :3624 — short-circuit). **Quirk mantido:** o clamp de MaxNumMob
  ignora o líder → bloco MaxNumMob=1 fica com 2 mobs. Follower herda os waypoints randomizados
  do líder (offsets de `g_pFormation` deferidos c/ Formation); `emptyCellNear` porta o scan de
  caixas do GetEmptyMobGrid (GetFunc.cpp:2027, sem rand; check de altura-127 não feito no world).
  Contabilidade centralizada: `SpawnMobAt` incrementa CurrentNumMob (queue respawn também conta),
  `DespawnMob` decrementa (DeleteMob :7825) + limpa links de grupo (follower morto sai da
  PartyList; líder morto liberta os followers → voltam a ter aggro próprio). **Timer**: 1×/min
  (`d.tickCount%60`), bloco com `MinuteGenerate>0` dispara em `min%MG == idx%MG`
  (ProcessSecMinTimer.cpp:2727; hacks de skip {0,1,2,5,6,7} e ≥500 NÃO portados). **Política**:
  bloco de timer NÃO entra na fila de 15s (o timer repõe grupos inteiros); `MinuteGenerate<=0`
  (~45% dos blocos — no legado só spawna via evento!) usa a fila de 15s = divergência deliberada.
  Gate global `generateWorldCap`=20000 protege os slots. **Grupos em combate**: follower
  (`Leader!=0`) NUNCA agride sozinho (CMob.cpp:158); bater em qualquer membro arrasta o grupo
  (`setGroupBattle` = SetBattle Server.cpp:8013 + propagação de PartyList
  ProcessSecMinTimer.cpp:1882; caixa ±23; membro já engajado mantém o alvo; líder incluído no
  drag — pequena divergência coerente). **Boot** agora = `RegisterGenerators` + 1 `GenerateMob`
  por bloco (~equilíbrio 12-13k mobs; MaxNumMob real: 3858 blocos com 1); boot queima o LCG
  (não há ordem legada de boot p/ divergir — o original começa VAZIO). WaitTicks inicial =
  SegWait[0] (paridade Server.cpp:3665). Testes: `generator_test.go` (grupo/clamp-quirk/
  contabilidade/fila), `TestSetGroupBattleDragsGroup`, `TestFollowerDoesNotSelfAggro` (e2e),
  `TestGroupFocusesAttacker` (e2e wiring PROD), `TestGenerateMobsTimer`.
- **B3 ENCERRADO (iteração 2 completa).** Deferidos p/ frentes futuras: mob-vs-mob de guardas
  (summon-vs-mob FEITO na issue #21), Formation (g_pFormation), FightAction/DieAction (chat de mob),
  EnemyList[13]/SelectTargetFromEnemyList, KEFRA_BOSS, ~1400 templates de Leader sem arquivo,
  reveal de players ao cruzar visão andando (compartilha com B1).
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
