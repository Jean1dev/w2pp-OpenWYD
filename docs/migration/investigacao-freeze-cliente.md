# Investigação: freeze do cliente ("jogo para de responder e cai")

> **Status:** ABERTO — causa-raiz não confirmada. Instrumentação S→C adicionada em 2026-07-19
> (branch `investigando-bug-logoff`); aguardando a próxima reprodução com os novos logs.
> Este arquivo é o estado da investigação para sessões futuras: leia antes de re-derivar
> qualquer coisa dos logs do Railway.

## Sintoma

Relato do usuário (2026-07-19): "o jogo não está mais respondendo após alguns minutos logado…
o servidor fica lento e depois cai". Começou em **2026-07-18**; acontece em praticamente toda
sessão (1,5–4 min de jogo e trava). Não é a rede do notebook.

## Conclusão parcial (o que JÁ está estabelecido — não reinvestigar)

O travamento é **do cliente (WYD.exe), não do servidor**. Evidência colhida via Railway CLI
(skill `docs/migration/prompts/railway-troubleshooting-skill.md`):

- **tm-server saudável durante todos os incidentes**: CPU <1% (de 8 vCPU), memória ~133–147MB
  estável (de 8GB), zero `panic`/`level=ERROR`, zero `session out queue full` em 24h.
- **O loop nunca parou**: processou logins novos *enquanto* outra sessão estava congelada
  (ex.: conn=2 logou 00:16:02Z com conn=1 travada desde 00:14:16Z).
- **db-server limpo**: zero ERROR/WARN; logins respondidos em ~100–200ms nas janelas dos incidentes.
- **Rede Railway limpa**: flow logs todos `OK`/0ms, sem drop/reset no proxy TCP.
- **Prova-chave**: durante o freeze das 11:36Z (19/07) o servidor seguiu enviando ~14KB/4s
  e o cliente seguiu **ACKando no TCP** — os bytes chegam, mas o jogo não reage. O cliente
  recebe e **descarta/não processa**.
- Toda sessão termina com `err=EOF` = o usuário matou o cliente travado. O "silêncio" C→S
  antes do EOF é o usuário parado olhando a tela congelada (cliente WYD ocioso não manda nada).
