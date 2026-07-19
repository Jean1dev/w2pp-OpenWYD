package handler

import (
	"encoding/binary"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
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
		if skillnum == 97 && !validateGuardianCannon(w, &body, payload) {
			return
		}
	}
	if cast.isSkill {
		// Mana (BASE_GetManaSpent → abort broke): insufficient MP cancels the cast;
		// otherwise spend and echo the authoritative MP in the attack body
		// (_MSG_Attack.cpp:240-256). ReqMp is the server-owned CUser.ReqMp target,
		// initialized on login and clamped by SetReqMp.
		spent := combat.ManaSpent(cast.spell.ManaSpent, int(e.SaveMana), cast.special)
		if int(e.MP)-spent < 0 {
			d.sendSetHpMp(w, s, e)
			return
		}
		reqMpBefore := s.ReqMp
		e.MP -= int32(spent)
		s.ReqMp -= int32(spent)
		setReqMp(s, e)
		body.ReqMp = int16(s.ReqMp)

		// BM evocation (InstanceType 11, _MSG_Attack.cpp:809-837): spawn the
		// summons once per cast — the Dam entries carry no damage for it. A cast
		// that spawns nothing refunds its mana (the legacy restores Mp/ReqMp).
		if cast.spell.InstanceType == 11 {
			count := summonCount(cast.spell.InstanceValue, effectiveSpecial(e, 2))
			if !d.generateSummon(w, s, e, cast.spell.InstanceValue-1, count) {
				e.MP += int32(spent)
				s.ReqMp = reqMpBefore
				body.ReqMp = int16(s.ReqMp)
			}
		}
	}

	if cast.isSkill && skillnum == combat.ResurrectSkill && e.HP <= 0 {
		d.applyBookResurrection(w, s, e)
		body.ReqMp = int16(s.ReqMp)
	}

	// Server-authoritative attack power = CurrentScore.Damage + the equipped weapon's
	// damage, with the Divine buff's +20% folded in (effectiveDamage).
	atkDamage := int(d.effectiveDamage(e))
	doubleCritical := uint8(0)
	doubleCriticalReady := false
	var healExp int64
	var hpSyncTargets []int
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
		pvpHit := world.IsPlayer(tid) && tid != s.Conn
		combatHit := !skillHit || cast.spell.Aggressive != 0
		// PvP gate: combat damage (melee or an aggressive skill) requires the
		// attacker to opt into PK mode (K key, _MSG_PKMode). Do not use
		// world.Village as a town safe-zone approximation here: the legacy rule
		// keys off an attribute-map PK bit on the ATTACKER tile plus war-state
		// bypasses, and the coarse city rectangles caused issue #67 by zeroing
		// PvP damage near every spawn point.
		if pvpHit && combatHit && !e.PKMode && !d.dueling(s.Conn, tid) {
			writeDamage(payload, i, 0)
			continue
		}

		var dmg int
		if skillHit {
			if !d.validateSkillTarget(w, s, e, target, i, cast, tick) {
				writeDamage(payload, i, 0)
				continue
			}
			dmg = d.resolveSkillHit(w, e, target, tid, skillnum, cast)
			skipGenericAffect := d.applySkillSpecial(w, s, e, target, tid, skillnum, cast, &body, &dmg)
			if dmg > 0 && tid != s.Conn {
				if miss := combat.ResolveParry(w.Rand(), skillnum, d.parryRate(e, target), target.Rsv&world.RsvBlock != 0); miss != 0 {
					dmg = miss
				}
			}
			if !skipGenericAffect {
				d.applyCastAffect(w, e, target, tid, cast)
			}
		} else {
			if !doubleCriticalReady {
				progress := body.Progress
				doubleCritical, _ = combat.DoubleCritical(w.Rand(), attackRunOf(e), int(effectiveCritical(e)), &s.CriticalProgress, &progress)
				body.Progress = progress
				body.DoubleCritical = doubleCritical
				writeAttackProgress(payload, progress)
				writeDoubleCritical(payload, doubleCritical)
				doubleCriticalReady = true
			}
			dmg = combat.ResolveHit(w.Rand(), combat.HitInput{
				AttackerDamage: atkDamage,
				TargetAC:       int(effectiveAC(target)),
				TargetIsPlayer: world.IsPlayer(tid),
				DoubleCritical: doubleCritical,
				Master:         e.Master,
				UseSkill:       false,
				SkillIndex:     skillnum,
				ParryRate:      d.parryRate(e, target),
				TargetRsvBlock: target.Rsv&world.RsvBlock != 0,
			})
			dmg = d.applyAirBladeProc(w, e, target, h.Type, &body, payload, dmg)
		}
		if dmg > 0 {
			dmg = applyHuntressForceDamage(e, target, tid, dmg)
			dmg = d.applyManaControl(w, e, target, tid, dmg)
			hpBefore := target.HP
			target.HP -= int32(dmg)
			if target.HP < 0 {
				target.HP = 0
			}
			ts := w.Session(tid)
			// Drop the victim's heal target by the damage, or the regen tick heals
			// it straight back (_MSG_Attack.cpp:1638-1642).
			damageReqHp(ts, target, int32(dmg))
			if ts != nil && target.HP != hpBefore {
				seen := false
				for _, syncID := range hpSyncTargets {
					if syncID == tid {
						seen = true
						break
					}
				}
				if !seen {
					hpSyncTargets = append(hpSyncTargets, tid)
				}
			}
			d.applyOnHitAffects(w, e, target, tid)
			d.applyHpAbs(w, s, e, dmg)
			if pvpHit && combatHit && !d.dueling(s.Conn, tid) {
				// Landing a PvP hit starts (or refreshes) the chaotic red-blink
				// timer, independent of whether it's already running. On the 0→1
				// transition, re-broadcast the PK state so the attacker's nick turns
				// red (MobName[12]=0) for itself and everyone in view. A duel hit
				// must never mark PK (issue #118 acceptance criteria).
				wasGuilty := pkGuilty(e)
				e.GuiltyUntil = time.Now().Add(pkGuiltyDuration).Unix()
				if !wasGuilty {
					d.broadcastPKState(w, s, e)
				}
			}
		} else if dmg < 0 && cast.isSkill && cast.spell.InstanceType == 6 {
			// Heal: a negative Dam is the healed amount; clamp to the target's max.
			before := target.HP
			target.HP += d.foemaHealAmount(target, -int32(dmg))
			if m := effectiveMaxHP(target); target.HP > m {
				target.HP = m
			}
			if ts := w.Session(tid); ts != nil {
				ts.ReqHp = target.HP
				setReqHp(ts, target)
			}
			healExp += healExpGain(e, target, before)
		}
		// A struck mob either dies (rewards) or records the attacker in its
		// EnemyList — and drags its spawn group into the fight (SetBattle +
		// PartyList propagation, setGroupBattle). Provocation happens even on a
		// blocked hit, matching the original AddEnemyList-on-attack.
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

	if healExp > 0 {
		if healExp > 200 {
			healExp = 200
		}
		e.Exp += healExp
		if e.Exp > level.MaxExp {
			e.Exp = level.MaxExp
		}
		d.applyLevelUps(w, s, e)
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
	for _, tid := range hpSyncTargets {
		ts := w.Session(tid)
		target := w.Entity(tid)
		if ts != nil && target != nil {
			d.sendSetHpMp(w, ts, target)
		}
	}
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
	if tick != protocol.SkipCheckTick {
		// Class gate only applies to the four base-class blocks. Shared/Sephira
		// rows (96+) skip it, but still use the same live learned gate:
		// LearnedSkill & (1 << (skillnum % 24)). The old SecLearnedSkill branch in
		// _MSG_Attack is unreachable because it sits under skillnum>=MAX_SKILLINDEX.
		if content.SkillClass(skillnum) <= 3 && content.SkillClass(skillnum) != int(e.Class) {
			return castInfo{}, false
		}
		if e.LearnedSkill&learnedSkillBit(skillnum) == 0 {
			w.AddCrackError(s, 8, 10)
			return castInfo{}, false
		}
	}
	cast.special = effectiveSpecial(e, content.SkillKind(skillnum))
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

func learnedSkillBit(skillnum int) int32 {
	return int32(1) << uint(skillnum%content.MaxSkill)
}

func (d *Dispatcher) validateSkillTarget(w *world.World, s *world.Session, caster, target *world.Entity, targetSlot int, cast castInfo, tick uint32) bool {
	sp := cast.spell
	// MaxTarget is a legacy max Dam[] index, not a count: _MSG_Attack.cpp rejects
	// only i > MaxTarget. Keep that off-by-one shape for packet parity.
	if tick != protocol.SkipCheckTick && sp.MaxTarget >= 0 && targetSlot > sp.MaxTarget {
		w.AddCrackError(s, 10, 28)
		return false
	}
	if (sp.Index == 41 || sp.Index == 44) && targetSlot >= foemaMultiBuffTargetCap(cast.special) {
		return false
	}
	if sp.BParty != 0 && !skillSameLeaderOrGuild(w, caster, target) {
		w.AddCrackError(s, 10, 27)
		return false
	}
	if sp.Range > 0 && mobDistance(caster.X, caster.Y, target.X, target.Y) > sp.Range {
		return false
	}
	// TargetType is otherwise client/UI guidance in the local legacy _MSG_Attack
	// source. The one safe server gate is target type 0 for self-only effects;
	// party-wide skills and summons are exempt because their target fan-out is
	// controlled by bParty/InstanceType.
	if sp.TargetType == 0 && sp.BParty == 0 && sp.InstanceType != 11 && sp.Aggressive == 0 &&
		(sp.AffectType > 0 || sp.TickType > 0) && target.ID != caster.ID {
		return false
	}
	return true
}

func foemaMultiBuffTargetCap(special int) int {
	n := special/25 + 2
	if n >= protocol.MaxTarget {
		return protocol.MaxTarget
	}
	if n <= 1 {
		return 2
	}
	return n
}

func skillSameLeaderOrGuild(w *world.World, a, b *world.Entity) bool {
	leader := a.Leader
	if leader == 0 {
		leader = a.ID
	}
	targetLeader := b.Leader
	if targetLeader == 0 {
		targetLeader = b.ID
	}
	if leader == targetLeader {
		return true
	}
	guild := int(a.Guild)
	if s := w.Session(a.ID); s != nil && s.GuildDisable {
		guild = 0
	}
	targetGuild := int(b.Guild)
	if s := w.Session(b.ID); s != nil && s.GuildDisable {
		targetGuild = 0
	}
	return guild != 0 && guild == targetGuild
}

func validateGuardianCannon(w *world.World, body *protocol.MsgAttackBody, payload []byte) bool {
	if body.PosX == 0 || body.PosY == 0 || int(body.PosX) >= w.GridDim() || int(body.PosY) >= w.GridDim() {
		return false
	}
	gi := w.GroundItemAt(int16(body.PosX), int16(body.PosY))
	if gi == nil || gi.Item.Index != 746 {
		return false
	}
	body.Motion = 1
	if len(payload) >= protocol.MsgAttackDamOffset {
		payload[34] = 1
	}
	return true
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
	// refreshEquip recomputes visual gear/glow (where the beast override lives),
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
		Str:     int(effectiveStr(e)),
		Int:     int(effectiveInt(e)),
		Damage:  int(d.effectiveDamage(e)),
		Magic:   int(effectiveMagic(e)),
		Special: cast.special,
	}
	// UNVERIFIED: CurrentWeather is not modeled (weather 0 = neutral).
	raw := combat.SkillBaseDamage(skillnum, sp, caster, 0, int(d.weaponDamage(e)))

	switch {
	case sp.InstanceType >= 1 && sp.InstanceType <= 5:
		if skillnum == 79 {
			def := int(effectiveAC(target))
			if world.IsPlayer(tid) {
				def *= 3
			}
			dmg := combat.Damage(w.Rand(), raw, def, cast.master)
			if dmg > 0 {
				dmg /= 2
			}
			return dmg
		}
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
		if target.Clan == 4 {
			return 0
		}
		heal := 3*cast.special/2 + cast.spell.InstanceValue
		if skillnum == 27 {
			heal = 2*cast.special + cast.spell.InstanceValue
		}
		healCap := 1100
		if e.ClassMaster != classMasterMortal && e.ClassMaster != classMasterArch {
			heal *= 2
			healCap = 2200
		}
		if heal >= healCap {
			heal = healCap
		}
		if heal > 0 && heal < 6 {
			heal = 6
		}
		return -heal // heal rides as negative damage
	default:
		return 0 // buffs/specials: no direct damage (affects land in M4)
	}
}

