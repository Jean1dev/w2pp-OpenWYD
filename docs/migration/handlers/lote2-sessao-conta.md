# Contratos — Sessão & Conta (lote 2)

> Handlers de conta/sessão. Padrão geral: cast da struct → checagem de `Mode` → ação → resposta.
> Fonte: `Source/Code/TMSrv/_MSG_*.cpp`. Types na Fase 1; fluxos na Fase 6.

---

## `_MSG_AccountSecure` (0x0FDE) — PIN numérico
- **Gatilho/struct:** `MSG_AccountSecure` (`_MSG_AccountSecure.cpp`).
- **Validações:** `conn` em `(0, MAX_USER)`; senão retorna sem fazer nada.
- **Efeitos:** seta `m->ID = conn` e **encaminha ao DBSrv** (`DBServerSocket.SendOneMessage`,
  `Type` permanece `_MSG_AccountSecure`). A verificação/alteração do PIN (`NumericToken`) é feita no
  DBSrv (ver Fase 6 / ProcessDBMessage).
- **Saídas:** nenhuma direta ao cliente; resposta vem do DBSrv.
- **Riscos (migração):** PIN trafega/compara em **texto plano** (Fase 2 §1.3) → hash/HMAC na stack
  nova. É um simples relay; no novo `dbServer` vira uma chamada gRPC tipada.

## `_MSG_DeleteCharacter` (0x0211) — deletar personagem
- **Gatilho/struct:** `MSG_DeleteCharacter` (`Slot`, `MobName[16]`, `Password[12]`).
- **Validações:** força `MobName[NAME_LENGTH-1]=0`; exige `pUser[conn].Mode == USER_SELCHAR`.
- **Efeitos:** se em SELCHAR → reescreve `Type=_MSG_DBDeleteCharacter`, `ID=conn`, seta
  `Mode = USER_WAITDB` e **envia ao DBSrv**. Se não estiver em SELCHAR → só envia mensagem
  "Deleting Character. wait a moment." e loga erro.
- **Saídas:** confirmação `_MSG_CNFDeleteCharacter` vem **depois** do DBSrv.
- **Anti-cheat/risco:** a senha de confirmação está na struct (texto plano); validação real no DBSrv.
  Preservar o gate `USER_SELCHAR` (não deletar com personagem em jogo).

## `_MSG_CharacterLogout` (0x0215) — sair para a seleção
- **Gatilho/struct:** `MSG_STANDARD` (`_MSG_CharacterLogout.cpp`).
- **Efeitos:** `RemoveParty(conn)` → `CharLogOut(conn)` (salva e devolve à tela de seleção) →
  `pUser[conn].cSock.SendMessageA()` (flush).
- **Saídas:** o fluxo de `CharLogOut` dispara save no DBSrv + `_MSG_CNFCharacterLogout`.
- **Risco:** sem checagem de estado aqui; `CharLogOut` precisa ser idempotente/seguro no novo código.

## `_MSG_Restart` (0x0289) — reviver / voltar à cidade
- **Gatilho/struct:** `MSG_STANDARD`.
- **Validações:** exige `Mode == USER_PLAY` (senão `SendHpMode`).
- **Efeitos:** `Hp = 2`; zera `NumError`; `SendScore`+`SendSetHpMp`. Teleporte condicional por
  região/clan: se em região de captial (X 1017–1290, Y 1911–2183) e `Clan==7` → teleporta para
  ~(1061,2129); `Clan==8` → ~(1237,1966); senão `DoRecall(conn)` (volta ao ponto padrão). `SendEtc`.
- **Anti-cheat/risco:** revive com 2 de HP (não cura full). Coordenadas/clans hardcoded → mover para
  config no novo servidor. RNG no destino (seed para paridade, Fase 8).

## `_MSG_Deprivate` (0x028C) — *deprivate*
- **Gatilho/struct:** `MSG_STANDARDPARM` → chama `DoDeprivate(conn, m->Parm)`.
- **Efeitos:** delega 100% a `DoDeprivate` (lógica fora do handler; documentar junto de `Server.cpp`).
- **Risco:** handler é só um relay; o contrato real está em `DoDeprivate` (auditar separadamente).
