package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Combine result codes for _MSG_CombineComplete.Parm (game-rules.md §3.1).
const (
	combineInvalid = 0 // recipe did not match (inputs NOT consumed)
	combineSuccess = 1
	combineFailed  = 2
)

// Magic constants of the base Anct combine (game-rules.md §3.1/§7).
const (
	jewelBase  = 2441 // joia = Item[1].sIndex - 2441 (0..3)
	resultSanc = 7    // BASE_SetItemSanc(result, 7, 0)
)

// CombineFamily parametrizes one combine system (Anct/Ehre/Tiny/…). The ~9
// variants differ ONLY in Rate (GetMatchCombine<X>) and Apply (the result), so
// one engine handles them all (consolidation, guidelines §19.3).
type CombineFamily struct {
	Name  string
	Rate  func(items []world.Item) int        // recipe rate 0..100; 0 = no match
	Apply func(items []world.Item) world.Item // result item on success
}

// combineItemTypes are the Item[]-based combine variants sharing the engine.
// MsgCombineItemOdin is NOT here — its recipes don't fit the generic
// CombineFamily{Rate,Apply} shape, so it gets its own dedicated handler
// (combine_odin.go) instead.
var combineItemTypes = []protocol.Type{protocol.MsgCombineItem}

// defaultCombineFamily is the UNVERIFIED placeholder used until the recipe/rate
// tables (Common/Settings/CompRate.txt) and ItemList are loaded (Phase 5): every
// recipe is "no match" (Rate 0), so combines report invalid rather than guess.
func defaultCombineFamily(name string) CombineFamily {
	return CombineFamily{Name: name, Rate: func([]world.Item) int { return 0 }, Apply: anctApply}
}

// anctApply is the base Anct result fallback when no content catalog is mounted.
//
// Like combine.AnctResult, the sanc only lands if the base item already carries a
// sanc pair — BASE_SetItemSanc allocates nothing and returns FALSE otherwise
// (Basedef.cpp:2312). This resolves the old "which slot does sanc live in?"
// UNVERIFIED: it is whichever slot already holds EF_SANC or a [116,125] effect,
// scanning 0→1→2, never a fixed Effects[2].
func anctApply(items []world.Item) world.Item {
	result := items[0]
	if len(items) >= 2 {
		if joia := int(items[1].Index) - jewelBase; joia >= 0 && joia <= 3 {
			result.Index = int16(joia)
		}
	}
	refine.Set(&result, resultSanc, 0)
	return result
}

// resolveComboInputs validates a combine packet's claimed inputs against the
// live accessible carry slots (bounds + sameItem anti-cheat) — the skeleton every
// Item[]-based combine variant shares, generic and Odin alike. It returns the
// items and their carry slots still indexed by ORIGINAL combine-message
// position (zero Item/slot where the client left that position empty), plus
// the list of active positions in ascending order. ok is false once the
// caller has already sent a response (RemoveTrade or _NN_Wrong_Combination)
// and must return immediately without consuming anything.
func (d *Dispatcher) resolveComboInputs(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgCombineItemBody) (items [protocol.MaxCombine]world.Item, slots [protocol.MaxCombine]int, active []int, ok bool) {
	active = make([]int, 0, protocol.MaxCombine)
	for i := 0; i < protocol.MaxCombine; i++ {
		if body.Item[i].Index == 0 {
			continue
		}
		pos := int(body.InvenPos[i])
		if !carrySlotAccessible(e, pos) {
			// The one refusal that used to emit nothing at all — not even the bare
			// CombineComplete — so the client's window stayed locked on a machine the
			// server had already given up on. Reachable in ordinary play: a slot of the
			// Bolsa do Andarilho while the bag is not active is out of range.
			d.log.Info("combine recusado: slot inacessível",
				"conn", s.Conn, "pos", pos, "limite", activeCarryLimit(e))
			d.refuseCombine(w, s, msgWrongCombination)
			d.removeTrade(w, s) // out of range → RemoveTrade (anti-cheat)
			return items, slots, active, false
		}
		if !sameItem(body.Item[i], e.Carry[pos]) {
			// This one stayed mute when the other nineteen refusals learned to speak:
			// the substitution that gave them a voice keyed on the line ENDING in the
			// call, and this line carries a trailing comment. It is also the refusal a
			// player is most likely to hit, because it fires whenever the item the
			// client describes differs from the one in the slot by so much as one
			// effect byte.
			//
			// Both sides go in the log because the mismatch is invisible from either
			// alone: the same index with different effects reads as the same item to a
			// person looking at the grid.
			d.log.Info("combine recusado: item difere do inventário",
				"conn", s.Conn, "celula", i, "slot", pos,
				"pacote_index", body.Item[i].Index,
				"pacote_efeitos", wireEffectsForLog(body.Item[i]),
				"bolsa_index", e.Carry[pos].Index,
				"bolsa_efeitos", itemEffectsForLog(e.Carry[pos]))
			d.refuseCombine(w, s, msgWrongCombination)
			return items, slots, active, false
		}
		items[i] = e.Carry[pos]
		slots[i] = pos
		active = append(active, i)
	}
	return items, slots, active, true
}

