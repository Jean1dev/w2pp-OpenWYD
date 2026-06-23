package handler

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

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
		return func(w *world.World, s *world.Session) {
			s.Mode = world.UserSelChar
			if err != nil || !ok {
				w.Send(s, protocol.MsgNewCharacterFail, nil)
				return
			}
			w.Send(s, protocol.MsgCNFDeleteCharacter, nil)
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
	// Spawn position: the relational position is not carried over gRPC yet (0,0),
	// so fall back to the class template's valid spawn coordinates.
	spawnX, spawnY := st.X, st.Y
	if tmpl, ok := d.baseMobs[st.Class]; ok && len(tmpl) == content.BaseMobSize && spawnX == 0 && spawnY == 0 {
		spawnX, spawnY = protocol.BaseMobSpawn(tmpl)
	}
	// Inject the player entity into the world (the slot was docked at connect).
	if e := w.Entity(s.Conn); e != nil {
		e.Mode = world.MobUser
		e.Name = st.Name
		e.Class = uint8(st.Class)
		e.X, e.Y = spawnX, spawnY
		e.HP, e.MaxHP = st.HP, st.MaxHP
		e.Damage, e.AC, e.Master = st.Damage, st.AC, st.Master
		e.Level, e.Coin = int32(st.Level), st.Coin
		e.Clan, e.Guild, e.GuildLevel, e.ClassMaster = st.Clan, st.GuildID, st.GuildLevel, st.ClassMaster
		e.Str, e.Int, e.Dex, e.Con, e.ScoreBonus = st.Str, st.Int, st.Dex, st.Con, st.ScoreBonus
		e.Carry = st.Carry
		// Visual gear codes for how OTHER players see this character (MSG_CreateMob).
		if tmpl, ok := d.baseMobs[st.Class]; ok && len(tmpl) == content.BaseMobSize {
			eq := protocol.MobEquip(tmpl)
			for i := range eq {
				e.EquipVisual[i] = eq[i].Index
			}
		}
	}
	s.Mode = world.UserPlay
	var shortSkill [16]uint8
	// Prefer the per-class BaseMob template (real STRUCT_MOB with starter equipment
	// → correct class model and no client crash); patch name + position.
	if tmpl, ok := d.baseMobs[st.Class]; ok && len(tmpl) == content.BaseMobSize {
		var carry [64]protocol.SelItem
		for i := range st.Carry {
			if i >= 64 {
				break
			}
			carry[i] = itemToSel(st.Carry[i])
		}
		body := protocol.EncodeCNFCharacterLoginRaw(tmpl, st.Name, st.Coin, carry, s.Slot, s.Conn, 0, shortSkill)
		d.log.Info("char login: sending CNFCharacterLogin (template)",
			"conn", s.Conn, "class", st.Class, "name", st.Name, "x", spawnX, "y", spawnY, "body", len(body))
		w.SendTo(s, protocol.Header{Type: protocol.MsgCNFCharacterLogin, ID: protocol.IDScene}, body)
		d.enterWorldView(w, s)
		return
	}
	d.log.Info("char login: sending CNFCharacterLogin (fallback, no template)",
		"conn", s.Conn, "class", st.Class)
	// Fallback: build the snapshot from the stored relational state (no equipment).
	// Byte-exact MSG_CNFCharacterLogin (STRUCT_MOB + pos + skillbar), ID=30000.
	m := protocol.MobSnapshot{
		Name:  st.Name,
		Clan:  st.Clan,
		Guild: st.GuildID,
		Class: uint8(st.Class),
		Coin:  st.Coin,
		Exp:   st.Exp,
		SPX:   st.X, SPY: st.Y,
		Level: int32(st.Level), Ac: st.AC, Damage: st.Damage,
		MaxHp: st.MaxHP, MaxMp: st.MaxMP, Hp: st.HP, Mp: st.MP,
		Str: st.Str, Int: st.Int, Dex: st.Dex, Con: st.Con,
		ScoreBonus: st.ScoreBonus, GuildLevel: st.GuildLevel,
	}
	for i := range st.Carry {
		if i >= len(m.Carry) {
			break
		}
		m.Carry[i] = protocol.SelItem{
			Index: uint16(st.Carry[i].Index),
			Eff: [3][2]uint8{
				{st.Carry[i].Effects[0].Effect, st.Carry[i].Effects[0].Value},
				{st.Carry[i].Effects[1].Effect, st.Carry[i].Effects[1].Value},
				{st.Carry[i].Effects[2].Effect, st.Carry[i].Effects[2].Value},
			},
		}
	}
	body := protocol.EncodeCNFCharacterLoginBody(s.Slot, s.Conn, 0, m, shortSkill)
	w.SendTo(s, protocol.Header{Type: protocol.MsgCNFCharacterLogin, ID: protocol.IDScene}, body)
	d.enterWorldView(w, s)
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
	selfMob := protocol.EncodeCreateMobBody(createMobFrom(self, 2))
	w.ForEachInView(s.Conn, func(vs *world.Session, ve *world.Entity) {
		// (A) other players see the newcomer
		w.MarkSeen(vs, s.Conn)
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, selfMob)
		// (B) the newcomer sees each player already in view
		w.MarkSeen(s, ve.ID)
		w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene},
			protocol.EncodeCreateMobBody(createMobFrom(ve, 0)))
	})
	// (C) the newcomer sees the NPCs/monsters in view.
	d.revealMobsInView(w, s)
}

