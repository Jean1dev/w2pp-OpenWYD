package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// attackCadence is the minimum ms between attacks (handlers/_MSG_Attack.md §4):
// ClientTick < LastAttackTick + 800 ⇒ AddCrackError(1,107).
const attackCadence = 800

// Per-target Dam sentinel values (_MSG_Attack.cpp:355): the client marks each
// entry melee or skill; any other non-zero claimed damage is a crack.
const (
	damMelee = -2
	damSkill = -1
)

// attack handles _MSG_Attack / _MSG_AttackOne / _MSG_AttackTwo (0x0367/039D/039E),
// handlers/_MSG_Attack.md. Damage is SERVER-AUTHORITATIVE: the client's Dam[]
// damage is recomputed via the combat formulas (game-rules.md §4) and overwritten
// before broadcast. Skill casts are validated (learned mask, class, Passive) and
// charged mana here, mirroring _MSG_Attack.cpp.
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

	// Liveness: the dead may only act with the resurrect skill (99). Use <= 0 (not
	// == 0) so a negative-HP edge can never slip an action through.
	if e.HP <= 0 && int(body.SkillIndex) != combat.ResurrectSkill {
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

	skillnum := int(body.SkillIndex)
	// REGRESSION GUARD (B12, TestMeleeAlwaysDamagesMob): the skill path (learn
	// mask, class gate, mana) only engages when the packet actually MARKS a
	// skill hit — a Dam entry with the -1 sentinel. The 12000 client's melee
	// encoding of SkillIndex/Dam is UNVERIFIED (agent-prompt-skills.md), and
	// gating the whole attack on those fields once silently killed all
	// player→mob damage. Melee (no -1 entries) must NEVER be dropped or
	// mana-charged, whatever SkillIndex says — damage is server-authoritative.
	skillIntent := false
	for i := range body.Dam {
		if body.Dam[i].Damage == damSkill {
			skillIntent = true
			break
		}
	}
	var cast castInfo
	if skillIntent {
		var ok bool
		cast, ok = d.validateCast(w, s, e, skillnum, tick)
		if !ok {
			return
		}
	}
	if cast.isSkill {
		// Mana (BASE_GetManaSpent → abort broke): insufficient MP cancels the cast;
		// otherwise spend and echo the authoritative MP in the attack body
		// (_MSG_Attack.cpp:240-256). ReqMp mirrors the client's counter minus the
		// spend (the original tracks pUser.ReqMp server-side — UNVERIFIED init, so
		// the echoed value rides the client's own).
		spent := combat.ManaSpent(cast.spell.ManaSpent, int(e.SaveMana), cast.special)
		if int(e.MP)-spent < 0 {
			d.sendScore(w, s, e) // refresh the real MP (no _MSG_SetHpMp layout yet)
			return
		}
		e.MP -= int32(spent)
		body.ReqMp -= int16(spent)

		// BM evocation (InstanceType 11, _MSG_Attack.cpp:809-837): spawn the
		// summons once per cast — the Dam entries carry no damage for it. A cast
		// that spawns nothing refunds its mana (the legacy restores Mp/ReqMp).
		if cast.spell.InstanceType == 11 {
			count := summonCount(cast.spell.InstanceValue, effectiveSpecial(e, 2))
			if !d.generateSummon(w, s, e, cast.spell.InstanceValue-1, count) {
				e.MP += int32(spent)
				body.ReqMp += int16(spent)
			}
		}
	}

	// Server-authoritative attack power = CurrentScore.Damage + the equipped weapon's
	// damage, with the Divine buff's +20% folded in (effectiveDamage).
	atkDamage := int(d.effectiveDamage(e))
	for i := range body.Dam {
		tid := int(body.Dam[i].TargetID)
		target := w.Entity(tid)
		if target == nil || target.Mode == world.MobEmpty {
			writeDamage(payload, i, 0)
			continue
		}
		// Per-entry claim: -1 marks a skill hit; everything else resolves as
		// melee. The legacy cracks claims other than -2/-1/0 (_MSG_Attack.cpp:355),
		// but the 12000 client's melee encoding is UNVERIFIED and the claimed
		// value is IGNORED anyway (server-authoritative) — so tolerate + log
		// instead of zeroing, or unknown encodings silently disable all player
		// damage again (B12).
		claim := body.Dam[i].Damage
		if claim != damMelee && claim != damSkill && claim != 0 {
			d.log.Debug("attack: unexpected Dam claim (resolving as melee)",
				"conn", s.Conn, "claim", claim, "skill", skillnum)
		}
		// Untouchable NPCs (shops/banks/quest givers) never take damage.
		if !world.IsPlayer(tid) && target.Merchant != 0 {
			writeDamage(payload, i, 0)
			continue
		}
		// Dead targets can only be hit by the resurrection skills (31/99)
		// (_MSG_Attack.cpp:333).
		if target.HP <= 0 && skillnum != 31 && skillnum != combat.ResurrectSkill {
			writeDamage(payload, i, 0)
			continue
		}
		// skillHit: this entry resolves through the skill pipeline; anything
		// else (sentinel -2, 0, or an unknown claim) is melee.
		skillHit := cast.isSkill && claim == damSkill
		// Town safe zone: players in a city cannot be hit by melee or aggressive
		// skills (the legacy gates on the attribute map's PK bit + PKMode; we use
		// the city rectangles until the attribute map semantics are verified).
		if world.IsPlayer(tid) && tid != s.Conn && world.Village(target.X, target.Y) >= 0 {
			if !skillHit || cast.spell.Aggressive != 0 {
				writeDamage(payload, i, 0)
				continue
			}
		}

		var dmg int
		if skillHit {
			dmg = d.resolveSkillHit(w, e, target, tid, skillnum, cast)
			d.applyCastAffect(w, e, target, tid, cast)
		} else {
			// UNVERIFIED: DoubleCritical should be server-computed (BASE_GetDoubleCritical)
			// and ParryRate from GetParryRate; both UNVERIFIED (game-rules.md §4.3-4.4).
			// Until captured we use the packet's DoubleCritical and no parry/reflect.
			dmg = combat.ResolveHit(w.Rand(), combat.HitInput{
				AttackerDamage: atkDamage,
				TargetAC:       int(effectiveAC(target)),
				TargetIsPlayer: world.IsPlayer(tid),
				DoubleCritical: body.DoubleCritical,
				Master:         e.Master,
				UseSkill:       false,
				SkillIndex:     skillnum,
				TargetRsvBlock: target.Rsv&world.RsvBlock != 0,
			})
		}
		if dmg > 0 {
			target.HP -= int32(dmg)
			if target.HP < 0 {
				target.HP = 0
			}
		} else if dmg < 0 && cast.isSkill && cast.spell.InstanceType == 6 {
			// Heal: a negative Dam is the healed amount; clamp to the target's max.
			target.HP -= int32(dmg)
			if m := effectiveMaxHP(target); target.HP > m {
				target.HP = m
			}
		}
		// A struck mob either dies (rewards) or retaliates: it focuses the attacker
		// — and drags its spawn group into the fight (SetBattle + the PartyList
		// propagation, setGroupBattle) — so the AI tick (mobai.go) chases and
		// fights back. Provocation happens even on a blocked hit, matching the
		// original AddEnemyList-on-attack.
		if !world.IsPlayer(tid) {
			if target.HP == 0 {
				d.mobKilled(w, e, target)
			} else {
				setGroupBattle(w, tid, target, e)
				// The attacker's summons join their owner's fight (the legacy
				// EnemyList propagation; summon.go).
				d.commandSummons(w, s.Conn, target)
			}
		}
		writeDamage(payload, i, int32(dmg))
	}

	// Overwrite the attacker's status with the server's authoritative values so every
	// recipient (and the attacker's own client) sees the real HP/MP and the post-kill
	// experience. CurrentExp is how the client refreshes its exp bar — there is no
	// separate exp packet (MSG_UpdateScore carries no exp).
	writeAttackerStatus(payload, e.HP, e.MP, e.Exp, body.ReqMp)

	// Broadcast the server-authoritative result with HEADER.ID = ESCENE_FIELD, exactly
	// as the original (_MSG_Attack.cpp:25 `m->ID = ESCENE_FIELD`). This matters for the
	// exp bar: the client applies Dam[] to the named targets regardless of header (so a
	// mob's attack with HEADER.ID = the mob still hurts the player), but it only applies
	// the ATTACKER's own CurrentExp/CurrentHp/CurrentMp when the attack arrives as a
	// field/scene event. With HEADER.ID = the attacker conn the exp bar never moved.
	// The original GridMulticast (around the target) includes the attacker, so we both
	// echo to the attacker and send to the in-view players.
	hdr := protocol.Header{Type: protocol.MsgAttack, ID: protocol.IDScene}
	w.SendTo(s, hdr, payload)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, hdr, payload)
	})
}

