package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// odinRate is g_pOdinRate[12] (Basedef.cpp:95-99), indexed by recipe id.
var odinRate = [12]int{100, 35, 100, 40, 100, 100, 100, 100, 100, 100, 100, 100}

// Odin recipe result item indices (_MSG_CombineItemOdin.cpp). The Capa
// Celestial recipe targets the same cape slot as reinoCapeSlot (chat.go).
const (
	odinPistaDeRunasResult     = 5134
	odinPedraDaFuriaResult     = 3020
	odinSecretaBase            = 5334 // água=5334, terra=+1, sol=+2, vento=+3
	odinSementeDeCristalResult = 4032
)

// combineItemOdin handles _MSG_CombineItemOdin (0x02D2): the 11-recipe Odin
// family (Composição de Sets / de armas, the "+12+" +11→+15 refine tier,
// Pista de runas, Destrave Lv40, Pedra da fúria, the 4 Secreta stones,
// Semente de cristal, Capa celestial). This is a dedicated handler, not part
// of the generic CombineFamily engine (combine.go): unlike Anct/Ehre/…, these
// recipes don't share a "rate in, one result at slot 0" shape — results land
// at varying slots, one recipe mutates equipped gear directly (Equip[15]),
// and one sets a quest flag with no item at all.
//
// combine.MatchOdin returning combine.OdinNoMatch (0) is NOT "invalid" the
// way a 0 rate is for every other combine family: GetMatchCombineOdin's
// default return IS "Composição de Sets", a real (near-always-succeeding)
// recipe keyed off Item[0]'s catalog Extra field — Exec always attempts it.
// There is no "no recipe matched" rejection anywhere in the legacy handler.
//
// Two deliberate deviations from Source (issue #138 plan): the wall-clock
// srand() reseeds sprinkled through _MSG_CombineItemOdin.cpp are dropped —
// roll straight off w.Rand(), like every other combine/refine path; and the
// "+12+" recipe's failure branch, byte-identical to its success branch in the
// live (uncommented) code, is ported as it actually runs: it always succeeds
// once the pre-checks below pass.
func (d *Dispatcher) combineItemOdin(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	d.combineOdin(w, s, e, payload)
}

