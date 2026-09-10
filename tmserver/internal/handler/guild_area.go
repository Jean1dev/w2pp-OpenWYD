package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// attrGuildArea is the AttributeMap bit that marks a guild's ground inside a
// city (_MSG_Action.cpp:224). In the shipped map it covers exactly five
// blocks, one around each city's guild spawn point (g_pGuildZone.GuildSpawnX/Y):
// Armia (2072,2136)-(2103,2159), Azran (2528,1680)-(2555,1711), Erion
// (2448,1968)-(2475,1983), Nippleheim (3604,3100)-(3631,3143) and Noatum
// (1036,1752)-(1075,1775).
const attrGuildArea = 0x20

const (
	// msgZonaDeOutraGuilda is _NN_Only_Guild_Members (Language.txt:47), what the
	// legacy says to someone walking into a zone their guild does not own.
	msgZonaDeOutraGuilda = "Você não pode entrar na zona de outra guilda."
	// msgAreaDeGuild is for a zone nobody owns yet, where "another guild's zone"
	// would be a lie.
	msgAreaDeGuild = "Esta é uma área de guild. Só a administração pode entrar."
)

// guildAreaOwner is the guild that owns the guild area under (x, y), and
// whether (x, y) is a guild area at all.
//
// The zone comes from the city limits, as BASE_GetGuild does (Basedef.cpp:4820):
// a tile flagged 0x20 outside every city — the map carries a few thin strips of
// it near (236-427, 228-411) — returns zone 5 there, which the legacy check
// rejects, so it is not a guild area for this rule either.
func (d *Dispatcher) guildAreaOwner(x, y int16) (owner uint16, ok bool) {
	if d.attributes == nil || d.attributes.At(int(x)/4, int(y)/4)&attrGuildArea == 0 {
		return 0, false
	}
	zone := world.Village(x, y)
	if zone < 0 || zone >= len(d.guildZones) {
		return 0, false
	}
	return d.guildZones[zone].ChargeGuild, true
}

// guardGuildAreas sends out of a city's guild area anyone who does not belong
// there: the legacy's move check (_MSG_Action.cpp:222-235), run every tick so
// it also catches whoever logs in, respawns or is teleported onto the ground.
//
// Two differences from the legacy, both asked for by the staff:
//
//   - A zone nobody owns is closed. The legacy compared the player's guild to
//     the owner's, so with no owner (0) a guildless player (0) matched and
//     walked right in — which is how a character with no guild was standing in
//     Armia's guild garden.
//   - The pass is the staff role on the account (the same authority as /gm),
//     not the legacy's "level above MAX_LEVEL".
//
// A dead player is stood up at 2 HP before the recall, as ClearAreaGuild does
// (Server.cpp:6378): the destination flow expects a living entity.
func (d *Dispatcher) guardGuildAreas(w *world.World) {
	if d.attributes == nil {
		return
	}
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		msg, sai := d.guildAreaVerdict(s.AccessLevel, e)
		if !sai {
			return
		}
		if e.HP <= 0 {
			e.HP = 2
			d.sendScore(w, s, e)
		}
		d.log.Info("guild area: sent out", "conn", s.Conn, "account", s.AccountName,
			"x", e.X, "y", e.Y, "guild", e.Guild)
		sendClientMessage(w, s, msg)
		d.recall(w, s, e)
	})
}

// guildAreaVerdict decides whether e has to leave the guild area it stands on,
// and what it is told: staff stays, a member of the owning guild stays, and
// everyone else — everyone, while the zone has no owner — goes.
func (d *Dispatcher) guildAreaVerdict(access world.AccessLevel, e *world.Entity) (msg string, sai bool) {
	if access >= world.AccessModerator {
		return "", false
	}
	owner, ok := d.guildAreaOwner(e.X, e.Y)
	if !ok || (owner != 0 && e.Guild == owner) {
		return "", false
	}
	if owner == 0 {
		return msgAreaDeGuild, true
	}
	return msgZonaDeOutraGuilda, true
}
