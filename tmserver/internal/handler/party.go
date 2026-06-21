package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// partyDif is the max level difference allowed in a party (PARTY_DIF).
//
// UNVERIFIED: the value and the ClassMaster/MAX_CLEVEL tier adjustments
// (lote2-party-guilda-guerra.md) are not documented; placeholder + simplified.
const partyDif = 100

// partyLevelOK applies the (simplified) party level rule shared by invite and
// accept: same tier, very high level, or within partyDif (UNVERIFIED tiers).
func partyLevelOK(a, b *world.Entity) bool {
	if a.Level >= 1000 || b.Level >= 1000 || a.ClassMaster == b.ClassMaster {
		return true
	}
	diff := int(a.Level) - int(b.Level)
	if diff < 0 {
		diff = -diff
	}
	return diff < partyDif
}

// sendReqParty handles _MSG_SendReqParty (0x037F): invite a player to a party.
func (d *Dispatcher) sendReqParty(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgSendReqPartyBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if int(body.PartyID) != s.Conn { // PartyID must be the inviter
		return
	}
	if e.Leader != 0 && e.Leader != s.Conn { // already a member elsewhere
		return
	}
	target := int(body.Target)
	other := w.Session(target)
	te := w.Entity(target)
	if target <= 0 || target >= world.MaxUser || other == nil || other.Mode != world.UserPlay || te == nil {
		return
	}
	if te.Leader != 0 || !partyLevelOK(e, te) { // target already partied / level gate
		return
	}
	te.LastReqParty = s.Conn // anti-forge gate for AcceptParty
	w.SendTo(other, protocol.Header{Type: protocol.MsgSendReqParty, ID: uint16(s.Conn)}, payload)
}

// acceptParty handles _MSG_AcceptParty (0x03AB): join the inviter's party. The
// LastReqParty gate blocks forged accepts (PARTYHACK).
func (d *Dispatcher) acceptParty(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgAcceptPartyBody
	if err := body.Decode(payload); err != nil {
		return
	}
	leaderConn := int(body.LeaderID)
	if leaderConn <= 0 || leaderConn >= world.MaxUser {
		return
	}
	if e.LastReqParty != leaderConn { // forged accept without a matching invite
		d.log.Warn("PARTYHACK: accept without invite", "conn", s.Conn, "leader", leaderConn)
		w.AddCrackError(s, 1, 0)
		return
	}
	leaderSess, le := w.Session(leaderConn), w.Entity(leaderConn)
	if leaderSess == nil || leaderSess.Mode != world.UserPlay || le == nil {
		return
	}
	if e.Leader != 0 || (le.Leader != 0 && le.Leader != leaderConn) || !partyLevelOK(le, e) {
		return
	}

	if le.Leader == 0 { // form the party, leader takes the first slot
		le.Leader = leaderConn
		addMember(le, leaderConn)
	}
	if !addMember(le, s.Conn) {
		return // party full
	}
	e.Leader = leaderConn
	e.LastReqParty = 0

	// Sync the (new) party list to every member.
	for _, m := range le.PartyList {
		if m == 0 {
			continue
		}
		if ms := w.Session(m); ms != nil {
			w.SendTo(ms, protocol.Header{Type: protocol.MsgAcceptParty, ID: uint16(s.Conn)}, payload)
		}
	}
}

// removeParty handles _MSG_RemoveParty (0x037E): leave (member) or dissolve
// (leader). Parm is the target; out of range or self means "me".
func (d *Dispatcher) removeParty(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.Leader == 0 {
		return
	}
	if e.Leader == s.Conn {
		d.dissolveParty(w, s.Conn) // leader leaves → dissolve
	} else {
		d.leaveParty(w, s.Conn)
	}
}

func (d *Dispatcher) dissolveParty(w *world.World, leaderConn int) {
	le := w.Entity(leaderConn)
	if le == nil {
		return
	}
	for _, m := range le.PartyList {
		if m == 0 {
			continue
		}
		if me := w.Entity(m); me != nil {
			me.Leader = 0
			me.PartyList = [world.MaxParty]int{}
		}
		if ms := w.Session(m); ms != nil {
			w.SendTo(ms, protocol.Header{Type: protocol.MsgRemoveParty, ID: uint16(leaderConn)}, nil)
		}
	}
}

func (d *Dispatcher) leaveParty(w *world.World, conn int) {
	e := w.Entity(conn)
	if e == nil {
		return
	}
	leaderConn := e.Leader
	e.Leader = 0
	e.PartyList = [world.MaxParty]int{}
	if le := w.Entity(leaderConn); le != nil {
		removeMember(le, conn)
		for _, m := range le.PartyList {
			if m == 0 {
				continue
			}
			if ms := w.Session(m); ms != nil {
				w.SendTo(ms, protocol.Header{Type: protocol.MsgRemoveParty, ID: uint16(conn)}, nil)
			}
		}
	}
	if ms := w.Session(conn); ms != nil {
		w.SendTo(ms, protocol.Header{Type: protocol.MsgRemoveParty, ID: uint16(conn)}, nil)
	}
}

// addMember puts conn in the leader's first free party slot (idempotent),
// returning false if the party is full.
func addMember(leader *world.Entity, conn int) bool {
	for i := range leader.PartyList {
		if leader.PartyList[i] == conn {
			return true
		}
		if leader.PartyList[i] == 0 {
			leader.PartyList[i] = conn
			return true
		}
	}
	return false
}

func removeMember(leader *world.Entity, conn int) {
	for i := range leader.PartyList {
		if leader.PartyList[i] == conn {
			leader.PartyList[i] = 0
		}
	}
}