func (d *Dispatcher) applySkillSpecial(w *world.World, s *world.Session, e, target *world.Entity, tid, skillnum int, cast castInfo, body *protocol.MsgAttackBody, dmg *int) bool {
	switch {
	case skillnum == 6: // Furia Divina: displacement only.
		*dmg = 0
		d.applyDivineFury(w, e, target, tid, cast.special)
		return true

	case cast.spell.InstanceType == 7: // Flash: clear combat state.
		target.Target = 0
		clearEnemyList(target)
		if !world.IsPlayer(tid) && target.Mode == world.MobCombat {
			target.Mode = world.MobPeace
		}
		return true

	case cast.spell.InstanceType == 8:
		d.clearDetoxAffects(w, e, target, tid)
		if skillnum == 31 {
			d.applyFoemaResurrection(w, s, e, target, tid, body)
		}
		return true

	case cast.spell.InstanceType == 9:
		d.applyFoemaSummon(w, e, target, tid, body)
		return true

	case cast.spell.InstanceType == 12: // Chamas Etereas.
		d.applyEtherealFlame(w, e, target, tid)
		return true

	case skillnum == 30: // Julgamento Divino.
		*dmg += int(e.HP)
		e.HP = e.HP/6 + 1
		s.ReqHp = e.HP
		setReqHp(s, e)
		d.sendSetHpMp(w, s, e)
		return false

	case skillnum == 22: // Exterminar spends all remaining MP into the hit.
		currentMP := e.MP
		e.MP = 0
		s.ReqMp = 0
		body.ReqMp = 0
		*dmg += int(currentMP) + int(effectiveInt(e))/2
		d.applyExterminarMotion(w, e, target, tid)
		return false

	case skillnum == 98: // Muro de Espinhos creates a Vinha/Vine mob.
		*dmg = 0
		d.createVine(w, body)
		return true

	case skillnum == 47: // Cancelamento removes block first.
		if target.ClearFirstAffect(19) {
			d.refreshScore(target)
			if ts := w.Session(tid); ts != nil {
				d.sendScore(w, ts, target)
				d.sendAffect(w, ts, target)
			}
			return true
		}
		return false
	}
	return false
}

