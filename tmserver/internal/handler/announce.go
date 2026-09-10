package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Server-wide announcements for the things worth interrupting everyone for: a
// rebirth, an Ancient item, a high refine.
//
// DELIBERATE DIVERGENCE: the legacy announces guild wars, the Kefra kill and
// event drops (SendNotice, MobKilled.cpp:1480 and CWarTower.cpp), but nothing
// for these four. They are announced here because on a live server they are the
// events players actually gather around, and a rebirth nobody sees is a rebirth
// that may as well not have happened.
//
// They ride SendNotice — the message panel to every player in world — so they
// arrive as the server speaking, not as a player talking (HEADER.ID zero, see
// sendClientMessage).

// anctFamilyName is the CombineFamily name of the Ancient combine, shared so the
// announcement and the family definition cannot drift apart.
const anctFamilyName = "Anct"

// announceRefineLevel is the refine level from which a success is worth telling
// the server about. Below it the announcements would be constant noise: +9 and
// under are routine.
const announceRefineLevel = 10

// announceArch says a Mortal completed the Arch rebirth.
func (d *Dispatcher) announceArch(w *world.World, name string) {
	if name == "" {
		return
	}
	broadcastNotice(w, fmt.Sprintf("[EVENTO] %s renasceu como Arch! Parabéns!", name))
	d.log.Info("announce arch", "name", name)
}

// announceCelestial says an Arch ascended to Celestial.
func (d *Dispatcher) announceCelestial(w *world.World, name string) {
	if name == "" {
		return
	}
	broadcastNotice(w, fmt.Sprintf("[EVENTO] %s ascendeu a Celestial! Parabéns!", name))
	d.log.Info("announce celestial", "name", name)
}

// The three machines players gather around — the +10, the compositor and the
// Agatha — announce EVERY roll to the whole server, win or lose, with the number
// drawn against the chance it had to beat: "Fulano falhou em 47/41 ao passar
// Espada para +10."
//
// DELIBERATE DIVERGENCE, asked for by the server's staff: the legacy tells only
// the player, and only some machines print the "%d/%d" at all (as a debug
// suffix, _MSG_CombineItem.cpp:105). Here it is the point — a failure in front of
// everyone is what makes the next success worth watching.
//
// The chance printed is the one the roll actually compared against, after the
// Mesa das Máquinas and its band for that item, so two items on the same machine
// can read /41 and /30. It is taken from the call site, never recomputed here,
// so the line can never disagree with the outcome it reports.
//
// The announcement replaces the player's own outcome line on these machines: the
// broadcast reaches them too, and the same news twice in a row is noise.

// rollText is the "47/41": the number drawn, then the chance — the order the
// legacy prints them in. A roll at or under the chance succeeds (combine.Roll).
func rollText(roll, chance int) string {
	return fmt.Sprintf("%d/%d", roll, chance)
}

// announceRoll is the one shape every machine's line takes:
//
//	Fulano conseguiu em 25/41 passar Espada para +10!
//	Fulano falhou em 47/41 ao passar Espada para +10.
//
// acao is the infinitive clause ("passar Espada para +10", "compor X"), so every
// machine reads the same way and a new one is one call, not a new template.
func (d *Dispatcher) announceRoll(w *world.World, name, acao string, roll, chance int, success bool) {
	if name == "" {
		return
	}
	if success {
		broadcastNotice(w, fmt.Sprintf("%s conseguiu em %s %s!", name, rollText(roll, chance), acao))
	} else {
		broadcastNotice(w, fmt.Sprintf("%s falhou em %s ao %s.", name, rollText(roll, chance), acao))
	}
	d.log.Info("announce machine roll", "name", name, "acao", acao, "roll", roll, "chance", chance, "success", success)
}

// announceMais10 is the +10 machine's (Ailyn) line.
func (d *Dispatcher) announceMais10(w *world.World, name string, item int16, roll, chance int, success bool) {
	d.announceRoll(w, name, "passar "+d.itemName(item)+" para +10", roll, chance, success)
}

// announceComposicao is the compositor's line. item is what the combine makes —
// on a failure, what it would have made — because "compor Espada Anciente" is
// the news, not the name of the +9 that went into it.
func (d *Dispatcher) announceComposicao(w *world.World, name string, item int16, roll, chance int, success bool) {
	d.announceRoll(w, name, "compor "+d.itemName(item), roll, chance, success)
}

// announceAgatha is the ADD machine's line. It names the item that receives the
// ADD and never the ADD itself: which bonus a player just moved onto their
// weapon is theirs to show, not the server's to publish.
func (d *Dispatcher) announceAgatha(w *world.World, name string, item int16, roll, chance int, success bool) {
	d.announceRoll(w, name, "passar o ADD para "+d.itemName(item), roll, chance, success)
}

// announceSemSorteio is for a machine that did not roll — the Lindy with no
// chance set on the panel is a certain unlock, as it has always been — so there
// is no "47/41" to print, only the news.
func (d *Dispatcher) announceSemSorteio(w *world.World, name, feito string) {
	if name == "" {
		return
	}
	broadcastNotice(w, fmt.Sprintf("%s %s!", name, feito))
	d.log.Info("announce machine", "name", name, "feito", feito)
}

// announceRefine says a player took an item to +10 or beyond.
func (d *Dispatcher) announceRefine(w *world.World, name string, item int16, level int) {
	if name == "" || level < announceRefineLevel {
		return
	}
	broadcastNotice(w, fmt.Sprintf("[EVENTO] %s levou %s ao +%d!", name, d.itemName(item), level))
	d.log.Info("announce refine", "name", name, "item", item, "level", level)
}
