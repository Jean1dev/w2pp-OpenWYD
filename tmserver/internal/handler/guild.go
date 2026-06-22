package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Guild invite costs (lote2-party-guilda-guerra.md). UNVERIFIED: hardcoded in the
// original; should become config.
const (
	guildInviteCost  = 4_000_000
	guildSpecialCost = 100_000_000
	guildLeaderLevel = 9
)

// inviteGuild handles _MSG_InviteGuild (0x03D5, MSG_STANDARDPARM2:
// Parm1=TargetID, Parm2=InviteType): add a same-clan, guildless player to the
// inviter's guild for a gold cost.
//
// UNVERIFIED: the Sunday block (tm_wday==0) is not applied (no calendar here).
func (d *Dispatcher) inviteGuild(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay {
		return
	}
	p1, p2, ok := protocol.StandardParm2(payload)
	if !ok {
		return
	}
	target, inviteType := int(p1), int(p2)
	if target <= 0 || target >= world.MaxUser || inviteType < 0 || inviteType >= 4 {
		return
	}
	if e.Guild == 0 || e.GuildLevel == 0 { // inviter must have a guild and be an official
		return
	}
	if inviteType != 0 && e.GuildLevel != guildLeaderLevel { // special invites need the leader
		return
	}
	other, te := w.Session(target), w.Entity(target)
	if other == nil || other.Mode != world.UserPlay || te == nil {
		return
	}
	if te.Guild != 0 || te.Clan != e.Clan { // target must be guildless and same clan
		return
	}
	cost := int32(guildInviteCost)
	if inviteType != 0 {
		cost = guildSpecialCost
	}
	if e.Coin < cost {
		return // not enough gold
	}

	e.Coin -= cost
	te.Guild = e.Guild
	te.GuildLevel = 0 // member

	w.BroadcastInView(target, protocol.MsgCreateMob, nil) // refresh the target's tag in view
	w.Send(other, protocol.MsgMessagePanel, nil)          // welcome (UNVERIFIED payload)
}

// guildAlly handles _MSG_GuildAlly (0x0E12): relay an alliance to the dbServer.
func (d *Dispatcher) guildAlly(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	d.guildRelay(w, s, payload, "ally")
}

// war handles _MSG_War (0x0E0E): relay a war declaration to the dbServer.
func (d *Dispatcher) war(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	d.guildRelay(w, s, payload, "war")
}

// guildRelay validates the leader and forwards the alliance/war to the dbServer.
//
// UNVERIFIED: the dbServer alliance/war RPC is not implemented yet — this
// validates and logs; persistence/propagation lands with the dbServer (Phase 6).
func (d *Dispatcher) guildRelay(w *world.World, s *world.Session, payload []byte, kind string) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	guild, _, ok := protocol.StandardParm2(payload)
	if !ok {
		return
	}
	if e.Guild != uint16(guild) || e.GuildLevel != guildLeaderLevel { // leader of own guild only
		return
	}
	d.log.Info("guild relay (DB propagation pending)", "kind", kind, "conn", s.Conn, "guild", guild)
}

// challange handles _MSG_Challange (0x028E): zone challenge / tax collection.
//
// UNVERIFIED: the WeekMode-driven zone-tax economy (GuildImpostoID, Exp-as-vault,
// item 4011) is not reproduced (lote2-party-guilda-guerra.md) — stub.
func (d *Dispatcher) challange(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("Challange not yet implemented (UNVERIFIED zone-tax economy)", "conn", s.Conn)
}

// challangeConfirm handles _MSG_ChallangeConfirm (0x028F).
//
// UNVERIFIED: Challange(conn,target,0) lives in Server.cpp — stub.
func (d *Dispatcher) challangeConfirm(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("ChallangeConfirm not yet implemented (UNVERIFIED)", "conn", s.Conn)
}
