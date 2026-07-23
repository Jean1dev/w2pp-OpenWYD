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
	if e.Leader != 0 { // already a member elsewhere
		return
	}
	target := int(body.Unk)
	if target == 0 && body.Target != 0 {
		target = int(body.Target)
	}
	other := w.Session(target)
	te := w.Entity(target)
	if target <= 0 || target >= world.MaxUser || other == nil || other.Mode != world.UserPlay || te == nil {
		return
	}
	if isInParty(te) || !partyLevelOK(e, te) { // target already partied / level gate
		return
	}
	te.LastReqParty = s.Conn // anti-forge gate for AcceptParty
	out := d.reqPartyBody(e, s.Conn)
	w.SendTo(other, protocol.Header{Type: protocol.MsgSendReqParty, ID: protocol.IDScene}, out.Encode())
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
	if cstr(body.MobName[:]) != le.Name {
		return
	}
	if isInParty(e) || le.Leader != 0 || !partyLevelOK(le, e) {
		return
	}
	slot, ok := addMember(le, s.Conn)
	if !ok {
		return // party full
	}
	e.Leader = leaderConn
	e.LastReqParty = 0
	d.syncAcceptedParty(w, leaderConn, s.Conn, slot+1)
}

// removeParty handles _MSG_RemoveParty (0x037E): members leave, leaders can
// kick a member, and a leader leaving promotes the next member when possible.
// Parm is the target; out of range or self means "me".
func (d *Dispatcher) removeParty(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	target := s.Conn
	if parm, ok := protocol.StandardParm(payload); ok && parm > 0 && int(parm) < world.MaxUser {
		target = int(parm)
	}
	if target != s.Conn && !hasMember(e, target) {
		target = s.Conn
	}
	if e.Leader != 0 {
		d.leaveParty(w, s.Conn)
		return
	}
	if target == s.Conn {
		d.leaderLeaveParty(w, s.Conn)
		return
	}
	d.kickPartyMember(w, s.Conn, target)
}

func (d *Dispatcher) reqPartyBody(e *world.Entity, partyID int) protocol.MsgSendReqPartyBody {
	body := protocol.MsgSendReqPartyBody{
		Class:    e.Class,
		PartyPos: 0,
		Level:    uint16(clampPartyWire(e.Level)),
		MaxHP:    uint16(partyHPWire(effectiveMaxHP(e))),
		HP:       uint16(partyHPWire(e.HP)),
		PartyID:  int16(partyID),
	}
	copy(body.MobName[:], e.Name)
	return body
}