// combineItem is the shared engine handler for the Item[]-based variants. It
// follows the original ORDER exactly: validate recipe FIRST (invalid ⇒ inputs
// kept), then consume inputs, then roll — so a failed roll still consumes the
// inputs (the intended WYD behaviour, game-rules.md §3.1).
func (d *Dispatcher) combineItem(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	fam, ok := d.combineFamilies[h.Type]
	if !ok {
		return
	}
	var body protocol.MsgCombineItemBody
	if err := body.Decode(payload); err != nil {
		return
	}

	byPos, slotByPos, active, ok := d.resolveComboInputs(w, s, e, body)
	if !ok {
		return
	}
	items := byPos[:]

	rate := 0
	if len(items) > 0 {
		rate = fam.Rate(items)
	}
	if rate == 0 {
		// _NN_Wrong_Combination — inputs are NOT consumed.
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	// The recipe rules are the legacy's; what each sacrifice is worth, and the
	// curve per item tier, are the Mesa das Máquinas'.
	if fam.Name == anctFamilyName {
		if n, ok := combine.AnctSacrifices(d.combineCatalog, items); ok {
			rate = d.composicaoChance(items[0], n)
		}
	}

	// A pile in a machine slot costs one unit, not the pile (combine_pilha.go).
	// Refused before anything is spent when the remainder has nowhere to go.
	if !d.separarUnidadesParaMaquina(w, s, e, slotsAtivosDoCombine(slotByPos[:], active)) {
		d.refuseCombine(w, s, msgPilhaSemEspaco)
		return
	}

	// Consume the inputs BEFORE the roll (lost on failure, by design).
	for _, pos := range active {
		sl := slotByPos[pos]
		e.Carry[sl] = world.Item{}
		sendCarrySlot(w, s, e, sl)
	}

	roll, success := combine.Roll(w.Rand(), rate)
	if !success {
		// Apply is pure — it only builds the result — so naming what the roll
		// would have made costs nothing and draws nothing from the RNG.
		d.announceComposicao(w, e.Name, fam.Apply(items).Index, roll, rate, false)
		sendCombineComplete(w, s, combineFailed)
		return
	}

	ipos := slotByPos[active[0]]
	e.Carry[ipos] = fam.Apply(items)
	d.announceComposicao(w, e.Name, e.Carry[ipos].Index, roll, rate, true)
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, ipos)
}

