package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// applyBonus handles _MSG_ApplyBonus (0x0277): spend a free attribute point
// (protocol-spec.md §3.5). This batch implements the Score path (Str/Int/Dex/Con);
// Special/Skill bonuses are UNVERIFIED.
func (d *Dispatcher) applyBonus(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgApplyBonusBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if body.BonusType != protocol.BonusScore || e.ScoreBonus == 0 {
		return
	}
	switch int(body.Detail) {
	case protocol.DetailStr:
		e.Str++
	case protocol.DetailInt:
		e.Int++
	case protocol.DetailDex:
		e.Dex++
	case protocol.DetailCon:
		e.Con++
	default:
		return
	}
	e.ScoreBonus--
	w.Send(s, protocol.MsgUpdateScore, nil) // UNVERIFIED score payload
}

// accountSecure handles _MSG_AccountSecure (0x0FDE): the numeric PIN, relayed to
// the dbServer (lote2-sessao-conta.md). Deferred relay — the PIN check/change is
// a dbServer RPC.
//
// UNVERIFIED: the dbServer PIN RPC is not implemented; this acknowledges the
// request. The PIN must be hashed/HMACed on the dbServer (never plaintext).
func (d *Dispatcher) accountSecure(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("AccountSecure relay (DB PIN RPC pending)", "conn", s.Conn)
}

// quest handles _MSG_Quest (0x028B): NPC quest interaction.
//
// UNVERIFIED: the quest engine (38 NPC types, 2753-line handler, quest flags in
// STRUCT_MOBEXTRA.QuestInfo) is the largest progression surface and is not
// reproduced — stub pending data-driven quest tables (lote2-quest-ranking-cash.md).
func (d *Dispatcher) quest(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("Quest not yet implemented (UNVERIFIED, 38 NPCs)", "conn", s.Conn)
}

// reqRanking handles _MSG_ReqRanking (0x039F): duel request/accept.
//
// UNVERIFIED: the request→accept duel state machine and DoRanking (Server.cpp)
// are not reproduced — stub.
func (d *Dispatcher) reqRanking(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("ReqRanking not yet implemented (UNVERIFIED duel)", "conn", s.Conn)
}

// capsuleInfo handles _MSG_CapsuleInfo (0x02CD): a pure relay to the dbServer.
//
// UNVERIFIED: becomes a dbServer cash/capsule RPC (Phase 6).
func (d *Dispatcher) capsuleInfo(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("CapsuleInfo relay (DB cash RPC pending)", "conn", s.Conn)
}

// putoutSeal handles _MSG_PutoutSeal (0x03CC).
//
// UNVERIFIED: seal semantics not documented — stub.
func (d *Dispatcher) putoutSeal(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("PutoutSeal not yet implemented (UNVERIFIED)", "conn", s.Conn)
}
