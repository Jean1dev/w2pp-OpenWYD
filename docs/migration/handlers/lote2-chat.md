# Contratos — Chat (lote 2)

> Os dois handlers de chat são, na prática, **interpretadores de comando** além de transporte de
> texto. Fonte: `Source/Code/TMSrv/_MSG_MessageChat.cpp` (196) e `_MSG_MessageWhisper.cpp` (1710).

---

## `_MSG_MessageChat` (0x0333) — chat público + comandos de barra
- **Gatilho/struct:** `MSG_MessageChat` (`String[MESSAGE_LENGTH]`).
- **Validações:** trunca `String`; seta `m->ID = conn`; exige `Mode == USER_PLAY`.
- **Despacho de comando:** faz `sscanf(String, "%s %s", szCmd, szString)` e compara `szCmd` com uma
  lista de comandos **antes** de tratar como fala pública:
  - `guildon` / `guildoff` → liga/desliga `GuildDisable` (esconde tag de guilda) + `SendScore`.
  - `guildtax <0..30>` → líder (`GuildLevel==9`) define imposto da zona que sua guilda controla;
    1×/dia (`TaxChanged[i]`), persiste com `CReadFiles::WriteGuild()`.
  - `whisper` → alterna `pUser[conn].Whisper` (bloquear sussurros).
  - `partychat` / `kingdomchat` / `guildchat` / `chatting` → canais de chat (party/reino/guilda).
- **Fala pública (default):** se não for comando → **`GridMulticast`** da mensagem aos jogadores na
  visão (`_MSG_MessageChat.cpp:187`).
- **Anti-cheat/risco:** comandos administrativos/sociais embutidos no chat. **UNVERIFIED:** confirmar
  a lista completa e a semântica de `partychat/kingdomchat/guildchat/chatting` (roteamento por
  party/guild/reino). Imposto de guilda toca persistência (Fase 2). Sem rate-limit de chat (flood).

## `_MSG_MessageWhisper` (0x0334) — sussurro + **console de comandos**
- **Gatilho/struct:** `MSG_MessageWhisper` (`MobName[16]` = destino, `String[MESSAGEWHISPER_LENGTH]`).
- **Validações:** trunca `MobName`/`String`; exige `Mode == USER_PLAY`.
- **Despacho de comando (o grosso do arquivo):** quando `MobName` é uma palavra-chave, o handler
  trata como **comando do jogador/admin** em vez de sussurro. Exemplos confirmados no topo:
  `cp` (mostra pontos de chao/PK), `buffs` (limpa affects), `getout` (encerra cidadania),
  `srv <n>` (troca de servidor/canal), e **muitos outros** (arquivo de 1710 linhas, organizado em
  `#pragma region /<cmd>`).
- **Sussurro real (default):** quando `MobName` é o nome de outro personagem online → entrega a
  mensagem ao destinatário (respeitando o flag `Whisper` do alvo; "Deny whisper"/"Not connected"
  quando bloqueado/offline).
- **Anti-cheat/risco:** **este handler concentra a maior parte dos "comandos" do jogo** — é uma
  superfície grande de confiança. **UNVERIFIED (a fechar):** enumerar TODOS os `#pragma region /<cmd>`
  (gatilho, args, pré-condições de permissão, efeitos). Na migração, **extrair esses comandos para um
  módulo de comandos dedicado** (com autorização explícita), separado do caminho de sussurro. Vários
  comandos mutam estado sensível (cidadania, buffs, troca de canal) — exigem checagem de permissão
  que hoje é implícita.

> **Recomendação de migração (ambos):** separar claramente **transporte de chat** (público/sussurro/
> canais) de **comandos** (um command-bus com autorização). Hoje estão fundidos, o que dificulta
> auditoria e é fonte de exploits.
