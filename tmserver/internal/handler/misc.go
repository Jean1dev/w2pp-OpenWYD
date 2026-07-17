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
	if isKingQuestNPC(npc) {
		d.kingArch(w, s, e, int(confirm))
		return
	}
	d.log.Debug("quest NPC not implemented", "conn", s.Conn, "npc", npcIndex, "merchant", npc.Merchant, "grade", npc.Grade)
}

const (
	kingQuestMerchant = 111
	kingQuestGrade    = 4

	idealStoneEquipSlot = 10
	sephirotEquipSlot   = 11
	archSephirotMin     = 1760
	archSephirotMax     = 1763
)

func isKingQuestNPC(npc *world.Entity) bool {
	if npc == nil {
		return false
	}
	// The Go loader stores CurrentScore.Merchant (111) plus EF_GRADE0 (4), while
	// the legacy switch routes KING from MOB.Merchant 14/15. Accept both shapes so
	// tests and future loaders can use either representation.
	return npc.Merchant == kingQuestMerchant && npc.Grade == kingQuestGrade ||
		npc.Merchant == 14 || npc.Merchant == 15
}

func (d *Dispatcher) kingArch(w *world.World, s *world.Session, e *world.Entity, confirm int) {
	if confirm == 0 {
		return
	}
	if e.ClassMaster != classMasterMortal || e.Level < 299 {
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

const masterGriffTravelDelay = 9500 * time.Millisecond

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
func (d *Dispatcher) reqRanking(_ *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("ReqRanking not yet implemented (UNVERIFIED duel)", "conn", s.Conn)
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
