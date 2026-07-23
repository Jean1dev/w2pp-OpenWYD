package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// mountExpiryDays is the lifetime granted to a Perzen reward mount ("X(30dias)",
// BASE_SetItemDate(30)).
const mountExpiryDays = 30

// applyBonus handles _MSG_ApplyBonus (0x0277): spend a free point
// (protocol-spec.md §3.5, _MSG_ApplyBonus.cpp). BonusType routes: 0 = attribute
// point (Str/Int/Dex/Con), 1 = mastery point (Special tree), 2 = learn a skill
// from a class master.
func (d *Dispatcher) applyBonus(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgApplyBonusBody
	if err := body.Decode(payload); err != nil {
		return
	}
	switch body.BonusType {
	case protocol.BonusScore:
		d.applyScoreBonus(w, s, e, int(body.Detail))
	case protocol.BonusSpecial:
		d.applySpecialBonus(w, s, e, int(body.Detail))
	case protocol.BonusSkill:
		d.learnSkill(w, s, e, int(body.Detail), int(body.TargetID))
	}
}

// applyScoreBonus is ApplyBonus BonusType==0 (_MSG_ApplyBonus.cpp:32-84):
// allocate attribute points into the BaseScore; refreshScore folds them into
// the live CurrentScore (= base + equipment) and sendScore shows it. Parity
// quirks: a ScoreBonus stockpile ≥300 spends 100 points per click, and Int/Con
// also add 2×points to MaxMP/MaxHP directly.
func (d *Dispatcher) applyScoreBonus(w *world.World, s *world.Session, e *world.Entity, detail int) {
	if e.ScoreBonus == 0 {
		d.sendScore(w, s, e)
		return
	}
	points := int16(1)
	if e.ScoreBonus >= 300 {
		points = 100
	}
	switch detail {
	case protocol.DetailStr:
		e.BaseStr += points
	case protocol.DetailInt:
		e.BaseInt += points
		e.BaseMaxMP += 2 * int32(points)
	case protocol.DetailDex:
		e.BaseDex += points
	case protocol.DetailCon:
		e.BaseCon += points
		e.BaseMaxHP += 2 * int32(points)
	default:
		return
	}
	e.ScoreBonus -= uint16(points)
	d.refreshScore(e)
	d.sendEtc(w, s, e) // remaining points live in UpdateEtc (B10)
	d.sendScore(w, s, e)
}

// accountSecure handles _MSG_AccountSecure (0x0FDE): the numeric PIN
// (lote2-sessao-conta.md). ChangeNumeric selects verify (0) vs set/change (1). The
// PIN is validated against an argon2id hash on the dbServer (issue #120) — never
// plaintext, never logged. On success the client gets a header-only _MSG_AccountSecure
// (ID=ESCENE_FIELD) so it advances past the secure-password screen; on a bad PIN it
// gets _MSG_AccountSecureFail and stays put.
//
// First-time set: a verify against an account with no PIN yet takes the supplied
// token as the new PIN (legacy NumericToken[0]==-1 behavior). Without a dbServer the
// step is acknowledged so bring-up still works (allow-all, like billing).
func (d *Dispatcher) accountSecure(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgAccountSecureBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if s.AccountID == 0 {
		return // no account bound yet — ignore
	}
	pin := body.PIN()
	change := body.ChangeNumeric == 1
	accountID := s.AccountID
	p := w.Persistence()

	w.Go(s, func() func(*world.World, *world.Session) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var ok bool
		var err error
		switch {
		case change:
			ok, err = p.SetPin(ctx, accountID, pin)
		default:
			var res world.PinResult
			res, err = p.VerifyPin(ctx, accountID, pin)
			if res == world.PinNotSet && err == nil {
				ok, err = p.SetPin(ctx, accountID, pin) // first-time set: the typed PIN becomes the PIN
			} else {
				ok = res == world.PinOK
			}
		}
		return func(w *world.World, s *world.Session) { d.completeAccountSecure(w, s, ok, err) }
	})
}

// completeAccountSecure runs back in the loop with the PIN result. A missing
// backend (errNoPersistence) degrades to allow-all so early bring-up isn't blocked
// on the secure-password screen.
func (d *Dispatcher) completeAccountSecure(w *world.World, s *world.Session, ok bool, err error) {
	if err != nil {
		d.log.Warn("account secure check failed", "conn", s.Conn, "err", err)
		ok = true // degrade to allow (no dbServer, or transient error) — never leak the PIN
	}
	if ok {
		w.SendTo(s, protocol.Header{Type: protocol.MsgAccountSecure, ID: protocol.IDScene}, nil)
		return
	}
	w.SendTo(s, protocol.Header{Type: protocol.MsgAccountSecureFail, ID: protocol.IDScene}, nil)
}