func (d *Dispatcher) applyDivineFury(w *world.World, caster, target *world.Entity, tid, special int) bool {
	if target == nil {
		return false
	}
	if !world.IsPlayer(tid) && target.Merchant != 0 {
		return false
	}
	switch target.Equip[0].Index {
	case 219, 220, 362:
		return false
	}
	if target.GenIndex == 8 || target.GenIndex == 9 || target.Clan == 6 {
		return false
	}

	nx, ny := caster.X, caster.Y
	if nx < target.X {
		nx++
	} else if nx > target.X {
		nx--
	}
	if ny < target.Y {
		ny++
	} else if ny > target.Y {
		ny--
	}
	x, y, ok := d.freeCellAtOrNear(w, nx, ny)
	if !ok {
		return false
	}

	kindValue := special/10 + 20
	if !world.IsPlayer(tid) {
		kindValue = special/5 + 40
	}
	chance := kindValue + divineFuryLevelDiff(caster, target)/4
	if chance > 50 {
		chance = 50
	}
	if w.Rand().Intn(100) >= chance {
		return false
	}
	target.Route[0] = 0
	d.moveEntityWithAction(w, tid, x, y, 2, 6)
	if !world.IsPlayer(tid) {
		setGroupBattle(w, tid, target, caster)
	}
	return true
}

