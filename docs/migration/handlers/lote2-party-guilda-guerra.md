# Contratos — Party, Guilda & Guerra (lote 2)

> Grupo, guilda, aliança/guerra e desafio/imposto de zona. Fonte: `TMSrv/_MSG_*.cpp`.
> `MAX_PARTY`, `PARTY_DIF`, `MAX_GUILD`, `g_pGuildZone[]` em `Basedef.h`/`Server.*`.

---

## `_MSG_SendReqParty` (0x037F) — convidar para party
- **Gatilho/struct:** `MSG_SendReqParty` (`PartyID`=líder, `unk`=alvo, `MobName`).
- **Validações:** `PartyID==conn` e em `(0,MAX_USER)`; líder não pode já ter party (`Leader`); alvo em
  `(0,MAX_USER)`, `USER_PLAY`, sem party; **regra de nível**: diferença `< PARTY_DIF` (com ajuste de
  `MAX_CLEVEL` por `ClassMaster`, exceto MORTAL/ARCH) ou níveis ≥1000 ou mesma `ClassMaster`; bloqueio
  por área de Battle Royale (`BrState`/`BRItem` + região).
- **Efeitos:** `pMob[target].LastReqParty = conn`; `SendReqParty(target, ...)` (envia convite).
- **Risco:** a fórmula de nível efetivo (com `MAX_CLEVEL`/`ClassMaster`) é compartilhada com
  `AcceptParty` — extrair para função única (Fase 4). Preservar para paridade.

## `_MSG_AcceptParty` (0x03AB) — aceitar convite
- **Gatilho/struct:** `MSG_AcceptParty` (`LeaderID`, `MobName`).
- **Validações:** `LeaderID` em `(0,MAX_USER)`; nome bate com o líder; **`LeaderID ==
  pMob[conn].LastReqParty`** (senão `CrackLog "PARTYHACK"` — anti-forja de aceite); líder sem party;
  líder `USER_PLAY`; eu sem party; **mesma regra de nível** do `SendReqParty`.
- **Efeitos:** insere `conn` no primeiro slot livre de `pMob[leader].PartyList[]`; `pMob[conn].Leader =
  leader`; dispara `SendAddParty` para todos os membros (sincroniza a lista). Party cheia →
  "Partys_Full". Loga.
- **Anti-cheat/risco:** o gate `LastReqParty` impede entrar em party sem convite — preservar. Lógica
  de slots/broadcast é detalhada; replicar 1:1 (Fase 8).

## `_MSG_RemoveParty` (0x037E) — sair/expulsar
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = target`).
- **Validações:** `target` fora de `(0,MAX_USER)` → vira `conn`; se `target != conn`, precisa estar na
  `PartyList` (senão vira `conn`).
- **Efeitos:** se eu sou líder (`Leader`) → `RemoveParty(conn)` (dissolve/saída do líder); senão
  `RemoveParty(target)`.
- **Risco:** semântica líder-vs-membro embutida; documentar `RemoveParty` (em `Server.cpp`).

## `_MSG_InviteGuild` (0x03D5) — convidar para guilda
- **Gatilho/struct:** `MSG_STANDARDPARM2` (`Parm1=TargetID`, `Parm2=InviteType` 0..3).
- **Validações:** `TargetID` em `(0,MAX_USER)`; `InviteType` em `[0,4)`; convidante **tem** guilda;
  alvo **sem** guilda; **mesmo clan**; `GuildLevel>0` (oficial); `InviteType!=0` exige líder
  (`GuildLevel==9`); **bloqueio aos domingos** (`tm_wday==0`); custo em gold
  (4.000.000 normal / 100.000.000 para tipos especiais).
- **Efeitos:** debita custo; `pMob[Target].MOB.Guild = minha guilda`, `GuildLevel=0` (membro); envia
  mensagem de boas-vindas (nome da guilda via `BASE_GetGuildName`) e **multicast** `CreateMob` (atualiza
  a tag). Loga.
- **Risco:** custos/dia da semana hardcoded → config. Mutação direta de `MOB.Guild` no alvo (sem ida
  ao DBSrv aqui; persistência no save). Preservar regra de clan/oficial.

## `_MSG_GuildAlly` (0x0E12) — aliança de guilda
- **Gatilho/struct:** `MSG_GuildAlly` (`Guild`, `Ally`).
- **Validações:** `Guild`/`Ally` em `(0,65536)`; o jogador deve ser **líder** da própria guilda
  (`MOB.Guild==Guild` && `GuildLevel==9`).
- **Efeitos:** monta `MSG_STANDARDPARM2` (`Parm1=Guild`,`Parm2=Ally`) e **envia ao DBSrv** (a aliança
  é persistida/propagada pelo DBSrv).
- **Risco:** relay para o DBSrv → vira RPC no `dbServer`. Sem checagem de `Mode`.

## `_MSG_War` (0x0E0E) — declarar guerra de guilda
- **Gatilho/struct:** `MSG_STANDARDPARM2` (`Parm1=Guild`, `Parm2=Enemy`).
- **Validações:** idênticas a `GuildAlly` (ranges + líder da própria guilda).
- **Efeitos:** monta `MSG_STANDARDPARM2` (`Type=_MSG_War`) e **envia ao DBSrv**.
- **Risco:** mesmo padrão de `GuildAlly`; consolidar no `dbServer` (declaração de guerra/aliança).

## `_MSG_Challange` (0x028E) — desafio de zona / recolher imposto
- **Gatilho/struct:** `MSG_STANDARDPARM` (`Parm = target` NPC).
- **Validações:** `target` em `(0,MAX_MOB)`; `zone = pMob[target].MOB.BaseScore.Level` em
  `[0,ValidGuild)` e `zone!=5`.
- **Efeitos (depende de `WeekMode`):**
  - `WeekMode==4`: se o jogador é **líder da guilda cobradora** da zona, **recolhe o imposto**
    acumulado (armazenado em `pMob[GuildImpostoID[zone]].MOB.Exp`): converte cada 1e9 de Exp em item
    `sIndex 4011` (ou em gold se <1e9), respeitando teto de 2G. Senão mostra o valor do imposto.
  - `WeekMode==5`: abre o sinal de desafio (`_MSG_ReqChallange`).
  - `WeekMode` "ativo": mostra campeão/desafiante da zona (`SendSay`).
- **Risco:** **`Exp` usada como cofre de imposto** e `BaseScore.Level` do NPC usado como índice de
  zona — quirks de modelagem que o schema novo deve representar explicitamente (Fase 2). `WeekMode` e
  índices (4011) hardcoded. Caminho de economia → golden cases.

## `_MSG_ChallangeConfirm` (0x028F) — confirmar desafio
- **Gatilho/struct:** `MSG_STANDARDPARM2` (`Parm1 = target` NPC).
- **Validações:** `target` em `(0,MAX_MOB)`; `zone = pMob[target].MOB.Merchant - 6` em
  `[0,ValidGuild)` e `zone!=4`.
- **Efeitos:** `Challange(conn, target, 0)` (efetiva o desafio à zona — lógica em `Server.cpp`).
- **Risco:** `zone` derivada de `Merchant-6` aqui vs `BaseScore.Level` no `_MSG_Challange` —
  inconsistência de fonte de "zona" entre os dois; documentar/unificar na migração.