// quest handles _MSG_Quest (0x028B): NPC quest interaction. Body is
// MSG_STANDARDPARM2 (Parm1 = npcIndex, Parm2 = confirm). The NPC sub-type comes
// from MOB.Merchant (+ EF_GRADE0 for Merchant==100), see _MSG_Quest-npcs.md.
//
// This batch implements the PERZEN item-exchange NPCs (Merchant 100, grade 7/8/9)
// and Mestre Grifo (Merchant 23), a server-content shortcut into the Quest 256
// arenas, plus the King Arch creation path. The remaining quest NPC types (level
// chains, tutorials, teleports) are UNVERIFIED and not yet routed.
func (d *Dispatcher) quest(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // interacting with a quest NPC cancels a trade
	}
	npcIndex, confirm, ok := protocol.StandardParm2(payload)
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
	if npc.Merchant == 23 {
		d.mestreGrifo(w, s, e, npc)
		return
	}
	// CAPAVERDE_TELEPORT (Merchant 100, EF_GRADE0 14): apprentice-arena teleport
	// (issue #139, _MSG_Quest.cpp:2168).
	if npc.Merchant == 100 && npc.Grade == 14 {
		d.capaverdeTeleport(w, s, e)
		return
	}
	// MOLARGARGULA (Merchant 100, EF_GRADE0 15): _MSG_Quest.cpp:2236.
	if npc.Merchant == 100 && npc.Grade == 15 {
		d.molarGargula(w, s, e)
		return
	}
	// CAPAVERDE_TRADE (Merchant 8, "Chefe_Treina."): green-cape hand-in
	// (issue #139, _MSG_Quest.cpp:2189).
	if npc.Merchant == 8 {
		d.capaverdeTrade(w, s, e)
		return
	}
	// AMU_MISTICO (Merchant 10, "Cap.Mercenario"): Terra Mistica party quest
	// (_MSG_Quest.cpp:254).
	if npc.Merchant == 10 {
		d.amuMistico(w, s, e, int(confirm))
		return
	}
	// BLACKORACLE (Merchant 78): merges the two soul items into the Pedra Ideal
	// (_MSG_Quest.cpp:2257).
	if npc.Merchant == 78 {
		d.blackOracle(w, s, e)
		return
	}
	// COMP_SEPHI (Merchant 19): crafts the class Sephirot from 8 Pedras + coin
	// (_MSG_Quest.cpp:2103).
	if npc.Merchant == 19 {
		d.compSephi(w, s, e, npc, int(confirm))
		return
	}
	if isKingQuestNPC(npc) {
		d.kingArch(w, s, e, npc, int(confirm))
		return
	}
	d.log.Debug("quest NPC not implemented", "conn", s.Conn, "npc", npcIndex, "merchant", npc.Merchant, "grade", npc.Grade)
}

const (
	kingQuestMerchant = 111

	idealStoneItem       = 1742
	idealStoneEquipSlot  = 10
	sephirotEquipSlot    = 11
	archSephirotMin      = 1760
	archSephirotMax      = 1763
	capeEquipSlot        = 15
	pedraIdealsCraftCoin = 30_000_000
)

func isKingQuestNPC(npc *world.Entity) bool {
	if npc == nil {
		return false
	}
	// The Go loader stores real King templates by CurrentScore.Merchant (111),
	// while the legacy switch routes KING from top-level MOB.Merchant 14/15.
	// Accept both shapes so content and future loaders can use either source.
	return npc.Merchant == kingQuestMerchant || npc.Merchant == 14 || npc.Merchant == 15
}

// kingClanFromCape derives the player's effective Clan for the King's faction
// gate: certain capes override the wearer's guild Clan to 7 or 8 regardless of
// their actual clan (_MSG_Quest.cpp:704-726). CapeMode (the cape-reward
// branches further down the legacy case) is out of scope here — only the Clan
// override matters for the Arch gate.
func kingClanFromCape(clan uint8, capeIndex int16) uint8 {
	switch capeIndex {
	case 543, 545, 734, 736, 3191, 3194, 3197:
		return 7
	case 544, 546, 735, 737, 3192, 3195, 3198:
		return 8
	default:
		return clan
	}
}