func divineFuryLevelDiff(caster, target *world.Entity) int {
	casterLevel := int(caster.Level)
	if caster.ClassMaster != classMasterMortal && caster.ClassMaster != classMasterArch {
		casterLevel += int(level.MaxLevel)
	}
	targetLevel := int(target.Level)
	if target.ClassMaster != classMasterMortal && target.ClassMaster != classMasterArch {
		targetLevel += int(level.MaxLevel)
	}
	return casterLevel - targetLevel
}

func (d *Dispatcher) applyExterminarMotion(w *world.World, caster, target *world.Entity, tid int) bool {
	if target == nil {
		return false
	}
	if target.Equip[0].Index == 219 || target.Equip[0].Index == 220 {
		return false
	}
	x, y, ok := d.freeCellNear(w, target.X, target.Y)
	if !ok {
		return false
	}
	d.moveEntityWithAction(w, tid, x, y, 2, 2)
	if !world.IsPlayer(tid) {
		setGroupBattle(w, tid, target, caster)
	}
	return true
}

func (d *Dispatcher) moveEntityWithAction(w *world.World, id int, x, y int16, effect, speed int32) bool {
	e := w.Entity(id)
	if e == nil {
		return false
	}
	oldX, oldY := e.X, e.Y
	w.SetEntityPos(id, x, y)
	body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Effect: effect, Speed: speed, TargetX: x, TargetY: y}
	payload := body.Encode()
	d.moveMulticast(w, id, oldX, oldY, protocol.MsgAction, payload)
	if s := w.Session(id); s != nil && s.Mode == world.UserPlay {
		w.SendTo(s, protocol.Header{Type: protocol.MsgAction, ID: uint16(id)}, payload)
	}
	return true
}

