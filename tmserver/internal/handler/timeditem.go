package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// A temporary item has two lives, and the difference is the whole point of this
// file. Before it is activated it holds a DURATION and does not age: a Conjunto
// bought today and left in the bag is still worth thirty days next month. Once
// activated it holds a DEADLINE and ages on the wall clock.
//
// Those two states are the item's own fields, so they persist with it and no new
// column is needed:
//
//	un-started: Effects = EF_WDAY/EF_HOUR/EF_MIN, ExpiresAt = 0
//	running:    ExpiresAt = the instant it dies, Effects cleared
//
// The fairies are the exception the legacy already made: their tooltip says "Não
// gasta caso não esteja equipado", and ProcessSecMinTimer.cpp:612 backs it by
// ticking BASE_CheckFairyDate only on Equip[13]. They therefore never convert —
// they stay in the duration form for life and are decremented a minute at a time
// while worn.
// fairyEquipSlot (Equip[13], the slot ProcessSecMinTimer watches) lives in
// exp_bonus.go, which already needed it for the fairy's EXP bonus.
const (
	// Fairy item range (BASE_CheckFairyDate bails outside it).
	fairyFirstIndex = 3900
	fairyLastIndex  = 3913
	// fairyTickPeriod is how many 1s loop ticks make the legacy's minute pulse.
	fairyTickPeriod = 60
)

func isFairy(index int16) bool {
	return index >= fairyFirstIndex && index <= fairyLastIndex
}

// durationEffects renders a lifetime as the three effect slots an un-started
// item carries. This is also exactly what the client shows, so an item waiting
// in the bag reads "30 Dia(s)" without any conversion.
func durationEffects(d time.Duration) [3]world.Effect {
	if d < 0 {
		d = 0
	}
	days := int64(d.Hours()) / 24
	if days > 255 {
		days = 255
	}
	return [3]world.Effect{
		{Effect: efWDay, Value: uint8(days)},
		{Effect: efHour, Value: uint8(int64(d.Hours()) % 24)},
		{Effect: efMin, Value: uint8(int64(d.Minutes()) % 60)},
	}
}

// effectsDuration reads back what durationEffects wrote. It reports 0 when the
// slots hold something else — a refine, a damage bonus — which is the common case.
func effectsDuration(eff [3]world.Effect) time.Duration {
	var out time.Duration
	for _, e := range eff {
		switch e.Effect {
		case efWDay:
			out += time.Duration(e.Value) * 24 * time.Hour
		case efHour:
			out += time.Duration(e.Value) * time.Hour
		case efMin:
			out += time.Duration(e.Value) * time.Minute
		}
	}
	return out
}

// itemLifetime is how long an un-started item will run once activated. The item's
// own effects win, so a GM or the item editor can hand out a shorter one; the
// catalog is the fallback and covers everything obtained normally, because the
// lifetime is written into the item's NAME — "Conjunto_Yin-Yang(30dias)". That is
// what makes 7- and 14-day variants work with no code change: a new row in the
// catalog is enough.
func (d *Dispatcher) itemLifetime(it world.Item) time.Duration {
	if life := effectsDuration(it.Effects); life > 0 {
		return life
	}
	if days := d.itemDurations[int(it.Index)]; days > 0 {
		return time.Duration(days) * 24 * time.Hour
	}
	if days := shopMountDefaultDays[it.Index]; days > 0 {
		return time.Duration(days) * 24 * time.Hour
	}
	return 0
}

// shopMountDefaultDays is the lifetime of the three cash-shop mounts when the
// item carries none. Their names lost the "(3dias)" on 2026-09-11 — the shop
// sells each one for 24h, 3 or 5 days, written on the item it delivers — so the
// catalog no longer knows a lifetime for them; without this, one handed out with
// no duration (a GM, an old delivery) would never run out. Three days is what
// they had before.
var shopMountDefaultDays = map[int16]int{
	3980: 3, // Shire
	3981: 3, // Thoroughbred
	3982: 3, // Klazedale
}

// startTimedItem begins a temporary item's life the first time it is equipped,
// and reports whether it changed anything. Costumes, spheres and mounts convert
// to a deadline; a fairy only has its duration seeded, because it burns solely
// while worn (tickFairies).
//
// It is a no-op for a permanent item, and for one already running — equipping a
// costume for the second time must not hand back another thirty days.
func (d *Dispatcher) startTimedItem(it *world.Item, now time.Time) bool {
	if it == nil || it.Index == 0 || it.ExpiresAt != 0 {
		return false
	}
	life := d.itemLifetime(*it)
	if life <= 0 {
		return false
	}
	if isFairy(it.Index) {
		if effectsDuration(it.Effects) > 0 {
			return false // already counting
		}
		it.Effects = durationEffects(life)
		return true
	}
	it.ExpiresAt = now.Add(life).Unix()
	it.Effects = [3]world.Effect{}
	return true
}

// tickFairies burns a minute off every equipped fairy, and only off those:
// ProcessSecMinTimer.cpp:612 passes Equip[13] and nothing else, which is what
// the item's own "Não gasta caso não esteja equipado" promises. One that runs out
// is cleared from the slot, as BASE_CheckFairyDate does at day 0 hour 0 min <= 1.
func (d *Dispatcher) tickFairies(w *world.World) {
	if d.tickCount%fairyTickPeriod != 0 {
		return
	}
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		it := &e.Equip[fairyEquipSlot]
		if !isFairy(it.Index) {
			return
		}
		left := effectsDuration(it.Effects) - time.Minute
		if left <= 0 {
			*it = world.Item{}
			d.sendSlot(w, s, world.ItemPlaceEquip, fairyEquipSlot, *it)
			d.refreshEquip(w, s, e)
			d.log.Info("fairy expired", "account", s.AccountName, "conn", s.Conn)
			return
		}
		it.Effects = durationEffects(left)
		d.sendSlot(w, s, world.ItemPlaceEquip, fairyEquipSlot, *it)
	})
}
