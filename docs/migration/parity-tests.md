# Fase 8 — Paridade / Golden Cases (w2pp-OpenWYD)

> **Objetivo:** bateria de casos que validam que o servidor novo == o atual. Crítico no big-bang.
> Cada caso = **estado inicial + pacote(s) de entrada → estado final + pacote(s) de saída**.

---

## 1. Formato do caso de teste (golden case)

```yaml
id: refine_anct_success
subsystem: combine
seed: 12345                 # seed do RNG (ver §4)
given:                      # estado inicial
  account: { name: TEST, pass_hash: ... }
  char:    { slot: 0, level: 100, carry[3]: {sIndex: 1100, sanc: 6}, ... }
when:                       # pacotes de entrada (hex do payload já DESOFUSCADO)
  - type: 0x03A6            # _MSG_CombineItem
    fields: { Item: [...], InvenPos: [3, 4] }
then:                       # estado final + saídas esperadas
  state:
    char.carry[3]: { sIndex: <joia+extra>, sanc: 7 }
  out:
    - type: 0x03A7          # _MSG_CombineComplete
      fields: { Parm: 1 }   # sucesso
    - type: 0x0182          # _MSG_SendItem (slot atualizado)
```

Regras:
- Os payloads de entrada/saída são comparados **desofuscados** (a camada de transporte é testada
  separadamente — ver §3). Assim o caso independe do `iKeyWord` aleatório.
- Estado é comparado campo-a-campo nas structs relevantes (`STRUCT_MOB`, `CUser`, `pItem[]`).
- Incluir casos de **borda** e de **erro** (não só caminho feliz).

---

## 2. Casos concretos por subsistema

### 2.1. Login / conta
- `login_ok`: conta+senha válidas, `ClientVersion=7640` → `_MSG_CNFAccountLogin` + SELCHAR correto.
- `login_bad_pass`: senha errada → `_MSG_DBAccountLoginFail_Pass`; após 3× → `_NN_3_Tims_Wrong_Pass`.
- `login_bad_version`: `ClientVersion != 7640` → `_NN_Version_Not_Match_Rerun` + close.
- `login_already_playing`: conta já online → `_MSG_AlreadyPlaying`.
- `login_wrong_mode`: `_MSG_AccountLogin` fora de `USER_ACCEPT` → "Login now" + CrackLog.

### 2.2. Personagem
- `create_ok`: slot vazio, nome válido → `_MSG_CNFNewCharacter` + char no arquivo de conta.
- `create_invalid_name`: nome inválido → `_MSG_NewCharacterFail`.
- `delete_ok` / `delete_wrong_pass`: com/sem senha correta.
- `charlogin_ok`: slot válido → `_MSG_CNFCharacterLogin` (STRUCT_MOB, pos inicial, skillbar).
- `charlogin_billing_block`: `BILLING==2 && level>=FREEEXP` → `_NN_Wait_Checking_Billing`/404.

### 2.3. Movimento
- `move_ok`: rota válida → broadcast `_MSG_Action` com mesma rota; `pMobGrid` atualizado.
- `move_speedhack`: ticks rápidos demais → `AddCrackError`, sem broadcast.
- `move_out_of_bounds`: `GridX>=4096` → rejeitado.

### 2.4. Combate
- `attack_hit`: dano determinístico (seed) → `_MSG_Attack` com `Dam[]` esperado, HP do alvo reduz.
- `attack_too_fast`: `< 800ms` desde o último → `AddCrackError(1,107)`, sem efeito.
- `attack_while_dead`: `Hp==0 && SkillIndex!=99` → `AddCrackError(1,8)` + `SendHpMode`.
- `mob_killed`: morte → EXP distribuída (Fase 4 §1, tabela de divisores por nível/tier) + drop.