// castInfo is the resolved skill context for one attack packet.
type castInfo struct {
	isSkill bool
	spell   content.Spell
	special int // CurrentScore.Special[kind] (or Level for Sephira skills)
	master  int // skill-mitigation mastery (TK bit-14 rule; 0 otherwise)
}

// validateCast runs the skill gates of _MSG_Attack.cpp:98-215 for SkillIndex ∈
// [0,MaxSkillIndex): Passive reject, class gate, learned-mask check (crack), and
// resolves the Special/mastery the damage formulas need. Melee (SkillIndex out of
// range, typically -1) passes through with isSkill=false. Returns ok=false when
// the whole attack must be dropped.
func (d *Dispatcher) validateCast(w *world.World, s *world.Session, e *world.Entity, skillnum int, tick uint32) (castInfo, bool) {
	if skillnum < 0 || skillnum >= content.MaxSkillIndex || d.spells == nil {
		return castInfo{}, true // melee
	}
	spell, ok := d.spells.Get(skillnum)
	if !ok {
		return castInfo{}, false
	}
	if spell.Passive == 1 && tick != protocol.SkipCheckTick {
		return castInfo{}, false
	}
	cast := castInfo{isSkill: true, spell: spell}
	if skillnum >= 96 {
		// Extra-class (Sephira) skills: learned bit 1<<(skillnum-72); their power
		// scales with Level, not a Special tree (_MSG_Attack.cpp:180-192).
		if tick != protocol.SkipCheckTick && e.LearnedSkill&(1<<(skillnum-72)) == 0 {
			w.AddCrackError(s, 208, 1)
			return castInfo{}, false
		}
		cast.special = int(e.Level)
	} else {
		// Class skills: class gate + learned bit skillnum%24 (_MSG_Attack.cpp:146/201).
		if tick != protocol.SkipCheckTick {
			if content.SkillClass(skillnum) != int(e.Class) {
				return castInfo{}, false
			}
			if e.LearnedSkill&(1<<(skillnum%content.MaxSkill)) == 0 {
				w.AddCrackError(s, 8, 10)
				return castInfo{}, false
			}
		}
		cast.special = effectiveSpecial(e, content.SkillKind(skillnum))
	}
	// Escudo Dourado (85) charges 100×Special gold on cast (_MSG_Attack.cpp:222).
	if skillnum == 85 {
		coin := int32(100 * cast.special)
		if e.Coin < coin {
			return castInfo{}, false
		}
		e.Coin -= coin
		d.sendEtc(w, s, e)
	}
	// Skill mitigation mastery: only a TK with bit 14 learned gets Special[2]/20
	// (clamped 0..15); everyone else casts with 0 (_MSG_Attack.cpp:258-268).
	if e.Class == 0 && e.LearnedSkill&(1<<14) != 0 {
		m := int(e.Special[2]) / 20
		if m < 0 {
			m = 0
		}
		if m > 15 {
			m = 15
		}
		cast.master = m
	}
	return cast, true
}

