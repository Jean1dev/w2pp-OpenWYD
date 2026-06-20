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
- **Enumeração completa:** os 55 comandos (`#pragma region`) estão catalogados em
  **[_MSG_MessageWhisper-comandos.md](_MSG_MessageWhisper-comandos.md)** — gatilho, gate de permissão
  e efeito de cada um.
- **Anti-cheat/risco:** **este handler concentra a maior parte dos "comandos" do jogo** — é uma
  superfície grande de confiança. Achados principais (detalhe no doc dedicado): GM é gated só por
  `Level>=1000` (frágil); `/gm` e `/cmd` entram no `ProcessImple` (`imple.cpp`); e há **backdoors de
  progressão sem permissão** (`/destravar40`, `/destravar90`, `/arcana`) a remover/auditar. Na
  migração, **separar comandos (com autorização explícita) do caminho de sussurro**.

> **Recomendação de migração (ambos):** separar claramente **transporte de chat** (público/sussurro/
> canais) de **comandos** (um command-bus com autorização). Hoje estão fundidos, o que dificulta
> auditoria e é fonte de exploits.
