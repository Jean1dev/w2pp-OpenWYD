package itembrowser

// Command documents one GM/moderation subcommand of the in-game command bus.
//
// Everything here is transcribed from the implementation
// (tmserver/internal/handler/gm.go and weather.go), not from the legacy
// Source/Comandos GM.txt: the legacy list includes commands this rewrite has
// not ported, and a reference that promises them would send a GM chasing a
// command that silently does nothing.
type Command struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
	Args    string   `json:"args,omitempty"`
	Summary string   `json:"summary"`
	// Target says who the command acts on, so the reader does not have to guess
	// whether an argument is missing — several commands only affect the caller.
	Target string `json:"target"`
	// Notes carry the traps: silent no-ops, clamps and known divergences.
	Notes []string `json:"notes,omitempty"`
}

// CommandBus describes the entry point shared by every subcommand.
type CommandBus struct {
	Entry    string    `json:"entry"`
	Access   string    `json:"access"`
	Notes    []string  `json:"notes"`
	Commands []Command `json:"commands"`
}

// CommandReference returns the GM command tab's content.
func CommandReference() CommandBus {
	return CommandBus{
		Entry:  "/gm <subcomando> [parâmetros]",
		Access: "account.role = 'moderator' ou 'admin'",
		Notes: []string{
			"O cliente envia /gm como um sussurro para o alvo \"gm\"; o servidor intercepta antes da entrega normal (handler/chat.go).",
			"Sem o role necessário o comando é ignorado em silêncio — é o comportamento do legado, não um bug.",
			"O privilégio é lido só no login: mudar account.role exige relogar.",
			"Todo comando é registrado no log de auditoria (conta, id, role, subcomando e parâmetros) ANTES de executar.",
			"Hoje 'moderator' e 'admin' executam a mesma lista; o tier só diferencia quem pode ser kickado.",
		},
		Commands: []Command{
			{
				Name: "kick", Args: "<personagem>", Target: "outro jogador",
				Summary: "Desconecta um jogador online pelo nome do personagem.",
				Notes: []string{
					"Recusa alvo de tier igual ou superior ao seu.",
					"A desconexão faz o teardown completo, salvando o personagem.",
					"Alvo offline responde com o aviso de \"não conectado\".",
				},
			},
			{
				Name: "notice", Aliases: []string{"aviso"}, Args: "<texto>", Target: "servidor",
				Summary: "Anúncio para todos os jogadores em jogo.",
				Notes: []string{
					"Sai como linha de chat prefixada \"[GM]\", não como o aviso dourado centralizado: o pacote do aviso real ainda não foi capturado (UNVERIFIED).",
					"Sem filtro de distância — alcança todas as sessões.",
				},
			},
			{
				Name: "goto", Aliases: []string{"ir"}, Args: "<personagem>", Target: "você",
				Summary: "Teleporta VOCÊ até um jogador online.",
			},
			{
				Name: "pos", Aliases: []string{"xy"}, Args: "<x> <y>", Target: "você",
				Summary: "Teleporta VOCÊ para uma coordenada do mapa.",
				Notes: []string{
					"É o goto para onde não há ninguém a quem seguir.",
					"Fora do mapa o comando recusa e diz qual é o limite.",
				},
			},
			{
				Name: "summon", Aliases: []string{"puxar"}, Args: "<personagem>", Target: "outro jogador",
				Summary: "Traz um jogador online até a SUA posição.",
				Notes:   []string{"É o inverso do goto: aqui quem se move é o alvo."},
			},
			{
				Name: "spawn", Args: "<id>", Target: "mundo",
				Summary: "Cria uma criatura na sua posição.",
				Notes: []string{
					"O id é o índice no roster de summons do BM (o único catálogo de templates indexado por id em memória), não o índice do mob no NPCGener.",
					"Id inexistente ou mundo cheio falha em silêncio, só com registro no log.",
				},
			},
			{
				Name: "item", Args: "<índice> [quantidade]", Target: "você",
				Summary: "Coloca um item no seu inventário.",
				Notes: []string{
					"A quantidade grava EF_AMOUNT e é limitada a 120; sem ela o item vem com 1.",
					"Precisa de um slot livre acessível na mochila, senão avisa e não faz nada.",
					"O índice é o mesmo da aba Itens.",
				},
			},
			{
				Name: "setlevel", Args: "<n>", Target: "você",
				Summary: "Eleva o SEU nível.",
				Notes: []string{
					"Só SOBE: pedir um nível igual ou menor que o atual não faz nada.",
					"Teto de 399 (Mortal/Arch); valores maiores são cortados.",
					"Em personagem Celestial o número pedido não é o que sai: o Exp é calculado na curva Mortal mas consumido na curva Celestial, cujo teto é 199.",
					"Os portões celestiais de 39 e 89 continuam travando sem /destravar40 e /destravar90.",
				},
			},
			{
				Name: "setgold", Args: "<n>", Target: "você",
				Summary: "Define o SEU ouro carregado.",
				Notes:   []string{"Valor negativo é rejeitado."},
			},
			{
				Name: "questreset", Args: "<355|370|cristal|arch>", Target: "você",
				Summary: "Limpa as flags de quest do Arch para poder refazê-las.",
				Notes: []string{
					"Ferramenta de teste: as quests são de uma vez por personagem, e isto existe para exercitá-las depois de mudar o que elas concedem.",
					"Limpar 355 ou 370 rearma a trava daquele nível — o personagem volta a não ganhar experiência ali até refazer a quest. É proposital: restaura o estado que a quest espera encontrar.",
					"NÃO desfaz o que já foi concedido. HP e MP dos cristais ficam no personagem e empilham se a quest for refeita; AC e resistência voltam ao normal porque são derivados das flags ou vivem na capa.",
					"Anote os atributos antes de testar se os números exatos importarem.",
				},
			},
			{
				Name: "ban", Args: "<personagem|conta>", Target: "conta",
				Summary: "Bloqueia uma conta e derruba o jogador.",
				Notes: []string{
					"Se o jogador estiver online, o argumento é o nome do PERSONAGEM e o ban recai sobre a conta dele; offline, o argumento é tratado como nome de CONTA.",
					"Persistido pelo dbServer fora do loop; o login já rejeita conta bloqueada, então a volta é negada de imediato.",
				},
			},
			{
				Name: "unban", Args: "<conta>", Target: "conta",
				Summary: "Remove o bloqueio de uma conta.",
			},
			{
				Name: "guildname", Args: "<id> <nome>", Target: "guilda",
				Summary: "Registra o nome de exibição de uma guilda.",
				Notes: []string{
					"Existe porque ainda não há criação de guilda em jogo: é a única forma de popular o nome que o /nick mostra.",
					"Id válido de 1 a 65535.",
				},
			},
			{
				Name: "guildfame", Args: "<id> <fama>", Target: "guilda",
				Summary: "Define a pontuação de fama de uma guilda.",
				Notes:   []string{"Exige exatamente dois parâmetros numéricos."},
			},
			{
				Name: "weather", Aliases: []string{"clima"}, Args: "<0|1|2|auto>", Target: "mundo",
				Summary: "Força o clima ou devolve o controle ao sorteio automático.",
				Notes: []string{
					"0 é céu limpo; 1 e 2 são as outras duas faixas, cujo nome real não está documentado na fonte.",
					"O legado não validava nada e mandava qualquer inteiro ao cliente; aqui valores fora de 0–2 são rejeitados.",
					"Sem argumento, responde com o texto de uso.",
				},
			},
		},
	}
}
