package handler

import (
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Arch unlock quests (Lindy's combine, _MSG_CombineItemLindy.cpp) are handed
// out at exactly level 355 and 370 — the legacy checks the level for EQUALITY.
// What keeps a character inside that one-level window is the pair of walls at
// GetFunc.cpp:1032 (no experience) and CMob.cpp:1110 (no level-up); both now
// exist here (level.ExpApply and applyLevelUps).
//
// Characters stranded above a wall before those gates existed still need a way
// out, so this server accepts the recipe at or ABOVE the quest level and walks
// the character back down to it on success. That is a deliberate divergence from
// the legacy equality check, and it is one-directional: it never lets an Arch
// skip a quest — the recipe, the items and the flags are unchanged — it only
// lets a stranded one finally run the quest it should have run, paying for it
// with the levels it gained while past the wall.

// archUnlockLevel reports which unlock the character is due next and whether its
// level has reached it. The 355 quest always comes first: an Arch that is past
// both walls with neither flag set runs 355, drops to 354, and climbs to 369
// again for the second one — the same order it would have followed.
func archUnlockLevel(e *world.Entity) (questLevel int32, eligible bool) {
	if e.ClassMaster != classMasterArch {
		return 0, false
	}
	if e.ArchLv355 == 0 {
		return level.ArchGateLv355, e.Level >= level.ArchGateLv355
	}
	if e.ArchLv370 == 0 {
		return level.ArchGateLv370, e.Level >= level.ArchGateLv370
	}
	return 0, false // both unlocks already done
}

// archSkillBonusPerLevel is the SkillBonus granted per level at the Arch quest
// levels. Both walls sit far above 200, so the +4 branch (CMob.cpp:1121) is the
// only one that can apply to the levels being taken back.
const archSkillBonusPerLevel = 4

// archSpecialBonusPerLevel is the SpecialBonus granted per level (CMob.cpp:1126).
const archSpecialBonusPerLevel = 2

// downlevelArch walks a stranded Arch back to its quest level, undoing the
// per-level grants applyLevelUps handed out above it. It is only ever called for
// a character that levelled past a wall it should not have crossed.
//
// Not everything can be undone exactly: SkillBonus and SpecialBonus may already
// be spent, so they are clawed back out of the unspent pool and floored at zero
// rather than forcing skills to be unlearned. The claw-back is therefore a lower
// bound on what was granted — never more than the character actually has.
func (d *Dispatcher) downlevelArch(e *world.Entity, questLevel int32) {
	lost := e.Level - questLevel
	if lost <= 0 {
		return
	}
	e.Level = questLevel
	// Experience has to come down with the level. Left where it was, the very
	// next applyLevelUps — now unblocked by the flag this quest just set — would
	// replay every level in a single tick, undoing the whole point.
	e.Exp = level.NextLevelExp(questLevel - 1)
	// The grants that ACCUMULATE on the entity. BaseAC is not among them:
	// playerBaseAC derives it from the level, so re-deriving is both correct and
	// safe against drift.
	e.BaseMaxHP = addClamp(e.BaseMaxHP, -level.IncHP(e.Class)*lost, level.MaxHPCap)
	e.BaseMaxMP = addClamp(e.BaseMaxMP, -level.IncMP(e.Class)*lost, level.MaxMPCap)
	e.BaseAC = playerBaseAC(e)
	e.SkillBonus = subBonus(e.SkillBonus, archSkillBonusPerLevel*lost)
	e.SpecialBonus = subBonus(e.SpecialBonus, archSpecialBonusPerLevel*lost)
	// ScoreBonus is a pure function of level and allocated attributes, so the
	// lower level rebuilds it. Note it goes UP: BASE_GetBonusScorePoint subtracts
	// 8 points per level above 354 (Basedef.cpp:924), which is the legacy's own
	// way of saying those levels were never meant to be reached unpaid.
	e.ScoreBonus = uint16(level.ScoreBonus(scoreBonusInput(e)))
	d.refreshScore(e)
	e.HP, e.MP = min(e.HP, effectiveMaxHP(e)), min(e.MP, effectiveMaxMP(e))
}

// subBonus subtracts n from an unsigned point pool, flooring at zero.
func subBonus(pool uint16, n int32) uint16 {
	if int32(pool) <= n {
		return 0
	}
	return pool - uint16(n)
}

// sendClientMessage is SendClientMessage (SendFunc.cpp:27): one line of server
// text in the client's message panel. It is the channel the legacy uses for
// every "here is what just happened" line — including the Lindy unlock's
// _NN_Processing_Complete — and this port had the opcode but never a payload
// encoder, so nothing the server said ever reached the player.
// HEADER.ID is ZERO, not the receiver's conn (SendFunc.cpp:34, `sm_mp.ID = 0`).
// The id is how the client decides who said a line: sending the player's own
// conn makes their own name the source, which is why server text arrived looking
// like the player had typed it ("[Nick]> O servidor esta sendo reiniciado").
// Zero means "the server".
func sendClientMessage(w *world.World, s *world.Session, text string) {
	if s == nil || text == "" {
		return
	}
	w.SendTo(s, protocol.Header{Type: protocol.MsgMessagePanel, ID: 0}, protocol.EncodeMessagePanelBody(text))
}

// msgProcessingComplete is _NN_Processing_Complete (Language.txt:158), the line
// the legacy shows when a combine succeeds. Kept as a literal rather than read
// from the string table because nothing else in this port loads Language.txt
// yet; the text is copied verbatim from the shipped file.
const msgProcessingComplete = "Processo de combinação foi concluído."

// msgCapeBonusFailed explains the one way the 370 reward can be lost: the cape
// has no free effect slot, or none is equipped at all.
const msgCapeBonusFailed = "Destrave concluído, mas a capa não tinha espaço para o bônus."

// The reward for the level-370 unlock. SERVER RULE, NOT PARITY: the original
// grants nothing here — QuestInfo.Arch.Level370 is read in exactly three places
// (CMob.cpp:1110, GetFunc.cpp:1035 and the Lindy handler), all of them gates. A
// quest that costs a point of Fame and holds the character's progression
// hostage until it is done deserves to leave something behind, so this server
// attaches a bonus to the kingdom cape it already granted at 355.
const (
	archCape370HP     uint8 = 120
	archCape370Resist uint8 = 8
)

// applyArchCapeBonus writes the level-370 reward onto the kingdom cape as
// INSTANCE effects — EF_HP 120 and EF_RESISTALL 8 — the same shape a jewel or a
// refine already leaves on an item.
//
// It goes on the ITEM rather than on the character deliberately. The client
// reads instance effects straight off the item, so the tooltip shows the bonus
// without shipping new client content; and the server already folds both in when
// it scores equipment, with efResistAll spreading across all four elements.
// Putting it on the character instead would need a base term that Resist simply
// does not have, and would leave the cape describing itself wrongly.
//
// Returns false when there is no room: an item carries three effect slots and a
// refined cape already spends one on EF_SANC. The caller says so out loud rather
// than silently swallowing a reward the player paid Fame for.
func applyArchCapeBonus(cape *world.Item) bool {
	if cape == nil || cape.Index == 0 {
		return false
	}
	want := [2]world.Effect{
		{Effect: efHp, Value: archCape370HP},
		{Effect: efResistAll, Value: archCape370Resist},
	}
	for _, ef := range want {
		if !setInstanceEffect(cape, ef) {
			return false
		}
	}
	return true
}

// setInstanceEffect stores ef in the first free slot, or overwrites the slot
// already holding the same effect id so that re-running never stacks a duplicate.
func setInstanceEffect(it *world.Item, ef world.Effect) bool {
	for i := range it.Effects {
		if it.Effects[i].Effect == ef.Effect {
			it.Effects[i] = ef
			return true
		}
	}
	for i := range it.Effects {
		if it.Effects[i].Effect == 0 && it.Effects[i].Value == 0 {
			it.Effects[i] = ef
			return true
		}
	}
	return false
}

// gmQuestReset clears an Arch quest flag so the quest can be run again. It is a
// TEST tool, not a game mechanic: the quests are deliberately one-shot, and the
// only reason to undo one is to exercise it after changing what it grants.
//
//	/gm questreset 355      the Lindy unlock at 355
//	/gm questreset 370      the Lindy unlock at 370
//	/gm questreset cristal  all four crystal stages
//	/gm questreset arch     all of the above
//
// Clearing 355 or 370 also re-arms the level wall that flag opens, so the
// character stops earning experience at that level until the quest is redone.
// That is the point rather than a side effect: it restores the exact state the
// quest expects to find.
//
// What it CANNOT undo is what the quest already granted. The crystals' HP/MP and
// the 370 cape's HP land on the BaseScore and are indistinguishable from any
// other points there, so redoing a quest after a reset stacks its HP/MP a second
// time. AC and resistance are derived from the flags, so those do return to
// zero. Note the character's stats before testing if exact numbers matter.
func (d *Dispatcher) gmQuestReset(w *world.World, s *world.Session, rest string) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	cleared, ok := applyQuestReset(e, rest)
	if !ok {
		sendClientMessage(w, s, "Uso: /gm questreset 355 | 370 | cristal | arch")
		return
	}
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)
	d.log.Info("gm questreset", "account", s.AccountName, "cleared", cleared,
		"arch355", e.ArchLv355, "arch370", e.ArchLv370, "cristal", e.ArchCristal)
	sendClientMessage(w, s, "Quest resetada: "+cleared)
	w.SaveCharacterAsync(s)
}