func (d *Dispatcher) createVine(w *world.World, body *protocol.MsgAttackBody) bool {
	if d.vineMob == nil || body == nil {
		return false
	}
	x, y := int16(body.TargetX), int16(body.TargetY)
	if !d.cellAvailable(w, x, y) {
		return false
	}
	id := w.SpawnMobAt(world.MobSpawn{Template: d.vineMob, X: x, Y: y, RouteType: 3, GenIndex: -1})
	if id < 0 {
		return false
	}
	mob := w.Entity(id)
	if mob == nil {
		return false
	}
	mob.Mode = world.MobPeace
	mob.WaitTicks = 40
	payload := protocol.EncodeCreateMobBody(createMobFrom(mob, 2))
	w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
		if w.MarkSeen(vs, id) {
			w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, payload)
		}
	})
	return true
}

func (d *Dispatcher) clearDetoxAffects(w *world.World, caster, target *world.Entity, tid int) {
	changed := false
	for i := range target.Affect {
		t := target.Affect[i].Type
		if t == 1 || t == 3 || t == 5 || t == 7 || t == 10 || t == 12 || t == 20 ||
			(t == 32 && caster.LearnedSkill&(1<<7) != 0) {
			target.Affect[i] = world.Affect{}
			changed = true
		}
	}
	if !changed {
		return
	}
	d.refreshScore(target)
	if ts := w.Session(tid); ts != nil {
		d.sendScore(w, ts, target)
		d.sendAffect(w, ts, target)
	}
}

func (d *Dispatcher) applyFoemaResurrection(w *world.World, s *world.Session, caster, target *world.Entity, tid int, body *protocol.MsgAttackBody) {
	hp := int32((w.Rand().Intn(10) + 10) * int((effectiveMaxHP(caster)+1)/100))
	caster.MP = 0
	s.ReqMp = 0
	body.ReqMp = 0
	d.sendSetHpMp(w, s, caster)
	if w.Rand().Intn(100) >= 70 {
		return
	}
	target.HP = hp
	if ts := w.Session(tid); ts != nil {
		ts.CrackError = 0
		ts.ReqHp = target.HP
		setReqHp(ts, target)
		d.sendScore(w, ts, target)
		d.sendSetHpMp(w, ts, target)
		d.sendEtc(w, ts, target)
	}
	bodyPayload := protocol.EncodeCreateMobBody(createMobFrom(target, 0))
	w.ForEachInView(tid, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, bodyPayload)
	})
}