// revealMobsInView sends a MSG_CreateMob for every NPC/monster now in the player's
// view that the client hasn't seen yet (once per entity). Called on entry and on
// each move, so NPCs appear as the player explores.
func (d *Dispatcher) revealMobsInView(w *world.World, s *world.Session) {
	w.ForEachMobInView(s.Conn, func(me *world.Entity) {
		if w.MarkSeen(s, me.ID) {
			w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene},
				protocol.EncodeCreateMobBody(createMobFrom(me, 0)))
		}
	})
}

// createMobFrom builds MSG_CreateMob data from a world entity (player or NPC). The
// visual Equip codes come from the entity's EquipVisual (set at login/spawn from
// the relevant STRUCT_MOB template). createType: 0 normal, 2 "just entered".
func createMobFrom(e *world.Entity, createType uint16) protocol.CreateMobData {
	return protocol.CreateMobData{
		MobID:           e.ID,
		Name:            e.Name,
		PosX:            e.X,
		PosY:            e.Y,
		Guild:           e.Guild,
		GuildMemberType: e.GuildLevel,
		Level:           e.Level,
		Ac:              e.AC,
		Damage:          e.Damage,
		MaxHp:           e.MaxHP,
		Hp:              e.HP,
		Str:             e.Str, Int: e.Int, Dex: e.Dex, Con: e.Con,
		Merchant:   e.Merchant,
		Equip:      e.EquipVisual,
		CreateType: createType,
	}
}

// characterLogout handles _MSG_CharacterLogout (0x0215): return to the selection
// screen. (Saving the in-play character is a later batch once SaveCharacter is on
// the port.)
func (d *Dispatcher) characterLogout(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	w.SaveCharacterAsync(s) // persist before leaving the world (still UserPlay)
	// Despawn this entity for in-view players (back to character selection).
	body := protocol.EncodeRemoveMobBody(2)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(s.Conn)}, body)
	})
	if e := w.Entity(s.Conn); e != nil {
		e.Mode = world.MobUserDock
	}
	s.Mode = world.UserSelChar
	w.Send(s, protocol.MsgCNFCharacterLogout, nil)
}

// restart handles _MSG_Restart (0x0289): revive with 2 HP (not a full heal) and
// recall. handlers/lote2-sessao-conta.md.
//
// UNVERIFIED: the hardcoded capital-region teleport coordinates and per-clan
// destinations (and DoRecall) are not reproduced; they must become config and be
// validated by capture. Batch 1 only applies the HP=2 revive + HP/MP refresh.
func (d *Dispatcher) restart(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		w.Send(s, protocol.MsgSetHpMp, nil)
		return
	}
	if e := w.Entity(s.Conn); e != nil {
		e.HP = 2
	}
	w.Send(s, protocol.MsgSetHpMp, nil)
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
