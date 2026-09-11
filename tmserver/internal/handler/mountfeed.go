package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Ração: the hunger meter every mount carries in stEffect[2].cEffect
// (EF_MOUNTFEED, Basedef.cpp:1610). The Âmago refills it to 100; an hour online
// empties part of it; a mount that runs out dies of hunger (RegenMob,
// Server.cpp:4895-4929).
//
// The port had the refill and not the drain, so the meter sat at 100 forever and
// the only cost of owning a mount — keeping it fed — never came due.

// mountFeedPerHour is what an hour online costs a mount's ração.
//
// The legacy means two for the first half of each row and four for the second —
// `MountDiv > 15 ? 4 : 2` (Server.cpp:4898-4903) — but writes the row as
// `sIndex - 2330 % 30`, which C reads as sIndex - (2330 % 30) = sIndex - 20.
// That is always far above 15, so every mount has always paid four. Four is the
// behaviour players know: a full meter lasts 25 hours online.
const mountFeedPerHour = 4

// msgMountStarved is _NN_Mount_died (Language.txt:264), copied verbatim.
const msgMountStarved = "Sua montaria morreu de fome."

// tickMountFeed runs once per online hour for each player (the same gate that
// pays back a point of Chaos, pkPointRecoverPeriod) and charges the worn mount's
// ração.
//
// Only a LIVE cria or adult pays (Server.cpp:4895): a mount already at zero HP
// has nothing left to lose, and an egg is fed by incubation, not by ração.
func (d *Dispatcher) tickMountFeed(w *world.World, s *world.Session, e *world.Entity) {
	m := &e.Equip[mountEquipSlot]
	if m.Index < mountLo || m.Index >= mountHi || mountHP(*m) <= 0 {
		return
	}
	feed := int(m.Effects[2].Effect) - mountFeedPerHour
	if feed > 1 {
		m.Effects[2].Effect = uint8(feed)
		d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *m)
		return
	}

	// Starved. The legacy writes HP 0 and ração 0 (it stamps 4 and then 0 on the
	// same item through two pointers, :4910-4917; 0 is what survives).
	putShort(&m.Effects[0], 0)
	m.Effects[2].Effect = 0
	sendClientMessage(w, s, msgMountStarved)
	d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *m)
	if m.Index >= criaHi {
		// An adult stops lending its attributes the moment its HP is gone
		// (mountBonusFor gates on it), so the score has to be redrawn now, not on
		// the next unrelated refresh (Server.cpp:4921-4922).
		d.refreshScore(e)
		d.sendScore(w, s, e)
		return
	}
	// A cria walks beside its owner as a summon; a dead one has to go
	// (MountProcess, Server.cpp:4925).
	d.refreshBabyMountSummon(w, s, e)
}

// The ração item itself (EF_VOLATILE 15, _MSG_UseItem.cpp:1478-1561): dropped on
// the worn mount, it gives back some HP and a little of the meter the hour drains.
const (
	// racaoBase and racaoPackBase start the two ração rows. The (P) row starts one
	// BEFORE its first item (3367 is Água das Fadas, 3368 the Javali pack), so the
	// same `% 30` lands both rows on the same slot (:1514-1516).
	racaoBase     = 2420
	racaoPackBase = 3367

	racaoHP      = 5000  // HP one ração gives back (:1522-1525)
	mountHPCap   = 30000 // the ceiling that gain stops at
	racaoFeed    = 2     // meter one ração refills (:1527)
	mountFeedCap = 100
)

// msgMountNeedsCure is this fork's line for a ração dropped on a dead mount. The
// legacy hands the item back without a word (:1549-1550), which reads as "ração
// is broken" — the one thing the player needs to hear is where to take it.
const msgMountNeedsCure = "Sua montaria está morta. Leve-a ao Mestre de Montaria para curá-la."

// mountRacaoSlot is the ração row a mount eats from (:1494-1512). Every horse
// and the Svadilfari eat the Cavalo ração, the winged horses and the Sleipnir
// the Unicórnio's, the three griffins the Grifo's.
//
// That leaves the Ração de Svadilfari (2431) and de Sleipnir (2432) matching no
// mount at all — in the legacy too, and no NPC sells them (C._de_Montaria's shop
// is exactly the rows below).
func mountRacaoSlot(index int16) int {
	slot := (int(index) - mountLo) % mountRowSize
	switch {
	case slot >= 6 && slot <= 15, slot == 27:
		return 6
	case slot == 19:
		return 7
	case slot == 20:
		return 8
	case slot >= 21 && slot <= 23, slot == 28:
		return 9
	case slot >= 24 && slot <= 26:
		return 10
	case slot == 29:
		return 19
	}
	return slot
}

// racaoSlot is the row a ração item feeds, for either of the two item rows.
func racaoSlot(index int16) int {
	if index >= racaoPackBase {
		return (int(index) - racaoPackBase) % mountRowSize
	}
	return (int(index) - racaoBase) % mountRowSize
}

// useRacao feeds one ração to the worn mount (_MSG_UseItem.cpp:1478-1561).
//
// One per click, as in the legacy: unlike the Âmago there is no roll and no
// result to tally, so a batch would only hide how much each one gives.
func (d *Dispatcher) useRacao(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src int) {
	if body.DestType != 0 || body.DestPos != mountEquipSlot {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	dst := &e.Equip[mountEquipSlot]
	if dst.Index < mountLo || dst.Index >= mountHi {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	// The row is checked before the HP, in the legacy's order: the wrong ração on a
	// dead mount says "wrong ração", which is the mistake to fix first.
	if mountRacaoSlot(dst.Index) != racaoSlot(e.Carry[src].Index) {
		d.notify(w, s, NoticeMountNotMatch)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	// A dead mount does not eat: ração keeps a mount alive, only the Mestre de
	// Montaria brings one back (mountmaster.go).
	hp := int(mountHP(*dst))
	if hp <= 0 {
		sendClientMessage(w, s, msgMountNeedsCure)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	// DELIBERATE DIVERGENCE, inherited from amago.go: the legacy then runs
	// ProcessAdultMount, which clips an adult's HP to its evocation template's
	// MaxHp (Server.cpp:4738). This port clips nowhere — the Âmago sets 20000 —
	// and clipping here alone would make a ração LOWER the HP of a freshly fed
	// Unicórnio (template 12000). It would also leave the adult Javali, Lobo and
	// Urso at 100 HP, since those share the BM evocation files.
	hp += racaoHP
	if hp > mountHPCap {
		hp = mountHPCap
	}
	putShort(&dst.Effects[0], uint16(hp))
	feed := int(dst.Effects[2].Effect) + racaoFeed
	if feed > mountFeedCap {
		feed = mountFeedCap
	}
	dst.Effects[2].Effect = uint8(feed)
	racao := e.Carry[src].Index
	consumeOneItem(&e.Carry[src])

	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *dst)
	// A cria's summon carries the mount's HP; respawn it so the new HP shows
	// (MountProcess, :1535).
	d.refreshBabyMountSummon(w, s, e)
	d.log.Info("racao fed",
		"conn", s.Conn, "account", s.AccountName, "mount", dst.Index, "racao", racao,
		"hp", hp, "feed", feed)
}
