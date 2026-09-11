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
✅ /arch: se teleportará para a cidade dos reinos (apenas o teleporte; o destrave em si é feito na NPC Lindy, ver abaixo) <br/>
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
✅ /nt: mostra quantas entradas de Pesadelo Arcano o personagem tem (`extra.NT`). Persistido em `character.nightmare_tickets`; a Escritura do Pesadelo dá 13 e cada entrada no Arcano gasta 1 ([pesadelo-plan.md](./migration/pesadelo-plan.md)) <br/>
✅ /nig: mostra o horário de cada tier do Pesadelo — qual está aberto e quanto falta para os outros. Desvio consciente: o legado imprime só o relógio (`!!HHMMSS`) e deixa o cliente calcular <br/>
✅ **/\<nomedojogador\>** (sem escrever nada depois): mostra o nick, a **cidadania** e a **fama** daquele jogador, mais a guild entre colchetes quando ele tem uma, e a **mensagem** dele (`/snd`) numa segunda linha se ele tiver posto uma. É o `_NN_Check_User_Info` do legado (`Cidadania: %d / Fama: %d`) — um sussurro **sem texto** não é sussurro, é "inspecionar". Escrever `/fulano oi` continua sendo sussurro normal <br/>
✅ /nick \<jogador\>: o mesmo que acima, em forma de comando — existe porque é descobrível, enquanto "digite o nome e não escreva nada" não é <br/>
✅ /snd \<texto\>: define a sua mensagem de recado, que aparece para quem te inspecionar. `/snd` sozinho limpa. Vale só enquanto você está logado — o legado também apaga a cada login — e é cortada em 96 caracteres <br/>
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
✅ /gm guerra torre aviso [min]: anuncia a Guerra de Torres agora; abre depois de `min` minutos (padrão 1) <br/>
✅ /gm guerra torre abrir [min]: abre a guerra na hora, por `min` minutos (padrão 24, o mesmo das 20:06 às 20:30) <br/>
✅ /gm guerra torre fim: encerra agora — a guilda com a torre ganha a fama, como no fim normal <br/>
✅ /gm guerra torre estado: fase, dono, horários e a agenda do painel <br/>
⏳ /gm guerra cidade / noatum: responde que ainda não existe; entram quando essas guerras forem portadas <br/>

> `notice` sai na linha de aviso do servidor (MSG_MessagePanel, ID 0 — o `SendNotice` do legado),
> prefixada `[GM]`. A guerra forçada ignora a hora e o interruptor do painel até terminar, e passa
> pelas mesmas transições da agendada: mesmos avisos a todos, mesma limpeza da área, mesmo prêmio.
>
> `ban`/`unban` gravam em `account.is_blocked` — o login já rejeita contas bloqueadas; a migração do ban administrativo para o binServer
> (entitlement) fica para uma issue futura (`web-platform-plan.md §binServer`).

# Evoluções 
NPC Evoluções vende poeira, upe o seu Mortal, Arch, Celestial e Sub Celestial com ela.

Para liberar a Soul permanente do Mortal no nível técnico 369+, leve à Kibita a Pedra Secreta da
classe: TransKnight usa Água (5334), Foema usa Sol (5336), Beastmaster usa Terra (5335) e Huntress
usa Vento (5337). A pedra será consumida, a capa será substituída pela versão do reino e o
personagem retornará à seleção para recarregar a progressão.

Faça as quest dos quatros cristais no seu Arch para liberar mais pontos.
- Dê /red ou /blue para ir direto para o rei desejado.
• Não precisa transformar o Lac, somente separe 10 que já vai funcionar

**Destrave do Arch (níveis 355 e 370) — NPC Lindy.** O Arch para de ganhar
experiência ao chegar no 355 e no 370 até fazer o destrave; é assim no servidor
original e é o que mantém o personagem dentro da janela da quest. A receita é
posicional, nos 7 primeiros espaços da composição e nesta ordem exata:

| Espaço | Item |
|---|---|
| 1 e 2 | Poeira de Lactolerium (#413) em pilha de **exatamente 10** |
| 3 | Pergaminho Selado (#4127) |
| 4 a 7 | Poeira de Lactolerium (#413) avulsa, uma por espaço |

O destrave do 355 entrega a capa do reino (Hekalotia, Akelonia ou Aventureiros,
conforme o clã); o do 370 consome 1 de Fama e exige Fama > 0.

**Bônus da capa no 370 — regra deste servidor, não do legado.** Concluir o
destrave do 370 grava dois efeitos **na própria capa do reino**: `EF_HP 120`
(+120 de HP máximo) e `EF_RESISTALL 8` (+8 de resistência nos quatro elementos).
O servidor original não dá nada nesse destrave — a flag
`QuestInfo.Arch.Level370` é lida em apenas três lugares, todos travas de nível ou
de experiência. A divergência é deliberada: a quest custa 1 de Fama e segura a
progressão até ser feita, então deixa algo em troca.

Os efeitos vão na **instância do item**, não no personagem. É assim que joia e
refino já funcionam, e tem duas vantagens: o tooltip da capa mostra o bônus sem
precisar alterar o conteúdo do cliente, e o servidor já soma os dois efeitos ao
pontuar equipamento. A capa tem três espaços de efeito e um `+9` ocupa um deles;
se não houver espaço para os dois, o destrave conclui mesmo assim e o jogador é
avisado — o bônus não some em silêncio.

**Comando de teste:** `/gm questreset 355 | 370 | cristal | arch` limpa as flags
para refazer a quest. Ele não desfaz o que já foi concedido — refazer depois de
um reset empilha HP/MP no personagem — então anote os números antes de testar.

Se o personagem passou do nível sem ter feito o destrave (possível em contas
antigas, antes do gate de experiência existir), a NPC ainda aceita a receita —
mas o personagem **volta para o nível da quest**, perdendo os níveis ganhos
indevidamente. Isso é uma divergência deliberada do servidor original, que exige
o nível exato e deixaria a conta travada para sempre.

Para destravar o lv 40 e 90 do Cele utilize o comando /destravar40 e /destravar90

Pegue lv 200 no seu Cele e vire Sub Cele, faça a quest da Cythera Arcana e os três resets. (Escreva /arcana para fazer a quest da Arcana automaticamente).
• Refine a capa para +9 logo após disso.
