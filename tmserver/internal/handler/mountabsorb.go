package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
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
	if !mountrate.IsAdultMount(mount.Index) || mountHP(mount) <= 0 {
		return dam
	}
	percent := defaultMountAbsorb
	if p, ok := d.mountAbsorb.Percent(mount.Index, byPlayer); ok {
		percent = p
	}
	rider, absorbed := combat.MountAbsorb(dam, percent)
	if absorbed > 0 {
		// HALF of what was absorbed is charged to the mount, not all of it
		// (Server.cpp ProcessAdultMount call site, _MSG_Attack.cpp:1628). The
		// mount is a discount, not a second health bar.
		d.damageMount(w, victim, absorbed/2)
	}
	return rider
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