### 2.5. Itens (drop/get/dup)
- `drop_ok`: item dropável → `_MSG_CNFDropItem` + `pItem[]` criado + slot zerado.
- `drop_blacklisted`: sIndex ∈ {508,509,522,526..537,446,747,3993,3994} → bloqueado.
- `get_ok`: a ≤3 células → `_MSG_CNFGetItem`, item sai do chão.
- `get_too_far`: distância >3 → falha (sem pickup).
- `get_decayed`: item já removido → `_MSG_DecayItem`.
- `dup_race` (regressão de concorrência): dois `get` no mesmo item → só um sucede.

### 2.6. Refino / combine (RNG)
- `refine_success` / `refine_fail`: com seed fixa, comparar `parm` (1/2) e estado do item.
- `combine_invalid`: receita não casa → `_NN_Wrong_Combination` + `parm=0` (insumos NÃO consumidos).
- `combine_consumes_on_fail`: receita válida + roll de falha → insumos consumidos, sem item.
- Validar a **distribuição** de `rand()%115` (achatamento ≥100⇒-15) sobre N amostras.

### 2.7. Trade
- `trade_ok`: ambos confirmam → swap atômico de itens+gold.
- `trade_cancel`: `_MSG_QuitTrade` → estado revertido em ambos.
- `trade_dup`: tentar dropar/usar item durante trade → trade cancelado (Fase 5 DropItem).

### 2.8. War / Castle
- `gtorre_start`: relógio chega em `GTorreHour` → janela abre, estado `TowerStage` muda.
- `castle_keydrop`: morte do boss → `CCastleZakum::KeyDrop` dropa prêmio.
- (Capturar sequências reais — UNVERIFIED, Fase 6 §7-8.)

---

## 3. Testes da camada de transporte (separados)

Independentes do gameplay; validam Fase 1 byte-a-byte:
- `header_roundtrip`: encode→decode de um payload conhecido reproduz os bytes originais.
- `keyword_transform_vectors`: para `iKeyWord` fixo, o payload cifrado deve casar com um vetor de
  referência **capturado do servidor atual** (prova a `pKeyWord` + transform).
- `checksum_value`: o `CheckSum` calculado bate com o do servidor atual para o mesmo payload.
- `initcode_gate`: conexão sem `0x1F11F311` é rejeitada.
- `framing_partial`: pacote chega em pedaços → desenquadra só quando `Size` completo.
- `oversize`: `Size > 8192` ou `< 12` → `ErrorCode=2`.

---

## 4. Não-determinismo (RNG) e como testar

Fontes de RNG (libc `rand()` do MSVC, estado global):
- Drop (gold/item/evento): `MobKilled.cpp` `rand()%...`.
- Refino/combine: `_MSG_CombineItem.cpp:80` `rand()%115`.
- Ofuscação: `iKeyWord = rand()%256` (`CPSock.cpp:535`) — **irrelevante para paridade de gameplay**
  (o conteúdo desofuscado é idêntico independente do índice).
- Combate (acerto/crítico): `rand()%100` em `CMob.cpp`.

### 4.0. Achado-chave: **paridade EXATA de RNG é possível** (não só distribucional)

- **Não há `srand()` de inicialização** em TMSrv/DBSrv (grep confirmou: o único `srand` está em
  `_MSG_CombineItemOdin.cpp:124,134,209`). Logo, a CRT do MSVC usa a **seed default = 1** a cada boot
  e produz **sempre a mesma sequência** de `rand()`.
- `rand()` do MSVC é um **LCG conhecido e simples** — dá para reimplementar **byte-idêntico** em Go:
  ```text
  state : uint32 (inicia em 1)
  rand():  state = state*214013 + 2531011;  return (state >> 16) & 0x7FFF   // 0..32767
  ```
  Reproduzindo esse LCG + a **mesma ordem de chamadas** `rand()`, o servidor novo gera **exatamente**
  os mesmos drops/refinos/críticos do atual.
- **Implicação prática:** numa captura controlada (1 cliente roteirizado, a partir de um **boot
  limpo** do TMSrv), a sequência é determinística → cada caso vira **golden case exato** (valor, não
  histograma).