// combineExtracao handles _MSG_CombineItemExtracao (0x02D4): Huntress extraction
// uses MSG_STANDARDPARM2.Parm2 as the carry slot and consumes one Pedra do Sábio
// catalyst — one unit, not the whole stack: the NPC shop sells it in packs.
func (d *Dispatcher) combineExtracao(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	_, p2, ok := protocol.StandardParm2(payload)
	if !ok {
		return
	}
	slot := int(p2)
	if !carrySlotAccessible(e, slot) {
		return
	}
	it := e.Carry[slot]
	if it.Empty() || int(it.Index) >= world.MaxItem {
		return
	}
	itemLevel := d.itemAbility(it, efItemLevel)
	if itemLevel >= 5 || itemSanc(it) < 9 || d.itemAbility(it, efMobType) != 0 {
		return
	}
	switch d.itemPos[int(it.Index)] {
	case 2, 4, 8, 16, 32:
	default:
		return
	}
	catalyst := -1
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == itemPedraDoSabio {
			catalyst = i
			break
		}
	}
	if catalyst < 0 {
		return
	}
	consumeOneItem(&e.Carry[catalyst])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, catalyst, itemToSel(e.Carry[catalyst])))

	roll := w.Rand().Intn(115)
	if roll > 100 {
		roll -= 15
	}
	rate := d.huntressChance("Extracao", e)
	acao := "extrair " + d.itemName(it.Index)
	// Strictly under, as the legacy compares here — unlike combine.Roll's "at or
	// under". Kept: it is the extraction's own rule, one point either way.
	success := roll < rate
	d.announceRoll(w, e.Name, acao, roll, rate, success)
	if success {
		if it.Effects[1].Effect == efDamage {
			it.Effects[1].Value = addEffectByte(it.Effects[1].Value, d.itemBaseDamage(it))
		}
		if it.Effects[2].Effect == efDamage {
			it.Effects[2].Value = addEffectByte(it.Effects[2].Value, d.itemBaseDamage(it))
		}
		it.Effects[0] = world.Effect{Effect: efItemLevel, Value: uint8(itemLevel)}
		it.Index = extractionResultIndex(d.itemPos[int(it.Index)])
		e.Carry[slot] = it
	} else {
		e.Carry[slot] = world.Item{}
	}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
}

func addEffectByte(v uint8, add int32) uint8 {
	sum := int32(v) + add
	if sum > 255 {
		return 255
	}
	if sum < 0 {
		return 0
	}
	return uint8(sum)
}

func extractionResultIndex(pos int) int16 {
	switch pos {
	case 4:
		return 3022
	case 8:
		return 3023
	case 16:
		return 3024
	case 32:
		return 3025
	default:
		return 3021
	}
}

// sendCombineComplete answers a combine with _MSG_CombineComplete (0x03A7).
//
// The body is MSG_STANDARDPARM — a 4-byte int Parm (Basedef.h:1254-1258), not a
// short: a 2-byte body left the client reading Parm's high half out of the next
// frame. HEADER.ID is ESCENE_FIELD, as SendClientSignalParm sets it
// (SendFunc.cpp:300-310), not the sender's conn.
// wireEffectsForLog / itemEffectsForLog render an item's three effect pairs as
// a flat slice, so the two sides of a mismatch line up when read side by side.
func wireEffectsForLog(wi protocol.WireItem) []int {
	out := make([]int, 0, 6)
	for i := 0; i < 3; i++ {
		out = append(out, int(wi.Effects[i].Effect), int(wi.Effects[i].Value))
	}
	return out
}

func itemEffectsForLog(it world.Item) []int {
	out := make([]int, 0, 6)
	for i := 0; i < 3; i++ {
		out = append(out, int(it.Effects[i].Effect), int(it.Effects[i].Value))
	}
	return out
}

func sendCombineComplete(w *world.World, s *world.Session, parm int32) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgCombineComplete, ID: protocol.IDScene}, protocol.EncodeStandardParm(parm))
}

// refuseCombine is what every machine owed the player and none of them paid.
//
// _MSG_CombineComplete with parm 0 only releases the client's window: nothing is
// drawn, so a refusal looked exactly like a dead button. The original never sends
// it bare — every refusal in every machine is preceded by a SendClientMessage
// naming the reason (_MSG_CombineItemAilyn.cpp:48-49 and :59 are the two for the
// +10 machine). Ten machines shared the same silence here.
func (d *Dispatcher) refuseCombine(w *world.World, s *world.Session, text string) {
	sendClientMessage(w, s, text)
	sendCombineComplete(w, s, combineInvalid)
}

// msgWrongCombination is _NN_Wrong_Combination (Language.txt:271): the recipe on
// the grid is not one the machine knows. It is by far the most common refusal —
// the +10 machine alone wants seven filled cells, two identical items, a Pedra do
// Sábio and four jewels chosen by the item's own grade.
const msgWrongCombination = "Há algo de errado na combinação."

// msgPilhaSemEspaco refuses a machine run whose input holds a pile the bag
// cannot split: one unit goes into the machine and the remainder needs a free
// slot (combine_pilha.go). The alternative is charging the whole pile for one
// attempt, so the refusal says what to do about it.
const msgPilhaSemEspaco = "Sem espaço na bolsa para separar a pilha."