// applyCastAffect applies a cast's affect/tick to one target (_MSG_Attack.cpp
// buff block ~1172-1240): aggressive casts skip allies (same leader/guild) and
// roll the resist gate; then SetAffect (players only) + SetTick land and the
// target's score/icons refresh. Delay = 100 + Special, Level = Special.
func (d *Dispatcher) applyCastAffect(w *world.World, e, target *world.Entity, tid int, cast castInfo) {
	sp := cast.spell
	if sp.AffectType <= 0 && sp.TickType <= 0 {
		return
	}
	if sp.Aggressive != 0 {
		leader := e.Leader
		if leader == 0 {
			leader = e.ID
		}
		tleader := target.Leader
		if tleader == 0 {
			tleader = tid
		}
		guild, tguild := int(e.Guild), int(target.Guild)
		if guild == 0 && tguild == 0 {
			guild = -1
		}
		if leader == tleader || guild == tguild {
			return // allies never take hostile affects
		}
		// Resist roll: rand()%100 vs RegenMP + AffectResist + level advantage.
		// UNVERIFIED: RegenMP is not modeled on the Entity yet (0); tier level
		// adders (celestial +MAX_LEVEL) wait on the tier system.
		if sp.AffectResist >= 1 && sp.AffectResist <= 4 {
			difLevel := -(int(target.Level) - int(e.Level)) / 2
			if w.Rand().Intn(100) > sp.AffectResist+difLevel {
				return
			}
		}
		if target.Rsv&world.RsvBlock != 0 {
			return
		}
		if world.IsPlayer(e.ID) && target.Clan == 6 {
			return // clan 6 is immune to player-cast affects
		}
	}
	delay := 100 + cast.special
	applied := target.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, cast.special)
	if target.SetTick(sp.TickType, sp.TickValue, sp.AffectTime, sp.Aggressive, delay, cast.special) {
		applied = true
	}
	if !applied {
		return
	}
	// A landed transform (skills 64/66/68/70/71) also swaps the body mesh, which
	// everyone in view must render — the legacy follows the SetAffect with
	// GetCurrentScore + SendScore + SendEquip(conn,0) (_MSG_Attack.cpp:1242-1248).
	// refreshEquip recomputes EquipVisual (where the beast override lives),
	// broadcasts UpdateEquip self+in-view and re-sends the score.
	if sp.AffectType == affectTransform {
		if ts := w.Session(tid); ts != nil {
			d.refreshEquip(w, ts, target)
			d.sendAffect(w, ts, target)
			return
		}
	}
	d.refreshScore(target)
	if ts := w.Session(tid); ts != nil {
		d.sendScore(w, ts, target) // UpdateScore carries the Affect[32] icon array
		d.sendAffect(w, ts, target)
	}
}