// kingArch handles the KING npcMode's Arch-creation path (_MSG_Quest.cpp:694-867).
// Ported: the Clan/cape faction gate (:696-741), the confirm==0 ask leg
// (:762-767, without the Sapphire-price text — UNVERIFIED wire format), and the
// Equip[10]==1742 (Pedra Ideal) + Equip[11]∈[1760,1763] (Sephirot) + Mortal/level
// gate (:769, :842-866). NOT ported: the elemental-essence branch (:769-840,
// items 5334-5337 — consumes them into item 5338 and destroys the King gear
// INSTEAD of creating Arch) and the CapeMode/Sapphire/Celestial cape-reforge
// branches (:869-1124) — neither is what issue #139 reports broken, and a
// player carrying the 4 essence items alongside 1742/1760-1763 will get Arch
// creation here rather than the legacy "blessing" side-effect, an accepted
// simplification.
func (d *Dispatcher) kingArch(w *world.World, s *world.Session, e, npc *world.Entity, confirm int) {
	clan := kingClanFromCape(e.Clan, e.Equip[capeEquipSlot].Index)
	if clan != 0 && clan != npc.Clan {
		return
	}
	if confirm == 0 {
		return
	}
	if e.ClassMaster != classMasterMortal || e.Level < 299 {
		return
	}
	if e.Equip[idealStoneEquipSlot].Index != idealStoneItem {
		return
	}
	sephirot := int(e.Equip[sephirotEquipSlot].Index)
	if sephirot < archSephirotMin || sephirot > archSephirotMax {
		return
	}
	class := sephirot - archSephirotMin
	mortalFace := int(e.Equip[0].Index)
	if mortalFace <= 0 {
		if body := d.classBody(int(e.Class)); !body.Empty() {
			mortalFace = int(body.Index)
		}
	}
	if mortalFace <= 0 {
		d.log.Warn("king arch: missing mortal face", "conn", s.Conn, "name", e.Name, "class", e.Class)
		return
	}

	accID := s.AccountID
	name := e.Name
	mortalSlot := s.Slot
	s.Mode = world.UserWaitDB
	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		slot, ok, err := p.CreateArchCharacter(context.Background(), accID, name, class, mortalFace, mortalSlot)
		var chars []world.CharSummary
		var listErr error
		if err == nil && ok {
			chars, listErr = p.ListCharacters(context.Background(), accID)
		}
		return func(w *world.World, s *world.Session) {
			d.completeKingArch(w, s, slot, ok, err, chars, listErr)
		}
	})
}

func (d *Dispatcher) completeKingArch(w *world.World, s *world.Session, archSlot int, ok bool, err error, chars []world.CharSummary, listErr error) {
	if err != nil {
		s.Mode = world.UserPlay
		d.log.Warn("king arch: db create failed", "conn", s.Conn, "account", s.AccountID, "err", err)
		d.notify(w, s, NoticeDBError)
		return
	}
	if !ok {
		s.Mode = world.UserPlay
		d.notify(w, s, NoticeNoEmptySlot)
		return
	}
	if listErr != nil {
		d.log.Warn("king arch: list after create failed", "conn", s.Conn, "account", s.AccountID, "err", listErr)
	}

	s.Mode = world.UserPlay
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	e.Equip[idealStoneEquipSlot] = world.Item{}
	e.Equip[sephirotEquipSlot] = world.Item{}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, idealStoneEquipSlot, itemToSel(e.Equip[idealStoneEquipSlot])))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, sephirotEquipSlot, itemToSel(e.Equip[sephirotEquipSlot])))

	body := protocol.EncodeRemoveMobBody(2)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(s.Conn)}, body)
	})

	w.SaveCharacterThen(s, func(w *world.World, s *world.Session) {
		w.SaveCargoThen(s, func(w *world.World, s *world.Session) {
			if e := w.Entity(s.Conn); e != nil {
				e.Mode = world.MobUserDock
				e.ResetAffects()
			}
			s.Mode = world.UserSelChar
			w.Send(s, protocol.MsgCNFCharacterLogout, nil)
			if listErr == nil {
				w.SendTo(s, protocol.Header{Type: protocol.MsgCNFNewCharacter, ID: protocol.IDNewCharacter},
					protocol.EncodeCNFNewCharacterBody(d.selCharsFrom(chars)))
			}
			w.SendTo(s, protocol.Header{Type: protocol.MsgSendArchEffect, ID: protocol.IDScene},
				protocol.EncodeStandardParm(int32(archSlot)))
		})
	})
}

