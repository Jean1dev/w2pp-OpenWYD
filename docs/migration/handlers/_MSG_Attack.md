# Contrato — `_MSG_Attack`

- **Gatilho:** Type `0x0367` (também `_MSG_AttackOne` 0x039D, `_MSG_AttackTwo` 0x039E — todos caem
  em `Exec_MSG_Attack`). Struct `MSG_Attack` (Fase 1 §3.5; inclui `SkillIndex`, `PosX/Y`,
  `TargetX/Y`, `Dam[MAX_TARGET=13]`).
- **Fonte:** `TMSrv/_MSG_Attack.cpp:21+`.

## Pré-condições e validações
1. `pUser[conn].TradeMode == 0` — não pode atacar em auto-trade (`:27-31`).
2. `pUser[conn].Mode == USER_PLAY` — senão `SendHpMode` (`:33-37`).
3. Vivo: `Hp != 0` **ou** `SkillIndex == 99` (ressurreição) — senão `SendHpMode` +
   `AddCrackError(1,8)` (`:40-45`).
4. **Anti-speed (cadência de ataque):** se `ClientTick < LastAttackTick + 800` (ms) → rejeita +
   `AddCrackError(1,107)` (`:59-66`). Limite de **800 ms** entre ataques.
5. **Sanidade de tick:** se `ClientTick < LastAttackTick - 100` → `AddCrackError(4,7)` (`:70-71`).
   Janela de tick válida: `ClientTick15sec .. CurrentTime+15000` (`:79-88`); fora → erro.
6. `ClientTick == SKIPCHECKTICK` pula as checagens (eventos internos do servidor).

## Efeitos colaterais
- Força `m->ID = ESCENE_FIELD` (broadcast no campo) (`:25`).
- Atualiza `pUser[conn].LastAttackTick = ClientTick`, `LastAttack = SkillIndex` (`:73-76`).
- Resolve dano por alvo (até `MAX_TARGET=13`): aplica fórmula de combate (Fase 4 §4), reduz `Hp` do
  alvo, marca `TargetKilled[]`; em morte de mob → `MobKilled` (exp/drop, Fase 4 §1-2).
- Consome MP da skill (`ReqMp`), aplica affects/efeitos de skill.

## Saídas (S→C)
- `_MSG_Attack` (broadcast aos jogadores na visão): posição, `AttackerID`, `Motion`, `SkillIndex`,
  `DoubleCritical`, e `Dam[]` (TargetID+Damage por alvo), `CurrentHp/Mp/Exp` do atacante.
- `_MSG_SetHpMp`/`_MSG_SetHpDam` para HP/MP; `_MSG_CNFMobKill` em morte; `_MSG_RemoveMob` ao despawn.

## Anti-cheat
- Cadência 800 ms + validação de `ClientTick` (anti-speedhack/macro). `AddCrackError` acumula "cra
  points" → logout (Fase 3 §2.1 / TMSrv-Core deep-dive).
- Dano é **server-authoritative**: o servidor recalcula; os campos de dano enviados pelo cliente
  são sobrescritos. (Confirmar que nenhum campo de dano do cliente é confiado — risco de dup/cheat.)

### Travas do legado no rewrite (`tmserver/internal/handler/combat.go`)
Até a restauração, o rewrite só tinha a cadência de 800 ms, e ela compara o `ClientTick` com o
tick anterior do próprio cliente. Um cliente alterado atacava na velocidade que quisesse, de
qualquer distância e com até 13 alvos de corpo a corpo num pacote. Agora:

| Trava | Legado | No rewrite | Recusa |
|---|---|---|---|
| `janela` | `:79-96` | fiel: `ClientTick` fora de `[agora-120000, agora+15000]` do relógio do **servidor** | `AddCrackError(1,107)`, sem eco. `LastAttackTick` já andou antes (ordem `:75`→`:79`), então quem adianta o relógio se tranca na cadência |
| `distancia` | `:424-426` | fiel: distância `BASE_GetDistance` entre `PosX/Y` e `TargetX/Y` **do pacote** > 23 (o `Range` do jogador é sobrescrito com 23 em `CMob.cpp:696-697`) | o ataque inteiro, em silêncio. Só barra cliente que diz onde está |
| `tela` | `:347-351` | fiel, com a exceção da skill 42: alvo a mais de 33 casas da posição **do servidor** de quem ataca | o alvo sai do ataque e o cliente de quem bateu recebe `RemoveMob` tipo 1. É a única trava que não depende do cliente |
| `segundo_alvo` | `:431` | **divergência deliberada**: do segundo alvo de corpo a corpo em diante, só Caçadora (classe 3) ou quem tem a skill `0x40` | o alvo extra fica com dano 0, em silêncio, **sem** crack error |

Sobre a `segundo_alvo`: a linha do legado tem `m->Size < sizeof(MSG_AttackTwo)`, e o laço de
`:297-306` só lê a segunda entrada de um pacote maior que o AttackOne. Então ela só disparava num
tamanho malformado entre os dois; o pacote cheio de 13 alvos passava. No rewrite a conta de entradas
é `(len-48)/8`, e a letra nunca dispararia. O que voltou foi a intenção. O crack error do legado
ficou de fora até se saber se o cliente real manda segundo alvo de corpo a corpo em outra classe;
a recusa é contada e vai ao log.

**Crack error, como o legado conta:** `AddCrackError(conn, peso, tipo)` SOMA o peso em `NumError`
(`Server.cpp:1006`) e só derruba em 2.000.000.000 (`:1008`), ou seja, na prática nunca: o legado
registra e mantém o jogador. O rewrite chegou a contar +1 por chamada e derrubar em 10 (um
placeholder sem fonte), o que derrubou cliente honesto com rede ruim; voltou ao legado em
`world.AddCrackError`. Quem protege o servidor é a recusa de cada trava, não a desconexão.

Cada recusa conta por conta e por trava (`Session.AttackRefusals`). O log registra a 1ª, a 10ª e a
100ª recusa (`attack refused by a restored legacy gate`) e o total na desconexão
(`session attack refusals`). Uma linha dessas com conta de jogador honesto, principalmente na
`janela`, que depende de campo do cliente 12000 ainda não confirmado por captura, é motivo pra
reverter antes de virar suporte.

## Riscos (migração)
- A fórmula de dano (`BASE_GetDamage`/`BASE_GetSkillDamage`, pipeline do golpe, acerto/parry/reflect)
  está documentada na **Fase 4 §4** (fonte real em `Basedef.cpp`/`_MSG_Attack.cpp`); usa `rand()` →
  validar por **distribuição** com golden cases (Fase 8). Resta tabelar coef. Dex/Str por classe×arma.
- `SkillIndex == 99` como "ressurreição" é constante mágica — preservar.
- A janela de tick depende de `CurrentTime` (clock do servidor) — reproduzir a unidade (ms).
- **UNVERIFIED — gate de "safe zone"/PK ainda não portado.** O legado zera o dano quando o
  ATACANTE está num tile com o bit de attribute-map (`GetAttribute(...) & 64`) **e** o alvo não
  está `PKMode`/`Guilty` (`_MSG_Attack.cpp:405-419`), com bypass total durante
  RvR/Castle/GTorre/newbie event. Nenhum desses campos (`PKMode`, `GetGuilty`, bit de attribute-map
  por tile, estado de guerra) existe no servidor Go hoje. Uma tentativa anterior de aproximar isso
  com os retângulos de `world.Village()` (issue #67) bloqueava dano PvP incondicionalmente perto de
  toda cidade/spawn, já que nenhum bypass do legado era alcançável — foi removida (ver B12: não
  travar gameplay em campo não-implementado/não-verificado). Até que attribute-map + PKMode + Guilty
  + estado de guerra sejam implementados, dano PvP não tem proteção de safe-zone (funciona em
  qualquer lugar).