- O fix "teleport view reconciliation" (86e5513, PR #163, deploy 19/07 11:28Z) **não resolveu**:
  duas sessões congelaram 11:30–11:37Z com ele em produção.

Descartado portanto: crash/lentidão do servidor, OOM, deploy no meio da sessão, DB, rede/proxy
Railway, fila S→C cheia (o WARN nunca disparou), notebook do usuário.

## Linha do tempo observada

| Janela (UTC) | Deploy | Código | Sessões | Resultado |
|---|---|---|---|---|
| 19/07 00:00–00:36 | `cca81866` (18/07 23:49Z) | main @ 892c27b (pré-fix) | ≥6 | todas EOF; 1 saída limpa (00:01:56–00:04:15), resto freeze |
| 19/07 11:30–11:37 | `a3fd73d7` (11:28Z) | main @ 3b51252 (com fix #163) | 2 | ambas freeze→kill |

Logs Railway de deploys anteriores a 18/07 23:49Z **já expiraram** — não dá para datar o início
exato da regressão por logs. O usuário situa o início em **18/07**, dia em que entraram
(deploys 12:11Z, 12:19Z, 15:44Z, 15:51Z, 18:31Z, 23:49Z):

- #143 gema-estelar, #151 `/reino`, #149 tinturas, #160 removedor-de-tinta,
  #152 itens-quest, #155 sistema-de-quests (celestial), fix 45e5ed3 (magic bean).

Esse conjunto é a **janela de regressão suspeita** (na noite de 17/07 entraram ainda #146
template-de-reis, #154 `/cp`, #144 nick, #150, #148 — só relevantes se o problema for anterior
ao que o usuário percebeu).

## Momentos exatos dos freezes (para correlação)

O cliente congela e o usuário ainda manda comandos sem efeito (toggle PK ×4, whispers
repetidos) antes de desistir:

- 00:10:44Z — 8s após teleporte para **(1703,1730)** (≈ destino de `/reino` {1702,1728}±3;
  também ≈ `/arch` {1706,1723}); 2 passos andados e morto.
- ~00:14:16Z — após 3 teleportes rápidos + tentativa de logout (0x03ae + 0x0215); o relogin
  veio numa conexão NOVA (cliente antigo ficou pendurado).
- ~00:17:28Z — spawn na cidade 2 (2456,2007 ≈ Erion), 2 ataques (0x0368), 2 whispers finais.
- ~00:33:47Z — após vários teleportes; 4× 0x0399 (PK toggle) sem efeito e silêncio.
- ~11:34:37Z (19/07) — após 2 teleportes de cidade + quit-cancelado + 17 cliques de
  ApplyBonus (0x0277) a ~1,1s.
- ~11:36:35Z (19/07) — spawn na **cidade 3 (3664,3128 ≈ `/gelo` {3650,3130})**; 262×
  ApplyBonus em 48s (cadência ~130ms ≈ RTT = usuário segurando o botão), 1 whisper, morto.

Padrão: o freeze acontece **logo após chegar numa área nova** (teleporte de cidade ou login
fora de Armia). A única sessão limpa da amostra ficou em **Armia**. MAS a amostra é pequena —
não tratar como lei.

## Hipóteses vivas (em ordem)

1. **Frame S→C malformado dessincroniza o parser do cliente.** O cliente lê o stream por
   `Size` do header; um único frame com Size/encoding errado faz TODO o resto decodificar
   lixo → o cliente passa a dropar tudo silenciosamente (bate com "TCP ACKa, jogo morto").
   Candidato natural: um `CreateMob` (0x0364) de algum NPC/template específico das áreas de
   destino, ou algo no burst de chegada. Com a instrumentação nova, o "session last sends"
   do disconnect mostra exatamente o tail enviado.
2. **Keyword queue do cliente** (gap conhecido, ver memória `decompiled-client-reference` e
   `docs/agents/RELATED-PROJECTS-COMPARISON-2026-06-19.md` §4): se alguma mensagem NOVA dos
   PRs de 17–18/07 arma a `RecvQueue[16]` do cliente, ele passa a rejeitar pacotes com
   `ErrorCode=3` **em silêncio**. Checar no fonte do tm-project o que escreve em
   `RecvQueue`/`EncodeByte` e se emitimos esse opcode agora.
3. **NPC do overlay de moderador com visual inválido** (83 NPCs, `npc config version=151`):
   um mesh/equip fora de faixa congela o render do cliente quando entra na view — também
   explicaria a dependência de área. Cruzar `npc_definition` com as áreas dos freezes.

## Instrumentação adicionada (2026-07-19, esta branch)

Tudo pensado para deixar **post-mortem automático em todo disconnect** — não precisa de flag
para o essencial:

| Log | Onde | Significado |
|---|---|---|
| `session send stats` (INFO) | disconnect (`world/sendstats.go`) | total de frames/bytes S→C, high-water da fila e contagem por tipo (`by_type="0x0364:120 …"`, dominante primeiro) |
| `session last sends` (INFO) | disconnect | os **últimos 64 frames S→C** como `0xTIPO(HEADER.ID)/len/-idade_ms` — o que o cliente recebeu por último antes do usuário matar o processo |
| `teleport` (INFO) | `handler/movement.go doTeleport` | origem→destino + quantos CreateMob/RemoveMob o teleporte gerou para o cliente |
| `chat command` (INFO) | `handler/chat.go` | keyword de todo slash command executado (antes era invisível — só se via o 0x0334) |
| `session out queue high` (WARN) | `world.enqueue` | novo high-water ≥ 50% da capacidade (default 64) — backpressure do writer ANTES do drop |
| `slow world event` (WARN) | `world.Run` | um evento do loop levou ≥100ms (identifica frame/tick/callback) — se isso aparecer, a hipótese "servidor lento" volta ao jogo |
| `send packet` (INFO) | `world.enqueue`, **atrás de flag** | espelho S→C do "recv packet": `-log-sends` ou `W2PP_LOG_SENDS=true`. Alto volume; ligar só durante reprodução |

## Playbook da próxima reprodução

1. (Opcional, recomendado com 1 jogador) ligar `W2PP_LOG_SENDS=true` no serviço tm-server do
   Railway antes do teste — pede confirmação do usuário, é variável de produção.
2. Jogar até travar. Anotar hora local exata do travamento (não do kill).
3. Matar o cliente (gera o EOF → dispara os logs de post-mortem).
4. Colher (ver comandos no skill do Railway):
   ```bash
   railway logs --service tm-server --since <UTC-2min> --until <UTC+2min>
   # focar: "session last sends", "session send stats", "teleport", "chat command",
   #        "slow world event", "session out queue high"
   ```
5. No `session last sends`: identificar o(s) último(s) tipos/tamanhos. Comparar os `len`
   com o esperado do protocol-spec para o tipo (um len anômalo = hipótese 1 confirmada
   e aponta o encoder culpado).
6. Se o tail parecer normal: partir para captura no cliente (Wireshark na máquina Windows,
   workflow do agente Windows em `docs/migration/SESSION-PRIMER.md`) e validar frame a frame
   os Size/checksum do stream — e investigar as hipóteses 2 e 3.
7. Repetir o teste em áreas distintas (ficar só em Armia vs teleportar para `/reino`,
   `/gelo`, Erion) para confirmar/refutar a dependência de área.

## Higiene encontrada no caminho (não é a causa, mas corrigir)

- `0x03ae` (delay-quit) chega com `routed=false` — sem handler. Benigno no quit limpo
  (o cliente fecha sozinho ~4s depois), mas merece rota explícita.
- **Duas migrations com número 0012**: `0012_celestial_quest` e `0012_character_fame` —
  renumerar antes que o tracker pule uma delas.
- `0x0334×2 + 0x0291` aparece ~1s após todo char login (sequência automática do cliente
  e/ou comando de cidade) — não confundir com ação deliberada do usuário ao ler timelines.

## Referências

- Skill de triagem Railway: `docs/migration/prompts/railway-troubleshooting-skill.md`
  (projeto `wyd`, serviço `tm-server`; converter horário de Brasília para UTC−3→UTC).
- Memória do agente: `logoff-freeze-client-side` (resumo vivo desta investigação).
- Cliente descompilado (parser CPSock + keyword queue): https://github.com/EricSantos00/tm-project
  — análise em `docs/agents/RELATED-PROJECTS-COMPARISON-2026-06-19.md` §4.
- Fix que NÃO resolveu (não re-tentar o mesmo caminho): PR #163 / commit 86e5513.