const (
	capaverdeMinLevel = 99
	capaverdeMaxLevel = 150

	capaverdeTicketEquipSlot = 13
	capaverdeTicketItem      = 4080
	greenCapeItem            = 4006
)

// capaverdeTeleport handles CAPAVERDE_TELEPORT (Merchant 100, EF_GRADE0 14):
// the apprentice-arena teleport step of the green-cape chain
// (_MSG_Quest.cpp:2168-2186). No item is consumed and the legacy case never
// checks confirm.
func (d *Dispatcher) capaverdeTeleport(w *world.World, s *world.Session, e *world.Entity) {
	if e.ClassMaster != classMasterMortal || e.Level < capaverdeMinLevel || e.Level >= capaverdeMaxLevel {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	d.doTeleport(w, s, 2245+int16(w.Rand().Intn(5)-3), 1576+int16(w.Rand().Intn(5)-3))
}

// capaverdeTrade handles CAPAVERDE_TRADE (Merchant 8, "Chefe_Treina."): the
// green-cape hand-in (_MSG_Quest.cpp:2189-2234). Requires item 4080 equipped
// in slot 13 (the apprentice graduation token) and no cape already equipped;
// consumes the token and grants the green cape (item 4006) into the cape
// slot. The legacy case never checks confirm.
func (d *Dispatcher) capaverdeTrade(w *world.World, s *world.Session, e *world.Entity) {
	if e.ClassMaster != classMasterMortal || e.Level < capaverdeMinLevel || e.Level >= capaverdeMaxLevel {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	if e.Equip[capaverdeTicketEquipSlot].Index != capaverdeTicketItem {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	if e.Equip[capeEquipSlot].Index != 0 {
		d.notify(w, s, NoticeAlreadyDone)
		return
	}
	e.Equip[capaverdeTicketEquipSlot] = world.Item{}
	e.Equip[capeEquipSlot] = world.Item{Index: greenCapeItem}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, capaverdeTicketEquipSlot, itemToSel(e.Equip[capaverdeTicketEquipSlot])))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, capeEquipSlot, itemToSel(e.Equip[capeEquipSlot])))
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.log.Info("capaverde trade complete", "conn", s.Conn, "name", e.Name)
}

const (
	molarGargulaMinLevel = 199
	molarGargulaMaxLevel = 254
)

// molarGargula handles MOLARGARGULA (Merchant 100, EF_GRADE0 15):
// _MSG_Quest.cpp:2236-2255. No item, no persisted flag, no confirm gate — a
// pure level/class-gated teleport.
func (d *Dispatcher) molarGargula(w *world.World, s *world.Session, e *world.Entity) {
	if e.ClassMaster != classMasterMortal || e.Level < molarGargulaMinLevel || e.Level >= molarGargulaMaxLevel {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	d.doTeleport(w, s, 817+int16(w.Rand().Intn(5)-3), 4062+int16(w.Rand().Intn(5)-3))
}

// amuMistico handles AMU_MISTICO (Merchant 10, "Cap.Mercenario"): the Terra
// Mistica party quest (_MSG_Quest.cpp:254-292). Requires ClassMaster Mortal,
// the quest not already done, and the caller to be the leader of a non-solo
// party. confirm==0 is a pure ask in the legacy case (SendSay only, no state
// change) — mirrored here as a no-op, matching kingArch's ask leg.
func (d *Dispatcher) amuMistico(w *world.World, s *world.Session, e *world.Entity, confirm int) {
	if e.TerraMistica != 0 || e.ClassMaster != classMasterMortal {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	if confirm == 0 {
		return
	}
	// Legacy party leaders keep Leader==0; the non-solo proof is PartyList.
	if e.Leader != 0 || partyMemberCount(e) == 0 {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	e.TerraMistica = 1
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("terra mistica complete", "conn", s.Conn, "name", e.Name)
}

const (
	soulItem1        = 1740
	soulItem2        = 1741
	sapphireUnit1    = 697  // 1 sapphire unit
	sapphireUnit10   = 4131 // 10 sapphire units
	blackOracleCost  = 10   // sapphire units required
	pedraIdealReward = idealStoneItem
)

// blackOracle handles BLACKORACLE (Merchant 78): merges the two soul items
// (1740 immediately followed by 1741 in Carry) plus 10 sapphire units into the
// Pedra Ideal (item 1742) the King later requires (_MSG_Quest.cpp:2257-2334).
// A 697 counts as 1 unit, a 4131 as 10. The legacy case never checks confirm.
func (d *Dispatcher) blackOracle(w *world.World, s *world.Session, e *world.Entity) {
	soulSlot := -1
	limit := activeCarryLimit(e)
	for i := 0; i < limit-1; i++ {
		if e.Carry[i].Index == soulItem1 && e.Carry[i+1].Index == soulItem2 {
			soulSlot = i
			break
		}
	}
	if soulSlot < 0 {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	units := 0
	for i := 0; i < limit; i++ {
		it := e.Carry[i]
		switch it.Index {
		case sapphireUnit1:
			units++
		case sapphireUnit10:
			units += 10
		}
	}
	if units < blackOracleCost {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	remaining := blackOracleCost
	for i := 0; i < limit; i++ {
		if remaining <= 0 {
			break
		}
		switch e.Carry[i].Index {
		case sapphireUnit1:
			e.Carry[i] = world.Item{}
			w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, i, itemToSel(e.Carry[i])))
			remaining--
		case sapphireUnit10:
			e.Carry[i] = world.Item{}
			w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, i, itemToSel(e.Carry[i])))
			if remaining -= 10; remaining < 0 {
				remaining = 0
			}
		}
	}
	e.Carry[soulSlot] = world.Item{}
	e.Carry[soulSlot+1] = world.Item{}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, soulSlot, itemToSel(e.Carry[soulSlot])))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, soulSlot+1, itemToSel(e.Carry[soulSlot+1])))

	slot := firstEmptyCarry(&e.Carry)
	if slot < 0 {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	e.Carry[slot] = world.Item{Index: pedraIdealReward}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
	d.log.Info("black oracle soul merge", "conn", s.Conn, "name", e.Name)
}

