package handler

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// dropExpired clears any timed item whose expiry has passed (now = Unix seconds).
func dropExpired(items []world.Item, now int64) {
	for i := range items {
		if items[i].ExpiresAt != 0 && now >= items[i].ExpiresAt {
			items[i] = world.Item{}
		}
	}
}

// countStacksMissingAmount reports how many stackables are stored without an
// explicit EF_AMOUNT — the shape that crashes the client on arrival.
//
// The server reads a missing EF_AMOUNT as "one" (itemAmount), so such an item
// works perfectly on this side and is invisible to every server-side check. The
// CLIENT does not agree: a Caixa da Sabedoria in a bag with no amount effect —
// "3:4117" beside "0:401(61/117)" in the login log — killed the client the
// moment the login blob arrived, and kept killing it on every retry, leaving the
// character unplayable.
//
// Counting rather than repairing is deliberate. The legacy never mints one
// (BASE_SetItemAmount runs wherever a stackable is created), so an item in this
// state means some path here skipped it, and the log is how that path gets
// found. Repairing in place would hide it — and worse, setItemAmount claims the
// first free effect slot while the combine recipes match on effect position, so
// the repair could silently change what an item combines into.
func countStacksMissingAmount(items []world.Item) int {
	n := 0
	for _, it := range items {
		if it.Index != 0 && isSplittable(it.Index) && !hasAmountEffect(it) {
			n++
		}
	}
	return n
}

// hasAmountEffect reports whether the item carries an explicit EF_AMOUNT slot.
// itemAmount cannot answer this: it returns 1 both for "one of them" and for
// "nobody ever wrote an amount", and the difference is what breaks the client.
func hasAmountEffect(it world.Item) bool {
	for _, ef := range it.Effects {
		if ef.Effect == efAmount {
			return true
		}
	}
	return false
}

// createCharacter handles _MSG_CreateCharacter (0x020F),
// handlers/_MSG_CreateCharacter.md. Requires USER_SELCHAR and a valid name, then
// relays creation to the dbServer.
func (d *Dispatcher) createCharacter(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgCreateCharacterBody
	if err := body.Decode(payload); err != nil {
		d.log.Warn("create char: decode failed", "conn", s.Conn, "len", len(payload), "err", err)
		w.Send(s, protocol.MsgNewCharacterFail, nil)
		return
	}
	name := cstr(body.MobName[:])
	d.log.Info("create char", "conn", s.Conn, "slot", body.Slot, "class", body.MobClass,
		"name", name, "mode", s.Mode, "want_mode", world.UserSelChar)
	if s.Mode != world.UserSelChar {
		w.Send(s, protocol.MsgNewCharacterFail, nil)
		return
	}
	if !validCharName(name) {
		d.log.Warn("create char: invalid name", "conn", s.Conn, "name", name)
		w.Send(s, protocol.MsgNewCharacterFail, nil)
		return
	}

	slot := int(body.Slot)
	class := int(body.MobClass)
	accID := s.AccountID
	s.Mode = world.UserWaitDB

	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		ok, err := p.CreateCharacter(context.Background(), accID, slot, name, class)
		if err != nil || !ok {
			return func(w *world.World, s *world.Session) {
				s.Mode = world.UserSelChar
				d.log.Warn("create char: dbServer rejected", "conn", s.Conn, "ok", ok, "err", err)
				w.Send(s, protocol.MsgNewCharacterFail, nil)
			}
		}
		// Success: re-fetch the list and resend the full SELCHAR (the original
		// replies MSG_CNFNewCharacter with the whole selection, now with the new char).
		chars, lerr := p.ListCharacters(context.Background(), accID)
		return func(w *world.World, s *world.Session) {
			s.Mode = world.UserSelChar
			if lerr != nil {
				d.log.Warn("create char: list after create failed", "conn", s.Conn, "err", lerr)
				w.Send(s, protocol.MsgNewCharacterFail, nil)
				return
			}
			d.log.Info("create char: OK", "conn", s.Conn, "name", name, "slot", slot, "total", len(chars))
			body := protocol.EncodeCNFNewCharacterBody(d.selCharsFrom(chars))
			w.SendTo(s, protocol.Header{Type: protocol.MsgCNFNewCharacter, ID: protocol.IDNewCharacter}, body)
		}
	})
}

// deleteCharacter handles _MSG_DeleteCharacter (0x0211),
// handlers/lote2-sessao-conta.md. Requires USER_SELCHAR; password is verified by
// the dbServer.
func (d *Dispatcher) deleteCharacter(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgDeleteCharacterBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if s.Mode != world.UserSelChar {
		d.notify(w, s, NoticeDeletingWait)
		return
	}
	slot := int(body.Slot)
	name := cstr(body.MobName[:])
	pass := cstr(body.Password[:])
	accID := s.AccountID
	s.Mode = world.UserWaitDB

	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		ok, err := p.DeleteCharacter(context.Background(), accID, slot, name, pass)
		if err != nil || !ok {
			return func(w *world.World, s *world.Session) {
				s.Mode = world.UserSelChar
				w.Send(s, protocol.MsgNewCharacterFail, nil)
			}
		}
		// Success: re-fetch the list and resend the full SELCHAR, same as
		// createCharacter. MSG_CNFDeleteCharacter (Basedef.h:1646-1652) carries the
		// identical `STRUCT_SELCHAR sel` body as MSG_CNFNewCharacter — without
		// resending the refreshed list, the client keeps its stale pre-delete slot
		// array, which desyncs the remaining characters on screen until relogin.
		chars, lerr := p.ListCharacters(context.Background(), accID)
		return func(w *world.World, s *world.Session) {
			s.Mode = world.UserSelChar
			if lerr != nil {
				d.log.Warn("delete char: list after delete failed", "conn", s.Conn, "err", lerr)
				w.Send(s, protocol.MsgNewCharacterFail, nil)
				return
			}
			respBody := protocol.EncodeCNFNewCharacterBody(d.selCharsFrom(chars))
			w.SendTo(s, protocol.Header{Type: protocol.MsgCNFDeleteCharacter, ID: protocol.IDNewCharacter}, respBody)
		}
	})
}

