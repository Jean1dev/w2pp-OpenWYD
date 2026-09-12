package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// /xp answers "what XP bonus am I actually getting right now?".
//
// It has NO legacy counterpart, and exists for the reason the login greeting
// does: every number that moves a player's XP — the chest, the fairy, the shop
// mount, the grade-7 and gem pieces, the server events, the Mesa de XP rate for
// the ground under their feet — is invisible in the client. The character window
// shows EXP and nothing about how fast it fills, so a player who buys a chest
// and feels no difference cannot tell a bonus that broke from one that was never
// applied.
//
// Two of those rules bite silently and are the real reason for the command: a
// bonus at or above limiteBonusXP is thrown away WHOLE, and the Fada Suprema's
// +30 does not pay inside Pesadelo. Both read as "the item does nothing" from
// the inside.

// linhaPainelMax is what one panel line holds. MSG_MessagePanel carries
// MessageLength-2 bytes of text and cuts the rest without a word
// (protocol.EncodeMessagePanelBody), so the breakdown is wrapped rather than
// truncated: whatever falls off the end is the piece the player was after.
const linhaPainelMax = protocol.MessageLength - 2

// estadoXP is the picture /xp reports, read off the world so the wording can be
// built and tested without standing up a dispatcher.
type estadoXP struct {
	// Zona is where the character is STANDING. The reward branch is chosen by
	// the corpse's block (level.ZoneForKill), so this answers for a kill made
	// where the player is — which is the question being asked.
	Zona level.Zone

	Parcelas expBonusParcelas // the equipment split
	Bau      int32            // AffExpBonus: the Baú de XP affect
	BauTicks uint32           // its remaining affect ticks, 0 when not running
	Suprema  int32            // fairyContentBonus: the Fada Suprema's flat +30

	// EmGrupo only says whether to mention the party rule. The number a party
	// actually pays is the best bonus among everyone IN THE FIGHT
	// (bonusDoGrupo), which depends on who is standing near the corpse at the
	// moment of the kill — not something a command can answer in advance.
	EmGrupo bool

	TaxaPercent int32 // the Mesa de XP rate for this zone and tier
	Eventos     expEventsView
}

// showXPBonus backs /xp (alias /bonus).
func (d *Dispatcher) showXPBonus(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	st := estadoXP{
		Zona:     level.ZoneForTile(int32(e.X), int32(e.Y)),
		Parcelas: d.equipExpBonusParcelas(e),
		Bau:      e.AffExpBonus,
		BauTicks: afetoRestante(e, world.AffectExpChest),
		Suprema:  fairyContentBonus(e),
		EmGrupo:  e.Leader != 0 || partyMemberCount(e) > 0,
		Eventos: expEventsView{
			DoubleMode:  d.expEvents.DoubleMode,
			NewbieEvent: d.expEvents.NewbieEvent,
			KefraLive:   d.expEvents.KefraLive,
		},
	}
	st.TaxaPercent = d.xpConfig.RatePercent(st.Zona, level.TierKey(e.ClassMaster))
	for _, linha := range textoXP(st) {
		sendClientMessage(w, s, linha)
	}
}

// afetoRestante is the time left on one affect slot, in affect ticks. 0 means
// the affect is not running.
func afetoRestante(e *world.Entity, tipo uint8) uint32 {
	for i := range e.Affect {
		if e.Affect[i].Type == tipo {
			return e.Affect[i].Time
		}
	}
	return 0
}

