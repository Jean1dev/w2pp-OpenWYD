# Agent prompt — descobrir o que faz o NICK PISCAR VERMELHO (PK) no cliente 12000

Cole isto no Claude Code da máquina Windows (que tem a FONTE COMPLETA do WYD + o dumper
`_layout_probe/dump_layout.cpp`). Salve o resultado em `captura-wyd-pk-nick.md`.

---

## Contexto

Estou migrando o servidor do WYD para Go (`w2pp-openwyd`), mirando o **cliente `WYD.exe`
original sem mods, ClientVersion 12000** (protocolo base 7640). Header CPSock = 12 bytes.
Já funcionam: login, criação/seleção de char, entrar no mundo, andar, ver outros players,
NPCs, lojas, combate, EXP.

**Problema (issue #59):** o **nick do PRÓPRIO personagem aparece VERMELHO PISCANDO** (o
indicador de "PK"/chaos) **assim que entra no mundo, mesmo num personagem recém-criado que
NUNCA apertou a tecla K e nunca atacou ninguém**. E não some sozinho depois de ~1-2 min.

## O que eu JÁ TENTEI no servidor Go e NÃO resolveu (confirmado no cliente real)

1. **Enviei `_MSG_PKInfo` (0x0166, `102|FLAG_GAME2CLIENT`, MSG_STANDARDPARM) com `Parm=0`**
   para o próprio conn no login e emparelhado com todo CreateMob de player. Confirmei na
   captura de bytes (cliente sintético) que o pacote sai com `Parm=0`. **O nick continuou
   piscando.**
2. **Zerei o byte `STRUCT_SCORE.ChaosRate` (offset 15 dentro de STRUCT_SCORE)** no blob de
   `MSG_CNFClientCharacterLogin` (nos dois scores: BaseScore e CurrentScore) e no CreateMob.
   Os templates BaseMob trazem esse byte como `0xCC` (204, lixo de memória não-inicializada).
   Confirmei na captura que virou `0`. **O nick continuou piscando.**
   - OBS: no `Source/Code/NPTool/ProcessMessage.cpp:159` esse mesmo byte
     (`mob->BaseScore.ChaosRate`) é exibido no editor de NPC como campo **`IDC_ERegen`**
     (Regen!). Isso me faz suspeitar que o byte "ChaosRate" na verdade é REGEN, e NÃO o que
     controla a cor do nick. **Preciso que você confirme.**

Ou seja: nem `_MSG_PKInfo` nem `ChaosRate` (do jeito que interpretei) controlam o piscar do
próprio nick no cliente 12000. Estou às cegas quanto ao mecanismo real.

## Pista forte

`_MSG_PKMode = (153 | FLAG_GAME2CLIENT | FLAG_CLIENT2GAME)` (0x0399) é **BIDIRECIONAL** — o
servidor PODE enviar `_MSG_PKMode` para o cliente. Suspeito que o piscar do PRÓPRIO nick seja
um estado LOCAL de "modo PK" do cliente (ligado pela tecla K) e que o servidor sincronize/
force esse estado via `_MSG_PKMode` enviado ao cliente. Mas não sei — **confirme.**

## PERGUNTAS (responda com evidência `arquivo.cpp:linha` da fonte COMPLETA)

1. **Qual pacote/campo S→C faz o cliente 12000 pintar o PRÓPRIO nick de vermelho piscando?**
   É `_MSG_PKMode` (0x0399) enviado pelo servidor? `_MSG_PKInfo` (0x0166)? um campo de
   `STRUCT_SCORE`/`STRUCT_MOB` no `MSG_CNFClientCharacterLogin`? um item especial no inventário
   (ex.: `Carry[KILL_MARK=63]`, sIndex 547)? ou é 100% estado LOCAL do cliente (tecla K) que o
   servidor não controla?

2. **O servidor envia `_MSG_PKMode` para o cliente em algum momento** (login, entrar no mundo,
   toggle)? Cole TODA função que faz `AddMessage`/`SendOneMessage` com `Type = _MSG_PKMode`.
   Se sim, com que `Parm`? Isso liga ou desliga o piscar no cliente?

3. **O byte em `STRUCT_SCORE` offset 15 (chamado `ChaosRate` na nossa cópia) é chaos ou
   regen?** Cole a definição EXATA de `STRUCT_SCORE` (todos os campos, offsets via o dumper,
   tamanho total). Onde esse byte é ESCRITO no servidor? Onde o cliente o lê (se souber)?

4. **Fluxo completo do "guilty/chaos/PK" que faz o nick piscar:**
   - Onde o servidor SETA o estado ao MATAR/ATACAR outro player? Cole a função (procure
     `SetGuilty`, `SetPKPoint`, `Carry[KILL_MARK]`, `stEffect`, e QUALQUER call-site — na nossa
     cópia parcial `SetGuilty`/`SetPKPoint` existem em `GetFunc.cpp` mas **não têm nenhum
     call-site**, então a lógica de "atacou player → vira guilty" está só na fonte completa).
   - **Quanto tempo** o estado dura e **como decai** (o valor volta a 0)? Existe um timer/
     decremento por segundo/minuto? Cole a constante e a função de decaimento (`GetGuilty`
     zera se `>50` — mas o que INCREMENTA e em que ritmo DECREMENTA?).
   - Qual VALOR/threshold do campo faz o cliente piscar (ex.: `!= 0`? `>= X`?).

5. **Layout byte-exato dos pacotes** que carregam o estado do nick, via o dumper
   (`sizeof`/`offsetof`), para eu conferir meus offsets:
   - `MSG_CNFClientCharacterLogin` (o blob de login) — tamanho total e onde ficam o
     `STRUCT_MOB`/scores dentro dele.
   - `MSG_CreateMob` (o snapshot de outros players).
   - `MSG_STANDARDPARM` (usado por PKInfo/PKMode).
   - `_MSG_PKInfo` e `_MSG_PKMode`: o valor NUMÉRICO do `Type` no build 12000 (com
     `FLAG_GAME2CLIENT=0x100` / `FLAG_CLIENT2GAME=0x200`) — quero confirmar que 0x0166 e 0x0399
     batem, ou se o 12000 usa outro número.

6. **No login do original**, liste TODOS os pacotes S→C que o servidor manda relacionados a
   PK/chaos/nick (ex.: `SendPKInfo(conn, conn)`, algum `_MSG_PKMode`), na ORDEM em que saem,
   com o `Parm`/valor de cada um. (No `ProcessDBMessage.cpp` perto do envio do
   `MSG_CNFClientCharacterLogin` — no nosso parcial isso é ~linha 1021 com `SendPKInfo(conn,
   conn); SendGridMob(conn);`.)

## Formato da resposta

Salve em `captura-wyd-pk-nick.md`: para cada pergunta, a resposta direta + o trecho de código
(`arquivo.cpp:linha`) que a comprova. Priorize a pergunta 1 (o mecanismo) e a 4 (liga/desliga/
decai) — é o que destrava o fix.