- **Exceção `_MSG_CombineItemOdin`:** reseeda com `srand(time(...)*...)` → não-determinístico por
  design. Tratar à parte (testar por distribuição, ou tornar a seed injetável/configurável na stack
  nova — recomendado, registrando como divergência).
- **Cuidado:** `rand()` é um **estado global compartilhado** — com vários jogadores agindo
  concorrentemente o stream se intercala. Por isso a captura determinística exige **isolamento** (um
  cliente por vez, servidor recém-iniciado). Em produção/distribuição, cai-se no teste estatístico.

Estratégias:
1. **Paridade exata (preferida onde aplicável):** replicar o LCG do MSVC (acima) + a ordem de
   chamadas; comparar valor-a-valor. Vale para drop, combine (exceto Odin), combate.
2. **Distribuição (fallback):** onde a ordem não for controlável (concorrência, Odin), rodar N=10⁵
   amostras e comparar histogramas com tolerância estatística (qui-quadrado, ver §10).
3. **Seed injetável no servidor novo:** abstrair o RNG atrás de uma interface (seedável por caso de
   teste) — permite tanto reproduzir o LCG do MSVC quanto fixar cenários.
4. **Isolar o RNG do transporte:** comparar sempre payloads **desofuscados**, então o `iKeyWord`
   aleatório (`rand()%256`) não polui os testes de gameplay.

---

## 5. Como capturar golden cases do servidor atual

Duas abordagens práticas (o prompt sugere ambas):

### 5.1. Proxy TCP de captura (recomendado para protocolo de fio)

Inserir um proxy "man-in-the-middle" em cada um dos 3 links. Como o proxy conhece a `pKeyWord`
(Fase 1 §4.4) e o `HEADER` (Fase 1 §1), ele **desenquadra, desofusca e loga** cada frame, e
reencaminha os bytes originais intactos.

**Onde interpor (usando os arquivos de rede reais — Fase 7/config-ops):**

| Link | Como apontar para o proxy | Arquivo (Release) |
|------|---------------------------|-------------------|
| Cliente → TMSrv | editar a **serverlist** que o cliente baixa (`serverlist.txt`/`serverlist.bin`) para o IP:porta do proxy; proxy encaminha ao TMSrv real (`localip.txt`, lido em `Server.cpp:3965`) | `TMsrv/run/serverlist.*`, `localip.txt` |
| TMSrv → DBSrv | apontar o endereço do DB que o TMSrv disca para o proxy (o TMSrv usa `ConnectServer`+INITCODE) | config de DB do TMSrv |
| TMSrv → BISrv | editar `biserver.txt` (`IP porta`) para o proxy; encaminha ao BISrv real | `TMsrv/run/biserver.txt` (`54.207.102.145 3000`) |

> Alternativa para o link de DB: o DBSrv tem um **redirect embutido** — se existir `redirect.txt`
> (`IP porta user pass`, ver `DBSrv/Server.cpp:589` e `redirect.sample.txt`), ele reconecta a outro
> servidor; pode ser usado como ponto de espelhamento sem proxy externo.

**O proxy reusa o codec da Fase 1** (mesmo `HEADER` + transform + checksum + INITCODE). Recomendado
escrevê-lo **em Go** (Fase 9), assim o codec de captura **é o mesmo** que vai no `tmServer` — captura
e implementação se validam mutuamente.

- Encaminha bytes intactos, mas **grava** cada frame (request/response) desofuscado + timestamp +
  direção + `conn`/`ID`.
- Saída: trilha de frames que vira (a) **vetores de transporte** (§3) e (b) **golden cases** de
  gameplay (§1), correlacionando entrada→saída pelo `ID`/ordem.
- Vantagem: zero alteração no servidor; captura tráfego real.

### 5.2. Cliente headless (`Wyd2Client` em C#)
- Usar o `Wyd2Client` (ou `Ferramentas.rar`/`Conversor.rar`) para **dirigir** o servidor atual com
  sequências roteirizadas (login, criar char, atacar, refinar) e gravar as respostas.