// characterLogin handles _MSG_CharacterLogin (0x0213),
// handlers/_MSG_CharacterLogin.md. Requires USER_SELCHAR and a valid slot; loads
// the character from the dbServer and injects the player into the world.
//
// The billing gate is the NEW boundary (binServer over gRPC, Fase 6): before
// loading the character we ask World.Billing whether the account may enter. The
// legacy hardcoded free-exp gate (Unk_*, BILLING, FREEEXP, g_Hour) is NOT
// reproduced — it is UNVERIFIED. The default gate (AllowAllBilling) is
// free-to-play, so this is non-breaking until a binServer is wired.
func (d *Dispatcher) characterLogin(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgCharacterLoginBody
	if err := body.Decode(payload); err != nil {
		return
	}
	slot := int(body.Slot)
	if slot < 0 || slot >= world.MobPerAccount {
		d.notify(w, s, NoticeSelectCharacter)
		return
	}
	if s.Mode != world.UserSelChar {
		d.notify(w, s, NoticeSelectCharacter)
		return
	}
	d.log.Info("character login request", "conn", s.Conn, "slot", slot, "mode", s.Mode)
	s.Slot = slot
	s.Mode = world.UserCharWait
	accID := s.AccountID
	accName := s.AccountName

	p := w.Persistence()
	b := w.Billing()
	// Both calls are blocking I/O; run them sequentially off the loop. The result
	// re-enters the loop via the returned callback.
	w.Go(s, func() func(*world.World, *world.Session) {
		allowed, berr := b.Check(context.Background(), accName)
		if berr != nil {
			return func(w *world.World, s *world.Session) { d.billingFailed(w, s, berr) }
		}
		if !allowed {
			return func(w *world.World, s *world.Session) { d.billingDenied(w, s) }
		}
		st, err := p.LoadCharacter(context.Background(), accID, slot)
		return func(w *world.World, s *world.Session) { d.completeCharacterLogin(w, s, st, err) }
	})
}

// billingDenied returns the player to character selection after the binServer
// refuses entry (expired/blocked). UNVERIFIED: the real S→C deny message is not
// captured; a notice placeholder stands in (parity-tests.md §5).
func (d *Dispatcher) billingDenied(w *world.World, s *world.Session) {
	s.Mode = world.UserSelChar
	d.notify(w, s, NoticeBillingDenied)
}

// billingFailed handles a billing-service error (treated as "deny, try again").
func (d *Dispatcher) billingFailed(w *world.World, s *world.Session, err error) {
	d.log.Error("billing check failed", "conn", s.Conn, "account", s.AccountName, "err", err)
	s.Mode = world.UserSelChar
	d.notify(w, s, NoticeDBError)
}

