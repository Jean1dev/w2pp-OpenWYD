package world

// A lojinha solta: the personal shop as an entity of its own.
//
// In the legacy the seller WAS the stall — _MSG_SendAutoTrade put his own body in
// the shop pose and RemoveTrade tore it down the moment he moved, which is why
// walking closed the shop (Server.cpp:8124). Here the stall gets its own mob and
// the seller walks away from it.
//
// This is only possible because the client never asks whether a stall is a player.
// MSG_CreateMobTrade copies the title into the entity's own buffer with no id test
// at all, and the click answers _MSG_ReqTradeList with the raw entity id — the
// id < MAX_USER test lives in the OTHER branch, the one taken by entities whose
// title is empty. Both confirmed by disassembly of WYD.exe 7662 (0x0048541D and
// 0x004604D1 respectively).
//
// The shop itself still lives on the owner's Session: the clone is a body, not a
// place to keep state. It sells out of the account Cargo exactly as before, so the
// anti-dup memcmp against the live Cargo slot is untouched.

// SpawnShopClone raises the stall body for the shop owned by conn and returns its
// mob id, or 0 when no clone could be raised — no template configured, no free
// cell, or the world out of mob slots. A 0 return is not an error: the caller
// falls back to the legacy pose, which still works, and merely pins the seller.
//
// Loop-only.
func (w *World) SpawnShopClone(owner int, name string) int {
	if len(w.cfg.ShopCloneTemplate) == 0 {
		return 0
	}
	oe := w.Entity(owner)
	if oe == nil {
		return 0
	}
	// Beside the owner, never on top of him: SetEntityPos would evict whoever
	// holds the cell from the grid, and the cell that is certainly occupied is
	// the one the owner is standing on.
	x, y, ok := w.EmptyCellNear(oe.X, oe.Y)
	if !ok {
		return 0
	}
	id := w.SpawnMobAt(MobSpawn{Template: w.cfg.ShopCloneTemplate, X: x, Y: y, GenIndex: -1})
	if id < 0 {
		return 0
	}
	ce := w.Entity(id)
	ce.ShopOwner = owner
	// The buyer needs to know whose shop this is, and the legacy answered that by
	// the stall being the seller himself. The clone carries his name instead.
	ce.Name = name
	// Merchant is zeroed deliberately, and NonCombatNPC set by hand rather than
	// left to nonCombatNPC(). The template ships Merchant=1, which is a service
	// NPC: clicking it would run the quest-NPC path in _MSG_Quest instead of
	// opening the shop. Zeroing it alone would then make the clone a killable
	// monster — and one carrying a Template, which is what the respawn queue
	// looks for. Setting both says what this thing actually is: untouchable
	// scenery that answers only to the shop messages.
	ce.Merchant = 0
	ce.NonCombatNPC = true
	// A stall does not fight, chase or wander: no route, and its own spawn point
	// as the leash anchor (already set by SpawnMobAt) with nothing to leash to.
	ce.RouteType = 0
	ce.Damage, ce.BaseDamage = 0, 0
	return id
}

// DespawnShopClone removes a stall body raised by SpawnShopClone. It is a no-op
// for 0 (the legacy-pose fallback), for an id that is not a clone, and for a
// clone belonging to someone else — so a stale CloneID can never take down the
// wrong entity.
//
// removeType 0, not 1: the stall is being taken down, not killed. Death is what
// feeds the respawn queue, and a shop that came back fifteen seconds after its
// owner closed it would be a shop nobody owns.
//
// Loop-only.
func (w *World) DespawnShopClone(id, owner int) {
	if id < MaxUser {
		return
	}
	e := w.Entity(id)
	if e == nil || e.ShopOwner != owner {
		return
	}
	w.DespawnMob(id, 0)
}
