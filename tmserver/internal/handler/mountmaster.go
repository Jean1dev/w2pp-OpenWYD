package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Mestre de Montaria (Merchant 58, MOUNT_MASTER — _MSG_Quest.cpp:170-236):
// brings back a mount that died, of hunger (tickMountFeed) or in combat
// (damageMount). It is the only way back: a dead mount refuses ração and Âmago.
//
// The cure costs twice. The gold is the mount's own catalog price, and the
// mount pays in vitality (EF_MOUNTLIFE, stEffect[1].cValue): each cure takes
// zero, one or two of its lives, and the one that takes the last destroys it.
const merchantMountMaster = 58

const (
	// mountCureLifeSpan is the legacy `vit -= rand() % 3` (:209): a cure takes
	// 0, 1 or 2 lives with equal odds.
	mountCureLifeSpan = 3
	// What a revived mount wakes up with (:215-216): 20 HP and 5 of ração. It is
	// alive, not fed — the next online hour takes 4, and 1 is already hunger
	// (tickMountFeed), so it has to be given ração before that hour is up.
	mountCureHP   = 20
	mountCureFeed = 5
	// mountCurePriceMax is the legacy's sanity cap on the price (:202).
	mountCurePriceMax = 2000000000
)

// The NPC's lines, copied verbatim from Language.txt (257-260) as the fallback
// for a server booted without -content.
const (
	msgCureAnimals = "Se trouxer um animal doente, eu irei curá-lo."
	msgCurePrice   = "Se deseja curar %s, o custo será de %d Gold." // _DS_S_cure_price_D (258)
	msgCured       = "Montaria foi curada."
	msgCureFailed  = "Tratamento de montaria falhou."
)

// mountMaster is the MOUNT_MASTER case of _MSG_Quest. confirm == 0 quotes the
// price; anything else pays and rolls.
func (d *Dispatcher) mountMaster(w *world.World, s *world.Session, e, npc *world.Entity, confirm int32) {
	m := &e.Equip[mountEquipSlot]
	// Every conversation is logged with what the NPC saw: the client decides
	// when to send the confirm, and a report of "the NPC does nothing" is only
	// answerable if we know whether it ever did.
	d.log.Info("mount master",
		"conn", s.Conn, "account", s.AccountName, "confirm", confirm,
		"mount", m.Index, "hp", mountHP(*m), "life", m.Effects[1].Value, "coin", e.Coin)

	// Nothing to cure: no mount, or one still alive (:175-185).
	if m.Index < mountLo || m.Index >= mountHi || mountHP(*m) > 0 {
		d.say(w, npc, "_NN_Cure_animals", msgCureAnimals)
		return
	}

	price := d.itemPrices[int(m.Index)]
	if confirm == 0 {
		sendSay(w, npc, fmt.Sprintf(msgCurePrice, d.itemName(m.Index), price))
		return
	}
	if e.Coin < price {
		d.notify(w, s, NoticeNotEnoughMoney)
		return
	}
	if price < 0 || price > mountCurePriceMax {
		return
	}
	e.Coin -= price

	// The lives come off before the outcome is known, and a mount left with none
	// is gone — the gold is not refunded (:205-222).
	life := int(m.Effects[1].Value) - w.Rand().Intn(mountCureLifeSpan)
	index := m.Index
	if life > 0 {
		m.Effects[1].Value = uint8(life)
		putShort(&m.Effects[0], mountCureHP)
		m.Effects[2].Effect = mountCureFeed
		d.say(w, npc, "_NN_Cured", msgCured)
	} else {
		*m = world.Item{}
		d.say(w, npc, "_NN_Cure_failed", msgCureFailed)
	}

	// A revived adult lends its attributes again (mountBonusFor gates on HP), and
	// a destroyed one stops for good: either way the score changed (:225-226).
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *m)
	d.refreshBabyMountSummon(w, s, e)
	d.sendEtc(w, s, e)
	d.log.Info("mount cured",
		"conn", s.Conn, "account", s.AccountName, "mount", index,
		"cured", life > 0, "life", max(life, 0), "price", price)
}