func (d *Dispatcher) completeCharacterLogin(w *world.World, s *world.Session, st world.CharacterState, err error) {
	if err != nil {
		d.log.Error("load character failed", "conn", s.Conn, "slot", s.Slot, "err", err)
		s.Mode = world.UserSelChar
		w.Send(s, protocol.MsgCharacterLoginFail, nil)
		return
	}
	// Drop any timed items (e.g. an expired 30-day Perzen mount) before injecting
	// the character — the expiry is enforced here on load.
	now := time.Now().Unix()
	dropExpired(st.Equip[:], now)
	dropExpired(st.Carry[:], now)
	// A stackable with no EF_AMOUNT crashes the client, but it is NOT repaired
	// here: setItemAmount claims the first free effect slot, and the combine
	// recipes match on effect POSITION, so rewriting stored items to please the
	// client would quietly change what they combine into. The amount is added at
	// the serialization boundary instead (itemToSel), where it reaches the client
	// without touching the item the server reasons about.
	if n := countStacksMissingAmount(st.Carry[:]) + countStacksMissingAmount(st.Equip[:]); n > 0 {
		d.log.Warn("stackable items stored with no EF_AMOUNT (sent as 1)",
			"conn", s.Conn, "account", s.AccountName, "slot", s.Slot, "items", n)
	}
	// Seed the starter gear for characters that have none yet (newly created, or
	// created before seeding existed). This restores the class look (the body item
	// in equip slot 0 is what gives a TK/FM/BM/HT its appearance) and hands out the
	// class armor/weapon. It persists on the next save. An empty equip is the
	// "fresh character" marker; the Arch twin is created with its body item, so it
	// never takes this path.
	//
	// The body items and nothing else: no potions, Esfera da Sorte or Baú de
	// Experiência from the template's Carry, and no gold (dbserver). The legacy
	// copied the whole BaseMob template, bag included (CFileDB.cpp:983-993); the
	// team decided on 2026-09-11 that a new Mortal starts bare, in the training
	// field (novo, below).
	novo := equipEmpty(st.Equip)
	if novo {
		st.Equip = d.starterEquip(st.Class)
	}
	// Heal characters whose equip slots were corrupted by an earlier bug that let
	// non-gear (a potion, a mount in the wrong hand) be equipped — most visibly a
	// consumable in the body slot, which makes the body invisible/wrong. Mis-slotted
	// items go back to the inventory and the class body item is restored.
	d.repairEquip(&st)
	// A character must never enter the world dead. Now that mobs can kill players
	// (mobai.go), one that was slain and then disconnected without restarting is
	// persisted at HP 0 — reviving it on login (full HP/MP) puts it back in play in
	// its city instead of logging in stuck/dead (passive regen excludes HP 0).
	if st.HP <= 0 {
		st.HP = st.MaxHP
		st.MP = st.MaxMP
		if st.HP <= 0 {
			st.HP = 1 // guard a broken/zero MaxHP so the player can still act
		}
	}
	// E a mana nunca entra negativa. O HP acima já tinha guarda; o MP não, e o
	// pacote de login (Mp: st.MP, logo abaixo) manda o valor CRU do banco. Um MP
	// negativo gravado — sobra do período em que debuff de monstro drenava a
	// mana antes de existir piso (cfb4131f) — chegava ao cliente em todo login, e
	// o cliente animava a barra devagar a partir dele: -19929 logo na tela de
	// boas-vindas, subindo tique a tique. O servidor já não produz MP negativo em
	// jogo (toda escrita tem guarda ou piso); o que faltava era não CONFIAR no que
	// foi gravado antes disso. O primeiro save depois deste login já grava o valor
	// saneado.
	if st.MP < 0 {
		st.MP = 0
	}
	// Login position follows the legacy split: STRUCT_MOB.SPX/SPY is the Gema
	// Estelar warp save-point (st.SaveX/SaveY, rehydrated onto the entity below),
	// while MSG_CNFCharacterLogin.PosX/PosY is the actual world-entry tile
	// (st.X/Y). An explicit loaded position (tests/captures) is honored when
	// present; live DB loads currently fall back to the last-city spawn rule.
	saveX, saveY := st.SaveX, st.SaveY
	loginX, loginY := st.X, st.Y
	if loginX == 0 && loginY == 0 {
		// Only a character that was just created is born in the training field:
		// never seeded AND still at level 0 with no experience, which is how the
		// dbserver creates one. An older character that merely has no gear keeps
		// entering in its city.
		loginX, loginY = pontoDeEntrada(st.LastCity, novo && st.Level == 0 && st.Exp == 0)
		if x, y, ok := w.EmptyCellNear(loginX, loginY); ok {
			loginX, loginY = x, y
		}
	}
	s.LoginSpawnX, s.LoginSpawnY = loginX, loginY
	s.LoginTick = w.Now()
	s.LoggedFirstAction = false
	// Inject the player entity into the world (the slot was docked at connect).
	if e := w.Entity(s.Conn); e != nil {
		e.Mode = world.MobUser
		// The entity is per-connection: wipe any affect left by a previously
		// played character before rehydrating this one's persisted slots (the
		// rehydrate below only ADDS slots, which caused the issue #21/#47
		// cross-character leaks).
		e.ResetAffects()
		e.Name = st.Name
		e.Class = uint8(st.Class)
		e.LastCity = st.LastCity
		e.SaveX, e.SaveY = st.SaveX, st.SaveY
		// Register the player in the spatial grid, not just the entity fields —
		// mob aggro (FindEnemyFromView) and the view reconciliation scan the
		// grid, so a player standing still since login must be there.
		w.SetEntityPos(s.Conn, loginX, loginY)
		e.HP, e.MaxHP = st.HP, st.MaxHP
		e.MP, e.MaxMP = st.MP, st.MaxMP
		// Critical is a save-side cache only: refreshScore below re-derives it from the
		// equipment (Basedef.cpp:3209), so the stored value never survives login.
		// Damage and AC are NOT read from the state at all: the DB contract carries
		// neither, so st.Damage/st.AC are always 0 — deriveBaseScore reconstructs
		// their base from the legacy rules and refreshScore fills the live fields
		// (issue #232).
		e.Master, e.Critical = st.Master, st.Critical
		e.Level, e.Coin, e.Exp = int32(st.Level), st.Coin, st.Exp
		e.Clan, e.Guild, e.GuildLevel, e.Citizen, e.ClassMaster, e.Soul = st.Clan, st.GuildID, st.GuildLevel, st.Citizen, st.ClassMaster, st.Soul
		e.Fame = st.Fame
		// Older rows created before ClassMaster was persisted may still carry 0.
		// Treat that as MORTAL (=2, Basedef.h:238) so EXP does not route through
		// the celestial divisor path (issue #43).
		if e.ClassMaster == 0 {
			e.ClassMaster = classMasterMortal
		}
		// PK/karma state (issue #210). PKPoint == 0 means "never persisted"
		// (SetPKPoint never legitimately writes 0; the DB column defaults to 75)
		// — pre-migration rows and zero-valued test fixtures both read back 0, so
		// treat that as neutral, the same convention as ClassMaster == 0 above.
		e.PKPoint, e.Guilty, e.CurKill, e.TotKill = st.PKPoint, st.Guilty, st.CurKill, st.TotKill
		if e.PKPoint == 0 {
			e.PKPoint = pkPointNeutral
		}
		// Celestial quest gates (set by /destravar40/90 and /arcana; CheckGetLevel
		// reads Lv40/Lv90 to unlock the 40/90 caps).
		e.CelLv40, e.CelLv90, e.CelCircle = st.CelLv40, st.CelLv90, st.CelCircle
		e.ArchLv355, e.ArchLv370, e.ArchCristal = st.ArchLv355, st.ArchLv370, st.ArchCristal
		e.MortalLevel, e.CelestialArchLevel = st.MortalLevel, st.CelestialArchLevel
		e.NightmareTickets = st.NightmareTickets
		e.TerraMistica = st.TerraMistica
		e.NewbieQuest = st.NewbieQuest
		e.Str, e.Int, e.Dex, e.Con, e.ScoreBonus = st.Str, st.Int, st.Dex, st.Con, st.ScoreBonus
		// Skill state: the learned mask, allocated mastery and the hotbar come
		// straight from the DB; SkillBonus is re-derived from level + learned
		// costs (BASE_GetBonusSkillPoint on character load, ProcessDBMessage.cpp:816).
		e.LearnedSkill, e.SecLearnedSkill, e.SpecialBonus = st.LearnedSkill, st.SecLearnedSkill, st.SpecialBonus
		e.BaseSpecial, e.SkillBar = st.BaseSpecial, st.SkillBar
		s.ShortSkill = st.ShortSkill
		d.deriveSkillBonus(e)
		e.Equip = st.Equip
		e.Carry = st.Carry
		// Capture the equipment-free BaseScore from the loaded CurrentScore, so later
		// equip/unequip recomputes (refreshScore) reflect gear changes without double-
		// counting the gear already baked into the stored CurrentScore.
		d.deriveBaseScore(e)
		// A Celestial's pools are rebuilt from the formula on every load, as the
		// legacy does for everyone (BASE_GetHpMp, ProcessDBMessage.cpp:810). That
		// is what repairs a Celestial born before the +MAX_LEVEL share was ported:
		// its stored pools were a level-1 character's. Mortal and Arch keep the
		// derived base — this port stores permanent grants (the Arch crystals)
		// straight in it, and the legacy recompute would erase them.
		celestialPools(e)
		// ScoreBonus is re-derived here for the same reason SkillBonus is, and from
		// the same place in the legacy: ProcessDBMessage.cpp:816-817 calls
		// BASE_GetBonusSkillPoint AND BASE_GetBonusScorePoint side by side when a
		// character loads. Only the skill half was ported, so the stored grant was
		// simply trusted — a character whose points were computed by an older or
		// wrong formula kept the wrong number forever, with no way to repair it
		// short of gaining a level. It runs after deriveBaseScore because it reads
		// the equipment-free attributes that call establishes.
		e.ScoreBonus = uint16(level.ScoreBonus(scoreBonusInput(e)))
		// Re-apply a still-active Divine buff from the persisted deadline (the buff is
		// read-time, so this doesn't affect the base just derived). Expired → dropped.
		if st.DivineEnd > time.Now().Unix() {
			if slot := e.EmptyAffect(world.AffectDivine); slot >= 0 {
				e.DivineEnd = st.DivineEnd
				e.Affect[slot] = world.Affect{Type: world.AffectDivine, Level: 1, Time: divineAffectTime}
			}
		}
		// Rehydrate the other persisted buff slots (Time in 8s affect ticks; the
		// tick sweep resumes counting them down).
		for _, a := range st.Affects {
			if a.Type == 0 || a.Type == world.AffectDivine || a.Time == 0 {
				continue
			}
			if slot := e.EmptyAffect(a.Type); slot >= 0 {
				e.Affect[slot] = a
			}
		}
		// Recompute the live score once (idempotent right after deriveBaseScore:
		// current = base + equip reproduces the loaded values) — this is what fills
		// the live Special (= BaseSpecial + gear) and the affect caches, which are
		// not persisted.
		d.refreshScore(e)
		s.ReqHp, s.ReqMp = e.HP, e.MP
		s.CriticalProgress = 0
		// Visual gear codes from the character's REAL equipment, so others (and the
		// own client, via UpdateEquip) see what is actually equipped — not the class
		// starter set. Empty slots → 0 (no item). AFTER the affect rehydrate +
		// refreshScore, so a persisted transform (affect 16) renders its beast mesh.
		e.EquipVisual, e.EquipAnct = equipVisual(e)
	}
	s.Mode = world.UserPlay
	// From here the loop owns this character's inventory, and an edit written to
	// the database underneath it would be lost at the next save. The mark is what
	// lets the staff panel refuse instead of pretending; it is bookkeeping, so it
	// goes off the loop and a failure never touches the login.
	if e := w.Entity(s.Conn); e != nil {
		d.markPresence(w, e.Name, true)
	}
	// The persisted skill block rides the login snapshot (mask/points/bar/Special);
	// the live Special (base + equip) is on the Entity after refreshScore-on-login
	// hasn't run yet, so send base+equip via the entity when available.
	var skill protocol.SkillState
	loginPKPoint := pkPointNeutral // MobName[12] for the own nick: 75 = white (fresh char is never guilty)
	if e := w.Entity(s.Conn); e != nil {
		skill = protocol.SkillState{
			LearnedSkill: e.LearnedSkill,
			ScoreBonus:   e.ScoreBonus, SpecialBonus: e.SpecialBonus, SkillBonus: e.SkillBonus,
			Special: e.Special, BaseSpecial: e.BaseSpecial, SkillBar: e.SkillBar,
		}
		loginPKPoint = pkPoint(e)
	}
	shortSkill := s.ShortSkill
	// Prefer the per-class BaseMob template (real STRUCT_MOB with starter equipment
	// → correct class model and no client crash); patch name + position.
	if tmpl, ok := d.baseMobs[st.Class]; ok && len(tmpl) == content.BaseMobSize {
		var equip [16]protocol.SelItem
		for i := range st.Equip {
			equip[i] = itemToSel(st.Equip[i])
		}
		// Um BM que volta com a transformação ainda ativa nasce com o corpo da fera
		// para ELE MESMO. O próprio cliente desenha o personagem pelo Equip[0]
		// deste pacote, e o legado reescreve MOB.Equip[0].sIndex com a malha dentro
		// do GetCurrentScore (Basedef.cpp:4106) antes de mandar o login. Só o
		// índice muda, como lá; o item gravado continua intacto no banco.
		if e := w.Entity(s.Conn); e != nil {
			if value, _, ok := activeTransform(e); ok {
				equip[0].Index = transMesh(value)
			}
		}
		var carry [64]protocol.SelItem
		for i := range st.Carry {
			if i >= 64 {
				break
			}
			carry[i] = itemToSel(st.Carry[i])
		}
		// Weather rides the login snapshot rather than a separate packet, exactly
		// as the legacy does (sm.Weather = CurrentWeather, ProcessDBMessage.cpp:834).
		body := protocol.EncodeCNFCharacterLoginRaw(tmpl, st.Name, st.Coin, st.Exp, equip, carry, loginX, loginY, saveX, saveY, s.Slot, s.Conn, uint16(d.currentWeather()), shortSkill, skill, loginPKPoint)
		d.logCNFCharacterLogin("template", s, st, loginX, loginY, body)
		w.SendTo(s, protocol.Header{Type: protocol.MsgCNFCharacterLogin, ID: protocol.IDScene}, body)
		d.enterWorldView(w, s)
		// Before the pet and the buff snapshot: those are gameplay state the client
		// applies to the scene, and the greeting is a dialog the player dismisses.
		// Sending it here keeps the world-entry frames contiguous.
		d.sendWelcome(w, s)
		if e := w.Entity(s.Conn); e != nil {
			d.refreshBabyMountSummon(w, s, e)
		}
		d.sendLoginAffects(w, s)
		return
	}
	d.log.Info("char login: sending CNFCharacterLogin (fallback, no template)",
		"conn", s.Conn, "class", st.Class)
	// Fallback: build the snapshot from the stored relational state (no equipment).
	// Byte-exact MSG_CNFCharacterLogin (STRUCT_MOB + pos + skillbar), ID=30000.
	if saveX == 0 && saveY == 0 {
		saveX, saveY = loginX, loginY
	}
	m := protocol.MobSnapshot{
		Name:  st.Name,
		Clan:  st.Clan,
		Guild: st.GuildID,
		Class: uint8(st.Class),
		Coin:  st.Coin,
		Exp:   st.Exp,
		SPX:   saveX, SPY: saveY,
		Level: int32(st.Level), Ac: st.AC, Damage: st.Damage,
		MaxHp: st.MaxHP, MaxMp: st.MaxMP, Hp: st.HP, Mp: st.MP,
		Str: st.Str, Int: st.Int, Dex: st.Dex, Con: st.Con,
		AttackRun:  baseAttackRun,
		PKPoint:    loginPKPoint, // MobName[12]: 75 = white nick (issue #59)
		ScoreBonus: st.ScoreBonus, GuildLevel: st.GuildLevel,
		LearnedSkill: skill.LearnedSkill, SpecialBonus: skill.SpecialBonus,
		SkillBonus: skill.SkillBonus, Special: skill.Special, SkillBar: skill.SkillBar,
	}
	for i := range st.Carry {
		if i >= len(m.Carry) {
			break
		}
		// itemToSel, not a hand-rolled copy: the login inventory is the FIRST view a
		// player gets of a timed item, and open-coding the conversion here left it
		// the one path that skipped the remaining-life effects.
		m.Carry[i] = itemToSel(st.Carry[i])
	}
	body := protocol.EncodeCNFCharacterLoginBody(s.Slot, s.Conn, uint16(d.currentWeather()), loginX, loginY, m, shortSkill)
	d.logCNFCharacterLogin("fallback", s, st, loginX, loginY, body)
	w.SendTo(s, protocol.Header{Type: protocol.MsgCNFCharacterLogin, ID: protocol.IDScene}, body)
	d.enterWorldView(w, s)
	d.sendWelcome(w, s)
	if e := w.Entity(s.Conn); e != nil {
		d.refreshBabyMountSummon(w, s, e)
	}
	d.sendLoginAffects(w, s)
}

