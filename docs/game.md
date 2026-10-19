# Comandos do jogo

> Status no servidor Go: ✅ funciona · ⏳ pendente (depende de sistema ainda não modelado).
> Os comandos são digitados no chat (`/comando`); o cliente os envia como um "sussurro"
> cujo alvo é o nome do comando (`_MSG_MessageWhisper`).

✅ /torre: se teleportará para a guerra de torres <br/>
✅ /armia: se teleportará para a cidade de Armia <br/>
✅ /erion: se teleportará para a cidade de Erion <br/>
✅ /azran: se teleportará para a cidade de Azran <br/>
✅ /gelo: se teleportará para a cidade de Gelo <br/>
✅ /kefra: se teleportará para a cidade de Kefra. Ali, o NPC Sobrevivente troca o Pergaminho Selado (4127) por acessos ao Hall de Kefra (100 por pergaminho, ajustável com `-kefra-entries-per-scroll`), e cada entrada no Hall gasta um. Falar com ele sem pergaminho mostra os acessos restantes <br/>
✅ /noatun: se teleportará para Noatun <br/>
✅ /arch: se teleportará para a cidade dos reinos (apenas o teleporte; o destrave do Arch é ⏳) <br/>
✅ /rei ou /king: teleporta para o rei correspondente à capa equipada — Hekalotia (azul) vai para `(1748,1574)` e Akelonia (vermelha) para `(1748,1880)`; sem capa de reino, o comando não teleporta <br/>
✅ /reino ou /kingdom: teleporta para a área de comércio correspondente à capa equipada — Hekalotia (azul) vai para `(1690,1618)`, Akelonia (vermelha) para `(1690,1842)`, e qualquer capa neutra (sem capa, Capa Branca do Monstro #550, capa verde/Manto do Aprendiz #4006, …) para o centro dos reinos em `(1702,1726)` <br/>
⏳ /crias: se teleportará para o drop de crias (Sleipnir e Svaldfire) — sem coordenada na fonte legada <br/>
✅ /destravar40: destrava o level 40 do celestial (seta o gate `QuestInfo.Celestial.Lv40`; efetivo só para chars Celestial) <br/>
✅ /destravar90: destrava o level 90 do celestial (gate `Lv90` + dá a Cythera Mística item 3502) <br/>
✅ /arcana: realiza a quest da cythera arcana (seta `Circle` + põe o item 3507 no Equip[1]) <br/>
✅ Pedra da Fúria (3020): Lv90, Arcana e Reset do Celestial pelo item, cobrando 500 de Fama (#408) <br/>
✅ Diadema do Sub-Celestial: ao chegar aos níveis 120/150/180/199 (armazenados), o Equip[1] ganha EF_AC/EF_HP 30/40, 35/50, 45/60 e 50/70, uma vez por nível (`SetCircletSubGod`, flags `Add120..200`; #409) <br/>
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

# Pista de Runas
Fale com o Uxmal (perto de 3293,1693) nos minutos 16–19, 36–39 ou 56–59, sendo líder do grupo ou
jogando solo, com a Pista da Runas no inventário. A refinação da pista escolhe a sala (+0 Lich, +1
Torre, +2 Amon, +3 Sulrang, +4 Labirinto, +5 Balrog, +6 Coelho). A pista é consumida e aparece
"Entrada registrada.".

No minuto seguinte (:00, :20 ou :40), o líder e os membros do grupo que estiverem perto do Uxmal
entram na sala. Aos 15 minutos da rodada todos voltam para fora.

Ao concluir cada sala, o líder e cada membro em jogo ganham uma runa, e o líder ganha a pista da
sala seguinte:

- **+0 Lich:** cada Lich morto tem 20% de chance de concluir a sala. Se falhar, os Lichs voltam em
  dobro.
- **+1 Torre:** cada kill conta para o seu grupo enquanto a sua torre estiver de pé. Na saída, o
  grupo com mais kills, estritamente à frente dos outros dois, ganha.
- **+2 Amon:** matar o chefe conclui. Só líderes inscritos entram na câmara do chefe; quem não for
  volta para a entrada.
- **+3 Sulrang:** o primeiro Sulrang morto chama o Kalintz, e cada Kalintz morto chama o próximo e
  pontua para o grupo do matador. Na saída, ganha quem tiver mais pontos.
- **+4 Labirinto:** limpe os blocos até zerar o contador do grupo e enfrente o chefe. As armadilhas
  do chão causam 2000 de dano, exceto no bloco onde estiver um líder inscrito.
- **+5 Balrog:** os portais da sala custam uma Semente de Cristal (4032) por salto e levam o grupo
  para uma sala sorteada. A última sala tem o Balrog.
- **+6 Coelho:** o contador começa em 100. Cada coelho morto desconta 1, e cada um que foge devolve 1.
  No zero aparece o Rei Carbuncle, que só dá runas.

Se o líder sair do grupo, a inscrição passa para o novo líder.

- ✅ Inscrição, entrada e saída
- ✅ Kills, chefes, runas, prêmios, portais do Balrog, contador do Coelho, guarda do Amon e
  armadilhas do Labirinto

# Campo de Treino
Personagem novo deve chegar ao nível 35 em cerca de 15 minutos caçando no Campo de Treino (células
`0x80` do `AttributeMap.dat`, por volta de X 2064–2159, Y 1932–2051).

Kills de mobs que nascem no campo, feitos por mortais até o nível 35, pagam a recompensa base
multiplicada por 6. No campo não entram os eventos globais: nem a redução do Kefra (÷2), nem os ±15%
do newbie, nem o double. É um desvio intencional do legado (issue #383). Para calibrar, use
`-training-field-exp` / `W2PP_TRAINING_FIELD_EXP`; `0` volta à recompensa normal.

Os geradores de nível 18–34 do campo (#1650, #1671, #1684, #1697) mantêm até 3 mobs cada.

- ✅ Multiplicador de EXP do campo
- ⏳ Trava de entrada para nível 36+ (`_MSG_Action.cpp:213`)

# Evoluções 
NPC Evoluções vende poeira, upe o seu Mortal, Arch, Celestial e Sub Celestial com ela.

Para liberar a Soul permanente do Mortal no nível técnico 369+, leve à Kibita a Pedra Secreta da
classe: TransKnight usa Água (5334), Foema usa Sol (5336), Beastmaster usa Terra (5335) e Huntress
usa Vento (5337). A pedra será consumida, a capa será substituída pela versão do reino e o
personagem retornará à seleção para recarregar a progressão.

Faça as quest dos quatros cristais no seu Arch para liberar mais pontos.

Para criar o Celestial, use um Arch nível 356+ cujo Mortal de origem tenha chegado ao nível 100,
remova a armadura e clique com o botão direito na Pedra Ideal (1742). O Rei não cria Celestial:
no legado essa entrada é o uso da pedra (`_MSG_UseItem.cpp`). Com o Rei, a Pedra Ideal (equipada
junto da Sephirot) só serve para o Mortal 299+ virar Arch.
- Use /rei ou /king para ir ao rei correspondente à capa equipada.
• Não precisa transformar o Lac, somente separe 10 que já vai funcionar

Para destravar o lv 40 e 90 do Cele utilize o comando /destravar40 e /destravar90

Pegue lv 200 no seu Cele e vire Sub Cele, faça a quest da Cythera Arcana e os três resets. (Escreva /arcana para fazer a quest da Arcana automaticamente).
• Refine a capa para +9 logo após disso.
