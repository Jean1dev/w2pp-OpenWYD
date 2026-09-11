package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Mount absorption: an adult mount eats part of every blow its rider takes, and
// pays for it with its own HP (_MSG_Attack.cpp:1520-1533, ProcessAdultMount at
// Server.cpp:4718).
//
// The legacy absorbs a flat 25% whoever is swinging. Here the share is read per
// lineage AND per attacker kind (0035_mount_absorb), which is what makes a mount
// a PvE mount or a PvP mount instead of one of thirty identical shields. A
// lineage nobody configured absorbs defaultMountAbsorb on both axes, so an
// untouched database plays exactly as the original did.

// defaultMountAbsorb is the legacy's flat share (_MSG_Attack.cpp:1524), used for
// any lineage with no row of its own.
const defaultMountAbsorb = domain.DefaultMountAbsorb

// absorbBlow moves part of a landed blow from the rider to the mount it is
// riding, and returns what still reaches the rider.
//
// byPlayer picks the axis, and it is about who SWUNG, not who was hit: the mount
// is the victim's, so the question it answers is "how well does this lineage
// defend against people" versus "against monsters".
//
// A victim with no adult mount, or one whose mount is already down, takes the
// blow whole — the legacy gates on the mount's HP being above zero for the same
// reason it gates the attribute bonus on it (Basedef.cpp:1616).
func (d *Dispatcher) absorbBlow(w *world.World, victim *world.Entity, dam int, byPlayer bool) int {
	if dam <= 0 || victim == nil || !world.IsPlayer(victim.ID) {
		return dam
	}
	mount := victim.Equip[mountEquipSlot]
	if extra, ok := mountbonus.TempExtra(mount.Index); ok {
		return absorbTempMount(dam, extra, byPlayer)
	}
	if !mountrate.IsAdultMount(mount.Index) || mountHP(mount) <= 0 {
		return dam
	}
	percent := defaultMountAbsorb
	if p, ok := d.mountAbsorb.Percent(mount.Index, byPlayer); ok {
		percent = p
	}
	rider, absorbed := combat.MountAbsorb(dam, percent)
	if absorbed > 0 {
		d.damageMount(w, victim, mountCharge(absorbed, byPlayer))
	}
	// A landed blow always reaches the rider for at least 1, on both legacy
	// paths (`if (DamageNow <= 0) DamageNow = 1`, _MSG_Attack.cpp:1529 and
	// Server.cpp:10035). Without it a 1-damage hit came out as 0 for the rider,
	// so anyone mounted was immune to exactly the hits an armored character
	// takes most.
	if rider <= 0 {
		rider = 1
	}
	return rider
}

// absorbTempMount is the cash-shop mounts' absorption (mountbonus.TempExtra):
// new, not the legacy's — a temporary mount absorbed nothing there. It has no
// HP of its own to pay with, so it only takes its share off the blow; its limit
// is the time on the item, not its health. The rider still takes at least 1.
func absorbTempMount(dam int, extra mountbonus.Extra, byPlayer bool) int {
	percent := extra.AbsorbPvE
	if byPlayer {
		percent = extra.AbsorbPvP
	}
	if percent <= 0 {
		return dam
	}
	rider, _ := combat.MountAbsorb(dam, percent)
	return max(rider, 1)
}

// mountCharge is how much of what the mount absorbed comes off its own HP.
//
// The two legacy paths disagree, and the port had collapsed them into the
// PvP one. A PLAYER's blow charges half (ProcessAdultMount(idx, _calcDamage/2),
// _MSG_Attack.cpp:1629); a MONSTER's charges all of it (ProcessAdultMount(Target,
// damage), Server.cpp:10115 and ProcessSecMinTimer.cpp:2385).
//
// Halving the monster path was not a small drift. Against an armored rider a
// mob lands 1 to 6: 25% of that is 1, and half of 1 is zero — so the mount paid
// nothing, hit after hit, and its HP simply never moved while farming.
func mountCharge(absorbed int, byPlayer bool) int {
	if byPlayer {
		return absorbed / 2
	}
	return absorbed
}

// damageMount charges the mount for what it just ate (ProcessAdultMount,
// Server.cpp:4718): HP down, never below zero, and a mount that reaches zero
// loses its feed too, so reviving it is not free.
//
// It deliberately does NOT heal on a negative loss and does not clamp to the
// mount's maximum HP: the legacy's clamp exists because the same function is
// reused to feed the mount, and this caller only ever damages.
func (d *Dispatcher) damageMount(w *world.World, e *world.Entity, lost int) {
	if lost <= 0 {
		return
	}
	mount := &e.Equip[mountEquipSlot]
	hp := int(mountHP(*mount))
	if hp <= 0 {
		return
	}
	hp -= lost
	if hp < 0 {
		hp = 0
	}
	putShort(&mount.Effects[0], uint16(hp))
	if hp == 0 {
		// A mount at zero is unfed as well as down (Server.cpp:4747), so it stops
		// giving its attribute bonus until the owner both revives and feeds it.
		mount.Effects[2].Effect = 0
	}

	s := w.Session(e.ID)
	if s == nil {
		return
	}
	d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *mount)
	if hp == 0 {
		// Crossing zero takes the mount's damage/magic out of the score, the way
		// the legacy's SendEquip(conn, 0) makes the client redraw a mount that
		// stopped counting (Server.cpp:4750). Recomputing only on the crossing
		// keeps this off the hot path of every single hit.
		d.refreshScore(e)
		d.sendScore(w, s, e)
	}
}