func (d *Dispatcher) logCNFCharacterLogin(path string, s *world.Session, st world.CharacterState, spawnX, spawnY int16, body []byte) {
	var msgX, msgY, mobSPX, mobSPY uint16
	if len(body) >= 4+44 {
		msgX = binary.LittleEndian.Uint16(body[0:])
		msgY = binary.LittleEndian.Uint16(body[2:])
		mobSPX = binary.LittleEndian.Uint16(body[4+40:])
		mobSPY = binary.LittleEndian.Uint16(body[4+42:])
	}
	d.log.Info("char login: sending CNFCharacterLogin",
		"path", path,
		"conn", s.Conn,
		"slot", s.Slot,
		"client_id", s.Conn,
		"class", st.Class,
		"name", st.Name,
		"last_city", st.LastCity,
		"spawn_x", spawnX,
		"spawn_y", spawnY,
		"cnf_pos_x", int16(msgX),
		"cnf_pos_y", int16(msgY),
		"mob_spx", int16(mobSPX),
		"mob_spy", int16(mobSPY),
		"body", len(body),
		"carry", carrySummary(st.Carry[:]),
		"equip", carrySummary(st.Equip[:]))
}

// carrySummary renders the occupied slots as "slot:index(eff/val,…)" for the
// login log.
//
// The login blob is the first and largest thing the client parses, and a client
// that dies right after receiving it leaves no other evidence of WHAT it choked
// on: the server only sees an EOF. An item carrying an effect that does not
// belong to it — a stackable whose amount slot was overwritten, a quest reward
// that picked up an expiry — is invisible without this.
func carrySummary(items []world.Item) string {
	var b strings.Builder
	for i, it := range items {
		if it.Index == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d:%d", i, it.Index)
		for _, ef := range it.Effects {
			if ef.Effect != 0 {
				fmt.Fprintf(&b, "(%d/%d)", ef.Effect, ef.Value)
			}
		}
		if it.ExpiresAt != 0 {
			fmt.Fprintf(&b, "[exp:%d]", it.ExpiresAt)
		}
	}
	return b.String()
}