func (d *Dispatcher) applyFoemaSummon(w *world.World, caster, target *world.Entity, tid int, body *protocol.MsgAttackBody) bool {
	if caster == nil || target == nil || !world.IsPlayer(tid) || target.HP <= 0 {
		return false
	}
	ts := w.Session(tid)
	if ts == nil || ts.Mode != world.UserPlay {
		return false
	}
	x, y, ok := d.freeCellAtOrNear(w, int16(body.TargetX), int16(body.TargetY))
	if !ok {
		return false
	}
	d.moveEntityWithAction(w, tid, x, y, 1, 2)
	if c := world.Village(x, y); c >= 0 && c <= 3 {
		target.LastCity = int16(c)
	}
	return true
}

func (d *Dispatcher) applyManaControl(w *world.World, caster, target *world.Entity, tid, dmg int) int {
	if dmg <= 0 || target == nil || !target.HasAffect(18) {
		return dmg
	}
	ts := w.Session(tid)
	if ts == nil {
		return dmg
	}
	reduced, spent, ok := manaControlDamage(target, dmg, caster != nil && caster.LearnedSkill&(1<<23) != 0)
	if !ok {
		return dmg
	}
	ts.ReqMp -= spent
	setReqMp(ts, target)
	d.sendSetHpMp(w, ts, target)
	return reduced
}

// applyHpAbs is the Jóia da Absorção lifesteal (_MSG_Attack.cpp:1651): on a
// landed hit, a 50% roll heals the attacker AffHpAbs% of the damage dealt, capped
// at 350/hit. The original has a bug in its else branch (ReqHp = RecHP overwrites
// instead of adding); we add-then-clamp so the heal is coherent. No-op unless the
// attacker carries the buff.
func (d *Dispatcher) applyHpAbs(w *world.World, s *world.Session, e *world.Entity, dmg int) {
	if e.AffHpAbs == 0 || dmg < 1 || w.Rand().Intn(2) != 0 {
		return
	}
	rec := hpAbsHeal(dmg, e.AffHpAbs)
	if rec <= 0 {
		return
	}
	e.HP += rec
	s.ReqHp = e.HP
	setReqHp(s, e) // clamps HP/ReqHp to effectiveMaxHP
	d.sendSetHpMp(w, s, e)
}

// hpAbsHeal is the Jóia da Absorção heal amount: AffHpAbs% of the damage dealt,
// capped at 350 (_MSG_Attack.cpp:1653).
func hpAbsHeal(dmg int, absPct int32) int32 {
	rec := (int32(dmg)*absPct + 1) / 100
	if rec > 350 {
		rec = 350
	}
	if rec < 0 {
		rec = 0
	}
	return rec
}

func manaControlDamage(target *world.Entity, dmg int, enhanced bool) (int, int32, bool) {
	if dmg <= 0 || target == nil || !target.HasAffect(18) || target.MP <= effectiveMaxMP(target)/10 {
		return dmg, 0, false
	}
	spent := int32(dmg)
	target.MP -= spent
	if target.MP < 0 {
		target.MP = 0
	}
	divisor := int32(55)
	if enhanced {
		divisor = 50
	}
	reduced := ((spent >> 1) + (spent << 4)) / divisor
	if reduced < 0 {
		return 0, spent, true
	}
	return int(reduced), spent, true
}

