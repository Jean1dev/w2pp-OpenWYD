package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

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
