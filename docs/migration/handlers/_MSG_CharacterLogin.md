# Contrato — `_MSG_CharacterLogin`

- **Gatilho:** Type `0x0213` (531, `CLIENT2GAME`). Struct `MSG_CharacterLogin` (`Slot:int`,
  `Force:int`).
- **Fonte:** `TMSrv/_MSG_CharacterLogin.cpp:21-110`. **Handler com lógica de billing emaranhada**
  (`Unk_1816`, `Unk_2728`, `Unk_2732`, `BILLING`, `FREEEXP`) — difícil de ler; ver notas.

## Pré-condições e validações
1. `0 <= Slot < MOB_PER_ACCOUNT (4)` — senão `_NN_SelectCharacter` (`:25-29`).
2. Lógica de **billing/free-exp** (`:31-107`):
   - Se `Level < FREEEXP` **ou** `Level >= 999` **ou** `BILLING != 2` → caminho "Label1" (libera).
   - Caso contrário, exige estado de billing (`Unk_1816`): valores 3 = "_DN_Not_Allowed_Account",
     4 = "_NN_Using_Other_Server_Group"; aguarda checagem de billing (`SendBilling`, `:105`).
   - Há checagem de horário: `g_Hour > 7 && g_Hour < 23` e `_NN_Child_Pay` (proteção a menores).
3. No caminho de login efetivo: `pUser[conn].Mode == USER_SELCHAR` (`:71`).

## Efeitos colaterais
- Reescreve para `Type=_MSG_DBCharacterLogin (0x0804)`, `ID=conn`; encaminha ao DBSrv (`:73-81`).
- Transições: `pUser[conn].Mode = USER_CHARWAIT`; `pMob[conn].Mode = MOB_USER`;
  `pMob[conn].MOB.Merchant = 0` (`:76-79`).
- Billing: `SendBilling(conn, AccountName, 1, 1)` quando precisa validar (`:105`).

## Saídas (S→C)
- Sucesso: o DBSrv responde `_MSG_DBCNFCharacterLogin` → o TMSrv monta `_MSG_CNFCharacterLogin`
  (snapshot do `STRUCT_MOB` + posição + clima + skill bar) e injeta o jogador no mundo (Fase 6).
- Erros/avisos: `_NN_SelectCharacter`, `_NN_Wait_Checking_Billing`, `_DN_Not_Allowed_Account`,
  `_NN_Using_Other_Server_Group`, `_NN_Child_Pay`, signal 404.

## Anti-cheat / Riscos
- Depende de `pUser[conn].SelChar` (carregado no login de conta) para `Level` — server-authoritative.
- **A lógica de billing está hardcoded e com nomes `Unk_*`** — alto risco de paridade. Recomenda-se
  **reescrever o gate de billing como política explícita** na stack nova (estados claros) e validar
  por captura, em vez de replicar os `Unk_*`. Marcar `BILLING`, `FREEEXP`, `g_Hour` como config.
- `Force` (struct) não é usado neste trecho — **UNVERIFIED** seu efeito (provável kick de sessão
  anterior); confirmar no DBSrv (`_MSG_DBAlreadyPlaying`).