const compSephiCoinCost = pedraIdealsCraftCoin

// compSephi handles COMP_SEPHI (Merchant 19): crafts the class Sephirot
// (item 1759+npc.Class, i.e. 1760-1763) from 8 "Pedras" (items 1744-1751, one
// each) plus 30,000,000 coin (_MSG_Quest.cpp:2103-2166). confirm==0 is a pure
// ask in the legacy case (SendSay only) — mirrored as a no-op.
func (d *Dispatcher) compSephi(w *world.World, s *world.Session, e, npc *world.Entity, confirm int) {
	if confirm == 0 {
		return
	}
	const pedraCount = 8
	limit := activeCarryLimit(e)
	slots := make([]int, 0, pedraCount)
	for j := 0; j < pedraCount; j++ {
		found := -1
		for i := 0; i < limit; i++ {
			it := e.Carry[i]
			if it.Index == int16(1744+j) {
				found = i
				break
			}
		}
		if found < 0 {
			d.notify(w, s, NoticeReqNotMet)
			return
		}
		slots = append(slots, found)
	}
	if e.Coin < compSephiCoinCost {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	slot := firstEmptyCarry(&e.Carry)
	if slot < 0 {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	e.Coin -= compSephiCoinCost
	d.sendEtc(w, s, e)
	for _, i := range slots {
		e.Carry[i] = world.Item{}
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, i, itemToSel(e.Carry[i])))
	}
	e.Carry[slot] = world.Item{Index: archSephirotMin + int16(npc.Class)}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
	d.log.Info("sephirot created", "conn", s.Conn, "name", e.Name, "class", npc.Class)
}

type quest256Step struct {
	flag     uint8
	minLevel int32
	maxLevel int32
	x, y     int16
	area     questArea
}

var quest256Steps = []quest256Step{
	{flag: 1, minLevel: 39, maxLevel: 115, x: 2398, y: 2105, area: questArea{x1: 2379, y1: 2076, x2: 2426, y2: 2133}},
	{flag: 2, minLevel: 115, maxLevel: 190, x: 2234, y: 1714, area: questArea{x1: 2228, y1: 1700, x2: 2257, y2: 1728}},
	{flag: 3, minLevel: 190, maxLevel: 265, x: 464, y: 3902, area: questArea{x1: 459, y1: 3887, x2: 497, y2: 3916}},
	{flag: 4, minLevel: 265, maxLevel: 320, x: 668, y: 3756, area: questArea{x1: 658, y1: 3728, x2: 703, y2: 3762}},
	{flag: 5, minLevel: 320, maxLevel: 350, x: 1322, y: 4041, area: questArea{x1: 1312, y1: 4027, x2: 1348, y2: 4055}},
}

