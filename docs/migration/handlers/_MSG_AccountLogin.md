# Contrato — `_MSG_AccountLogin`

- **Gatilho:** Type `0x020D` (525, `CLIENT2GAME`). Struct `MSG_AccountLogin` (116 bytes; Fase 1 §3.5).
- **Fonte:** `TMSrv/_MSG_AccountLogin.cpp:21-96`. Estado esperado: `pUser[conn].Mode == USER_ACCEPT`.

## Pré-condições e validações
1. `conn > 0 && conn < (MAX_USER - ADMIN_RESERV)` — slots altos reservados a admin; senão
   `_NN_Reconnect` + `CloseUser` (`:30-39`).
2. `Size >= sizeof(MSG_AccountLogin)` **e** `m->ClientVersion == APP_VERSION (7640)` — senão
   `_NN_Version_Not_Match_Rerun` + `CloseUser` (`:44-54`). (Em `_PACKET_DEBUG` a versão não é checada.)
3. `pUser[conn].Mode == USER_ACCEPT` — senão "Login now, wait a moment." + `CrackLog` (`:56-63`).
4. `CheckFailAccount(AccountName) < 3` — 3+ falhas de senha bloqueiam temporariamente
   (`_NN_3_Tims_Wrong_Pass`, `:82-90`).

## Efeitos colaterais
- Captura MAC: copia `m->AdapterName` para `pUser[conn].Mac` (ou `0xFF` se pacote curto) (`:67-70`).
- Normaliza o nome: `sscanf` + `_strupr` → `pUser[conn].AccountName` (uppercase) (`:76-80`).
- **Reescreve a mensagem** para `Type=_MSG_DBAccountLogin (0x0803)`, `ID=conn` e a **encaminha ao
  DBSrv** via `DBServerSocket.SendOneMessage(sizeof(MSG_AccountLogin))` (`:73-92`).
- Transições: `pUser[conn].Mode = USER_LOGIN`; `pMob[conn].Mode = MOB_EMPTY` (`:94-95`).

## Saídas (S→C) e casos de erro
- Sucesso: nenhuma resposta direta ao cliente aqui — a confirmação vem **depois** do DBSrv
  (`_MSG_DBCNFAccountLogin` → tratado em `ProcessDBMessage`, que envia `_MSG_CNFAccountLogin` +
  `STRUCT_SELCHAR` ao cliente; ver Fase 6).
- Erros: `_NN_Reconnect`, `_NN_Version_Not_Match_Rerun`, `_NN_3_Tims_Wrong_Pass`,
  `_MSG_DBAccountLoginFail_*` (vindos do DBSrv).

## Anti-cheat
- Versão de cliente obrigatória (bloqueia clientes desatualizados/forjados).
- `CheckFailAccount`/`CrackLog` (brute-force de senha).
- MAC capturado para `Mac.txt`/bloqueio (Fase 7).

## Riscos conhecidos (migração)
- **Senha trafega e é comparada em texto plano** (a struct carrega `AccountPassword[12]` claro; o
  DBSrv compara contra `AccountPass` claro). Corrigir com hash na stack nova (Fase 2 §1.3, Fase 9).
- Confiança no `ClientVersion` enviado pelo cliente (mitigável, mas é só um inteiro).
- `ADMIN_RESERV` reserva faixa de `conn` — preservar a semântica de slots admin.
