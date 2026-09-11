package clientrarity

import "regexp"

// The accessory and consumable tiers, group by group, as approved on the
// "Mesa de Raridade" review (2026-09-11). Equipment follows a rule the catalog
// already encodes (grade, EF_MOBTYPE, level); these do not — a Colar and an
// Anel carry no letter — so they are listed. An item no group takes stays
// None: the tooltip is the client's own until someone decides.

// The accessory slots, as ItemList's nPos bit mask read unsigned: ring,
// amulet, orb, stone, guild, fairy, cape, and two combined masks the catalog
// uses for spirit stones and one weapon-like accessory.
func isAccessory(pos int) bool {
	switch pos {
	case 256, 512, 1024, 2048, 4096, 8192, 32768, 3840, 2667:
		return true
	}
	return false
}

func span(a, b int) []int {
	out := make([]int, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, i)
	}
	return out
}

func join(parts ...[]int) []int {
	var out []int
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// accessoryByIndex is built once from the group list below.
var accessoryByIndex = func() map[int]Tier {
	groups := []struct {
		tier Tier
		ids  []int
	}{
		{Comum, span(501, 506)},                        // Anéis, lv 47-50
		{Incomum, []int{507, 510, 511, 512, 513, 514}}, // Bracelete (Hércules … Hécate), lv 123-133
		{Raro, span(591, 595)},                         // Brincos, lv 161-174
		{Raro, span(516, 521)},                         // Pingentes, lv 184-185
		{Epico, span(640, 643)},                        // Colares, lv 306
		{Comum, span(551, 558)},                        // Amuletos de Prata e de Ouro
		{Incomum, span(559, 566)},                      // Amuletos Místico e de Cristal
		{Raro, span(567, 570)},                         // Amuleto Arcano
		{Comum, span(601, 604)},                        // Orbes
		{Incomum, span(608, 611)},                      // Olhos
		{Raro, span(612, 615)},                         // Defesas elementais
		{Comum, []int{651, 652, 653, 657}},             // Rubis
		{Incomum, span(762, 768)},                      // Netuno a Júpiter, lv 54
		{Incomum, span(654, 656)},                      // Pedras Necromânticas
		{Raro, span(658, 663)},                         // Gemas da Siren e Ankhs
		{Raro, span(1760, 1763)},                       // Sephirot
		{Epico, span(1752, 1759)},                      // Pedras de chefe
		{Epico, []int{540, 541, 631, 632, 633}},        // Pedras Espirituais
		{Lendario, []int{3464}},                        // Pedra Amunra
		{Incomum, join(span(3900, 3908), []int{3911, 3912, 3913, 3916})}, // Fadas com prazo
		{Raro, []int{3914}},                                                 // Fada Prateada
		{Epico, []int{3915}},                                                // Fada Dourada
		{Raro, []int{753, 769, 1726}},                                       // Familiares: Imp, Nyerdes, Griupan
		{Incomum, []int{447, 692}},                                          // Pedaços do Círculo Divino
		{Epico, []int{448, 449, 450}},                                       // Círculo Divino Puro
		{Lendario, []int{693, 694, 695}},                                    // Círculo Divino Completo Puro
		{Epico, []int{3457, 3458, 3459, 3460, 3462}},                        // Selos
		{Epico, []int{774, 775}},                                            // Pedras da Troca
		{Raro, []int{4080, 4081, 4060}},                                     // Emblemas de progresso
		{Comum, span(3397, 3406)},                                           // Tinturas
		{Raro, []int{543, 544, 545, 546, 548, 549, 1766, 1768, 1769, 4006}}, // Mantos iniciais
		{Epico, []int{734, 735, 736, 737}},                                  // Capas de reino
		{Lendario, span(3191, 3196)},                                        // Capas Elite e Herói
		{Mitico, []int{3197, 3198, 3199, 4008, 4009, 572, 573, 574, 1720, 1767, 1770, 1771}}, // Capas Mestre e Campeão
		{Lendario, []int{290}},                               // Manto Negro Lendário
		{Raro, []int{523, 1738}},                             // Itens de casal
		{None, join([]int{786, 1936, 1937}, span(794, 799))}, // itens de monstro e sem nome
	}
	m := map[int]Tier{}
	for _, g := range groups {
		for _, id := range g.ids {
			m[id] = g.tier
		}
	}
	return m
}()

var (
	reTraje      = regexp.MustCompile(`Conjunto`)
	reInsigniaGu = regexp.MustCompile(`Medalha|Símbolo`)
)

func classifyAccessory(it Item) Tier {
	if t, ok := accessoryByIndex[it.Index]; ok {
		return t
	}
	if it.Pos == 4096 {
		switch {
		case reTraje.MatchString(it.Name):
			return Raro // trajes de 30 dias
		case reInsigniaGu.MatchString(it.Name):
			return None // medalhas e símbolos de guilda: insígnia
		}
	}
	return None
}

// consumableGroups are tried in order; the first that takes an item decides.
// The order matters where names overlap — "Pergaminho da Ressurreição" is an
// evocation before it is a scroll, "Escritura de Oriharucon" a refining
// material before it is an entry.
var consumableGroups = []struct {
	tier Tier
	test func(it Item) bool
}{
	{None, match(`^(not used|used|N/A|NONAME|Moita|Presente de Teste)$`)},                    // sobras do catálogo
	{Epico, indexIn(5500, 5547)},                                                             // livros de skill
	{None, func(it Item) bool { return between(it, 5000, 5102) || between(it, 5400, 5447) }}, // skills em forma de item
	{Raro, func(it Item) bool { return between(it, 5110, 5133) || it.Name == "Runa" }},       // runas
	{Raro, match(`\(\d+\s*dias\)`)},                                                          // poções com prazo
	{Comum, match(`Poção|Potion|Kit de (Cura|Mana)|Ervas|Antídoto|P[ií]lula|Pão|Salsicha|Carne|Comida|Frango|Bebida|Marmita|Panqueca|Chocolate|Coração Doce|Remédio|Elixir|Água das Fadas|Feijão|Batedor`)},
	{Comum, match(`Ração|Curar Montaria|Retornar Cavalo|Acelerador|Mount Growth|Restaurador de Montaria`)},
	{Epico, match(`^(Diamante|Esmeralda|Coral|Garnet)$|^Gema de `)}, // gemas de refino
	{Lendario, match(`Pedra Ideal|Pedra Secreta`)},                  // evolução Celestial
	{Raro, match(`Poeira|Oriharucon|Lactolerium|Resto de|Pedra da Luz|Jóia d|Barra de Mithril|Cristal Extração|Pedaço de|Composto de|Semente de Cristal|Pedra de (Spinner|Beril|Tectita|Adamantita)|Pedra Lendária|Gema Estelar|Safira|Pedra da Fúria|Pedra do Sábio|Item Refinado`)},
	{Raro, match(`Catalisador|Restaurador de`)},      // catalisadores e restauradores
	{Raro, match(`^Cristal `)},                       // cristais
	{Lendario, match(`10ª Skill`)},                   // baú de 10ª skill
	{Epico, match(`Grande Baú|Raid Box|Money Cube`)}, // grandes baús
	{Raro, match(`Baú|Bau\b|Caixa|Pacote|Bolsa|Presente|Envelope|Esfera da Sorte|Caça Níquel|TOTO`)},
	{Incomum, match(`Cupom|Coupon`)},
	{Epico, match(`^Moeda WYD|^Barra de Ouro`)}, // crédito de cash (EF_DONATE), antes do grupo das moedas
	{Incomum, match(`Moeda|Barra de Prata`)},
	{Epico, match(`Traje Mont|Wooden horse`)},
	{Epico, match(`Emblema|Medalha|Selo|Honor|Marca do`)},
	{Epico, match(`^Classe `)},
	{Epico, match(`Bênção|Proteção Divina|Trava de Item|Perdão|Reforma|Retorno da Habilidade|Ticket Serviço|Peerage|Mandado`)},
	{Raro, match(`Evoca|Trans|Canhão|Muro de Espinhos|Ressurreição|Concentração|Força Espectral|Contrato`)},
	{Incomum, match(`Pergaminho|Pergamino|Mapa|Chamado Real|Olho Crescente`)},
	{Incomum, match(`Pesadelo|Portão do Inferno|Convite|Ingresso|Entrada|Passagem|Porta PvP|Portão Temporal|Carta de Duelo|Pedido de Caça|Chave|Declaração|Recusa|Escritura`)},
	{Comum, match(`Fogos|Natal|Bola de Neve|Galho|Freixo|Chapéu|Cenoura|Árvore|Evento|Trombeta|Vela|Colheita`)},
	{Incomum, match(`Unha|Coração|Cabelo|Alma|Folha|Fruto|Lágrima|Olho de|Molar|^Pedra |Janela|Escudo Remanescente|Sussurro|Adelas|Símbolo|Pergaminho Selado|Portão|Porta|Pedra Misteriosa|Back Ho|Bracelete`)},
}

func match(expr string) func(Item) bool {
	re := regexp.MustCompile(expr)
	return func(it Item) bool { return re.MatchString(it.Name) }
}

func indexIn(a, b int) func(Item) bool {
	return func(it Item) bool { return between(it, a, b) }
}

func between(it Item, a, b int) bool { return it.Index >= a && it.Index <= b }

func classifyConsumable(it Item) Tier {
	for _, g := range consumableGroups {
		if g.test(it) {
			return g.tier
		}
	}
	return None
}
