package handler

import (
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

// weatherAuto is the ForceWeather sentinel for "roll automatically"
// (ForceWeather = -1, Server.cpp:637).
const weatherAuto = int32(-1)

// weatherTickPeriod is the roll cadence in world ticks. The legacy rolls once
// per ProcessMinTimer pass (ProcessSecMinTimer.cpp:2791) and our tick is 1s.
const weatherTickPeriod = 60

// currentWeather is the single accessor for CurrentWeather. It feeds three
// consumers: the MSG_CNFCharacterLogin snapshot (ProcessDBMessage.cpp:834), the
// skill-damage formula (_MSG_Attack.cpp:520,594,972) and the broadcast below.
func (d *Dispatcher) currentWeather() int32 { return d.events.weather }

// tickWeather ports the weather block at the tail of ProcessMinTimer
// (ProcessSecMinTimer.cpp:2791-2818).
//
// The draw happens every minute even while a GM override pins the value: the
// legacy rolls before it looks at ForceWeather, so skipping it would advance
// the stream at a different rate. With an override active the roll is discarded
// and the forced value is (re)applied instead.
func (d *Dispatcher) tickWeather(w *world.World) {
	if d.tickCount%weatherTickPeriod != 0 {
		return
	}
	next, changed := worldevents.RollWeather(d.eventRNG, d.events.weather)
	if d.events.forceWeather != weatherAuto {
		// Override wins over the roll; it only reaches the wire once, when the
		// live value still disagrees with it (the legacy's ForceWeather !=
		// CurrentWeather guard).
		if d.events.forceWeather == d.events.weather {
			return
		}
		d.setWeather(w, d.events.forceWeather)
		return
	}
	if !changed {
		return
	}
	d.setWeather(w, next)
}

// setWeather commits a new CurrentWeather and broadcasts it. Callers must
// already have decided the value actually changes.
func (d *Dispatcher) setWeather(w *world.World, weather int32) {
	d.events.weather = weather
	d.log.Info("weather changed", "weather", weather)
	d.broadcastWeather(w)
}

// broadcastWeather ports SendWeather (SendFunc.cpp:1669-1696): one
// MSG_UpdateWeather to EVERY in-play session, with HEADER.ID = ESCENE_FIELD.
// This is a global broadcast, not a grid multicast — the legacy's geographic
// filter is commented out at SendFunc.cpp:1690.
func (d *Dispatcher) broadcastWeather(w *world.World) {
	body := protocol.EncodeUpdateWeather(d.events.weather)
	w.ForEachPlaying(-1, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgUpdateWeather, ID: protocol.IDScene}, body)
	})
}

// gmWeather is `/gm weather <0|1|2|auto>`, the port of the legacy admin
// `/weather` (imple.cpp:1122-1129), with three deliberate improvements:
//
//   - authority is the account's AccessLevel (checked by runGMCommand), not the
//     legacy "character Level >= 1";
//   - the value is validated — the legacy shipped any int straight to clients;
//   - "auto" restores ForceWeather = -1. The legacy command was one-way: once
//     used, the automatic rolls never resumed for the lifetime of the process.
func (d *Dispatcher) gmWeather(w *world.World, s *world.Session, rest string) {
	arg := strings.ToLower(firstToken(rest))
	switch arg {
	case "":
		d.sendChatText(w, s, "uso: /gm weather <0|1|2|auto>")
		return
	case "auto":
		d.events.forceWeather = weatherAuto
		d.log.Info("gm weather: automatic rolls resumed", "account", s.AccountName)
		d.sendChatText(w, s, "clima: automatico")
		return
	}
	n, err := strconv.Atoi(arg)
	if err != nil || !worldevents.ValidWeather(int32(n)) {
		d.sendChatText(w, s, "clima invalido (use 0, 1, 2 ou auto)")
		return
	}
	weather := int32(n)
	d.events.forceWeather = weather
	d.log.Info("gm weather forced", "account", s.AccountName, "weather", weather)
	if weather != d.events.weather {
		d.setWeather(w, weather)
	}
	d.sendChatText(w, s, "clima fixado em "+arg)
}
