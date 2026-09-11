package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// celestialPools rebuilds a Celestial's equipment-free MaxHP/MaxMP from
// BASE_GetHpMp (level.BasePools), where the level counts +MAX_LEVEL. Any other
// tier is left alone. The live HP/MP are not touched: they are clamped to the
// new maxima by the next refreshScore, and a Celestial repaired upward simply
// has room to regenerate into.
func celestialPools(e *world.Entity) {
	if !level.IsCelestialTier(e.ClassMaster) {
		return
	}
	e.BaseMaxHP, e.BaseMaxMP = level.BasePools(e.Class, e.ClassMaster, e.Level, int32(e.BaseCon), int32(e.BaseInt))
}