// sendLoginAffects pushes the rehydrated buff snapshot right after the world
// entry, so persisted buffs show their icons without waiting for a cast/score
// event (the CNFCharacterLogin blob carries no affect array).
func (d *Dispatcher) sendLoginAffects(w *world.World, s *world.Session) {
	if e := w.Entity(s.Conn); e != nil && e.HasAnyAffect() {
		d.sendAffect(w, s, e)
	}
}

// enterWorldView wires entity visibility after a player enters the world
// (ProcessDBMessage.cpp:1021): broadcast the newcomer's MSG_CreateMob to every
// in-view player (CreateType=2), and send each in-view player's MSG_CreateMob to
// the newcomer. Without this the client invents a duplicate avatar from every
// _MSG_Action of an unknown entity (B1). HEADER.ID is always IDScene (30000); the
// entity id travels in MobID.
func (d *Dispatcher) enterWorldView(w *world.World, s *world.Session) {
	self := w.Entity(s.Conn)
	if self == nil {
		return
	}
	w.ClearSeen(s) // fresh view set on (re)entering the world
	// Unicast, NOT the usual multicast sendScore: this runs before the newcomer's
	// CreateMob is broadcast below, so in-view clients would get an UpdateScore for
	// an entity they have not created yet (the B1 confusion). The legacy has no
	// SendScore in this path at all (ProcessDBMessage.cpp:1017-1037) — observers
	// learn the newcomer's HP from CreateMob.
	d.sendScoreSelf(w, s, self) // CurrentScore (attributes after equipment + active buffs)
	// E a confirmação de HP/MP, que o UpdateScore acima NÃO substitui.
	//
	// O cliente desconta a mana LOCALMENTE ao lançar e só reconcilia a barra com
	// um MSG_SetHpMp. Sem esta linha, um cliente que chegou com a conta desandada
	// de uma sessão anterior continuava errado depois de relogar — e uma barra
	// suficientemente negativa faz ELE recusar as magias sozinho (\"Mana
	// insuficiente\"), sem mandar nada que nos desse a chance de corrigir. Entrar
	// no mundo passa a ser o ponto de reconciliação garantido.
	//
	// Vai aqui, dentro da rajada do login, e não num batimento periódico: o
	// servidor é silencioso quando nada acontece, e várias partes do jogo contam
	// com isso.
	d.sendSetHpMp(w, s, self)
	if self.HasAnyAffect() {
		d.sendAffect(w, s, self) // buff icons/timers (e.g. a re-applied Divine)
	}
	selfMob := protocol.EncodeCreateMobBody(createMobFrom(self, 2))
	// Send the newcomer its OWN CreateMob (the legacy GridMulticast has skip=0, so
	// the conn — already in the grid — receives its own, ProcessDBMessage.cpp:1029).
	// This is what colors the player's OWN nick via MobName[12] (PKPoint): without
	// it the own nick renders forever from the login blob and can't recolor. PKInfo
	// (attackable flag) rides along for parity (SendPKInfo, SendFunc.cpp:1869).
	w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, selfMob)
	w.SendTo(s, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, protocol.EncodeStandardParm(pkInfoParm(self)))
	w.ForEachInView(s.Conn, func(vs *world.Session, ve *world.Entity) {
		// (A) other players see the newcomer
		w.MarkSeen(vs, s.Conn)
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, selfMob)
		w.SendTo(vs, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, protocol.EncodeStandardParm(pkInfoParm(self)))
		// (B) the newcomer sees each player already in view
		w.MarkSeen(s, ve.ID)
		ty, body := createMobViewPacket(w, ve, 0)
		w.SendTo(s, protocol.Header{Type: ty, ID: protocol.IDScene}, body)
		w.SendTo(s, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(ve.ID)}, protocol.EncodeStandardParm(pkInfoParm(ve)))
	})
	// (C) the newcomer sees the NPCs/monsters in view.
	d.revealMobsInView(w, s)
	// (D) and the Castelo Orc gate, when it stands in view.
	d.syncCasteloOrcGate(w, s, self.X, self.Y)
	// (E) e os três portões do campo de treino, pelo mesmo caminho.
	d.syncPortoesDoCampo(w, s, self.X, self.Y)
	d.syncCasteloOrcLeste(w, s, self.X, self.Y)
}