- Vantagem: cenários determinísticos e repetíveis (melhor para casos de borda específicos).

### 5.3. Snapshot de estado
- O estado "final" de cada caso pode ser lido do **arquivo de conta** (`STRUCT_ACCOUNTFILE`, Fase 2)
  após a ação — comparar o dump antes/depois confirma mutações persistidas.

---

## 6. Critério de "passou"

Um caso passa quando, para a mesma entrada e seed:
1. Os **pacotes S→C** (desofuscados) batem campo-a-campo (ordem e valores).
2. O **estado persistido** (conta/char/itens) bate campo-a-campo.
3. Os **efeitos colaterais observáveis** (broadcast a terceiros, logs de item) batem.

Cobertura mínima para corte (Fase 9): todos os casos de §2.1–§2.7 + §3 verdes; §2.8 (war/castle)
validado por captura.

---

## 7. Esquema de fixture (versionado)

Dois tipos de arquivo, em `tests/fixtures/`:

**(a) Vetor de transporte** (`transport/*.json`) — valida a Fase 1 isolada:
```json
{
  "kind": "transport", "v": 1,
  "iKeyWord": 42,
  "plain_hex":  "0c00 2a00 0d02 ...",   // payload desofuscado (com HEADER)
  "wire_hex":   "0c00 2a7f 0d02 ...",   // bytes no fio (ofuscados) capturados do servidor atual
  "checksum": 127
}
```
Teste: `encode(plain, iKeyWord) == wire` **e** `decode(wire) == plain` **e** `checksum(plain)==127`.

**(b) Golden case de gameplay** (`gameplay/<subsistema>/<id>.json`):
```json
{
  "kind": "gameplay", "v": 1, "id": "refine_anct_success",
  "subsystem": "combine",
  "rng": { "impl": "msvc_lcg", "seed": 1, "skip": 12 },   // §4.0; "skip" = nº de rand() antes
  "given": { "account_file": "snapshots/test_A_before.bin" },  // dump STRUCT_ACCOUNTFILE (Fase 2)
  "when":  [ { "dir": "C2S", "type": "0x03A6", "fields": { "Item": [...], "InvenPos": [3,4] } } ],
  "then": {
    "out":   [ { "dir": "S2C", "type": "0x03A7", "fields": { "Parm": 1 } },
               { "dir": "S2C", "type": "0x0182", "fields": { "Slot": 3 } } ],
    "state": { "account_file": "snapshots/test_A_after.bin",
               "diff": [ { "path": "Char[0].Carry[3].sanc", "from": 6, "to": 7 } ] }
  }
}
```
- **`given`/`state` por snapshot do arquivo de conta** (`STRUCT_ACCOUNTFILE`, 7952 B — Fase 2 §0.1):
  o diff antes/depois prova a mutação persistida, comparado por **offset** (independe de linguagem).
- **`when`/`out`** guardam o payload **desofuscado** + os campos decodificados (Fase 1 §3.5).
- `rng` fixa o modo (LCG MSVC ou seed injetável) e quantos `rand()` "consumir" antes (para alinhar a
  posição no stream a partir do boot limpo).

---

## 8. Harness de replay (servidor novo, Go)

```
fixture → [harness] → instancia tmServer em modo teste (RNG = msvc_lcg seedável; relógio fake)
                       carrega given.account_file no estado
                       injeta os pacotes `when` (já decodificados) no game-loop
                       captura os pacotes `out` emitidos + faz dump do estado final
                       assert: out == then.out (campo-a-campo)  &&  state == then.state (offset-a-offset)
```
- **Relógio fake:** `CurrentTime`/`GetTickCount` injetáveis (os checks de tick do `_MSG_Action`/
  `_MSG_Attack` dependem disso — Fase 5). Sem isso, casos de anti-speedhack/cooldown não são
  determinísticos.