// combineNeedsGold is _DN_D_Cost (Language.txt:204) built with the price, so the
// player learns the number instead of guessing it.
func combineNeedsGold(cost int32) string {
	return fmt.Sprintf("Você precisa de %d Gold.", cost)
}

// msgCombineFailed is _NN_CombineFailed (Language.txt:269), what every machine
// but the Odin composições says when the roll goes against the player.
const msgCombineFailed = "Combinação de item falhou."

// combineSucceeded and combineLost are the outcome twins of refuseCombine.
//
// Giving the refusals a voice left the two outcomes that matter most mute: after
// the roll, success and failure went out as a bare _MSG_CombineComplete, so the
// player watched the items and the gold vanish with no line saying which way it
// went. Every legacy machine names the outcome in the chat right before that
// signal — _MSG_CombineItemAilyn.cpp:128/145 for the +10 — and so does this now.
func combineSucceeded(w *world.World, s *world.Session) {
	sendClientMessage(w, s, msgProcessingComplete)
	sendCombineComplete(w, s, combineSuccess)
}

// combineLost is combineSucceeded's other half; see there.
func combineLost(w *world.World, s *world.Session) {
	sendClientMessage(w, s, msgCombineFailed)
	sendCombineComplete(w, s, combineFailed)
}

// sendCarrySlot pushes one carry slot's current contents to the client.
//
// MSG_SendItem is {short invType; short Slot; STRUCT_ITEM item} (Basedef.h:2037-2046)
// — a 12-byte body. The combine paths used to send a bare slot index instead, so the
// client parsed invType from the slot number and the item from whatever followed.
func sendCarrySlot(w *world.World, s *world.Session, e *world.Entity, slot int) {
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
}

// machineRate resolves a machine's success rate for one item, in the order the
// Mesa das Máquinas defines:
//
//	painel (taxa da chave)  →  CompRate.txt  →  padrão compilado
//
// and then scales it by the band the item's ReqLvl falls in. Every layer is
// optional: with no panel row the file wins, with no band the plain rate wins,
// and a server with no dbServer at all behaves exactly as it did before any of
// this existed.
func (d *Dispatcher) machineRate(family string, target world.Item) int {
	base := int32(d.compRate.ChanceBase(family))
	if v, ok := d.combineRates.Rate(family, "ChanceBase"); ok {
		base = v
	}
	return int(d.combineRates.Apply(base, d.slotKindOf(target), d.reqLvlOf(target)))
}

// On the three machines that announce their rolls, the Mesa das Máquinas holds
// the FINAL chance — the number after the slash in "47/41" — and not a legacy
// base the machine then transforms. What the moderator types is what the
// players read and what the roll compares against.
//
// The +10 moves to a new key because its meaning changed: "Ailyn ChanceBase 10"
// was a base (1 + 4×10 = 41%), and a row saved under the old reading would
// silently become a 10% machine. The Agatha keeps "ChanceBase" because its row
// already said "taxa fixa, igual para qualquer item" on the panel — the machine
// is what changes, to finally do what the screen promised.
const (
	chaveMais10Chance = "Chance"
	chaveAgathaChance = "ChanceBase"
)

// compositorChaves are the compositor's three sacrifice weights, spelled the way
// the panel saves them; the lookup is case-insensitive, so CompRate.txt's
// "ITEM_+7" is the same row.
var compositorChaves = [3]string{"Item_+7", "Item_+8", "Item_+9"}

// mais10Chance is the +10's chance for target: the Mesa's row, or the legacy
// chance for the file's base (41% for "Ailyn ChanceBase 10"), times the band of
// the item's tier.
func (d *Dispatcher) mais10Chance(target world.Item) int {
	chance := int32(combine.AilynLegacyChance(d.compRate.ChanceBase("Ailyn")))
	if v, ok := d.combineRates.Rate("Ailyn", chaveMais10Chance); ok {
		chance = v
	}
	return int(d.combineRates.Apply(chance, d.slotKindOf(target), d.reqLvlOf(target)))
}