// revealMobsInView sends a MSG_CreateMob for every NPC/monster now in the player's
// view that the client hasn't seen yet (once per entity). Called on entry and on
// each move, so NPCs appear as the player explores.
func (d *Dispatcher) revealMobsInView(w *world.World, s *world.Session) {
	w.ForEachMobInView(s.Conn, func(me *world.Entity) {
		if w.MarkSeen(s, me.ID) {
			// Through createMobViewPacket, not EncodeCreateMobBody: a personal-shop
			// clone is a mob, and a mob revealed with the plain packet reaches the
			// client with an empty title — which is the exact byte the client gates
			// the shop on, so the stall would be there and refuse to open for
			// anyone who walked up after it was raised.
			typ, body := createMobViewPacket(w, me, 0)
			w.SendTo(s, protocol.Header{Type: typ, ID: protocol.IDScene}, body)
		}
	})
}

// createMobFrom builds MSG_CreateMob data from a world entity (player or NPC).
// The visual equipment codes and glow overlays come from the entity's
// EquipVisual/EquipAnct, set at login/spawn from the relevant STRUCT_MOB data.
// createType: 0 normal, 2 "just entered".
func createMobFrom(e *world.Entity, createType uint16) protocol.CreateMobData {
	d := protocol.CreateMobData{
		MobID:           e.ID,
		Name:            e.Name,
		PosX:            e.X,
		PosY:            e.Y,
		Guild:           e.Guild,
		GuildMemberType: e.GuildLevel,
		Level:           e.Level,
		Ac:              e.AC,
		Damage:          e.Damage,
		// HP/MP must match the authoritative score (effective max incl. HP/MP% gear):
		// the self-CreateMob now drives the player's own bars, so a missing MaxMp/Mp
		// zeroed the client's MP (regression from adding the login self-CreateMob).
		MaxHp: effectiveMaxHP(e), Hp: e.HP,
		MaxMp: effectiveMaxMP(e), Mp: e.MP,
		Str: e.Str, Int: e.Int, Dex: e.Dex, Con: e.Con,
		Merchant:   merchantParaOCliente(e),
		AttackRun:  attackRunOf(e),
		Equip:      e.EquipVisual,
		AnctCode:   e.EquipAnct,
		CreateType: createType,
		// Players pack PKPoint/CurKill/TotKill into MobName[12..15] to color the nick
		// (75 neutral/white, 0 chaos/red) and show the kill-streak bytes (issue #210);
		// mobs send a raw name with no PK coloring.
		IsPlayer: world.IsPlayer(e.ID),
		PKPoint:  playerPKPoint(e),
		CurKill:  e.CurKill,
		TotKill:  e.TotKill,
	}
	for i := range e.Affect {
		if e.Affect[i].Type == 0 {
			continue
		}
		d.Affect[i] = protocol.PackAffect(protocol.AffectData{
			Type: e.Affect[i].Type,
			Time: e.Affect[i].Time,
		})
	}
	return d
}

