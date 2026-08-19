# Comandos do jogo

> Status no servidor Go: ✅ funciona · ⏳ pendente (depende de sistema ainda não modelado).
> Os comandos são digitados no chat (`/comando`); o cliente os envia como um "sussurro"
> cujo alvo é o nome do comando (`_MSG_MessageWhisper`).

✅ /torre: se teleportará para a guerra de torres <br/>
✅ /armia: se teleportará para a cidade de Armia <br/>
✅ /erion: se teleportará para a cidade de Erion <br/>
✅ /azran: se teleportará para a cidade de Azran <br/>
✅ /gelo: se teleportará para a cidade de Gelo <br/>
✅ /kefra: se teleportará para a cidade de Kefra <br/>
✅ /noatun: se teleportará para Noatun <br/>
✅ /red: se teleportará para o rei de Akelonia <br/>
✅ /blue: se teleportará para o rei de Hekalotia <br/>
✅ /arch: se teleportará para a cidade dos reinos (apenas o teleporte; o destrave do Arch é ⏳) <br/>
✅ /reino: teleporta de acordo com a capa — capa de Hekalotia (azul) leva ao rei de Hekalotia, capa de Akelonia (vermelha) ao rei de Akelonia, e qualquer capa neutra (sem capa, Capa Branca do Monstro #550, capa verde/Manto do Aprendiz #4006, …) à cidade dos reinos — comando novo, não existe na fonte legada <br/>
⏳ /crias: se teleportará para o drop de crias (Sleipnir e Svaldfire) — sem coordenada na fonte legada <br/>
✅ /destravar40: destrava o level 40 do celestial (seta o gate `QuestInfo.Celestial.Lv40`; efetivo só para chars Celestial) <br/>
✅ /destravar90: destrava o level 90 do celestial (gate `Lv90` + dá a FuryStone item 3502) <br/>
✅ /arcana: realiza a quest da cythera arcana (seta `Circle` + põe o item 3507 no Equip[1]) <br/>
⏳ /create: (nome da guild): cria guild — sistema de guild não modelado <br/>
✅ /sair: sai da sua guild (limpa a guild + atualiza a tag; metadados de guild não modelados) <br/>
⏳ /guild: mostra o index (ID) da sua guild — sistema de guild não modelado <br/>
✅ /buffs: Remove todos os buffs do personagem <br/>
✅ /cp: mostra os pontos de caos atuais do personagem (`PKPoint-75`; 0 = nick branco). Recuperam de duas formas: +1 por hora online (gate do `RegenMob` legado) e **+1 por nível subido**, ambas com teto no neutro 75 — o ganho por nível é um desvio consciente do legado, pedido na issue #279 <br/>
✅ /nick \<jogador\>: mostra nick, guild (nome/fama, se registrada — `world/guild.go`), cidadania e fama do jogador alvo <br/>
✅ /gritar \<mensagem\>: grito global — consome 1 Trombeta Mágica (item 3330) e envia `[Nome]> mensagem` a todos os jogadores online, em verde (`_MSG_MagicTrumpet`). Alias legado: `/spk`. Sem trombeta, avisa e não grita; o alcance é o deste canal (o fan-out entre canais do legado passava pelo DBSrv e não foi portado) <br/>

> Bônus já implementados (existem na fonte legada, fora da lista acima): `/selados`,
> `/amagos`, `/agua` (teleportes).

# Comandos de GM / moderação

> Digitados como `/gm <subcomando> <args>` — o cliente envia como sussurro ao alvo
> `gm` com o resto da linha no corpo (o mesmo truque do `_MSG_MessageWhisper`).
> Autoridade vem da coluna `account.role` (`moderator`/`admin`), carregada no login
> — **não** do frágil "Level ≥ 1000" do legado. Comando negado é silencioso. Toda
> execução é auditada (slog: conta, alvo, args). Implementação: `handler/gm.go`.

✅ /gm kick \<jogador\>: desconecta um jogador online (não derruba GM de nível igual/superior) <br/>
✅ /gm notice \<texto\> (ou /gm aviso): anúncio global a todos os jogadores <br/>
✅ /gm goto \<jogador\> (ou /gm ir): teleporta você até o jogador <br/>
✅ /gm summon \<jogador\> (ou /gm puxar): puxa o jogador até você <br/>
✅ /gm spawn \<id\>: cria uma criatura de teste (índice do roster de summons) na sua posição <br/>
✅ /gm item \<id\> \[qtd\]: coloca um item (por índice) no seu inventário; `qtd` (1–120) cria um stack <br/>
✅ /gm setlevel \<n\>: sobe o seu nível para n (apenas sobe — não rebaixa) <br/>
✅ /gm setgold \<n\>: define o seu ouro carregado <br/>
✅ /gm ban \<jogador|conta\>: bloqueia a conta (via `account.is_blocked`) e derruba se online <br/>
✅ /gm unban \<jogador|conta\>: remove o bloqueio da conta <br/>
✅ /gm guildname \<id\> \<nome\>: registra o nome de uma guild (issue #131; só em memória — não há fluxo de criação de guild ainda) <br/>
✅ /gm guildfame \<id\> \<fama\>: registra a fama de uma guild (issue #131; mesma ferramenta admin-only do legado `+guildfame set`) <br/>

> `notice` sai como linha de chat prefixada `[GM]` (o pacote de aviso dedicado é
> UNVERIFIED até uma captura). `ban`/`unban` gravam em `account.is_blocked` — o login
> já rejeita contas bloqueadas; a migração do ban administrativo para o binServer
> (entitlement) fica para uma issue futura (`web-platform-plan.md §binServer`).

# Evoluções 
NPC Evoluções vende poeira, upe o seu Mortal, Arch, Celestial e Sub Celestial com ela.

Faça as quest dos quatros cristais no seu Arch para liberar mais pontos.
- Dê /red ou /blue para ir direto para o rei desejado.
• Não precisa transformar o Lac, somente separe 10 que já vai funcionar

Para destravar o lv 40 e 90 do Cele utilize o comando /destravar40 e /destravar90

Pegue lv 200 no seu Cele e vire Sub Cele, faça a quest da Cythera Arcana e os três resets. (Escreva /arcana para fazer a quest da Arcana automaticamente).
• Refine a capa para +9 logo após disso.
