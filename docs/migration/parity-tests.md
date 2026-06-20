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

Estratégias:
1. **Distribuição, não valor exato:** rodar N=10⁵ amostras no servidor atual e no novo, comparar
   histogramas (drop rate, taxa de sucesso de refino). Tolerância estatística.
2. **Seed determinística:** instrumentar o servidor novo com um RNG injetável (seed por caso). Para
   capturar do atual, é possível **fixar a seed** (`srand(k)`) num build de captura e gravar a
   sequência de saídas — vira golden case exato.
3. **Isolar o RNG do transporte:** comparar sempre payloads desofuscados, então o `iKeyWord`
   aleatório não polui os testes.

---

## 5. Como capturar golden cases do servidor atual

Duas abordagens práticas (o prompt sugere ambas):

### 5.1. Proxy TCP de captura (recomendado para protocolo de fio)
Inserir um proxy entre `WYD.exe` e `TMSrv.exe` (e entre TMSrv↔DBSrv, TMSrv↔BISrv):
- Encaminha bytes, mas **grava** cada frame (request/response) com timestamp.
- Como o proxy conhece a `pKeyWord` (Fase 1 §4.4), pode **desofuscar e logar** o payload legível.
- Saída: pares `(estado, entrada) → (saída)` reais, viram fixtures YAML/JSON.
- Vantagem: zero alteração no servidor; captura tráfego real de jogadores.

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

> **Status da Fase 8: COMPLETO** como metodologia + bateria de casos. A geração das fixtures reais
> depende da captura (proxy/cliente) — é trabalho de implementação, não de documentação.
