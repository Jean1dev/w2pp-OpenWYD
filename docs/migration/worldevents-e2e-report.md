# Validação E2E dos eventos de mundo (issue #116)

Este documento registra o que foi **validado ponta a ponta contra uma stack viva** dos eventos de
mundo, o que **não é alcançável sem o cliente real** e por quê, e o checklist manual que sobra para
quem tem o `WYD.exe` na mão.

Artefatos:

- `tmserver/internal/world/worldevents_e2e_test.go` — cliente CPSock headless + os casos de teste.
- `scripts/e2e-worldevents.sh` — sobe o que falta, abre a janela do calendário e roda a suíte.

## 1. Por que um cliente headless

O contrato observável de um evento de mundo é **o frame S→C**. Um servidor que calcula o estado
certo e manda o pacote errado é indistinguível de um servidor quebrado num teste unitário — é a
lição registrada no `SESSION-PRIMER.md` §5.1 ("servidor-correto ≠ cliente-correto"). Os testes aqui
abrem um socket de verdade, fazem o handshake INITCODE, `MSG_AccountLogin`, `MSG_CreateCharacter` e
`MSG_CharacterLogin` como o `WYD.exe` faz, entram em `USER_PLAY` e **conferem os bytes na rede**.

O que isso cobre: a metade servidor de "o que o cliente veria". O que isso **não** cobre: a
renderização (céu, barra de vida, painel de aviso). Essa metade continua exigindo uma pessoa no
cliente real — ver §5.

## 2. Resultado

Stack: `docker compose` completo (PostgreSQL + dbServer + binServer + tmServer), conteúdo
`Release/` montado, 12.824 mobs gerados, conta `test` promovida a `admin`.