// agathaChance is the ADD machine's chance: the Mesa's fixed row when there is
// one, else the legacy base + grade×5 + bonus, which varies by item. No band —
// the ADD is fixed by design.
func (d *Dispatcher) agathaChance(items []world.Item) int {
	if v, ok := d.combineRates.Rate("Agatha", chaveAgathaChance); ok {
		return int(v)
	}
	return combine.AgathaLegacyChance(d.combineCatalog, items, d.compRate.ChanceBase("Agatha"))
}

// composicaoChance is the compositor's chance for a recipe with n sacrifices at
// +7/+8/+9: 1 plus each sacrifice's weight — the Mesa's, else the file's — times
// the compositor's own band for the item being composed.
func (d *Dispatcher) composicaoChance(target world.Item, n [3]int) int {
	weights := d.combineCatalog.AnctChance
	for i, key := range compositorChaves {
		if v, ok := d.combineRates.Rate("Compositor", key); ok {
			weights[i] = int(v)
		}
	}
	chance := int32(combine.AnctChanceFor(n, weights))
	return int(d.combineRates.Apply(chance, combine.CompositorKind(d.slotKindOf(target)), d.reqLvlOf(target)))
}

// odinChaves is the shared list the panel writes (domain.OdinRateKeys), indexed
// by the recipe id.
var odinChaves = domain.OdinRateKeys

// odinChance is an Odin recipe's chance: the Mesa's row, or what the server runs
// today — odinRate, compiled in.
//
// NOT CompRate.txt's Odin lines, although they name the same recipes: those
// were inert in the legacy (the case bug in CReadFiles.cpp) and have never been
// read by this port either, and they disagree with what runs — the file says
// "Item_Celestial 5" where the weapon composition has always rolled against 35.
// Reading them now would cut it to a seventh the day this ships.
//
// fromMesa tells the composições apart: their legacy chance carries a rand()%5
// jitter, and a chance the moderator typed must not wobble.
func (d *Dispatcher) odinChance(id int) (chance int, fromMesa bool) {
	if id < 0 || id >= len(odinChaves) {
		return 0, false
	}
	if v, ok := d.combineRates.Rate("Odin", odinChaves[id]); ok {
		return int(v), true
	}
	return odinRate[id], false
}

// skillChance is the Huntress machines' legacy chance, shared by the Alquimia
// and the Extração: it grows with the character's third skill tree,
// (special + 1) / 6, so it varies by who is standing at the machine.
func skillChance(e *world.Entity) int {
	return (effectiveSpecial(e, 2) + 1) / 6
}

// huntressChance is the Alquimia's or the Extração's chance: the Mesa's fixed
// row, or the skill-driven legacy one.
func (d *Dispatcher) huntressChance(family string, e *world.Entity) int {
	if v, ok := d.combineRates.Rate(family, "Chance"); ok {
		return int(v)
	}
	return skillChance(e)
}

// lindyChance is the Lindy's chance, and whether it rolls at all. The unlock has
// always been certain; only a row on the Mesa makes it a roll, so a server that
// never touches it keeps both the outcome and the RNG stream it has today.
func (d *Dispatcher) lindyChance() (chance int, rolls bool) {
	if v, ok := d.combineRates.Rate("Lindy", "Chance"); ok {
		return int(v), true
	}
	return 100, false
}

// machineKeyRate is machineRate for the families whose rate is named by the
// recipe rather than by "ChanceBase" — the Ehre's seven, where the moderator
// tunes each one on its own. No band applies: these are flat by design.
func (d *Dispatcher) machineKeyRate(family, key string, fallback int) int {
	if v, ok := d.combineRates.Rate(family, key); ok {
		return int(v)
	}
	return fallback
}

// reqLvlOf is the item's equip level, which is the axis the bands are cut on.
// An item the catalog does not know returns 0 and lands in whichever band covers
// zero, or in none at all — never in an error.
func (d *Dispatcher) reqLvlOf(it world.Item) int32 {
	r, ok := d.itemReqs[int(it.Index)]
	if !ok {
		return 0
	}
	return int32(r.Lvl)
}

// slotKindOf reads the item's nPos to decide which band table applies.
func (d *Dispatcher) slotKindOf(it world.Item) combine.SlotKind {
	return combine.SlotKindForPos(int32(d.combineCatalog.Pos[int(it.Index)]))
}