// resolveSkillHit runs the per-target skill pipeline (_MSG_Attack.cpp:552-607):
// raw power (SkillBaseDamage) → mitigation (SkillDamage with def ×2 vs players,
// ×1.5 vs Foema) → elemental resist scale. InstanceType 6 returns a NEGATIVE
// value (the heal amount, as the wire encodes it). Non-damage InstanceTypes
// (buffs/specials) return 0 here — the affect engine (M4) handles them.
func (d *Dispatcher) resolveSkillHit(w *world.World, e, target *world.Entity, tid, skillnum int, cast castInfo) int {
	sp := combat.SkillSpell{
		InstanceType:  cast.spell.InstanceType,
		InstanceValue: cast.spell.InstanceValue,
		AffectValue:   cast.spell.AffectValue,
	}
	caster := combat.SkillCaster{
		Class:   int(e.Class),
		Level:   int(e.Level),
		Str:     int(e.Str),
		Int:     int(e.Int),
		Damage:  int(d.effectiveDamage(e)),
		Magic:   int(e.Magic),
		Special: cast.special,
	}
	// UNVERIFIED: CurrentWeather is not modeled (weather 0 = neutral).
	raw := combat.SkillBaseDamage(skillnum, sp, caster, 0, int(d.weaponDamage(e)))

	switch {
	case sp.InstanceType >= 1 && sp.InstanceType <= 5:
		def := int(effectiveAC(target))
		if world.IsPlayer(tid) {
			def *= 2
		}
		if target.Class == 1 { // Foema resists skills ×1.5
			def = def * 3 / 2
		}
		dmg := combat.SkillDamage(w.Rand(), raw, def, cast.master)
		var resist [4]int16
		for k := range resist {
			resist[k] = target.Resist[k] + target.AffResist[k]
		}
		return combat.SkillResistScale(dmg, sp.InstanceType, resist, world.IsPlayer(tid))
	case sp.InstanceType == 6:
		return -raw // heal rides as negative damage
	default:
		return 0 // buffs/specials: no direct damage (affects land in M4)
	}
}

// writeDamage overwrites the server-authoritative damage of Dam[i] in the wire
// payload (the client value is ignored).
func writeDamage(payload []byte, i int, dmg int32) {
	off := protocol.MsgAttackDamOffset + i*protocol.MsgAttackDamStride + 4
	if off+4 <= len(payload) {
		binary.LittleEndian.PutUint32(payload[off:off+4], uint32(dmg))
	}
}

// writeAttackerStatus overwrites the attacker's CurrentHp@4, CurrentExp@12,
// CurrentMp@40 and ReqMp@46 in the MSG_Attack body with the server's
// authoritative values (MsgAttackBody layout, messages.go). These fixed fields
// sit below the Dam[] region (offset 48), so they never collide with per-target
// damage.
func writeAttackerStatus(payload []byte, hp, mp int32, exp int64, reqMp int16) {
	if len(payload) < protocol.MsgAttackDamOffset {
		return
	}
	binary.LittleEndian.PutUint32(payload[4:8], uint32(hp))
	binary.LittleEndian.PutUint64(payload[12:20], uint64(exp))
	binary.LittleEndian.PutUint32(payload[40:44], uint32(mp))
	binary.LittleEndian.PutUint16(payload[46:48], uint16(reqMp))
}
