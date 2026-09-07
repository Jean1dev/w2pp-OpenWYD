package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Catalisadores (_MSG_UseItem.cpp:5013-5090, EF_VOLATILE 94): the seven items
// 3344..3350 that turn a CRIA into its adult mount immediately, skipping the
// levels the âmago would otherwise have to buy.
//
// Each catalyst only serves one group of lineages, and the groups are not
// contiguous — the legacy folds twenty-seven cria rows into seven buckets with a
// hand-written table (:5036-5056). Using the wrong one refuses with the same
// "this mount does not match" line the âmago uses.
//
// The catalyst does NOT keep the cria's level. It rolls the adult's vitality
// from it (rand()%20 + level) and then resets the level to zero, exactly as the
// âmago's own growth branch does — an adult always starts at 1 whatever the
// cria reached.

const (
	// catalisadorLo is the first catalyst item (Catalisador de Kapel).
	catalisadorLo = 3344
	// criaCatalisavelLo is the first cria a catalyst can act on. The three below
	// it (2330..2332) are absent from the legacy's table and stay out: they are
	// the rows that grow on a level threshold instead (25/50/100 in criaGrowsAt).
	criaCatalisavelLo = 2333
)

// catalisadorGrupo maps a cria to the catalyst that serves it, and reports
// whether any does.
//
// Written as an explicit switch rather than the legacy's chain of ifs. The
// original overwrites `mount` in place and then keeps testing the OVERWRITTEN
// value against later ranges (:5039-5056) — it happens to be harmless with these
// numbers, but it is a trap for whoever edits the table next, and reproducing it
// would carry the trap along with the behaviour.
func catalisadorGrupo(criaIndex int16) (int, bool) {
	slot := int(criaIndex) - criaCatalisavelLo
	switch {
	case slot >= 0 && slot <= 2:
		return 0, true // Kapel
	case slot >= 3 && slot <= 6, slot >= 8 && slot <= 11:
		return 1, true // Acuban
	case slot == 7, slot == 12, slot == 24:
		return 2, true // Mencar
	case slot >= 13 && slot <= 15:
		return 3, true // Birago
	case slot >= 18 && slot <= 20, slot == 25:
		return 4, true // Yus
	case slot >= 21 && slot <= 23:
		return 5, true // Makav
	case slot >= 16 && slot <= 17:
		return 6, true // Alperath
	}
	return 0, false
}

// useCatalisador grows the worn cria into its adult, if the catalyst matches it.
func (d *Dispatcher) useCatalisador(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src int) {
	// The catalyst is applied TO the worn mount, so the client has to name the
	// mount slot as the destination — the same shape the âmago requires
	// (:5023-5031).
	if body.DestType != 0 || body.DestPos != mountEquipSlot {
		d.notify(w, s, NoticeMountNotMatch)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	dst := &e.Equip[mountEquipSlot]
	grupo, ok := catalisadorGrupo(dst.Index)
	if !ok || grupo != int(e.Carry[src].Index)-catalisadorLo {
		// Covers three refusals with one line, as the legacy does: nothing in the
		// slot, an adult already, or the wrong catalyst for this lineage.
		d.notify(w, s, NoticeMountNotMatch)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	dst.Index += mountRowSize
	// Vitality carries over from the cria's level with a roll on top; the level
	// itself resets, so the adult starts its own climb (:5069-5072).
	dst.Effects[1].Value = uint8(w.Rand().Intn(20) + int(dst.Effects[1].Effect))
	dst.Effects[1].Effect = 0
	dst.Effects[2].Value = 0
	consumeOneItem(&e.Carry[src])

	d.notify(w, s, NoticeMountGrowth)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *dst)
	// The cria walked beside its owner as a summon; the adult is ridden instead,
	// so the pet has to go — the same cleanup the âmago's growth path does.
	d.refreshBabyMountSummon(w, s, e)
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.log.Info("catalisador used",
		"conn", s.Conn, "account", s.AccountName, "mount", dst.Index, "grupo", grupo)
}