- **Transporte testado à parte** (§3/§7a): o harness de gameplay trabalha com pacotes já
  decodificados, então não depende do `iKeyWord`.
- **CI:** rodar a suíte a cada commit do `tmServer`; falha de qualquer golden case bloqueia merge.
  Os fixtures ficam versionados no repo (são pequenos: JSON + dumps de 7952 B).

---

## 9. Matriz de cobertura (caso → handler/regra)

Garante que cada handler da Fase 5 e cada fórmula da Fase 4 tenha ao menos 1 golden case.

| Subsistema | Casos (§2) | Handlers (Fase 5) | Regras (Fase 4) |
|------------|-----------|-------------------|-----------------|
| Login/conta | 2.1 | AccountLogin, AccountSecure, DeleteCharacter | — |
| Personagem | 2.2 | CreateCharacter, CharacterLogin, CharacterLogout, Restart | — |
| Movimento | 2.3 | Action(+2/3), Motion, ReqTeleport, ChangeCity | anti-speed (tick) |
| Combate | 2.4 | Attack | §4.1-4.6 (dano/acerto/parry/reflect) |
| Itens | 2.5 | DropItem, GetItem, UseItem, TradingItem, DeleteItem, SplitItem | drop (§2) |
| Refino | 2.6 | CombineItem + 8 variantes, UseItem(refino) | §3 (rolls + CompRate/SancRate) |
| Trade/loja | 2.7 | Trade, QuitTrade, SendAutoTrade, ReqBuy, Buy, Sell, Deposit, Withdraw | economia/imposto |
| Party/guilda/guerra | 2.8 | SendReqParty, AcceptParty, InviteGuild, GuildAlly, War, Challange | EXP de party (§1) |
| Quest/cash | (add) | Quest (38 NPCs), CapsuleInfo, PutoutSeal | quest flags |
| Chat/comandos | (add) | MessageChat, MessageWhisper (55 cmds) | — |

> Itens "(add)": casos a acrescentar à bateria §2 para cobrir os handlers documentados no lote 2.

---

## 10. Dimensionamento estatístico (quando for distribucional)

Para os casos sem ordem controlável (concorrência, Odin):
- **Drop / refino (sucesso/falha):** proporção `p`. Para detectar desvio de ±1% com 95% de confiança,
  `N ≈ 9600` amostras por célula; usar **N=10⁵** por folga.
- **Teste:** qui-quadrado entre o histograma do atual e do novo (ex.: faixas de dano, sucesso de
  refino por nível). Aceitar se `p-value > 0.01`.
- **`rand()%115` do combine (§3.1):** validar explicitamente o "achatamento" (`>=100⇒-15`) — os
  valores 85..99 devem aparecer ~2× mais que 0..84.

---

## 11. Sequência de captura (plano de execução)

1. **Vetores de transporte** (§7a): 1 sessão curta capturando login → já dá HEADER/transform/checksum
   reais. Destrava a Fase 1.
2. **Boot limpo + 1 cliente roteirizado** (LCG determinístico, §4.0): login → criar char → mover →
   atacar mob → dropar/pegar → refinar → trade → loja. Grava a trilha completa = golden cases exatos.
3. **`_AUTH_GAME` (billing):** capturar o link TMSrv↔BISrv (Fase 1 §4.3 UNVERIFIED) — fecha o
   layout de 196 bytes.
4. **Distribuições:** sessão de N=10⁵ (drop/refino) para os histogramas (§10).
5. **War/Castle (§2.8):** capturar uma janela de evento real (sequência temporal) — Fase 6.

> **Status da Fase 8: COMPLETO.** Metodologia, bateria de casos, **esquema de fixture**, **harness de
> replay**, **paridade exata via LCG do MSVC**, matriz de cobertura, dimensionamento estatístico e
> plano de captura definidos. A geração das fixtures reais é trabalho de implementação (rodar o
> proxy/cliente), não de documentação.
