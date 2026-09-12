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

// separarUnidadesParaMaquina leaves one unit in each machine input slot, moving
// the remainder of any pile to a free bag slot.
//
// It reports false when a remainder has nowhere to go. The caller MUST refuse
// the recipe then, before anything is consumed: the alternative is eating a
// whole pile for one attempt, which is the bug this exists to prevent.
//
// Call it after the inputs are validated and before they are charged. Earlier
// would change the items the client's copy is compared against; later is too
// late, the pile is already gone.
func (d *Dispatcher) separarUnidadesParaMaquina(w *world.World, s *world.Session, e *world.Entity, slots []int) bool {
	for _, sl := range slots {
		if !carrySlotAccessible(e, sl) {
			continue
		}
		n := itemAmount(e.Carry[sl])
		if n <= 1 {
			continue
		}
		livre := firstEmptyAccessibleCarry(e)
		if livre < 0 {
			d.log.Info("machine refused: no room to split a stack",
				"conn", s.Conn, "slot", sl, "item", e.Carry[sl].Index, "amount", n)
			return false
		}
		resto := e.Carry[sl]
		setItemAmount(&resto, n-1)
		e.Carry[livre] = resto

		unidade := e.Carry[sl]
		setItemAmount(&unidade, 1)
		e.Carry[sl] = unidade

		sendCarrySlot(w, s, e, livre)
		sendCarrySlot(w, s, e, sl)
		d.log.Info("machine split a stack",
			"conn", s.Conn, "item", unidade.Index, "slot", sl, "remainder", n-1, "moved_to", livre)
	}
	return true
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
