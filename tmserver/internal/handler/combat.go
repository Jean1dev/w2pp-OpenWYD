package handler

import (
	"encoding/binary"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// attackCadence is the minimum ms between attacks (handlers/_MSG_Attack.md §4):
// ClientTick < LastAttackTick + 800 ⇒ AddCrackError(1,107).
const attackCadence = 800

// The attack's ClientTick must sit inside this band of the SERVER clock
// (_MSG_Attack.cpp:79-96) — the same band the movement handler enforces
// (movement.go). The cadence above only compares the client's tick with the
// client's previous tick, so on its own it trusts a clock the client writes.
const (
	attackTickFutureWindow = 15000  // ClientTick > CurrentTime + 15000
	attackTickPastWindow   = 120000 // ClientTick < CurrentTime - 120000
)

// playerMeleeReach is pUser.Range as the legacy leaves it: CMob.cpp:696-697
// computes the EF_RANGE of the gear and then overwrites it with 23 for every
// player, so the melee reach test "dis > Range || dis > 23" is dis > 23.
const playerMeleeReach = 23

// Names of the anti-cheat gates restored from the legacy attack handler, as
// the refusal counter and the log count them.
const (
	travaJanela      = "janela"       // _MSG_Attack.cpp:79-96, ClientTick outside the server-clock band
	travaDistancia   = "distancia"    // _MSG_Attack.cpp:424-426, melee reach
	travaSegundoAlvo = "segundo_alvo" // the intent of _MSG_Attack.cpp:431, extra melee targets
	travaTela        = "tela"         // _MSG_Attack.cpp:347-351, target off the attacker's screen
)

// recusarAtaque counts one refusal by a restored legacy gate and logs it per
// account at the 1st, 10th, 100th… refusal: an honest client tripping a gate
// has to show up right away, and a cheater hammering one must not flood the
// log. The per-account total is logged again on disconnect
// (world.logAttackRefusals).
func (d *Dispatcher) recusarAtaque(s *world.Session, trava string, attrs ...any) {
	if s.AttackRefusals == nil {
		s.AttackRefusals = make(map[string]int, 2)
	}
	s.AttackRefusals[trava]++
	n := s.AttackRefusals[trava]
	for n%10 == 0 {
		n /= 10
	}
	if n != 1 {
		return
	}
	d.log.Warn("attack refused by a restored legacy gate",
		append([]any{"gate", trava, "account", s.AccountName, "conn", s.Conn,
			"refusals", s.AttackRefusals[trava]}, attrs...)...)
}

// Per-target Dam sentinel values (_MSG_Attack.cpp:355): the client marks each
// entry melee or skill; any other non-zero claimed damage is a crack.
const (
	damMelee = -2
	damSkill = -1
)

// affectSamaritano is affect 24 ON A PLAYER (SkillData.csv:14, the only skill row
// that uses it). The same id on a MOB is the summon lifespan counter —
// affectSummonLife, summon.go:25 — which is why every read of it is index-gated.
const affectSamaritano = 24

// removeSamaritano ports DoRemoveSamaritano (Server.cpp:9129), which the legacy
// calls on EVERY attack packet — melee or cast, before any damage is resolved
// (_MSG_Attack.cpp:274). Samaritano is a purely defensive buff: it is meant to
// hold only while its owner is not swinging back.
//
// This runs at the TOP of attack, while the cast that installs the affect only
// lands much later in applyCastAffect — so casting Samaritano never cancels the
// buff it is putting up.
func (d *Dispatcher) removeSamaritano(w *world.World, e *world.Entity) {
	if !e.ClearFirstAffect(affectSamaritano) {
		return
	}
	// Losing the buff shrinks MaxHP, so the refresh may clamp current HP down —
	// the same thing the legacy's GetCurrentScore does before SendScore.
	d.refreshScore(e)
	if s := w.Session(e.ID); s != nil {
		d.sendScore(w, s, e)
		d.sendAffect(w, s, e)
	}
}

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
		d.sendHpMode(w, s, 0)
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}

	// expBefore feeds the diagnostic at the end of this handler: it is how we tell
	// a swing that killed something from one that merely hurt it, without threading
	// a return value back out of the kill path.
	expBefore := e.Exp

	var body protocol.MsgAttackBody
	if err := body.Decode(payload); err != nil {
		return
	}

	// MANA PREVISTA: toda recusa deste handler manda a mana autoritativa antes
	// de sair.
	//
	// O cliente do WYD desconta a mana LOCALMENTE no instante em que o jogador
	// lança, e só corrige quando o servidor manda MSG_SetHpMp. Uma recusa que sai
	// calada deixa um buraco permanente na conta dele: a barra do cliente fica
	// abaixo da verdade e nunca volta sozinha. Repetido — e a guarda de cadência
	// recusa qualquer segundo lançamento dentro de 800ms, que é o que acontece quem
	// testa magia em sequência — a barra chega a valores absurdamente negativos:
	// -15108 num personagem cujo máximo é 5133, com a mana do SERVIDOR intacta o
	// tempo todo. Foi por isso que grampear o valor no servidor não mudou nada: o
	// servidor nunca esteve errado.
	//
	// O caminho de mana insuficiente logo abaixo sempre fez isso certo
	// (sendSetHpMp antes do return); faltava nas recusas por crack error. O legado
	// tem o mesmo cuidado, mandando SendHpMode antes de alguns desses códigos.

	// Liveness: the dead may only act with the resurrect skill (99). Use <= 0 (not
	// == 0) so a negative-HP edge can never slip an action through.
	if e.HP <= 0 && int(body.SkillIndex) != combat.ResurrectSkill {
		d.sendSetHpMp(w, s, e) // ver a nota sobre mana prevista, acima
		w.AddCrackError(s, 1, 8)
		return
	}

	// Anti-speed cadence + tick sanity (int64 math avoids uint32 underflow on the
	// first attack, when LastAttackTick == 0). SkipCheckTick bypasses the checks.
	tick := h.ClientTick
	if tick != protocol.SkipCheckTick {
		last := int64(s.LastAttackTick)
		if int64(tick) < last+attackCadence {
			d.sendSetHpMp(w, s, e)     // ver a nota sobre mana prevista, acima
			w.AddCrackError(s, 1, 107) // too fast
			return
		}
		if int64(tick) < last-100 {
			d.sendSetHpMp(w, s, e)   // ver a nota sobre mana prevista, acima
			w.AddCrackError(s, 4, 7) // tick too far in the past
			return
		}
	}
	s.LastAttackTick = tick
	s.LastAttack = int(body.SkillIndex)

	// FIDELIDADE AO LEGADO (restaurada): the attack's ClientTick must sit within
	// [now-120000, now+15000] of the server clock, or the attack is refused with
	// the same crack error as the cadence (_MSG_Attack.cpp:79-96). Without it the
	// 800 ms cadence only compared the client's clock with itself: a client that
	// wrote its own ticks 800 apart attacked as fast as it could send.
	//
	// Checked after LastAttackTick moves, in the legacy order (:75 before :79),
	// so a forged tick far in the future also locks its own sender out of the
	// cadence gate.
	if tick != protocol.SkipCheckTick {
		if t, now := int64(tick), int64(w.Now()); t > now+attackTickFutureWindow || t < now-attackTickPastWindow {
			d.recusarAtaque(s, travaJanela, "client_tick", t, "server_now", now)
			d.sendSetHpMp(w, s, e) // ver a nota sobre mana prevista, acima
			w.AddCrackError(s, 1, 107)
			return
		}
	}

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
			d.sendSetHpMp(w, s, e) // ver a nota sobre mana prevista, acima
			return
		}
		if skillnum == 97 && !validateGuardianCannon(w, &body, payload) {
			d.sendSetHpMp(w, s, e) // ver a nota sobre mana prevista, acima
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

	// Swinging drops Samaritano (_MSG_Attack.cpp:274), before the damage loop.
	d.removeSamaritano(w, e)

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
		// Untouchable NPCs (shops/banks/quest givers/town services) never take damage.
		if !world.IsPlayer(tid) && target.NonCombatNPC {
			writeDamage(payload, i, 0)
			continue
		}
		if !d.towerAttackAllowed(e, target) {
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
		if pvpHit && combatHit && !e.PKMode && !d.dueling(s.Conn, tid) && !d.towerPvP(e, target) {
			writeDamage(payload, i, 0)
			continue
		}
		// A maximally-chaotic attacker cannot damage a comparatively clean target
		// (_MSG_Attack.cpp "PK - War - Miss": pointPK<=10 && SummonerPointPK>10 ⇒
		// dam=0 + _DN_CantKillUser). Compares the raw PKPoint byte, not the -75
		// display value. No geography here either, for the same reason as above.
		if pvpHit && combatHit && !d.dueling(s.Conn, tid) && int(e.PKPoint) <= 10 && int(target.PKPoint) > 10 {
			d.sendChatText(w, s, fmt.Sprintf("Voce nao pode atacar este jogador (Pontos Caos: %d)", int(e.PKPoint)-75))
			writeDamage(payload, i, 0)
			continue
		}
		// Evocação do próprio grupo não apanha de quem a evocou nem dos
		// companheiros de grupo. O cliente em modo PK põe os pets do BM na lista de
		// alvos da magia de área, e o BM matava as próprias criaturas.
		if combatHit && evocacaoDoMesmoGrupo(e, target) {
			writeDamage(payload, i, 0)
			continue
		}

		// FIDELIDADE AO LEGADO (restaurada): a target outside the attacker's
		// screen is dropped from the attack, and the attacker's client is told
		// to remove it (_MSG_Attack.cpp:347-351; skill 42 is exempt there too).
		// Of the restored attack gates this is the one that does not trust the
		// client: both positions are the server's own. Without it anything in
		// the world could be hit from anywhere.
		if skillnum != 42 && (e.X < target.X-viewGridX || e.X > target.X+viewGridX ||
			e.Y < target.Y-viewGridY || e.Y > target.Y+viewGridY) {
			d.recusarAtaque(s, travaTela, "target", tid)
			w.SendTo(s, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(tid)}, protocol.EncodeRemoveMobBody(1))
			// The view set has to agree with what the client was just told, or
			// the target never comes back into view when the attacker walks up.
			w.UnmarkSeen(s, tid)
			continue
		}

		var dmg int
		airBlade := 0 // the HT proc share, which the PvP quarter leaves whole
		if skillHit {
			if !d.validateSkillTarget(w, s, e, target, i, cast, tick) {
				writeDamage(payload, i, 0)
				continue
			}
			dmg = d.resolveSkillHit(w, e, target, tid, skillnum, cast)
			skipGenericAffect := d.applySkillSpecial(w, s, e, target, tid, skillnum, cast, &body, &dmg)
			// The client can self-target aggressive skill rows through the hotbar.
			// Do not turn those rows into damage against the caster; explicit HP
			// costs, such as Julgamento Divino, are handled in applySkillSpecial.
			if dmg > 0 && tid == s.Conn && cast.spell.Aggressive != 0 && skillnum != 30 {
				dmg = 0
			}
			if dmg > 0 && tid != s.Conn {
				miss := combat.ResolveParry(w.Rand(), skillnum, d.skillParryRate(e, target), target.Rsv&world.RsvBlock != 0)
				if miss = capMissStreak(e, tid, miss, int(d.combatRules.MaxMissStreak)); miss != 0 {
					dmg = miss
				}
			}
			if !skipGenericAffect {
				d.applyCastAffect(w, e, target, tid, cast)
			}
		} else {
			// FIDELIDADE AO LEGADO (restaurada): melee farther than the reach is
			// refused whole and in silence — no crack error, no echo
			// (_MSG_Attack.cpp:424-426). The distance is the packet's own
			// PosX/Y against TargetX/Y, as there; it stops a client that says
			// where it is, not one that lies about it.
			if mobDistance(int16(body.PosX), int16(body.PosY), int16(body.TargetX), int16(body.TargetY)) > playerMeleeReach {
				d.recusarAtaque(s, travaDistancia,
					"pos_x", body.PosX, "pos_y", body.PosY, "target_x", body.TargetX, "target_y", body.TargetY)
				return
			}
			// DIVERGÊNCIA DELIBERADA DO LEGADO: from the second melee target on,
			// only a Huntress (class 3) or a character with skill 0x40 learned
			// may hit; for anyone else the extra targets are dropped in silence
			// and counted. The legacy's line is
			//   if (i > 0 && m->Size < sizeof(MSG_AttackTwo) && Class != 3 && !(LearnedSkill & 0x40))
			// (_MSG_Attack.cpp:431), and its size clause makes it dead code:
			// the target loop (:297-306) only reads a second entry from a packet
			// bigger than AttackOne, so it could fire only on a malformed size
			// between the two, while a full 13-target packet sailed through. Here
			// the entry count is (len-48)/8, so read literally it would never
			// fire at all. What comes back is the intent. Without the crack error
			// the legacy attaches: whether the real client ever sends a second
			// melee target for another class is unverified, so the refusal is
			// counted and logged, not charged to the player.
			if i > 0 && e.Class != 3 && e.LearnedSkill&0x40 == 0 {
				d.recusarAtaque(s, travaSegundoAlvo, "target", tid)
				writeDamage(payload, i, 0)
				continue
			}
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
			dmg, airBlade = d.applyAirBladeProc(w, e, target, h.Type, &body, payload, dmg)
		}
		if dmg > 0 {
			// The legacy PvP block (pvp.go): every blow on a player or a summon keeps
			// a quarter ("Perfuração"), and the panel's PvP share rides on top.
			dmg = perfuracao(target, tid, dmg, airBlade)
			if pvpHit {
				dmg = d.applyPvPRule(dmg, skillHit)
			}
			// Defesa de Evolução (tierdefense.go) — a server rule, not parity, so it
			// has no legacy position to copy. It goes FIRST, before every other
			// adjustment, because it is the defender's tier resisting the blow
			// itself: everything after, the mount absorb included, should work on
			// what actually got through. PvP only; a mob is not a Mortal.
			if pvpHit {
				dmg = applyTierDefense(e.ClassMaster, target.ClassMaster, dmg)
			}
			dmg = applyHuntressForceDamage(e, target, tid, dmg)
			// Ataque PvP, then the defender's flat reflect and Defesa PvP
			// (_MSG_Attack.cpp:1322-1331, 1494-1510), before the mount takes its share.
			if pvpHit {
				dmg = d.applyPvPStats(e, target, dmg)
			}
			dmg = d.applyManaControl(w, e, target, tid, dmg)
			// The victim's mount eats its share LAST, after every other adjustment,
			// because that is where the legacy puts it (_MSG_Attack.cpp:1520, after
			// reflect and the damage clamp). byPlayer is true: this whole path is one
			// player swinging.
			dmg = d.absorbBlow(w, target, dmg, true)
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
			// Landing a PvP hit against a comparatively clean target (PKPoint>10)
			// marks BOTH sides Guilty (_MSG_Attack.cpp: SetGuilty(conn,8);
			// SetGuilty(idx,8)) — re-broadcasting whichever side's nick wasn't
			// already red. A duel hit must never mark PK (issue #118 acceptance
			// criteria).
			if pvpHit && combatHit && !d.dueling(s.Conn, tid) && int(target.PKPoint) > 10 {
				d.markGuilty(w, s, e)
				d.markGuilty(w, w.Session(tid), target)
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
		} else if pvpHit && target.HP == 0 && !d.dueling(s.Conn, tid) {
			// Duel eliminations are handled by the duel-arena sweep (duel.go), not
			// as a chaos-relevant PvP kill — a duel death must not grant/cost PKPoint
			// or EXP (same exclusion as the Guilty-set gate above).
			d.pvpKilled(w, e, target)
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
	// recipient (and the attacker's own client) sees the real HP/MP and the current
	// experience. CurrentExp is how the client refreshes its exp bar — there is no
	// separate exp packet (STRUCT_SCORE has no Exp field, and MobKilled sends none).
	//
	// The experience is the total AFTER this swing, kill included. The original
	// happens to read it before MobKilled runs (_MSG_Attack.cpp:1743-1750) so its
	// gain rides out on the following frame, and reproducing that ordering here was
	// a mistake with a very visible symptom: the client derives the floating gain
	// from the DIFFERENCE between the totals it is sent, so a killing blow that
	// reports the pre-kill total is a difference of zero. It drew "EXP +0" and
	// "Adquiriu 0 de experiência" on every kill.
	writeAttackerStatus(payload, e.HP, e.MP, e.Exp, body.ReqMp)

	// Broadcast the server-authoritative result with HEADER.ID = ESCENE_FIELD, exactly
	// as the original (_MSG_Attack.cpp:25 `m->ID = ESCENE_FIELD`). This matters for the
	// exp bar: the client applies Dam[] to the named targets regardless of header (so a
	// mob's attack with HEADER.ID = the mob still hurts the player), but it only applies
	// the ATTACKER's own CurrentExp/CurrentHp/CurrentMp when the attack arrives as a
	// field/scene event. With HEADER.ID = the attacker conn the exp bar never moved.
	// The original GridMulticast (around the target) includes the attacker, so we both
	// echo to the attacker and send to the in-view players.
	//
	// The TYPE is the frame's own. The original edits the client's message in place
	// and multicasts that same buffer — `GridMulticast(TargetX, TargetY,
	// (MSG_STANDARD*)m, 0)` (_MSG_Attack.cpp:1750) — so an attack sent as
	// MSG_AttackOne comes back as MSG_AttackOne. Forcing MSG_Attack on every reply
	// meant this client, which attacks with 0x039D, was answered with a message it
	// had not asked for: no floating exp gain, no bar movement, only the level
	// changing (that arrives separately, in UpdateScore).
	// The CLIENT TICK is the frame's own too, and for the same reason: the original
	// keeps it and only substitutes server time for the SKIPCHECKTICK sentinel
	// (_MSG_Attack.cpp:1745). It is how the client tells the answer to its own swing
	// from a bystander's — and it only takes CurrentExp out of its own. Every other
	// message is stamped with the server clock, so this needs SendEcho.
	hdr := protocol.Header{Type: h.Type, ID: protocol.IDScene, ClientTick: h.ClientTick}
	// Diagnostic for the frozen exp bar. Players report that killing a mob raises
	// the level and the stored total but floats no gain and moves no bar, while the
	// damage from this very frame renders — so the client parses the reply and
	// rejects only the attacker's own experience out of it. Two attempts at that
	// (the message type, then the client tick) changed nothing, so this prints what
	// actually leaves the server for a swing that killed something: the frame it is
	// carried in, the tick it is stamped with, the exp written, and the eight bytes
	// as they sit at CurrentExp@12. Only on a kill, so it cannot flood.
	if e.Exp != expBefore {
		var expField []byte
		if len(payload) >= 20 {
			expField = payload[12:20]
		}
		d.log.Info("exp echo",
			"account", s.AccountName, "conn", s.Conn,
			"type", fmt.Sprintf("%#04x", uint16(hdr.Type)),
			"client_tick", h.ClientTick, "echo_tick", hdr.ClientTick,
			"exp_before", expBefore, "exp_after", e.Exp, "gain", e.Exp-expBefore, "exp_sent", e.Exp,
			"attacker_id", body.AttackerID, "payload_len", len(payload),
			"exp_bytes", fmt.Sprintf("% x", expField))
	}
	w.SendEcho(s, hdr, payload)
	// Bystanders get the same frame with THEIR OWN experience in it — that field
	// and no other. CurrentHp and CurrentMp stay the attacker's, because a
	// bystander draws the attacker's health bar from them.
	//
	// The legacy multicasts the buffer verbatim and trusts the client to drop
	// CurrentExp when the ClientTick is not the one it sent. That is no
	// guarantee: the tick is GetTickCount(), so two clients on one machine tick
	// almost identically and the bystander takes the attacker's total as its
	// own. In game that told a level-192 character it had gained 790.358.674
	// experience — exactly the level-313's total minus its own.
	w.ForEachInView(s.Conn, func(vs *world.Session, ve *world.Entity) {
		eco := payload
		if ve != nil {
			eco = protocol.AttackEchoFor(payload, ve.Exp)
		}
		w.SendEcho(vs, hdr, eco)
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
		// Class gate only applies to the four base-class blocks; the shared/Sephira
		// rows (96+) belong to no class and skip it. They are still gated on being
		// learned, but on their OWN bit — see learnedSkillBit, which restores the
		// skillnum-72 mapping the shipped legacy lost to a raised MAX_SKILLINDEX.
		if content.SkillClass(skillnum) <= 3 && content.SkillClass(skillnum) != int(e.Class) {
			return castInfo{}, false
		}
		if e.LearnedSkill&learnedSkillBit(skillnum) == 0 {
			d.log.Info("cast refused: skill not learned",
				"conn", s.Conn, "account", s.AccountName, "skill", skillnum,
				"need_bit", learnedSkillBit(skillnum), "mask", e.LearnedSkill)
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

// sephiraSkillLo / sephiraSkillHi bound the shared skills that are not owned by
// a class: 96-103 map one-to-one onto LearnedSkill bits 24-31 as skillnum-72.
// That is the range the Sephira books grant (Vol 31-38 → bit Vol-7, useSkillBook)
// and where the Kibita unlock puts the Soul (bit 30 ↔ skill 102, Limite da Alma).
const (
	sephiraSkillLo = 96
	sephiraSkillHi = 103
)

// learnedSkillBit is the LearnedSkill bit a cast requires.
//
// DIVERGENCE FROM THE SHIPPED LEGACY, deliberate. _MSG_Attack.cpp:155-190 has two
// gates: skillnum-72 for the 96+ shared skills, and skillnum%24 for class skills.
// The first sits under `if (skillnum >= MAX_SKILLINDEX)`, and MAX_SKILLINDEX was
// raised from 103 to 248 (Basedef.h:200 still carries the old value in a comment),
// so it became unreachable and every skill fell through to %24. That leaves skill
// 102 gated on bit 6 — an ordinary class skill — while the bit the Kibita quest
// actually grants (30) is never read, and the Sephira books grant bits nothing
// checks. The mapping skillnum-72 is not a coincidence: 96..103 → 24..31 is
// exactly the book range, and 102 → 30 is exactly the Soul. The dead branch is
// the intent; this restores it.
//
// The visible consequence: skills 96-101 now need their book, where before any
// character holding the first six class skills could cast them. Skill 97 (the
// mortar) keeps the bit gate rather than the legacy's exemption, because the
// legacy exempts it only to gate on a placed item 746 instead — a check this port
// does not model yet, so dropping the bit check would leave it with no gate.
//
// Indices past 103 keep the %24 fallback: their bit would shift out of an int32.
func learnedSkillBit(skillnum int) int32 {
	if skillnum >= sephiraSkillLo && skillnum <= sephiraSkillHi {
		return int32(1) << uint(skillnum-72)
	}
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

// evocacaoDoMesmoGrupo diz se target é uma evocação do grupo do atacante: a
// criatura do próprio BM, ou a de um companheiro de grupo.
//
// É o recorte, para evocações, da regra do legado
// `if (leader == mobleader || Guild == MobGuild) dam = 0;`
// (_MSG_Attack.cpp:1334). Ali o pet tem como líder o líder do grupo do dono
// (summon.go), então cai no `leader == mobleader`. O port só levou essa regra aos
// afetos (applyCastAffect), não ao dano. A regra inteira, que também protege
// companheiros de grupo e de guilda em PvP, fica de fora de propósito: mudaria o
// PK entre jogadores, e o pedido foi só sobre as evocações.
func evocacaoDoMesmoGrupo(a, b *world.Entity) bool {
	if b.Summoner == 0 || world.IsPlayer(b.ID) {
		return false
	}
	if b.Summoner == a.ID {
		return true
	}
	leader := a.Leader
	if leader == 0 {
		leader = a.ID
	}
	return b.Leader != 0 && b.Leader == leader
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
		if !world.IsPlayer(tid) && target.NonCombatNPC {
			return
		}
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
		// Resist roll (_MSG_Attack.cpp:1189-1195): the affect is RESISTED when the
		// roll exceeds RegenMP + AffectResist + level advantage. Note the polarity
		// — a bigger sum means a lower chance of resisting.
		//
		// Which makes the target's RegenMP read backwards: better mana regen makes
		// you EASIER to debuff. That is what the original computes, so it is what
		// this does; it was worth checking twice before porting, because a "fix"
		// here would silently change every crowd-control fight in the game. Say so
		// before inverting it.
		//
		// Still not modeled: the celestial +MAX_LEVEL adder on the two levels
		// (:1182-1183), which belongs to the tier work.
		if sp.AffectResist >= 1 && sp.AffectResist <= 4 {
			difLevel := -(int(target.Level) - int(e.Level)) / 2
			if w.Rand().Intn(100) > int(target.RegenMP)+sp.AffectResist+difLevel {
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
	applied := target.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, cast.special, d.affectDur)
	if target.SetTick(sp.TickType, sp.TickValue, sp.AffectTime, sp.Aggressive, delay, cast.special, d.affectDur) {
		applied = true
	}
	if !applied {
		return
	}
	// Affect 29 (Limite da Alma, skill 102) multiplies attributes by the
	// character's CONFIGURED Soul — every branch of the legacy reads extra.Soul
	// and none has a default (Basedef.cpp:3050-3110), so with Soul unset the buff
	// installs, ticks its full duration and changes nothing. On screen that is
	// indistinguishable from a skill that did not fire, and the client never says
	// why. The Soul is chosen with the Ehre combine (recipe 8).
	if sp.AffectType == affectSoul && target.Soul == 0 {
		if ts := w.Session(tid); ts != nil {
			sendClientMessage(w, ts, msgSoulNotConfigured)
		}
		d.log.Info("soul buff cast with no soul configured",
			"target", tid, "class_master", target.ClassMaster)
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
		// effectiveDamage above already carries the multiplier, so skill 79 (the only
		// branch reading Damage) must not be handed it twice — SkillBaseDamage applies
		// this one only on the magic branch, which reads Magic and never Damage.
		DamageMultiPct: d.spellDamageMultiPct(e),
		// The client picks its branch from the character's face, and only a
		// Mortal's face has %10 <= 5 — ClassMaster says the same thing without
		// being fooled by a BM transformation swapping the face mid-fight.
		Mortal:       e.ClassMaster == classMasterMortal,
		LearnedSkill: e.LearnedSkill,
	}
	// CurrentWeather scales InstanceType 2/3/5 output (_MSG_Attack.cpp:520,594,972
	// → BASE_GetSkillDamage). Weather 0 is neutral, so this is a no-op until a
	// roll or a GM override moves it (weather.go).
	raw := combat.SkillBaseDamage(skillnum, sp, caster, int(d.currentWeather()), int(d.weaponDamage(e)))

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
			resist[k] = effectiveResist(target, k)
		}
		return combat.SkillResistScale(dmg, sp.InstanceType, resist, world.IsPlayer(tid), int(d.combatRules.MobResistBase))
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
	if !world.IsPlayer(tid) && target.NonCombatNPC {
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
	if !world.IsPlayer(tid) && target.NonCombatNPC {
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
	return d.parryRateWith(attacker, target, int(effectiveDex(attacker)))
}

// skillParryRate is the dodge chance against a SKILL: the attacker's accuracy
// is the larger of DEX and the panel's share of INT (combatrule
// SpellIntAccuracyPct). The legacy reads DEX alone, which left a full-INT caster
// hitting like a DEX-12 character — ~43% of her spells dodged by a TK with 700
// DEX and a mount's evasion.
func (d *Dispatcher) skillParryRate(attacker, target *world.Entity) int {
	acc := int(effectiveDex(attacker))
	if pct := int(d.combatRules.SpellIntAccuracyPct); pct > 0 {
		acc = max(acc, int(effectiveInt(attacker))*pct/100)
	}
	return d.parryRateWith(attacker, target, acc)
}

func (d *Dispatcher) parryRateWith(attacker, target *world.Entity, accuracyDex int) int {
	attackDex := accuracyDex / 5
	if attacker.LearnedSkill&0x1000000 != 0 {
		attackDex += 100
	}
	if attacker.Rsv&world.RsvCast != 0 {
		attackDex += 500
	}
	attackDex += int(attacker.AffAccuracy)
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
	// The player/summon quarter used to live here, which made it run only for an
	// attacker carrying forced damage. It is perfuracao now, applied to every
	// blow before this; forced damage is added to what is left, as in the legacy.
	if dmg <= 1 {
		return int(attacker.AffForceDamage)
	}
	return dmg + int(attacker.AffForceDamage)
}

// applyAirBladeProc returns the blow with the proc added, and the proc alone —
// the PvP quarter divides the blow but leaves the proc whole (perfuracao).
func (d *Dispatcher) applyAirBladeProc(w *world.World, attacker, target *world.Entity, msgType protocol.Type, body *protocol.MsgAttackBody, payload []byte, dmg int) (int, int) {
	if dmg <= 0 || attacker == nil || target == nil || msgType != protocol.MsgAttackTwo ||
		attacker.Class != 3 || attacker.LearnedSkill&(1<<21) == 0 || w.Rand().Intn(4) != 0 {
		return dmg, 0
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
	return dmg + skillDam, skillDam
}

func (d *Dispatcher) applyOnHitAffects(w *world.World, attacker, target *world.Entity, tid int) {
	if attacker == nil || target == nil {
		return
	}
	if attacker.Rsv&world.RsvFrost != 0 && w.Rand().Intn(2) == 0 {
		d.applyOnHitSpell(w, target, tid, 36, effectiveSpecial(attacker, 1)+150, effectiveSpecial(attacker, 1), false)
	}
	if attacker.Rsv&world.RsvDrain != 0 && w.Rand().Intn(2) == 0 {
		d.applyOnHitSpell(w, target, tid, 40, effectiveSpecial(attacker, 1)+150, effectiveSpecial(attacker, 1), false)
	}
}

// aceitaMob abre a instalação do afeto em MONSTRO, e só o caminho do pet passa
// true — ver SetAffectOnMob. Os procs de item de jogador continuam com a regra do
// legado, que recusa alvo acima de MAX_USER.
func (d *Dispatcher) applyOnHitSpell(w *world.World, target *world.Entity, tid, skillnum, delay, level int, aceitaMob bool) {
	sp, ok := onHitSpell(d.spells, skillnum)
	if !ok {
		return
	}
	var applied bool
	if aceitaMob && !world.IsPlayer(target.ID) {
		applied = target.SetAffectOnMob(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, level, d.affectDur)
	} else {
		applied = target.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, level, d.affectDur)
	}
	if target.SetTick(sp.TickType, sp.TickValue, sp.AffectTime, sp.Aggressive, delay, level, d.affectDur) {
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

// writeAttackerStatus overwrites the attacker's own status in the attack body
// with the server's authoritative values: CurrentHp@4, CurrentExp@12,
// CurrentMp@40, ReqMp@46 — ONE layout for all three attack messages.
//
// Basedef.h declares MSG_AttackOne/Two with HP and MP swapped (2452-2514), and
// an earlier version of this function followed it. Nothing else does. The
// legacy handles every type through `MSG_Attack *m` (_MSG_Attack.cpp:23) and
// writes m->CurrentMp and m->CurrentHp at the MSG_Attack offsets (:254, :1744),
// and the client reads the attacker's mana at body+40 for all three types and
// builds its own frames the same way (WYD.exe: the attack handler at 0x48b2be
// serves 0x367/0x39D/0x39E alike; the Mp write is at 0x48b7a2). Swapping the
// fields told the client, on every single-target skill echo, that its mana was
// its HP.
//
// These fixed fields sit below the Dam[] region (offset 48), so they never
// collide with per-target damage.
func writeAttackerStatus(payload []byte, hp, mp int32, exp int64, reqMp int16) {
	if len(payload) < protocol.MsgAttackDamOffset {
		return
	}
	binary.LittleEndian.PutUint32(payload[4:8], uint32(hp))
	binary.LittleEndian.PutUint64(payload[12:20], uint64(exp))
	binary.LittleEndian.PutUint32(payload[40:44], uint32(mp))
	binary.LittleEndian.PutUint16(payload[46:48], uint16(reqMp))
}