// applyQuestReset clears the flags named by arg and reports which. Split from
// gmQuestReset so the selection can be tested without standing up a server: the
// decision is the part worth pinning, the packets around it are not.
func applyQuestReset(e *world.Entity, arg string) (cleared string, ok bool) {
	switch strings.ToLower(firstToken(arg)) {
	case "355":
		e.ArchLv355 = 0
		return "arch355", true
	case "370":
		e.ArchLv370 = 0
		return "arch370", true
	case "cristal", "cristais":
		e.ArchCristal = 0
		return "cristal", true
	case "arch":
		e.ArchLv355, e.ArchLv370, e.ArchCristal = 0, 0, 0
		return "arch355+arch370+cristal", true
	default:
		return "", false
	}
}

// The Pedra Ideal refusals. Only the first is a legacy string
// (_NN_Cant_with_armor, Language.txt:205); the others explain requirements the
// original enforces silently, which is what made a declined transformation
// indistinguishable from a broken one.
const (
	msgCantWithArmor = "No momento voce nao pode equipar arma e armadura."
	// The set has to come off before the stone is used, montaria and capa
	// included. The alternative would be destroying it: the Celestial is born
	// with the class base and nothing else, so anything still equipped would
	// simply cease to exist.
	msgIdealStoneUnequipAll  = "Retire TODOS os itens equipados, inclusive a montaria e a capa, antes de usar a Pedra Ideal."
	msgIdealStoneArchOnly    = "Somente um Arch pode renascer como Celestial."
	msgIdealStoneLevel       = "E preciso ser nivel 355 ou mais para renascer como Celestial."
	msgIdealStoneMortalLevel = "Este personagem nao registrou o nivel de Mortal exigido (99) para renascer."
)
