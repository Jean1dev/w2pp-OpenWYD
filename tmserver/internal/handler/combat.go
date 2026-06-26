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
	// Server-authoritative attack power = base CurrentScore.Damage + the equipped
	// weapon's damage (so equipping a weapon actually raises the hit).
	atkDamage := int(e.Damage) + int(d.weaponDamage(e))
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
			AttackerDamage: atkDamage,
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
		}
		// A struck mob either dies (rewards) or retaliates: it focuses the attacker
		// so the AI tick (mobai.go) chases and fights back. Provocation happens even
		// on a blocked hit, matching the original AddEnemyList-on-attack.
		if !world.IsPlayer(tid) {
			if target.HP == 0 {
				d.mobKilled(w, e, target)
			} else {
				target.Target = s.Conn
				target.Mode = world.MobCombat
			}
		}
		writeDamage(payload, i, int32(dmg))
	}

	// Overwrite the attacker's status with the server's authoritative values so every
	// recipient (and the attacker's own client) sees the real HP/MP and the post-kill
	// experience. CurrentExp is how the client refreshes its exp bar — there is no
	// separate exp packet (MSG_UpdateScore carries no exp).
	writeAttackerStatus(payload, e.HP, e.MP, e.Exp)

	// Broadcast the server-authoritative result to in-view players (HEADER.ID =
	// attacker). UNVERIFIED: the original forces ID=ESCENE_FIELD for field scope.
	w.BroadcastInView(s.Conn, protocol.MsgAttack, payload)
	// Echo to the attacker too: BroadcastInView excludes the source, but the attacker
	// needs its own CurrentExp/Hp/Mp (e.g. exp gained from a kill). UNVERIFIED whether
	// the original echoes MSG_Attack or uses MSG_SetHpMp; the CurrentExp field lives
	// in MSG_Attack, so the echo is the carrier we have.
	w.SendTo(s, protocol.Header{Type: protocol.MsgAttack, ID: uint16(s.Conn)}, payload)
}

// writeDamage overwrites the server-authoritative damage of Dam[i] in the wire
// payload (the client value is ignored).
func writeDamage(payload []byte, i int, dmg int32) {
	off := protocol.MsgAttackDamOffset + i*protocol.MsgAttackDamStride + 4
	if off+4 <= len(payload) {
		binary.LittleEndian.PutUint32(payload[off:off+4], uint32(dmg))
	}
}

// writeAttackerStatus overwrites the attacker's CurrentHp@4, CurrentExp@12 and
// CurrentMp@40 in the MSG_Attack body with the server's authoritative values
// (MsgAttackBody layout, messages.go). These fixed fields sit below the Dam[]
// region (offset 48), so they never collide with per-target damage.
func writeAttackerStatus(payload []byte, hp, mp int32, exp int64) {
	if len(payload) < protocol.MsgAttackDamOffset {
		return
	}
	binary.LittleEndian.PutUint32(payload[4:8], uint32(hp))
	binary.LittleEndian.PutUint64(payload[12:20], uint64(exp))
	binary.LittleEndian.PutUint32(payload[40:44], uint32(mp))
}