// createMobViewPacket mirrors legacy SendCreateMob: a player with an open
// personal shop must be revealed with MSG_CreateMobTrade, not the normal avatar
// packet, so clients that enter view after the shop opened still see the stall.
func createMobViewPacket(w *world.World, e *world.Entity, createType uint16) (protocol.Type, []byte) {
	data := createMobFrom(e, createType)
	if s := shopSessionOf(w, e); s != nil {
		data.Con = 0 // GetCreateMobTrade parity: shop pose hides the Con field.
		return protocol.MsgCreateMobTrade, protocol.EncodeCreateMobTradeBody(data, nil, s.AutoTrade.Title)
	}
	return protocol.MsgCreateMob, protocol.EncodeCreateMobBody(data)
}

// shopSessionOf returns the session whose personal shop this entity IS, or nil
// when it is not a stall. It answers for both shapes a shop can take: the clone
// mob (Entity.ShopOwner points at its owner) and the legacy pose, where the
// seller's own body is the stall.
func shopSessionOf(w *world.World, e *world.Entity) *world.Session {
	conn := e.ID
	if !world.IsPlayer(e.ID) {
		if e.ShopOwner == 0 {
			return nil
		}
		conn = e.ShopOwner
	}
	s := w.Session(conn)
	if s == nil || s.Mode != world.UserPlay || s.TradeMode != 1 || s.AutoTrade == nil {
		return nil
	}
	// A clone must still be the one this shop owns. Without this an id recycled
	// out from under a stale ShopOwner would answer for someone else's shop.
	if !world.IsPlayer(e.ID) && s.AutoTrade.CloneID != e.ID {
		return nil
	}
	return s
}

// playerPKPoint is pkPoint(e) for a player, or 0 for a mob (mobs never carry PK
// coloring and their MobName is sent raw, so the value is ignored anyway).
func playerPKPoint(e *world.Entity) uint8 {
	if !world.IsPlayer(e.ID) {
		return 0
	}
	return pkPoint(e)
}

// characterLogout handles _MSG_CharacterLogout (0x0215): return to the selection
// screen. (Saving the in-play character is a later batch once SaveCharacter is on
// the port.)
func (d *Dispatcher) characterLogout(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	d.returnToCharacterSelection(w, s, nil)
}

// returnToCharacterSelection performs the world cleanup and durable save shared
// by ordinary logout and legacy quest transitions that force a character reload.
// after runs in the world loop after the client has received the logout
// confirmation, so follow-up selection-screen effects cannot race the save.
func (d *Dispatcher) returnToCharacterSelection(w *world.World, s *world.Session, after func(*world.World, *world.Session)) {
	// Leave the party (and reap this character's summons) first, while the session
	// is still UserPlay — sendRemoveParty/sendAddParty only reach UserPlay
	// recipients (_MSG_CharacterLogout.cpp:23 calls RemoveParty for the same
	// reason). The disconnect-time hook is a no-op after this.
	d.SessionEnd(w, s)
	// Despawn this entity for in-view players (back to character selection).
	body := protocol.EncodeRemoveMobBody(2)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(s.Conn)}, body)
	})
	// Persist first, then confirm: the client re-reads the character from the DB
	// when it re-selects (and may reconnect/re-login the account), so the save must
	// commit before we hand it back the selection screen (otherwise the reload
	// races the write — last_city/coin). The account-shared cargo is saved in the
	// same flow: deposits/withdrawals exchange items between the character carry and
	// the cargo, so persisting the character without the cargo would duplicate a
	// withdrawn item (saved on the character row while the stale account_cargo row
	// still holds it) on the next load.
	// Read before the save: by the time the callback runs the entity has been
	// docked and this session may already hold a different character.
	var saindo string
	if e := w.Entity(s.Conn); e != nil {
		saindo = e.Name
	}
	w.SaveCharacterThen(s, func(w *world.World, s *world.Session) {
		w.SaveCargoThen(s, func(w *world.World, s *world.Session) {
			// The save above has committed, so the database is authoritative for
			// this character again and the panel may edit it.
			d.markPresence(w, saindo, false)
			if e := w.Entity(s.Conn); e != nil {
				e.Mode = world.MobUserDock
				// The save above already captured this character's buffs; drop
				// them from the per-connection entity so they can't bleed into
				// the next character selected on this session (issue #21/#47).
				e.ResetAffects()
			}
			// Drop any open personal shop (issue #115). Through closeAutoTrade, not by
			// clearing the fields: the shop may have a clone standing in the world, and
			// the RemoveMob above only removed the PLAYER. Zeroing the session state here
			// would strand the stall — an entity nobody owns, that no longer resolves to a
			// shop, and that nothing left alive knows to take down.
			d.closeAutoTrade(w, s)
			s.Mode = world.UserSelChar
			w.Send(s, protocol.MsgCNFCharacterLogout, nil)
			if after != nil {
				after(w, s)
			}
		})
	})
}

// returnPersistedCharacterToSelection completes a transition whose character
// snapshot was already committed off-loop. It deliberately does not save the
// character again, avoiding a second failure point after publishing the snapshot.
func (d *Dispatcher) returnPersistedCharacterToSelection(w *world.World, s *world.Session, after func(*world.World, *world.Session)) {
	d.SessionEnd(w, s)
	body := protocol.EncodeRemoveMobBody(2)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(s.Conn)}, body)
	})
	w.SaveCargoThen(s, func(w *world.World, s *world.Session) {
		if e := w.Entity(s.Conn); e != nil {
			e.Mode = world.MobUserDock
			e.ResetAffects()
		}
		// Same reason as the sibling path above: closeAutoTrade, so a shop clone
		// standing in the world comes down with its owner instead of being
		// stranded by a field assignment.
		d.closeAutoTrade(w, s)
		s.Mode = world.UserSelChar
		w.Send(s, protocol.MsgCNFCharacterLogout, nil)
		if after != nil {
			after(w, s)
		}
	})
}