// itemRawSanc returns an item's PACKED sanc cValue, not its refine level.
//
// The fairy heal-reduction divisor below is the one place that wants the raw
// number: the legacy reads Equip[13].stEffect[0].cValue straight off the struct
// (Server.cpp:10068, :10077) rather than going through BASE_GetItemSanc, so the
// pity counter deliberately feeds into the divisor. Use refine.Level anywhere a
// real refine level is meant.
func itemRawSanc(it world.Item) int {
	for _, ef := range it.Effects {
		if ef.Effect >= 116 && ef.Effect <= 125 {
			return int(ef.Value)
		}
		if ef.Effect == efSanc {
			return int(ef.Value)
		}
	}
	return 0
}

func (d *Dispatcher) foemaHealAmount(target *world.Entity, heal int32) int32 {
	switch target.Equip[fairyEquipSlot].Index {
	case 786:
		sanc := itemRawSanc(target.Equip[fairyEquipSlot])
		if sanc < 2 {
			sanc = 2
		}
		return heal / int32(sanc)
	case 1936:
		sanc := itemRawSanc(target.Equip[fairyEquipSlot])
		if sanc < 2 {
			sanc = 2
		}
		return heal / int32(sanc*100)
	case 1937:
		sanc := itemRawSanc(target.Equip[fairyEquipSlot])
		if sanc < 2 {
			sanc = 2
		}
		return heal / int32(sanc*20000)
	default:
		return heal
	}
}

func healExpGain(caster, target *world.Entity, before int32) int64 {
	if caster == nil || target == nil || caster.ID == target.ID || !world.IsPlayer(caster.ID) {
		return 0
	}
	if world.Village(target.X, target.Y) >= 0 {
		return 0
	}
	gain := (target.HP - before) >> 3
	if gain < 0 {
		return 0
	}
	if gain > 120 {
		gain = 120
	}
	return int64(gain)
}

func (d *Dispatcher) applyBookResurrection(w *world.World, s *world.Session, e *world.Entity) {
	if e.HP != 0 {
		return
	}
	rev := w.Rand().Intn(115)
	if rev > 100 {
		rev -= 15
	}
	if rev >= 40 {
		e.HP = 2
		s.CrackError = 0
		s.ReqHp = e.HP
		setReqHp(s, e)
		d.sendScore(w, s, e)
		d.sendSetHpMp(w, s, e)
		d.recall(w, s, e)
		d.sendEtc(w, s, e)
	}
	hp := int32((w.Rand().Intn(50) + 1) * int((effectiveMaxHP(e)+1)/100))
	mp := int32((w.Rand().Intn(50) + 1) * int((effectiveMaxMP(e)+1)/100))
	e.HP, e.MP = hp, mp
	s.CrackError = 0
	s.ReqHp, s.ReqMp = e.HP, e.MP
	d.sendScore(w, s, e)
	d.sendSetHpMp(w, s, e)
	d.sendEtc(w, s, e)
	body := protocol.EncodeCreateMobBody(createMobFrom(e, 0))
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
	})
}

func (d *Dispatcher) applyEtherealFlame(w *world.World, caster, target *world.Entity, tid int) {
	ts := w.Session(tid)
	if ts == nil {
		return
	}
	chance := (int(caster.BaseSpecial[1]) + 1) / 7
	if w.Rand().Intn(100) > chance {
		burn := ((target.MP + 1) / 100) * int32(10+w.Rand().Intn(10))
		target.MP -= burn
		if target.MP < 0 {
			target.MP = 0
		}
		ts.ReqMp = target.MP
		d.sendSetHpMp(w, ts, target)
		d.sendScore(w, ts, target)
		return
	}
	changed := false
	for i := range target.Affect {
		switch target.Affect[i].Type {
		case 14, 16, 18, 19:
			target.Affect[i] = world.Affect{}
			changed = true
		}
	}
	if changed {
		d.refreshEquip(w, ts, target)
		d.sendScore(w, ts, target)
		d.sendAffect(w, ts, target)
	}
}

func (d *Dispatcher) parryRate(attacker, target *world.Entity) int {
	attackDex := int(effectiveDex(attacker)) / 5
	if attacker.LearnedSkill&0x1000000 != 0 {
		attackDex += 100
	}
	if attacker.Rsv&world.RsvCast != 0 {
		attackDex += 500
	}
	return combat.ParryRate(int(effectiveDex(target)), target.Parry, attackDex, int(attacker.Rsv))
}