// tempoAfeto renders affect ticks the way a player reads a buff bar. One tick
// is 8 seconds of real time (affectTickPeriod).
func tempoAfeto(ticks uint32) string {
	sec := int(ticks) * affectTickPeriod
	switch {
	case sec >= 3600:
		return fmt.Sprintf("%dh%02dm", sec/3600, (sec%3600)/60)
	case sec >= 60:
		return fmt.Sprintf("%dm", sec/60)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

// textoXP builds the answer, one string per panel line.
func textoXP(st estadoXP) []string {
	bonus := st.Bau + st.Parcelas.Total()
	// The very function the reward calls, so the percentage printed here is the
	// one the next kill multiplies by — the 500% gate and the Pesadelo fairy
	// rule included.
	efetivo := level.ItemBonusApplied(st.Zona, bonus, st.Suprema)

	var linhas []string
	if efetivo == 0 && bonus <= 0 {
		linhas = append(linhas, fmt.Sprintf("Bônus de XP de itens aqui (%s): nenhum.", st.Zona.Name()))
	} else {
		linhas = append(linhas, fmt.Sprintf("Bônus de XP de itens aqui (%s): +%d%%", st.Zona.Name(), efetivo))
	}
	if bonus >= limiteBonusXP {
		linhas = append(linhas, fmt.Sprintf(
			"Os seus itens somam +%d%%, mas de %d%% para cima o jogo ignora o bônus inteiro.",
			bonus, limiteBonusXP))
	}

	minhaFada := st.Parcelas.Fada
	if st.Zona.CountsFairyContent() {
		minhaFada += st.Suprema
	}
	linhas = append(linhas, juntarPartes("Vem de: ", partesXP(st, minhaFada), linhaPainelMax)...)

	if st.Suprema > 0 {
		if st.Zona.CountsFairyContent() {
			linhas = append(linhas, "Os +30% da sua fada não valem dentro do Pesadelo.")
		} else {
			linhas = append(linhas, "A sua fada daria +30% a mais, mas isso não vale dentro do Pesadelo.")
		}
	}
	if st.EmGrupo {
		// Said as a rule and not as a number on purpose: bonusDoGrupo picks the
		// best bonus among whoever is IN the fight, so the figure changes with
		// who is standing near the corpse. Promising one here would be a lie the
		// next kill could contradict.
		linhas = append(linhas, "Em grupo vale o maior bônus entre quem está na luta, não o seu.")
	}
	linhas = append(linhas, linhaEventos(st.Eventos))
	if st.TaxaPercent != 100 {
		linhas = append(linhas, fmt.Sprintf("Taxa desta zona na Mesa de XP: %d%%", st.TaxaPercent))
	}
	return linhas
}

// partesXP is the breakdown of the character's own equipment and buffs.
// fadaVale is what the fairy is worth on this ground: its own bonus plus the
// Fada Suprema's +30 where the zone pays it.
func partesXP(st estadoXP, fadaVale int32) []string {
	var partes []string
	if st.Bau > 0 {
		p := fmt.Sprintf("Baú de XP +%d%%", st.Bau)
		if st.BauTicks > 0 {
			p += " (" + tempoAfeto(st.BauTicks) + ")"
		}
		partes = append(partes, p)
	}
	if fadaVale > 0 {
		partes = append(partes, fmt.Sprintf("Fada +%d%%", fadaVale))
	}
	if st.Parcelas.Montaria > 0 {
		partes = append(partes, fmt.Sprintf("Montaria +%d%%", st.Parcelas.Montaria))
	}
	if st.Parcelas.Grade7 > 0 {
		partes = append(partes, fmt.Sprintf("%d de grade 7 +%d%%", st.Parcelas.Grade7Pecas, st.Parcelas.Grade7))
	}
	if st.Parcelas.Joia > 0 {
		partes = append(partes, fmt.Sprintf("%d com joia +%d%%", st.Parcelas.JoiaPecas, st.Parcelas.Joia))
	}
	return partes
}

// linhaEventos is the server-wide half: the switches an operator flips, which
// no amount of gear explains.
func linhaEventos(ev expEventsView) string {
	linha := fmt.Sprintf("Servidor: EXP %s · %s", expMultiplierText(ev), kefraText(ev.KefraLive))
	if ev.NewbieEvent {
		linha += " · novato +25% até o nível 100"
	}
	return linha
}

// juntarPartes packs a breakdown into panel lines, joining the pieces with " · "
// and wrapping at the panel's width rather than letting the encoder cut the tail
// off. Continuation lines are indented so a wrapped list still reads as one
// answer.
func juntarPartes(prefixo string, partes []string, largura int) []string {
	if len(partes) == 0 {
		return nil
	}
	const sep = " · "
	const cont = "  "
	var out []string
	// The first piece always joins the prefix, even when the two together are
	// already too wide: moving it down would leave a line holding nothing but
	// "Vem de: ".
	linha := prefixo + partes[0]
	for _, p := range partes[1:] {
		if cand := linha + sep + p; larguraCliente(cand) <= largura {
			linha = cand
			continue
		}
		out = append(out, linha)
		linha = cont + p
	}
	return append(out, linha)
}

// larguraCliente is what a string costs on the wire: the panel counts CP1252
// bytes, not runes, so an accented line is measured as the client receives it.
func larguraCliente(s string) int { return len(protocol.ClientText(s)) }
