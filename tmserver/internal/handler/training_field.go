package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TrainingFieldAt reports whether world tile (x,y) is on the training grounds:
// AttributeMap bit 0x80, read the way GetAttribute does (GetFunc.cpp:1383 —
// one attribute byte per 4×4 tiles, out-of-range = 0). Boot uses it to mark the
// field generators; the movement handler uses it for the level lock
// (newbieZoneGuard).
func TrainingFieldAt(attrs *content.Grid, x, y int16) bool {
	if attrs == nil || x < 0 || y < 0 {
		return false
	}
	return attrs.At(int(x)/4, int(y)/4)&0x80 != 0
}

// Training-grounds lock (_MSG_Action.cpp:211-221). Level is the stored level, so
// 35 is displayed 36: displayed 35 is the last one allowed in. Stored 1000+ is
// GM, which the legacy leaves out of the range.
const (
	newbieZoneMinLevel = 35
	newbieZoneMaxLevel = 999
	// newbieZoneKickX/Y is the Armia drop point, randomized by rand()%3 on each axis.
	newbieZoneKickX int16 = 2091
	newbieZoneKickY int16 = 2101
)

// newbieZoneGuard ports the training-grounds check of _MSG_Action: a move whose
// destination carries the 0x80 bit is refused for stored level 35..999 or any
// non-mortal, with _NN_Newbie_zone and a teleport to Armia. It reports whether
// the move was refused. Like the legacy, only movement is guarded — teleports
// and login don't run this check.
func (d *Dispatcher) newbieZoneGuard(w *world.World, s *world.Session, e *world.Entity, x, y int16) bool {
	if !TrainingFieldAt(d.attrs, x, y) {
		return false
	}
	if (e.Level < newbieZoneMinLevel || e.Level > newbieZoneMaxLevel) && e.ClassMaster == classMasterMortal {
		return false
	}
	d.sendClientMessage(w, s, newbieZone)
	// Two rand()%3 draws, as in the legacy. Which one lands on X is UNVERIFIED
	// (MSVC usually evaluates call arguments right to left); the draw count is
	// what keeps later rolls in parity.
	kx := newbieZoneKickX + int16(w.Rand().Intn(3))
	ky := newbieZoneKickY + int16(w.Rand().Intn(3))
	d.doTeleport(w, s, kx, ky)
	return true
}