// combineOdin is combineItemOdin's body, split out (like useItem/useClasseItem)
// so tests can drive it with an explicit entity instead of needing one
// registered in the world's connection table.
func (d *Dispatcher) combineOdin(w *world.World, s *world.Session, e *world.Entity, payload []byte) {
	var body protocol.MsgCombineItemBody
	if err := body.Decode(payload); err != nil {
		return
	}

	items, slots, active, ok := d.resolveComboInputs(w, s, e, body)
	if !ok {
		return
	}

	id := combine.MatchOdin(d.odinCatalog, items[:])

	// Recipe-specific pre-gates (_MSG_CombineItemOdin.cpp:48-90): fail here
	// WITHOUT consuming anything, unlike every other outcome below. The two
	// locals below carry a value each gate already had to compute on to its
	// matching branch further down, instead of recomputing it there.
	//
	// Deviation from Source, not in the plan's three flagged ones but same
	// spirit: id 0 (Composição de Sets, the catch-all "nothing else matched"
	// default) is the one recipe whose pattern places no constraint on
	// Item[1] at all. Source still blindly writes to Carry[m->InvenPos[1]]
	// even when Item[1] was empty and its InvenPos never validated against
	// anything the player actually offered — an arbitrary-slot-overwrite
	// hazard this fork's single-owner/no-dup invariant (CLAUDE.md) can't
	// allow. Require slot 1 to have been a real, validated input.
	if (id == combine.OdinNoMatch || id == combine.OdinItemCelestial) && items[1].Empty() {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	var targetLevel, capeLevel int
	switch id {
	case combine.OdinPlus12:
		targetLevel = refine.Level(items[2])
		if odinSecretaUneven(items[:]) || targetLevel >= 15 || d.itemAbility(items[2], efMobType) == 3 {
			sendCombineComplete(w, s, combineInvalid)
			return
		}
	case combine.OdinDestraveLv40:
		if e.Level != 39 || e.CelLv40 != 0 || e.ClassMaster != classMasterCelestial {
			sendCombineComplete(w, s, combineInvalid)
			return
		}
	case combine.OdinCapaCelestial:
		capeLevel = itemSanc(e.Equip[reinoCapeSlot])
		if e.ClassMaster == classMasterMortal || e.ClassMaster == classMasterArch || capeLevel >= 9 {
			sendCombineComplete(w, s, combineInvalid)
			return
		}
	}

	// Consume every active input slot unconditionally — the ingredients are
	// spent whether or not the recipe below ultimately produces anything
	// (_MSG_CombineItemOdin.cpp:93-100), the same "spent either way" rule the
	// dust refine and Anct combine already follow.
	for _, i := range active {
		e.Carry[slots[i]] = world.Item{}
		sendCarrySlot(w, s, e, slots[i])
	}

	roll := combine.RollOdin(w.Rand())

	switch id {
	case combine.OdinNoMatch, combine.OdinItemCelestial:
		d.odinComposicao(w, s, e, items[0], items[1], slots[1], id, roll)
	case combine.OdinPlus12:
		d.odinPlus12(w, s, e, items[:], slots[2], targetLevel)
	case combine.OdinPistaDeRunas:
		d.odinFreshResult(w, s, e, slots[0], odinPistaDeRunasResult, roll, id)
	case combine.OdinDestraveLv40:
		d.odinDestraveLv40(w, s, e, roll)
	case combine.OdinPedraDaFuria:
		d.odinFreshResult(w, s, e, slots[0], odinPedraDaFuriaResult, roll, id)
	case combine.OdinSecretaAgua, combine.OdinSecretaTerra, combine.OdinSecretaSol, combine.OdinSecretaVento:
		d.odinFreshResult(w, s, e, slots[0], int16(odinSecretaBase+(id-combine.OdinSecretaAgua)), roll, id)
	case combine.OdinSementeDeCristal:
		d.odinFreshResult(w, s, e, slots[0], odinSementeDeCristalResult, roll, id)
	case combine.OdinCapaCelestial:
		d.odinCapaCelestial(w, s, e, roll, capeLevel)
	}
}

// odinSecretaUneven is the "+12+" recipe's four-elemental-stone cross-check
// (_MSG_CombineItemOdin.cpp:50-71): among the 8 slots, the four Secreta
// stones (água/terra/sol/vento, 5334-5337) must be either all absent or all
// present — one, two, or three present alone is rejected.
func odinSecretaUneven(items []world.Item) bool {
	var agua, terra, sol, vento bool
	for _, it := range items {
		switch it.Index {
		case 5334:
			agua = true
		case 5335:
			terra = true
		case 5336:
			sol = true
		case 5337:
			vento = true
		}
	}
	some := agua || terra || sol || vento
	all := agua && terra && sol && vento
	return some && !all
}

// odinComposicao is id 0 ("Composição de Sets") and id 1 ("Composição de
// armas") — the two share an identical shape in Source, differing only in
// their success rate. On success, item[1] is relabeled to catalyst's Extra
// and its sanc reset to 0; on failure item[1] is restored unchanged. Either
// way item[0] (the catalyst/selado) stays consumed.
func (d *Dispatcher) odinComposicao(w *world.World, s *world.Session, e *world.Entity, catalyst, base world.Item, baseSlot int, id, roll int) {
	rate := odinRate[id] + w.Rand().Intn(5)
	if roll > rate {
		e.Carry[baseSlot] = base
		sendCarrySlot(w, s, e, baseSlot)
		sendCombineComplete(w, s, combineFailed)
		return
	}
	result := base
	if extra := d.odinCatalog.Extra[int(catalyst.Index)]; extra > 0 {
		result.Index = int16(extra)
	}
	refine.Set(&result, 0, 0)
	e.Carry[baseSlot] = result
	sendCarrySlot(w, s, e, baseSlot)
	sendCombineComplete(w, s, combineSuccess)
}

// odinPlus12 is id 2, the "+11→+15" refine tier. Per the issue #138 plan
// (the live failure branch is a byte-identical copy of the success branch,
// with the real rate-gated failure entirely commented out in Source), this
// always succeeds once the caller's pre-gate has passed: the accumulated
// rate is still computed and logged (useful for future economy tuning) but
// does not gate the outcome. level is items[2]'s current refine level, already
// computed by the caller's pre-gate.
func (d *Dispatcher) odinPlus12(w *world.World, s *world.Session, e *world.Entity, items []world.Item, targetSlot, level int) {
	rate, protect := combine.Plus12Rate(odinRate[combine.OdinPlus12], items)
	newLevel, ok := combine.Plus12NewLevel(level)
	if !ok {
		newLevel = level
	}
	result := items[2]
	refine.Set(&result, newLevel, refine.Gem(items[2]))
	e.Carry[targetSlot] = result
	sendCarrySlot(w, s, e, targetSlot)
	sendCombineComplete(w, s, combineSuccess)
	d.log.Info("odin +12+ refine", "conn", s.Conn, "from", level, "to", newLevel, "rate", rate, "protect", protect)
}

// odinFreshResult covers the recipes whose result is a bare item stamped
// straight into an already-cleared slot (Pista de runas, Pedra da fúria, the
// 4 Secreta stones, Semente de cristal): Source assigns only .sIndex onto the
// slot the consume loop already zeroed, so — unlike odinComposicao/odinPlus12
// — no prior effects/amount survive. On failure the slot is left empty; none
// of these recipes restores anything (_MSG_CombineItemOdin.cpp:517-650).
func (d *Dispatcher) odinFreshResult(w *world.World, s *world.Session, e *world.Entity, slot int, result int16, roll, id int) {
	if roll > odinRate[id] {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	e.Carry[slot] = world.Item{Index: result}
	sendCarrySlot(w, s, e, slot)
	sendCombineComplete(w, s, combineSuccess)
}

// odinDestraveLv40 is id 4: no item is produced, only the Celestial level-40
// gate flips. Reuses destravarCelestial (chat.go), the same flag /destravar40
// already sets — the caller's pre-gate already confirmed Level==39,
// CelLv40==0 and ClassMaster==Celestial.
func (d *Dispatcher) odinDestraveLv40(w *world.World, s *world.Session, e *world.Entity, roll int) {
	if roll > odinRate[combine.OdinDestraveLv40] {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	d.destravarCelestialFor(w, s, e, false) // sends its own MsgCombineComplete + persists
}

// odinCapaCelestial is id 11: no combine input slot is touched here (they
// were already consumed above) — the equipped cape (Equip[15]) gets its sanc
// bumped by one, bootstrapping a fresh EF_SANC pair first if it has none yet.
// The caller's pre-gate already confirmed the class/cape-sanc requirements and
// computed level, the cape's current refine level.
func (d *Dispatcher) odinCapaCelestial(w *world.World, s *world.Session, e *world.Entity, roll, level int) {
	if roll > odinRate[combine.OdinCapaCelestial] {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	cape := &e.Equip[reinoCapeSlot]
	if !refine.Bootstrap(cape) {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	refine.Set(cape, level+1, 0)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, reinoCapeSlot, itemToSel(*cape)))
	sendCombineComplete(w, s, combineSuccess)
}
