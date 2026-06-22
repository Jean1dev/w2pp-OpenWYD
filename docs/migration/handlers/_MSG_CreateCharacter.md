# Contrato — `_MSG_CreateCharacter`

- **Gatilho:** Type `0x020F` (527, `CLIENT2GAME`). Struct `MSG_CreateCharacter` (`Slot:int`,
  `MobName[16]`, `MobClass:int`; Fase 1 §3.5).
- **Fonte:** `TMSrv/_MSG_CreateCharacter.cpp:21-51`.

## Pré-condições e validações
1. Trunca `MobName` (zera 2 últimos bytes) (`:25-26`).
2. `pUser[conn].Mode == USER_SELCHAR` — senão `_MSG_NewCharacterFail` + log (`:28-35`).
3. `BASE_CheckValidString(MobName)` — nome válido (caracteres permitidos) (`:37`). Falha →
   `_MSG_NewCharacterFail` (`:49-50`).

## Efeitos colaterais
- Reescreve para `Type=_MSG_DBCreateCharacter (0x0802)`, `ID=conn`; encaminha ao DBSrv
  (`SendOneMessage(sizeof(MSG_CreateCharacter))`) (`:39-44`).
- Transição: `pUser[conn].Mode = USER_WAITDB` (`:42`).
- Log `etc,createchar name:...`.

## Saídas (S→C)
- Sucesso: confirmação vem do DBSrv (`_MSG_DBCNFNewCharacter` → `_MSG_CNFNewCharacter` +
  `STRUCT_SELCHAR` ao cliente).
- Erro: `_MSG_NewCharacterFail` (0x011A).

## Anti-cheat / Riscos
- Validação de nome só por `BASE_CheckValidString` (sem código-fonte da função) — **UNVERIFIED** o
  conjunto exato de caracteres/profanidade; capturar casos.
- `MobClass` vindo do cliente: o DBSrv deve validar a classe permitida (criação inicial). Confirmar
  no DBSrv (`CFileDB::CreateCharacter`).
- Slot duplicado / nome existente: a unicidade é resolvida no DBSrv (arquivo de conta).