var masterGriffTravelDelay = 9500 * time.Millisecond

type masterGriffDestination struct {
	name string
	x, y int16
}

var masterGriffDestinations = []masterGriffDestination{
	{name: "Defensor de Almas", x: 2372, y: 2099},
	{name: "Jardim dos Deuses", x: 2220, y: 1714},
	{name: "Calabouco", x: 2365, y: 2279},
	{name: "Submundo", x: 1826, y: 1771},
}

// mestreGrifo handles the Mestre_Grifo NPC (Merchant 23) shipped in NPCGener.txt
// at Armia. This server-content shortcut sends the player to the Quest 256 arena
// matching their level and must set CMob.QuestFlag first; otherwise the legacy
// area guard recalls the player as an intruder.
func (d *Dispatcher) mestreGrifo(w *world.World, s *world.Session, e, npc *world.Entity) {
	step, ok := quest256StepForLevel(e.Level)
	if !ok {
		d.log.Debug("mestre grifo: level outside quest range", "conn", s.Conn, "npc", npc.ID, "level", e.Level)
		return
	}
	d.teleportQuest256Step(w, s, e, step)
	d.log.Info("mestre grifo teleport", "conn", s.Conn, "npc", npc.ID, "level", e.Level, "quest_flag", step.flag)
}

// masterGriff handles _MSG_MasterGriff (0x0AD9), the packet the 7662 client sends
// for the Mestre_Grifo warp UI. Real client logs show this opcode instead of
// _MSG_Quest. The client plays a travel animation after sending it, so the final
// DoTeleport is delayed a little; doing it immediately cuts the animation short.
func (d *Dispatcher) masterGriff(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	warpID, ty, ok := protocol.StandardParm2(payload) // MSG_MasterGriff: int WarpID; int Ty
	if !ok {
		return
	}
	dest, ok := masterGriffDestinationForWarpID(warpID)
	if !ok {
		d.log.Debug("master griff: no destination", "conn", s.Conn, "level", e.Level, "warp_id", warpID, "ty", ty)
		return
	}
	d.log.Info("master griff travel started", "conn", s.Conn, "level", e.Level, "warp_id", warpID, "ty", ty, "dest", dest.name, "x", dest.x, "y", dest.y)
	w.Go(s, func() func(*world.World, *world.Session) {
		time.Sleep(masterGriffTravelDelay)
		return func(w *world.World, s *world.Session) {
			e := w.Entity(s.Conn)
			if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
				return
			}
			d.doTeleport(w, s, dest.x, dest.y)
			d.log.Info("master griff teleport", "conn", s.Conn, "level", e.Level, "warp_id", warpID, "ty", ty, "dest", dest.name, "x", dest.x, "y", dest.y)
		}
	})
}

func (d *Dispatcher) teleportQuest256Step(w *world.World, s *world.Session, e *world.Entity, step quest256Step) {
	e.QuestFlag = step.flag
	d.doTeleport(w, s, step.x+int16(w.Rand().Intn(5)-3), step.y+int16(w.Rand().Intn(5)-3))
}

func quest256StepForLevel(level int32) (quest256Step, bool) {
	for _, step := range quest256Steps {
		if level >= step.minLevel && level < step.maxLevel {
			return step, true
		}
	}
	return quest256Step{}, false
}

func masterGriffDestinationForWarpID(warpID int32) (masterGriffDestination, bool) {
	switch {
	case warpID == 0:
		return masterGriffDestinations[0], true
	case warpID >= 1 && int(warpID) <= len(masterGriffDestinations):
		return masterGriffDestinations[warpID-1], true
	default:
		return masterGriffDestination{}, false
	}
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
	for i := 0; i < activeCarryLimit(e); i++ {
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

// capsuleInfo handles _MSG_CapsuleInfo (0x02CD): a pure relay to the dbServer.
//
// UNVERIFIED: becomes a dbServer cash/capsule RPC (Phase 6).
func (d *Dispatcher) capsuleInfo(_ *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("CapsuleInfo relay (DB cash RPC pending)", "conn", s.Conn)
}

// putoutSeal handles _MSG_PutoutSeal (0x03CC).
//
// UNVERIFIED: seal semantics not documented — stub.
func (d *Dispatcher) putoutSeal(_ *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("PutoutSeal not yet implemented (UNVERIFIED)", "conn", s.Conn)
}
