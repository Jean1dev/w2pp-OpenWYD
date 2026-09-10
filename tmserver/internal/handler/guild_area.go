package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// guildWarAreas are the guild war fields of the four cities that have one —
// WarAreaX1..Y2 of g_pGuildZone (Basedef.cpp:56-60), in the zone order of
// world.cities: Armia, Azran, Erion, Nippleheim.
//
// Noatum is left out: its WarArea row is a placeholder in the far corner of
// the map (4000,4000)-(4010,4010), and its guild ground is the castle, whose
// rectangle in the legacy (ClearAreaGuild, Server.cpp:4783) covers the city of
// Noatum itself — a guard on it would empty the city. The castle has its own
// war logic (castle.go).
var guildWarAreas = [...]areaBox{
	{197, 213, 238, 230}, // Armia
	{197, 149, 238, 166}, // Azran
	{141, 213, 182, 230}, // Erion
	{141, 149, 182, 166}, // Nippleheim
}

// msgAreaDeGuild is what a player reads on the way out.
const msgAreaDeGuild = "Esta é uma área de guild. Só a administração pode entrar."

// guardGuildWarAreas sends anyone who is not staff out of the guild war fields.
//
// SERVER RULE, not the legacy's: the legacy only cleared these fields at the
// start and end of a city war (Server.cpp:6686), and otherwise let anyone who
// found a way in stay there. The staff wants them closed outright while guilds
// are not live, with the administration free to go in — the same authority
// that runs /gm (account role, not character level). When the city war lands,
// the two guilds fighting over that zone will need a pass for its duration.
//
// A dead player is stood up at 2 HP before the recall, as ClearAreaGuild does
// (Server.cpp:6378): the destination flow expects a living entity.
func (d *Dispatcher) guardGuildWarAreas(w *world.World) {
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if s.AccessLevel >= world.AccessModerator || !inGuildWarArea(e.X, e.Y) {
			return
		}
		if e.HP <= 0 {
			e.HP = 2
			d.sendScore(w, s, e)
		}
		d.log.Info("guild area: sent out", "conn", s.Conn, "account", s.AccountName, "x", e.X, "y", e.Y)
		sendClientMessage(w, s, msgAreaDeGuild)
		d.recall(w, s, e)
	})
}

// inGuildWarArea reports whether a tile lies in any city's guild war field.
func inGuildWarArea(x, y int16) bool {
	for _, a := range guildWarAreas {
		if a.contains(x, y) {
			return true
		}
	}
	return false
}
