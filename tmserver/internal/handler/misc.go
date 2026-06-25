package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// mountExpiryDays is the lifetime granted to a Perzen reward mount ("X(30dias)",
// BASE_SetItemDate(30)).
const mountExpiryDays = 30

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
	d.sendScore(w, s, e) // refresh the status window with the new attribute
}

// accountSecure handles _MSG_AccountSecure (0x0FDE): the numeric PIN, relayed to
// the dbServer (lote2-sessao-conta.md). Deferred relay — the PIN check/change is
// a dbServer RPC.
//
// UNVERIFIED: the dbServer PIN RPC is not implemented; this acknowledges the
// request. The PIN must be hashed/HMACed on the dbServer (never plaintext).
func (d *Dispatcher) accountSecure(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	// Acknowledge the numeric-PIN step so the client advances (the original relays
	// to DBSrv and echoes a header-only _MSG_AccountSecure signal, ID=ESCENE_FIELD).
	// Without this ack the client stalls/resets on the secure-password screen.
	w.SendTo(s, protocol.Header{Type: protocol.MsgAccountSecure, ID: protocol.IDScene}, nil)
}

// quest handles _MSG_Quest (0x028B): NPC quest interaction. Body is
// MSG_STANDARDPARM2 (Parm1 = npcIndex, Parm2 = confirm). The NPC sub-type comes
// from MOB.Merchant (+ EF_GRADE0 for Merchant==100), see _MSG_Quest-npcs.md.
//
// This batch implements the PERZEN item-exchange NPCs (Merchant 100, grade 7/8/9):
// hand over the item the NPC wants for a mount. The remaining 37 quest NPC types
// (level chains, tutorials, teleports) are UNVERIFIED and not yet routed.
func (d *Dispatcher) quest(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if e := w.Entity(s.Conn); e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // interacting with a quest NPC cancels a trade
	}
	npcIndex, _, ok := protocol.StandardParm2(payload)
	if !ok || npcIndex < world.MaxUser || int(npcIndex) >= world.MaxMob {
		return
	}
	npc := w.Entity(int(npcIndex))
	if npc == nil || npc.Mode == world.MobEmpty {
		return
	}
	// PERZEN (Merchant 100, EF_GRADE0 ∈ {7,8,9}): the item exchange.
	if npc.Merchant == 100 && npc.Grade >= 7 && npc.Grade <= 9 {
		d.perzenExchange(w, s, npc)
		return
	}
	d.log.Debug("quest NPC not implemented", "conn", s.Conn, "npc", npcIndex, "merchant", npc.Merchant, "grade", npc.Grade)
}

// perzenExchange implements the Perzen NPCs (_MSG_Quest.cpp PERZEN): if the player
// carries the item the NPC requires (npc.Carry[0]), consume it and hand back the
// reward (npc.Carry[1], a mount) in the same slot. The trade is fully data-driven
// by the NPC's own inventory, so the same code serves every Perzen variant.
func (d *Dispatcher) perzenExchange(w *world.World, s *world.Session, npc *world.Entity) {
	e := w.Entity(s.Conn)
	input := npc.Carry[0].Index
	reward := npc.Carry[1]
	if e == nil || input == 0 || reward.Index == 0 {
		return
	}
	slot := -1
	for i := range e.Carry {
		if e.Carry[i].Index == input {
			slot = i
			break
		}
	}
	if slot < 0 {
		// UNVERIFIED: the original SendSay shows "bring item <name>" dialogue; the
		// _MSG_NPCQuiz/Say wire format is not captured, so we just no-op for now.
		d.log.Info("perzen: player lacks input item", "conn", s.Conn, "want", input)
		return
	}
	// Consume the input and grant the reward mount with a 30-day expiry
	// (BASE_SetItemDate(30); the reward items are the "X(30dias)" mounts). The expiry
	// is enforced server-side on load (dropExpired), independent of the legacy
	// in-item date encoding, which is UNVERIFIED.
	e.Carry[slot] = world.Item{
		Index:     reward.Index,
		ExpiresAt: time.Now().Add(mountExpiryDays * 24 * time.Hour).Unix(),
	}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
	d.log.Info("perzen exchange", "conn", s.Conn, "npc", npc.ID, "input", input, "reward", reward.Index)
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