func (d *Dispatcher) syncAcceptedParty(w *world.World, leaderConn, newMember, newMemberPartyPos int) {
	le := w.Entity(leaderConn)
	if le == nil {
		return
	}
	if partyMemberCount(le) == 1 {
		d.sendAddParty(w, leaderConn, leaderConn, 0)
		d.sendAddParty(w, newMember, newMember, newMemberPartyPos)
	}
	d.sendAddParty(w, newMember, leaderConn, 0)
	d.sendAddParty(w, leaderConn, newMember, newMemberPartyPos)
	for j, memberID := range le.PartyList {
		if memberID == 0 {
			continue
		}
		if memberID != newMember {
			d.sendAddParty(w, newMember, memberID, newMemberPartyPos)
		}
		d.sendAddParty(w, memberID, newMember, j+1)
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
	d.sendRemoveParty(w, conn, 0)
	if le := w.Entity(leaderConn); le != nil {
		removeMember(le, conn)
		for _, m := range le.PartyList {
			if m == 0 {
				continue
			}
			d.sendRemoveParty(w, m, conn)
		}
		d.sendRemoveParty(w, leaderConn, conn)
	}
}

func (d *Dispatcher) kickPartyMember(w *world.World, leaderConn, conn int) {
	e := w.Entity(conn)
	le := w.Entity(leaderConn)
	if e == nil || le == nil || e.Leader != leaderConn {
		return
	}
	e.Leader = 0
	e.PartyList = [world.MaxParty]int{}
	removeMember(le, conn)
	d.sendRemoveParty(w, conn, 0)
	d.sendRemoveParty(w, leaderConn, conn)
	for _, m := range le.PartyList {
		if m != 0 {
			d.sendRemoveParty(w, m, conn)
		}
	}
}

func (d *Dispatcher) leaderLeaveParty(w *world.World, leaderConn int) {
	le := w.Entity(leaderConn)
	if le == nil {
		return
	}
	members := partyMembers(le)
	le.PartyList = [world.MaxParty]int{}
	for _, memberID := range members {
		if me := w.Entity(memberID); me != nil && me.Leader == leaderConn {
			me.Leader = 0
			me.PartyList = [world.MaxParty]int{}
		}
		d.sendRemoveParty(w, memberID, 0)
	}
	d.sendRemoveParty(w, leaderConn, 0)
	if len(members) == 0 {
		return
	}
	newLeader := members[0]
	nle := w.Entity(newLeader)
	if nle == nil {
		return
	}
	nle.Leader = 0
	nle.PartyList = [world.MaxParty]int{}
	for _, memberID := range members[1:] {
		me := w.Entity(memberID)
		if me == nil {
			continue
		}
		slot, ok := addMember(nle, memberID)
		if !ok {
			return
		}
		me.Leader = newLeader
		d.syncAcceptedParty(w, newLeader, memberID, slot+1)
	}
}

func (d *Dispatcher) sendRemoveParty(w *world.World, conn, connExit int) {
	if conn <= 0 || conn >= world.MaxUser {
		return
	}
	if ms := w.Session(conn); ms != nil && ms.Mode == world.UserPlay {
		body := protocol.MsgRemovePartyBody{LeaderConn: int16(connExit)}
		w.SendTo(ms, protocol.Header{Type: protocol.MsgRemoveParty, ID: protocol.IDScene}, body.Encode())
	}
}

// addMember puts conn in the leader's first free party slot (idempotent).
func addMember(leader *world.Entity, conn int) (int, bool) {
	for i := range leader.PartyList {
		if leader.PartyList[i] == conn {
			return i, true
		}
		if leader.PartyList[i] == 0 {
			leader.PartyList[i] = conn
			return i, true
		}
	}
	return -1, false
}

func removeMember(leader *world.Entity, conn int) {
	for i := range leader.PartyList {
		if leader.PartyList[i] == conn {
			leader.PartyList[i] = 0
		}
	}
}

func hasMember(leader *world.Entity, conn int) bool {
	for _, memberID := range leader.PartyList {
		if memberID == conn {
			return true
		}
	}
	return false
}

func partyMemberCount(leader *world.Entity) int {
	n := 0
	for _, memberID := range leader.PartyList {
		if memberID != 0 {
			n++
		}
	}
	return n
}

func partyMembers(leader *world.Entity) []int {
	members := make([]int, 0, world.MaxParty)
	for _, memberID := range leader.PartyList {
		if memberID > 0 && memberID < world.MaxUser {
			members = append(members, memberID)
		}
	}
	return members
}

func isInParty(e *world.Entity) bool {
	return e.Leader != 0 || partyMemberCount(e) != 0
}

func (d *Dispatcher) sendAddParty(w *world.World, recipientID, entryID, partyPos int) {
	if recipientID <= 0 || recipientID >= world.MaxUser {
		return
	}
	rs := w.Session(recipientID)
	entry := w.Entity(entryID)
	if rs == nil || rs.Mode != world.UserPlay || entry == nil {
		return
	}
	leaderConn := uint16(30000)
	if partyPos == 0 {
		leaderConn = uint16(entryID)
	}
	body := protocol.MsgCNFAddPartyBody{
		LeaderConn: leaderConn,
		Level:      uint16(clampPartyWire(entry.Level)),
		MaxHP:      uint16(partyHPWire(effectiveMaxHP(entry))),
		HP:         uint16(partyHPWire(entry.HP)),
		PartyID:    uint16(entryID),
		Target:     52428,
	}
	copy(body.MobName[:], entry.Name)
	w.SendTo(rs, protocol.Header{Type: protocol.MsgCNFAddParty, ID: protocol.IDScene}, body.Encode())
}

func partyHPWire(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 32000 {
		v = (v + 1) / 100
	}
	return clampPartyWire(v)
}

func clampPartyWire(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 0xFFFF {
		return 0xFFFF
	}
	return v
}
