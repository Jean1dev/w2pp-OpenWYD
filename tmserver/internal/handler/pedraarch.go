package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Pedras Arch. Eight monster stones (1752..1759) are transmuted by dust into
// one of eight attribute stones (1744..1751), which a quest NPC later forges into
// a Sephirot (misc.go). The odds differ per source stone, and the two families do
// not mix: 1752-1755 only ever yield 1744-1747, and 1756-1759 only 1748-1751.
const (
	pedraArchLo = 1752 // Pedra do Lorde Orc — first stone that can be transmuted
	pedraArchHi = 1759 // Pedra do Rei Demonlord — last one

	// pedraArchSlot is Equip[11], the gem slot. Every stone ships nPos 2048
	// (1<<11), and the legacy refuses any other destination before reaching this
	// branch (_MSG_UseItem.cpp:338): the stone is transmuted while WORN, never
	// from the bag.
	pedraArchSlot = 11
)

// pedraArchOutcome is one rung of a stone's ladder: rolls under upTo produce
// stone. The rungs are read in order and the first match wins, exactly as the
// legacy's if/else chain does (_MSG_UseItem.cpp:525-659).
type pedraArchOutcome struct {
	upTo  int
	stone int16
}

// pedraArchTable is the legacy's eight ladders, verbatim. rate is the success
// cut: a roll above it fails and only costs the dust.
var pedraArchTable = map[int16]struct {
	rungs []pedraArchOutcome
	rate  int
}{
	// Lower family: 1744 Inteligência, 1745 Sabedoria, 1746 Misericórdia, 1747 Abismo.
	1752: {[]pedraArchOutcome{{56, 1744}, {80, 1745}, {90, 1746}, {93, 1747}}, 93},
	1753: {[]pedraArchOutcome{{21, 1744}, {76, 1745}, {86, 1746}, {90, 1747}}, 90},
	1754: {[]pedraArchOutcome{{3, 1744}, {21, 1745}, {76, 1746}, {85, 1747}}, 85},
	1755: {[]pedraArchOutcome{{3, 1744}, {10, 1745}, {25, 1746}, {80, 1747}}, 80},
	// Upper family: 1748 Beleza, 1749 Vitória, 1750 Originalidade, 1751 Reino.
	1756: {[]pedraArchOutcome{{50, 1748}, {62, 1749}, {68, 1750}, {70, 1751}}, 70},
	1757: {[]pedraArchOutcome{{9, 1748}, {59, 1749}, {63, 1750}, {65, 1751}}, 65},
	1758: {[]pedraArchOutcome{{2, 1748}, {8, 1749}, {58, 1750}, {62, 1751}}, 62},
	1759: {[]pedraArchOutcome{{2, 1748}, {5, 1749}, {10, 1750}, {60, 1751}}, 60},
}

func isPedraArch(index int16) bool {
	return index >= pedraArchLo && index <= pedraArchHi
}

// pedraArchRoll ports the branch's own die (_MSG_UseItem.cpp:517-520):
//
//	int _rd = rand() % 115;
//	if (_rd > 100) _rd -= 15;
//
// It is NOT the rand()%100 of the ordinary dust path. The fold leaves 0..100 with
// 86..99 landing twice as often as anything else — an oddity of the original that
// is kept because it is what tilts these odds in play.
func pedraArchRoll(r interface{ Intn(int) int }) int {
	rd := r.Intn(115)
	if rd > 100 {
		rd -= 15
	}
	return rd
}

// pedraArchResult resolves a roll into the stone it produces, and whether the
// attempt succeeded at all.
//
// DELIBERATE DIVERGENCE, one value wide: the legacy walks the ladder with `<` but
// tests success with `<=` (:661), so a roll landing exactly ON rate succeeds while
// matching no rung — and NextPedra keeps the `int NextPedra = 1744` it was
// initialised with (:522). For 1756-1759 that hands back a stone from the WRONG
// family: a Pedra do Lugefer turning into a Pedra da Inteligência, which no ladder
// of the upper family can otherwise produce. It is an off-by-one, not a design,
// and it fires on roughly one attempt in 115. Here the last rung is inclusive
// instead, so that roll yields the rarest stone of the correct family.
func pedraArchResult(source int16, roll int) (int16, bool) {
	t, ok := pedraArchTable[source]
	if !ok {
		return 0, false
	}
	if roll > t.rate {
		return 0, false
	}
	for i, rung := range t.rungs {
		last := i == len(t.rungs)-1
		if roll < rung.upTo || (last && roll == rung.upTo) {
			return rung.stone, true
		}
	}
	return 0, false
}

// refinePedraArch is the arch-stone branch of the dust path
// (_MSG_UseItem.cpp:514-714). On success the stone is transmuted IN PLACE, keeping
// its slot; on failure only the dust is spent.
//
// The failure is deliberately gentle, and that is parity: the legacy calls
// BASE_SetItemSanc(dest, 0, 0) there, which writes into an EF_SANC slot the stone
// does not have, so it returns without touching anything (Basedef.cpp:2319). The
// two neighbouring dust branches — sealed items and +10 earrings — really do
// destroy the target with BASE_ClearItem; this one does not, and porting it as a
// destroy would eat stones the original always gave back.
func (d *Dispatcher) refinePedraArch(w *world.World, s *world.Session, e *world.Entity, dst *world.Item, body protocol.MsgUseItemBody, src int) {
	// Worn in the gem slot, never refined from the bag: the legacy's destination
	// guard (:338) rejects any DestType other than EQUIP before this branch, and
	// every stone's nPos is 1<<11.
	if int(body.DestType) != world.ItemPlaceEquip || int(body.DestPos) != pedraArchSlot {
		d.refineReject(w, s, e, src, NoticeOnlyToEquips)
		return
	}

	source := dst.Index
	roll := pedraArchRoll(w.Rand())
	next, ok := pedraArchResult(source, roll)

	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])

	if !ok {
		// Only the dust burns. The stone stays exactly as it was, so another
		// attempt costs nothing but more dust.
		d.notify(w, s, NoticeFailToRefine)
		d.sendSlot(w, s, world.ItemPlaceEquip, int(body.DestPos), *dst)
		d.log.Info("pedra arch falhou", "conn", s.Conn, "pedra", source, "roll", roll)
		return
	}

	dst.Index = next
	d.refreshEquip(w, s, e)
	d.sendScore(w, s, e)
	d.sendSlot(w, s, world.ItemPlaceEquip, int(body.DestPos), *dst)
	d.notify(w, s, NoticeRefineSuccess)
	d.log.Info("pedra arch transmutada",
		"conn", s.Conn, "de", source, "para", next, "roll", roll)
}
