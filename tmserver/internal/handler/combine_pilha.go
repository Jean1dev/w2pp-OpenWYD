package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// Stacks in a machine slot.
//
// Every machine consumes an input by wiping its slot and writes the result over
// one of those slots. That is exactly the legacy behaviour and it was safe for
// as long as nobody kept a pile in a machine slot — the materials that stack
// (Restos, Poeiras, Âmagos, gems) were carried one per slot because merging them
// was manual work.
//
// The merging fairy (carry.go) ends that: a slot routinely holds 120 Âmagos, and
// a wipe charged all 120 for one attempt.
//
// Rather than teach every recipe about piles — each has its own idea of which
// slot the result lands in — the pile is taken apart before the recipe runs:
// exactly one unit stays in the input slot and the remainder moves to a free
// slot. From there the machine sees what it always saw, a slot holding one item,
// and the player keeps the rest of the pile.

// umaUnidade is the price of an input in almost every recipe: one item.
func umaUnidade(int) int { return 1 }

// separarUnidadesParaMaquina leaves in each machine input slot exactly what the
// recipe is going to spend — precisa(slot) units — and moves the rest of any
// pile to a free bag slot.
//
// The quantity is NOT always one. Three recipes are priced in poeira: the Ehre
// "Misteriosa" (10 of item 413 in cell 2), the Lindy (10 in cells 0 and 1) and
// the Odin +12 (10 in cells 0 and 1). They read the amount in the slot and the
// machine wipes the slot, which is how the legacy charges ten of something —
// leaving a single unit there would hand out the result for a tenth of its
// price. Whatever a recipe demands, it pays.
//
// It reports false when a remainder has nowhere to go. The caller MUST refuse
// the recipe then, before anything is consumed: the alternative is eating a
// whole pile for one attempt, which is the bug this exists to prevent.
//
// Call it after the inputs are validated and before they are charged. Earlier
// would change the items the client's copy is compared against; later is too
// late, the pile is already gone.
func (d *Dispatcher) separarUnidadesParaMaquina(w *world.World, s *world.Session, e *world.Entity, slots []int, precisa func(int) int) bool {
	if precisa == nil {
		precisa = umaUnidade
	}
	for _, sl := range slots {
		if !carrySlotAccessible(e, sl) {
			continue
		}
		n := itemAmount(e.Carry[sl])
		gasta := precisa(sl)
		if gasta < 1 {
			gasta = 1
		}
		// Not enough in the slot to owe a remainder — including the case the
		// recipe wanted ten and found ten, which is the whole slot and needs no
		// splitting at all.
		if n <= gasta {
			continue
		}
		livre := firstEmptyAccessibleCarry(e)
		if livre < 0 {
			d.log.Info("machine refused: no room to split a stack",
				"conn", s.Conn, "slot", sl, "item", e.Carry[sl].Index, "amount", n, "spends", gasta)
			return false
		}
		resto := e.Carry[sl]
		setItemAmount(&resto, n-gasta)
		e.Carry[livre] = resto

		cobrado := e.Carry[sl]
		setItemAmount(&cobrado, gasta)
		e.Carry[sl] = cobrado

		sendCarrySlot(w, s, e, livre)
		sendCarrySlot(w, s, e, sl)
		d.log.Info("machine split a stack",
			"conn", s.Conn, "item", cobrado.Index, "slot", sl,
			"spends", gasta, "remainder", n-gasta, "moved_to", livre)
	}
	return true
}

// precisaDeDez builds a precisa() that charges ten units in the named slots and
// one everywhere else — the shape of the three poeira-priced recipes.
func precisaDeDez(slots ...int) func(int) int {
	return func(sl int) int {
		for _, dez := range slots {
			if sl == dez {
				return 10
			}
		}
		return 1
	}
}

// slotsAtivosDoCombine lists the bag slots a machine run is about to charge, in
// the order the recipe sees them.
func slotsAtivosDoCombine(slotByPos []int, active []int) []int {
	out := make([]int, 0, len(active))
	for _, pos := range active {
		if pos >= 0 && pos < len(slotByPos) {
			out = append(out, slotByPos[pos])
		}
	}
	return out
}