| Evento (fase da issue #116) | Frame S→C | Status |
| --- | --- | --- |
| Clima — broadcast de mudança | `MSG_UpdateWeather` (0x018B) | ✅ validado |
| Clima — guarda de idempotência | (ausência de frame) | ✅ validado |
| Clima — rejeição de valor inválido | `MSG_MessageChat` | ✅ validado |
| Clima — snapshot no login | `MSG_CNFCharacterLogin` @1032 | ✅ validado |
| Newbie — handicap de HP no spawn | `MSG_CreateMob` (Hp/MaxHp) | ✅ validado |
| Newbie — recarga via config do portal | (efeito no spawn seguinte) | ✅ validado |
| Torre — aviso e início da guerra | `MSG_MessageChat` | ✅ validado |
| Reino (RvR) — muro e dano de área | `MSG_EnvEffect` + dano | ❌ inalcançável (§3) |
| Reino — morte do rei | `MSG_MessageChat` por área | ❌ inalcançável (§3) |
| Castelo/Zakum — abertura da quest | `MSG_StartTime` (0x03A1) | ❌ inalcançável (§3) |
| Evento de item (config do portal) | slot + `MSG_MessageChat` | ❌ inalcançável (§3) |

### 2.1 Clima (fase 1)

Quatro casos, todos passando:

- **Broadcast.** `/gm weather 1|2|0` produz exatamente um `MSG_UpdateWeather` por sessão em jogo,
  corpo de 4 bytes com o valor em `int32`, e `HEADER.ID = ESCENE_FIELD (30000)`. O `HEADER.ID`
  importa tanto quanto o `Type`: é ele que roteia o pacote para o handler de cena do cliente
  (`SendFunc.cpp:1669-1696`).
- **Idempotência.** Reforçar o clima que já está ativo **não** coloca um segundo pacote na rede —
  a guarda `ForceWeather != CurrentWeather` do legado. Um pacote redundante reinicia a transição de
  céu no cliente, então a guarda é visível.
- **Validação.** `/gm weather 9` responde com a linha de erro e **não** transmite. Divergência
  deliberada do legado, que mandava qualquer `int` direto para o cliente (`imple.cpp:1122-1129`).
- **Snapshot de login.** Um cliente que entra **depois** da mudança aprende o valor pelo
  `MSG_CNFCharacterLogin` (offset 1032 do corpo), não por um broadcast que ele nunca viu. Sem isso,
  quem faz relog no meio de uma tempestade renderiza céu limpo.

### 2.2 Newbie (fase 2)

O único efeito **visível ao cliente** dessa fase é o handicap de spawn: com `NewbieEventServer`
ligado, monstro abaixo do nível 120 nasce com 3/4 do HP e `MaxHp` intacto
(`Server.cpp:3326-3327` → `world.applyNewbieHandicap`). Isso chega ao cliente dentro do
`MSG_CreateMob`, e é o que faz a barra de vida do monstro nascer em três quartos.

Medido na rede, com o mesmo template (`/gm spawn 1`, mob nível 4):

- evento desligado → `100/100 HP`
- evento ligado → `75/100 HP`

O caminho exercitado é o **real**: a flag vem da config do portal (`world_event_config`), o
`world_event_meta.version` é incrementado, e o `pollWorldEventConfig` (a cada 15 ticks) recarrega no
servidor **em execução** — confirmado no log com `world event config reloaded version=1`.

O bônus de EXP da mesma flag **não tem frame próprio** e não é observável de um cliente; ele
continua coberto pelos testes unitários de `internal/level`.

### 2.3 Torre (fase 4)

A janela é "dia útil, hora 20, minuto ≤ 5, com `NewbieEventServer` ligado"
(`CWarTower.cpp:203` → `handler/towerwar.go`). O evento lê `time.Now()`, ou seja, **a hora local do
container** — então a janela é alcançável sem esperar uma terça à noite e **sem mockar relógio em
código de produção**: `scripts/e2e-worldevents.sh` procura um fuso em que agora sejam 20:0x, sobe um
segundo tmServer nele e conecta o cliente headless.

<!-- RESULTADO-TORRE -->

Resultado observado na stack viva: o aviso `"[Torre] A guerra da torre comecara em breve."` e,
273,75 segundos depois, o início `"[Torre] A guerra da torre comecou."` chegaram ao cliente
headless como `MSG_MessageChat`. O runner preserva e restaura o valor anterior de
`newbie_event_enabled`, mantendo-o ligado durante toda a execução para que a recarga periódica de
configuração não feche a janela antes da transição do minuto 6.

## 3. O que não é alcançável de um cliente headless

Não são falhas: são eventos cujo gatilho exige estado que nenhum comando disponível produz. Listar
isso explicitamente é o ponto — o risco real é um evento passar por "validado" porque o teste
simplesmente não conseguiu dispará-lo.

- **Muro do reino (RvR).** O pulso de `MSG_EnvEffect` + dano só ocorre com um jogador **dentro** das
  caixas `{1050,2108,1098,2146}` (azul) e `{1204,1947,1245,1988}` (vermelha). Não existe comando de
  GM de teleporte por coordenada (`gm.go` tem `goto <jogador>` e `summon <jogador>`, ambos relativos
  a alguém online), e nenhum `/cidade` do `teleportCmds` cai lá dentro. Andar até lá seriam ~1000
  tiles de pacotes de movimento com colisão real.
- **Morte do rei.** `kingdomKingKilled` depende de matar os geradores 8/9 (Harabard/Glantuar). Dá
  para **spawnar** o rei com `/gm spawn`, mas matá-lo exige um laço de combate real.
- **Castelo/Zakum (fase 5).** `openCastleQuest` só é chamado por `gate.go` quando o jogador abre um
  portão com `EF_KEYID == 10` **carregando uma chave cujo `EF_QUEST` nomeia o nível da quest**.
  `/gm item` entrega item **sem efeitos** (`gm.go:178`, "by index, no effects"), então `itemQuestID`
  devolve -1 e o portão nunca abre quest. A chave legítima vem de `castleKeyDrop`, que por sua vez
  exige uma quest já ativa — circular.
- **Drop do evento de item.** `tryWorldEventDrop` é chamado de `mobkilled.go:75`; exige matar um mob.

Todos os quatro se destravam com a mesma peça: **um caminho de combate no cliente headless** (mirar
+ atacar até a morte) e **um `/gm item` que aceite efeitos**. Ficam registrados como próximo passo,
não como cobertura existente.

## 4. Observação operacional: `-newbie-event` perde para o banco

`w.SetNewbieEvent(*newbieEvent)` roda no boot (`main.go:311`) e é **sobrescrito poucos
milissegundos depois** por `ApplyWorldEventConfigBoot` (`main.go:345`), que aplica
`world_event_config.newbie_event_enabled`. O comentário em `main.go:307-310` diz que isso é
intencional (o portal é a autoridade), mas o efeito prático merece registro:

> Com `-dbserver` ligado — que é a topologia normal — a flag `-newbie-event` **não tem efeito
> algum**, porque a linha de config nasce com `newbie_event_enabled = false`. Ligar o evento newbie
> (e, por tabela, a guerra da torre, que depende dele) é feito **no banco/portal**, não na linha de
> comando.

Foi exatamente isso que fez a primeira tentativa da janela da torre não disparar. Não é bug de
código; é uma pegadinha de operação que vale estar escrita.

## 5. Checklist manual no `WYD.exe`

O que os testes acima **não** conseguem afirmar é o lado da renderização. Com a stack de pé
(`./scripts/run-local.sh`) e uma conta promovida a `admin`:

1. **Clima.** `/gm weather 1`, depois `2`, depois `0`. O céu deve mudar em cada um e **não** piscar
   ao repetir o mesmo valor. Confirmar que `/gm weather 9` não muda nada.
2. **Clima no relog.** Com o clima em 2, sair para a seleção de personagem e entrar de novo: o céu
   deve **já** estar no clima 2 ao entrar, sem um frame de céu limpo antes.
3. **Newbie.** Com `newbie_event_enabled = true`, spawnar um monstro abaixo do nível 120 e conferir
   que a barra de vida nasce em três quartos.
4. **Torre.** Dentro da janela (§2.3), confirmar que os avisos `[Torre]` aparecem no painel de chat
   e que a torre aparece no terreno em `{2445,1850}-{2546,1920}`.
5. **Reino e Castelo.** Sem cobertura automatizada (§3) — validar inteiramente à mão.

## 6. Como reproduzir

```bash
./scripts/run-local.sh
docker compose exec db psql -U postgres -c "UPDATE account SET role='admin' WHERE name='test'"
docker compose run --rm dbserver seed-account -name test2 -pass test123   # snapshot de login

./scripts/e2e-worldevents.sh weather
./scripts/e2e-worldevents.sh newbie
./scripts/e2e-worldevents.sh tower      # espera a janela abrir (até ~30 min)
./scripts/e2e-worldevents.sh probe      # "a janela da torre está aberta agora?"
```

Os testes ficam atrás da tag de build `e2e`, então `go test ./...` normal não os enxerga.