// restart handles _MSG_Restart (0x0289): the death-respawn / town-recall button
// (TMSrv/_MSG_Restart.cpp). It revives the character at 2 HP (NOT a full heal —
// recalling always costs you down to 2 HP, even alive) and recalls it to its
// last-city spawn. This is how a player gets up after a mob kills it (mobai.go).
//
// UNVERIFIED / deferred: the original's per-clan capital-region destinations
// (clan 7/8 coordinate boxes) and the exact DoRecall save-point logic are not
// reproduced — we recall to the last-city default spawn. The dedicated
// _MSG_SetHpMp (0x0181) carries the HP/MP request targets; send it after the
// score refresh so the client bars snap to the post-restart state.
func (d *Dispatcher) restart(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	e.HP = 2         // revive (CurrentScore.Hp = 2)
	s.CrackError = 0 // NumError = 0
	d.sendScore(w, s, e)
	s.ReqHp = e.HP
	setReqMp(s, e)
	d.sendSetHpMp(w, s, e)

	d.recall(w, s, e)
	d.sendEtc(w, s, e) // SendEtc (gold + ScoreBonus)
}

func (d *Dispatcher) recall(w *world.World, s *world.Session, e *world.Entity) {
	e.QuestFlag = 0
	rx, ry := world.CitySpawn(int(e.LastCity))
	// Owning a city buys your guild its own respawn point, from anywhere on the
	// map. The legacy meant to do this — Server.cpp:8514 reads GuildSpawnX/Y
	// right here — but never assigned those fields, so its owning guild would
	// have landed at (0,0). The point comes from the database instead.
	if gx, gy, ok := d.guildSpawnFor(e.Guild); ok {
		// A fixed tile needs a free neighbour: SetEntityPos overwrites whatever
		// holds that grid cell, and a guild point is one exact tile that the
		// whole guild returns to. Without this the second member to die erases
		// the first from the grid. CitySpawn does not need it because the legacy
		// scatters city respawns over a 15-tile box; this one is a single point.
		if fx, fy, ok := w.EmptyCellNear(gx, gy); ok {
			rx, ry = fx, fy
		} else {
			rx, ry = gx, gy
		}
	}
	d.doTeleport(w, s, rx, ry)
}

// guildSpawnFor is the respawn point of the zone this guild owns, if it owns one
// and that zone has a point configured.
//
// Zero on either axis is "not configured", not a coordinate: it is the column
// default, and honouring it would drop the whole guild at the map corner — which
// is exactly the legacy bug this replaces.
func (d *Dispatcher) guildSpawnFor(guild uint16) (int16, int16, bool) {
	if guild == 0 {
		return 0, 0, false
	}
	for i := range d.guildZones {
		z := &d.guildZones[i]
		if z.ChargeGuild != guild || z.GuildSpawnX <= 0 || z.GuildSpawnY <= 0 {
			continue
		}
		return int16(z.GuildSpawnX), int16(z.GuildSpawnY), true
	}
	return 0, 0, false
}

// mountEquipSlot is the mount equip slot (Equip[14]). Mounts are acquired in-game
// (capture/shop, quest reward, the Perzen sphere trade-in — docs/migration/handlers/
// npc-map.md, _MSG_Quest-npcs.md) — no class starts with one, so starterEquip never
// populates this slot.
const mountEquipSlot = 14

// equipEmpty reports whether a character has no equipped items at all (a fresh or
// never-seeded character), so starter gear should be granted.
func equipEmpty(equip [world.MaxEquip]world.Item) bool {
	for _, it := range equip {
		if !it.Empty() {
			return false
		}
	}
	return true
}

// repairEquip moves any item sitting in an equip slot it does not belong in (nPos
// mismatch — a consumable in the body slot, a mount in a weapon hand) back into the
// inventory, then restores the class body item if slot 0 ended up empty. This heals
// characters corrupted before equip-slot validation existed; for clean characters it
// is a no-op.
func (d *Dispatcher) repairEquip(st *world.CharacterState) {
	for s := 0; s < world.MaxEquip; s++ {
		it := st.Equip[s]
		if it.Empty() || d.canEquipSlot(it.Index, s) {
			continue
		}
		if dst := firstEmptyCarry(&st.Carry); dst >= 0 {
			st.Carry[dst] = it // preserve the displaced item
		}
		st.Equip[s] = world.Item{}
	}
	if st.Equip[0].Empty() {
		st.Equip[0] = d.classBody(st.Class) // restore the class look
	}
}

// classBody returns the class's body item (template Equip slot 0), the item that gives
// a TK/FM/BM/HT its appearance. Empty when the class template is unavailable.
func (d *Dispatcher) classBody(class int) world.Item {
	return d.starterEquip(class)[0]
}

// starterEquip builds the new-character starter gear for a class: the class
// template's equipment (the body item @slot 0 — which drives the class look —
// plus armor and weapon). Returns an empty set if the class template is
// unavailable. The mount slot (14) is intentionally left empty — matching every
// shipped BaseMob template — since mounts are acquired in-game, not granted.
func (d *Dispatcher) starterEquip(class int) [world.MaxEquip]world.Item {
	var eq [world.MaxEquip]world.Item
	if tmpl, ok := d.baseMobs[class]; ok && len(tmpl) == content.BaseMobSize {
		for i, it := range protocol.MobEquip(tmpl) {
			eq[i] = world.Item{
				Index: int16(it.Index),
				Effects: [3]world.Effect{
					{Effect: it.Eff[0][0], Value: it.Eff[0][1]},
					{Effect: it.Eff[1][0], Value: it.Eff[1][1]},
					{Effect: it.Eff[2][0], Value: it.Eff[2][1]},
				},
			}
		}
	}
	return eq
}

// firstEmptyCarry returns the index of the first empty inventory slot, or -1.
func firstEmptyCarry(carry *[world.MaxCarry]world.Item) int {
	return firstEmptyCarrySlot(carry[:], carryLimit(carry[:]))
}

// validCharName approximates BASE_CheckValidString.
//
// UNVERIFIED: the exact allowed character set / profanity rules are not in the
// source (handlers/_MSG_CreateCharacter.md). This accepts 1..15 chars of
// ASCII letters, digits and underscore as a conservative placeholder.
func validCharName(name string) bool {
	if len(name) == 0 || len(name) > 15 {
		return false
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}
