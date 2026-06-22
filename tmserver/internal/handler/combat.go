package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// attackCadence is the minimum ms between attacks (handlers/_MSG_Attack.md §4):
// ClientTick < LastAttackTick + 800 ⇒ AddCrackError(1,107).
const attackCadence = 800

// attack handles _MSG_Attack / _MSG_AttackOne / _MSG_AttackTwo (0x0367/039D/039E),
// handlers/_MSG_Attack.md. Damage is SERVER-AUTHORITATIVE: the client's Dam[]
// damage is recomputed via the combat formulas (game-rules.md §4) and overwritten
// before broadcast.
func (d *Dispatcher) attack(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	if s.TradeMode != 0 {
		return // cannot attack while auto-trading
	}
	if s.Mode != world.UserPlay {
		return // SendHpMode in the original
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}

	var body protocol.MsgAttackBody
	if err := body.Decode(payload); err != nil {
		return
	}

	// Liveness: the dead may only act with the resurrect skill (99).
	if e.HP == 0 && int(body.SkillIndex) != combat.ResurrectSkill {
		w.AddCrackError(s, 1, 8)
		return
	}

	// Anti-speed cadence + tick sanity (int64 math avoids uint32 underflow on the
	// first attack, when LastAttackTick == 0). SkipCheckTick bypasses the checks.
	tick := h.ClientTick
	if tick != protocol.SkipCheckTick {
		last := int64(s.LastAttackTick)
		if int64(tick) < last+attackCadence {
			w.AddCrackError(s, 1, 107) // too fast
			return
		}
		if int64(tick) < last-100 {
			w.AddCrackError(s, 4, 7) // tick too far in the past
			return
		}
	}
	s.LastAttackTick = tick
	s.LastAttack = int(body.SkillIndex)

	useSkill := body.SkillIndex != 0
	for i := range body.Dam {
		tid := int(body.Dam[i].TargetID)
		target := w.Entity(tid)
		if target == nil || target.Mode == world.MobEmpty {
			writeDamage(payload, i, 0)
			continue
		}
		// UNVERIFIED: DoubleCritical should be server-computed (BASE_GetDoubleCritical)
		// and ParryRate from GetParryRate; both UNVERIFIED (game-rules.md §4.3-4.4).
		// Until captured we use the packet's DoubleCritical and no parry/reflect.
		dmg := combat.ResolveHit(w.Rand(), combat.HitInput{
			AttackerDamage: int(e.Damage),
			TargetAC:       int(target.AC),
			TargetIsPlayer: world.IsPlayer(tid),
			DoubleCritical: body.DoubleCritical,
			Master:         e.Master,
			UseSkill:       useSkill,
			SkillIndex:     int(body.SkillIndex),
		})
		if dmg > 0 {
			target.HP -= int32(dmg)
			if target.HP < 0 {
				target.HP = 0
			}
			if target.HP == 0 && !world.IsPlayer(tid) {
				d.mobKilled(w, e, target)
			}
		}
		writeDamage(payload, i, int32(dmg))
	}

	// Broadcast the server-authoritative result to in-view players (HEADER.ID =
	// attacker). UNVERIFIED: the original forces ID=ESCENE_FIELD for field scope.
	w.BroadcastInView(s.Conn, protocol.MsgAttack, payload)
}

// writeDamage overwrites the server-authoritative damage of Dam[i] in the wire
// payload (the client value is ignored).
func writeDamage(payload []byte, i int, dmg int32) {
	off := protocol.MsgAttackDamOffset + i*protocol.MsgAttackDamStride + 4
	if off+4 <= len(payload) {
		binary.LittleEndian.PutUint32(payload[off:off+4], uint32(dmg))
	}
}