func applyHuntressForceDamage(attacker, target *world.Entity, tid, dmg int) int {
	if attacker == nil || target == nil || dmg <= 0 {
		return dmg
	}
	if !world.IsPlayer(tid) && attacker.AffForceMobDamage != 0 {
		dmg += int(attacker.AffForceMobDamage)
	}
	if attacker.AffForceDamage == 0 {
		return dmg
	}
	if world.IsPlayer(tid) || target.Clan == 4 {
		dmg >>= 2
	}
	if dmg <= 1 {
		return int(attacker.AffForceDamage)
	}
	return dmg + int(attacker.AffForceDamage)
}

func (d *Dispatcher) applyAirBladeProc(w *world.World, attacker, target *world.Entity, msgType protocol.Type, body *protocol.MsgAttackBody, payload []byte, dmg int) int {
	if dmg <= 0 || attacker == nil || target == nil || msgType != protocol.MsgAttackTwo ||
		attacker.Class != 3 || attacker.LearnedSkill&(1<<21) == 0 || w.Rand().Intn(4) != 0 {
		return dmg
	}
	skillDam := effectiveSpecial(attacker, 3) + int(effectiveStr(attacker))
	skillDam = combat.Damage(w.Rand(), skillDam, int(effectiveAC(target)), attacker.Master)
	if skillDam > 0 {
		skillDam /= 2
	}
	if skillDam < 60 {
		skillDam = 60
	}
	body.DoubleCritical |= 4
	writeDoubleCritical(payload, body.DoubleCritical)
	return dmg + skillDam
}

func (d *Dispatcher) applyOnHitAffects(w *world.World, attacker, target *world.Entity, tid int) {
	if attacker == nil || target == nil {
		return
	}
	if attacker.Rsv&world.RsvFrost != 0 && w.Rand().Intn(2) == 0 {
		d.applyOnHitSpell(w, target, tid, 36, effectiveSpecial(attacker, 1)+150, effectiveSpecial(attacker, 1))
	}
	if attacker.Rsv&world.RsvDrain != 0 && w.Rand().Intn(2) == 0 {
		d.applyOnHitSpell(w, target, tid, 40, effectiveSpecial(attacker, 1)+150, effectiveSpecial(attacker, 1))
	}
}

func (d *Dispatcher) applyOnHitSpell(w *world.World, target *world.Entity, tid, skillnum, delay, level int) {
	sp, ok := onHitSpell(d.spells, skillnum)
	if !ok {
		return
	}
	applied := target.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, level)
	if target.SetTick(sp.TickType, sp.TickValue, sp.AffectTime, sp.Aggressive, delay, level) {
		applied = true
	}
	if !applied {
		return
	}
	d.refreshScore(target)
	if ts := w.Session(tid); ts != nil {
		d.sendScore(w, ts, target)
		d.sendAffect(w, ts, target)
	}
}

func onHitSpell(spells *content.SkillData, skillnum int) (content.Spell, bool) {
	if spells != nil {
		if sp, ok := spells.Get(skillnum); ok {
			return sp, true
		}
	}
	switch skillnum {
	case 36:
		return content.Spell{Index: 36, AffectType: 1, AffectValue: 2, AffectTime: 1, Aggressive: 1}, true
	case 40:
		return content.Spell{Index: 40, TickType: 20, TickValue: 10, AffectTime: 1, Aggressive: 1}, true
	default:
		return content.Spell{}, false
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

func writeAttackProgress(payload []byte, progress uint16) {
	if len(payload) >= protocol.MsgAttackDamOffset {
		binary.LittleEndian.PutUint16(payload[32:34], progress)
	}
}

func writeDoubleCritical(payload []byte, doubleCritical uint8) {
	if len(payload) >= protocol.MsgAttackDamOffset {
		payload[36] = doubleCritical
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
