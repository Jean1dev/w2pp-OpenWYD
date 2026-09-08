package handler

import (
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/route"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// BM evocations (spell InstanceType 11 → GenerateSummon, Server.cpp:2956).
// A summon is an NPC-slot mob bound to its owner: Clan 4 (the "pet" clan,
// friendly to players), Summoner = the owner's conn, Leader = the owner's
// party leader, RouteType 5. It follows the owner (summonTick), joins its
// fights (commandSummons), and dies when its Type-24 lifespan affect runs out
// or the owner leaves.

// summonClan marks a summoned pet (pMob.MOB.Clan = 4, Server.cpp:3168).
const summonClan = 4

// petsNoPainelDeGrupo decide se o cliente VÊ os pets na lista de grupo.
//
// Os pets continuam na PartyList do líder de qualquer jeito — é lá que o servidor
// guarda quem é pet de quem, e é por ela que andam a defesa do dono, a limpeza
// da re-invocação e a expiração. O que esta chave controla é só o que sai no
// fio.
//
// Desligado porque um BM com Evocação alta enche os doze slots de membro com os
// próprios bichos e não sobra lugar para gente. O legado mostra os pets
// (GenerateSummon manda SendAddParty, Server.cpp:3224-3229), então isto é
// divergência deliberada.
//
// O risco mora do lado do cliente e só o jogo responde: se ele usa a linha de
// grupo para saber que a criatura é aliada, tirá-la pode deixar o dono atacar os
// próprios pets. Por isso a decisão é UMA constante e não uma reescrita —
// voltar é trocar false por true.
const petsNoPainelDeGrupo = false

// affectSummonLife is the summon lifespan affect (Type 24 on a MOB — on a
// player the same type is Samaritano, affectSamaritano in combat.go; the legacy
// overloads it and disambiguates by idx >= MAX_USER, Server.cpp:5843).
const affectSummonLife = 24

// summonLifeTicks is the non-item summon lifespan: 20 affect ticks of 8s
// (Server.cpp:3193-3196). The 28..37 "permanent" mounts are out of scope.
const summonLifeTicks = 20

// Baby mounts (crias, item 2330..2359) are equipment-backed summons: while the
// item is alive in Equip[14], MountProcess creates the corresponding BaseSummon
// entry at sIndex-2320 and keeps it attached to the owner without a lifespan
// affect (Server.cpp:4633-4676, GenerateSummon sItem branch).
const (
	babyMountLo        = 2330
	babyMountHi        = 2360
	babyMountSummonAdd = -2320
	babyMountFaceLo    = 315
	babyMountFaceHi    = 345
)

// summonFollowMax / summonTeleport / summonFollowMin shape the follow AI
// (CMob.cpp:84-152): past 12 the pet warps to the owner, between 5 and 12 it
// walks, at 4 or less it idles. summonLeash drops the pet's fight when the
// owner walks away (CMob.cpp:268-274).
const (
	summonTeleport  = 13
	summonFollowMin = 4
	summonLeash     = 20
)

// summonBonus is the owner-scaling of each evocation: the stat gains
// Int*int/100 + Evo*evo/100 on top of the BaseSummon template
// (Server.cpp:3073-3086; Evo = Evocação = Special[2]).
//
// DELIBERATE DIVERGENCE, em duas frentes.
//
// A parte do Int está ZERADA. O legado (pSummonBonus, Basedef.cpp:745-756) faz
// o Int do dono pesar tanto quanto a Evocação, e num personagem real isso
// desanda: um Mortal 399 tem entre 2.657 e 3.366 de Int, contra um teto de 400
// de Evocação. O Int carregava de 60% a 78% do dano de todas as oito, o que
// deixava a build decidindo mais que a maestria — dois BMs com a mesma Evocação
// e Int diferente tinham bichos incomparáveis, e nenhum alvo de balanceamento
// era atingível enquanto esse termo existisse.
//
// Os multiplicadores de Evocação foram resolvidos para os alvos por unidade
// escolhidos por quem opera, medidos em Evocação 320. A progressão do legado
// estava invertida: o Tigre (magia de nível 84) entregava mais dano total que o
// Dragão (102) e quase quatro vezes a Succubus (220).
var summonBonus = [9]struct {
	damInt, damEvo int32
	acInt, acEvo   int32
	hpInt, hpEvo   int32
}{
	// Int  Dano   Int    AC   Int     HP        alvo por unidade @ Evocação 320
	{0, 145, 0, 120, 0, 294},   // 0 Condor      dano 500, AC 400, HP 1.000
	{0, 83, 0, 369, 0, 1219},   // 1 Javali      dano 300, AC 1.200, HP 4.000
	{0, 291, 0, 206, 0, 594},   // 2 Lobo        dano 1.000, AC 700, HP 2.000
	{0, 88, 0, 419, 0, 1531},   // 3 Urso        dano 350, AC 1.400, HP 5.000
	{0, 445, 0, 241, 0, 719},   // 4 Tigre       dano 1.500, AC 800, HP 2.400
	{0, 359, 0, 298, 0, 875},   // 5 Gorila      dano 1.200, AC 1.000, HP 3.000
	{0, 594, 0, 350, 0, 984},   // 6 Dragão      dano 2.000, AC 1.200, HP 3.500
	{0, 1203, 0, 278, 0, 1238}, // 7 Succubus    dano 4.000, AC 1.000, HP 4.200
	{0, 0, 0, 0, 0, 0},         // 8 Invocação Final: sem escalonamento nenhum
}

// summonHeads é quantas unidades cada criatura põe em campo, por Evocação.
//
// DELIBERATE DIVERGENCE: o legado agrupa as oito em três divisores
// (_MSG_Attack.cpp:817-828) — ÷30, ÷40, ÷80 —, e o resultado é uma progressão
// invertida, com o Condor de nível 33 saindo dez a dez e a Succubus de nível 220
// saindo sozinha. Aqui cada criatura tem o seu divisor e o seu TETO, escolhidos
// por quem opera, e o teto é o que impede uma Evocação alta de estourar o
// desenho: a conta cresce com a maestria até o número da criatura e para ali.
//
// Os divisores são calibrados para o teto cair exatamente em Evocação 320.
var summonHeads = [9]struct{ divisor, teto int }{
	{26, 12}, // 0 Condor
	{32, 10}, // 1 Javali
	{32, 10}, // 2 Lobo
	{35, 9},  // 3 Urso
	{40, 8},  // 4 Tigre
	{53, 6},  // 5 Gorila
	{64, 5},  // 6 Dragão Negro
	{80, 4},  // 7 Succubus
	{0, 1},   // 8 Invocação Final: sempre uma, sem depender da maestria
}

// summonCount é quantas unidades um lançamento põe em campo.
//
// instanceValue vem da linha do SkillData e o índice da criatura é ele menos um
// (_MSG_Attack.cpp:830), então o teto acompanha o BICHO e não o nome da magia —
// o que importa porque as skills 59 e 60 estão cruzadas com as criaturas no
// conteúdo do legado.
func summonCount(instanceValue, evocacao int) int {
	i := instanceValue - 1
	if i < 0 || i >= len(summonHeads) {
		return 0
	}
	h := summonHeads[i]
	if h.divisor <= 0 {
		return h.teto
	}
	n := evocacao / h.divisor
	if n > h.teto {
		n = h.teto
	}
	if n < 0 {
		n = 0
	}
	return n
}

// freeCellNear scans the rings around (x,y) for an unoccupied, walkable cell,
// matching GetEmptyMobGrid's 1..4 square expansion order. It deliberately
// consumes no RNG so the attack flow's rand-call parity is preserved.
func (d *Dispatcher) freeCellNear(w *world.World, x, y int16) (int16, int16, bool) {
	for ring := 1; ring <= 4; ring++ {
		for dy := -ring; dy <= ring; dy++ {
			for dx := -ring; dx <= ring; dx++ {
				if abs16(int16(dx)) != ring && abs16(int16(dy)) != ring {
					continue // interior cell: already scanned on a smaller ring
				}
				nx, ny := x+int16(dx), y+int16(dy)
				if d.cellAvailable(w, nx, ny) {
					return nx, ny, true
				}
			}
		}
	}
	return 0, 0, false
}

func (d *Dispatcher) freeCellAtOrNear(w *world.World, x, y int16) (int16, int16, bool) {
	if d.cellAvailable(w, x, y) {
		return x, y, true
	}
	return d.freeCellNear(w, x, y)
}

func (d *Dispatcher) cellAvailable(w *world.World, x, y int16) bool {
	if x < 0 || y < 0 || int(x) >= w.GridDim() || int(y) >= w.GridDim() {
		return false
	}
	if _, occupied := w.EntityAt(x, y); occupied {
		return false
	}
	// The baked grid marks impassable cells with route.Blocked (127); fine-grained
	// slope checks stay with the walker (route.Next).
	return d.heights == nil || d.heights.At(int(x), int(y)) != route.Blocked
}

// generateSummon ports GenerateSummon (Server.cpp:2956) for the evocations
// (no sItem/mount path): spawn count copies of template summonID around the
// caster, bound to it. Returns false when NOTHING was spawned (the cast then
// refunds its mana, _MSG_Attack.cpp:830-834). Partial success is true.
//
// The item-backed mount recall path uses refreshBabyMountSummon below; this
// function stays on the spell/sItem==nil branch.
func (d *Dispatcher) generateSummon(w *world.World, s *world.Session, e *world.Entity, summonID, count int) bool {
	if count <= 0 || summonID < 0 || summonID >= len(d.summonMobs) || summonID >= len(summonBonus) || d.summonMobs[summonID] == nil {
		return false
	}
	leaderID := e.Leader
	if leaderID == 0 {
		leaderID = s.Conn
	}
	le := w.Entity(leaderID)
	if le == nil {
		return false
	}
	face := summonTemplateFace(d.summonMobs[summonID])
	// A lineage already out blocks a different one, as in the legacy: the roster
	// is one creature at a time (GenerateSummon, Server.cpp:2991-2997).
	for _, memberID := range le.PartyList {
		if memberID < world.MaxUser {
			continue
		}
		pet := w.Entity(memberID)
		if pet == nil || pet.Clan != summonClan {
			continue
		}
		if pet.EquipVisual[0] != face {
			return false
		}
	}
	// DELIBERATE DIVERGENCE: re-casting wipes what is out and summons the whole
	// set again beside the caster.
	//
	// The legacy tops up instead — it counts what survives, teleports the strays
	// back to the owner (the Effect=8 recall jump, Server.cpp:3008-3020) and
	// spawns only the difference. This port never carried that recall, so a
	// re-cast could not bring anything home: the set drifted across the map and
	// the only way to regroup it was to let the pets time out. Topping up on top
	// of that reads as a leak — cast, walk away, cast again, and the party fills
	// with strays nobody can reach.
	//
	// Re-summoning whole is what the recall was reaching for and is what the cast
	// looks like it does: the creatures you just paid for are the ones standing
	// next to you. It also makes the head count mean something, because the set
	// is always exactly `count` and never an accumulation.
	d.despawnSummons(w, leaderID, func(pet *world.Entity) bool {
		return pet.Summoner == s.Conn && pet.EquipVisual[0] == face
	})

	// What survives are the pets of OTHER members, and they still count against
	// this cast: the head count belongs to the party, not to the caster
	// (GenerateSummon walks the LEADER's whole PartyList, Server.cpp:2981-3027).
	// Two evokers together share one set — recounted here, after the wipe, so the
	// caster's own creatures are not counted against themselves.
	existing := 0
	for _, memberID := range le.PartyList {
		if memberID < world.MaxUser {
			continue
		}
		pet := w.Entity(memberID)
		if pet == nil || pet.Clan != summonClan || pet.EquipVisual[0] != face {
			continue
		}
		existing++
	}
	if existing >= count {
		return false
	}
	bonus := summonBonus[summonID]
	spawned := 0
	for i := existing; i < count; i++ {
		slot := -1
		for j := range le.PartyList {
			if le.PartyList[j] == 0 {
				slot = j
				break
			}
		}
		if slot < 0 {
			break // party full (_NN_Party_Full_Cant_Summon; message UNVERIFIED — skipped)
		}
		x, y, ok := d.freeCellNear(w, e.X, e.Y)
		if !ok {
			break
		}
		id := w.SpawnMobAt(world.MobSpawn{Template: d.summonMobs[summonID], X: x, Y: y, RouteType: 5, GenIndex: -1})
		if id < 0 {
			break // world full (_NN_Cant_Create_More_Summons — skipped)
		}
		mob := w.Entity(id)
		// The '^' suffix marks the name as a pet on the client; underscores in
		// the template file names render as spaces (Server.cpp:3063-3070).
		mob.Name = strings.ReplaceAll(mob.Name, "_", " ") + "^"
		mob.Level = e.Level
		if mob.Level > level.MaxLevel {
			mob.Level = level.MaxLevel
		}
		ownerInt := int32(e.Int)
		evo := int32(effectiveSpecial(e, 2))
		mob.Damage += ownerInt*bonus.damInt/100 + evo*bonus.damEvo/100
		mob.AC += ownerInt*bonus.acInt/100 + evo*bonus.acEvo/100
		mob.MaxHP += ownerInt*bonus.hpInt/100 + evo*bonus.hpEvo/100
		mob.HP = mob.MaxHP
		mob.Clan = summonClan
		mob.Leader = leaderID
		mob.Summoner = s.Conn
		// A summon is combat content, whatever its template's Merchant byte says.
		// Every BaseSummon ships Merchant=16 (Dragao_Negro 64), and nonCombatNPC
		// reads ANY non-zero value as a service NPC — which took the pet out of
		// runsMobAI entirely (mobai.go:189). A pet outside the AI loop never
		// attacks and never ticks its own lifespan affect, so it also never
		// expires: one cause, both halves of the bug.
		//
		// The legacy protects only Merchant 1/4/43/100 (_MSG_Attack.cpp:339), so
		// none of these ever qualified there. Cleared here and not in
		// nonCombatNPC because widening that rule would un-shield ~376 spawn
		// blocks at once — the same call the water dungeon deferred (world/api.go).
		mob.NonCombatNPC = false
		mob.Affect[0] = world.Affect{Type: affectSummonLife, Time: summonLifeTicks}
		le.PartyList[slot] = id
		if petsNoPainelDeGrupo {
			if spawned == 0 {
				d.sendAddParty(w, leaderID, leaderID, 0)
			}
			d.sendSummonPartySlot(w, leaderID, id, slot+1)
		}

		// Reveal with CreateType|=3 (the summon-appear effect, Server.cpp:3210-3218).
		body := protocol.EncodeCreateMobBody(createMobFrom(mob, 3))
		w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
			if w.MarkSeen(vs, id) {
				w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
			}
		})
		spawned++
	}
	return spawned > 0
}

func (d *Dispatcher) refreshBabyMountSummon(w *world.World, s *world.Session, e *world.Entity) {
	d.removeBabyMountSummons(w, s.Conn, e)

	mount := e.Equip[mountEquipSlot]
	if !isBabyMount(mount) || mountHP(mount) <= 0 {
		return
	}
	summonID := int(mount.Index) + babyMountSummonAdd
	d.generateBabyMountSummon(w, s, e, summonID, mount)
}

func isBabyMount(it world.Item) bool {
	return it.Index >= babyMountLo && it.Index < babyMountHi
}

func mountHP(it world.Item) int16 {
	return int16(uint16(it.Effects[0].Effect) | uint16(it.Effects[0].Value)<<8)
}

func (d *Dispatcher) removeBabyMountSummons(w *world.World, ownerID int, owner *world.Entity) {
	leaderID := owner.Leader
	if leaderID == 0 {
		leaderID = ownerID
	}
	d.despawnSummons(w, leaderID, func(pet *world.Entity) bool {
		return pet.Summoner == ownerID &&
			pet.EquipVisual[0] >= babyMountFaceLo && pet.EquipVisual[0] < babyMountFaceHi
	})
}

// despawnSummonsOf removes every summon parked in leaderID's PartyList that
// belongs to ownerID; ownerID == 0 means all of them (the party dissolved).
// This is the DeleteMob sweep inside RemoveParty (Server.cpp:8185-8190 when a
// member leaves, Server.cpp:8237-8243 when the leader does) — without it the
// pets outlive the party bond that held them (issue #234).
func (d *Dispatcher) despawnSummonsOf(w *world.World, leaderID, ownerID int) {
	d.despawnSummons(w, leaderID, func(pet *world.Entity) bool {
		// Summoner 0 is a stray mob sitting in a player's party slot; the legacy
		// reaps those on any leave too (Server.cpp:8188-8190).
		return ownerID == 0 || pet.Summoner == ownerID || pet.Summoner == 0
	})
}

// despawnSummons is the shared walk of a leader's pet slots: match selects which
// pets go. The ids and the packet recipients are snapshotted first because
// DespawnMob frees the slot as it goes (world/api.go:215-223).
func (d *Dispatcher) despawnSummons(w *world.World, leaderID int, match func(*world.Entity) bool) {
	leader := w.Entity(leaderID)
	if leader == nil {
		return
	}
	var doomed []int
	recipients := []int{leaderID}
	for _, memberID := range leader.PartyList {
		if memberID <= 0 {
			continue
		}
		if world.IsPlayer(memberID) {
			recipients = append(recipients, memberID)
			continue
		}
		if pet := w.Entity(memberID); pet != nil && match(pet) {
			doomed = append(doomed, memberID)
		}
	}
	for _, id := range doomed {
		// DespawnMob only sends MsgRemoveMob, so the party row has to be dropped
		// explicitly or the client keeps the pet slot (legacy gets this from
		// DeleteMob → RemoveParty, Server.cpp:7840). Skipped when the rows were
		// never sent: there is nothing on the client to drop.
		if petsNoPainelDeGrupo {
			for _, recipientID := range recipients {
				d.sendRemoveParty(w, recipientID, id)
			}
		}
		w.DespawnMob(id, 3)
	}
}

func (d *Dispatcher) generateBabyMountSummon(w *world.World, s *world.Session, e *world.Entity, summonID int, mount world.Item) bool {
	if summonID < 0 || summonID >= len(d.summonMobs) || d.summonMobs[summonID] == nil {
		return false
	}
	leaderID := e.Leader
	if leaderID == 0 {
		leaderID = s.Conn
	}
	leader := w.Entity(leaderID)
	if leader == nil {
		return false
	}
	slot := -1
	for i := range leader.PartyList {
		if leader.PartyList[i] == 0 {
			slot = i
			break
		}
	}
	if slot < 0 {
		return false
	}
	x, y, ok := d.freeCellNear(w, e.X, e.Y)
	if !ok {
		return false
	}
	id := w.SpawnMobAt(world.MobSpawn{Template: d.summonMobs[summonID], X: x, Y: y, RouteType: 5, GenIndex: -1})
	if id < 0 {
		return false
	}
	mob := w.Entity(id)
	mob.Name = strings.ReplaceAll(mob.Name, "_", " ") + "^"
	mob.Level = e.Level
	if mob.Level > level.MaxLevel {
		mob.Level = level.MaxLevel
	}
	mob.Clan = summonClan
	mob.Leader = leaderID
	mob.Summoner = s.Conn
	// Same Merchant-byte trap as the evocations; see generateSummon.
	mob.NonCombatNPC = false
	mountSanc := int32(mount.Effects[1].Effect)
	if mountSanc > 100 {
		mountSanc = 100
	}
	mob.Damage += 6 * mountSanc
	hp := int32(mountHP(mount))
	if mob.MaxHP > 0 && hp > mob.MaxHP {
		hp = mob.MaxHP
	}
	mob.HP = hp
	leader.PartyList[slot] = id
	if petsNoPainelDeGrupo {
		d.sendAddParty(w, leaderID, leaderID, 0)
		d.sendSummonPartySlot(w, leaderID, id, slot+1)
	}

	body := protocol.EncodeCreateMobBody(createMobFrom(mob, 3))
	w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
		if w.MarkSeen(vs, id) {
			w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
		}
	})
	return true
}

func summonTemplateFace(template []byte) uint16 {
	eq := protocol.MobEquip(template)
	return eq[0].Index
}

func (d *Dispatcher) sendSummonPartySlot(w *world.World, leaderID, summonID, slot int) {
	sent := map[int]bool{}
	send := func(recipientID int) {
		if sent[recipientID] {
			return
		}
		sent[recipientID] = true
		d.sendAddParty(w, recipientID, summonID, slot)
	}
	send(leaderID)
	if leader := w.Entity(leaderID); leader != nil {
		for _, memberID := range leader.PartyList {
			if memberID > 0 && world.IsPlayer(memberID) {
				send(memberID)
			}
		}
	}
}

// petIsListed reports whether the pet still holds a slot in its leader's
// PartyList — the only registry summons have (generateSummon, commandSummons).
func petIsListed(w *world.World, id int, e *world.Entity) bool {
	leader := w.Entity(e.Leader)
	if leader == nil {
		return false
	}
	return hasMember(leader, id)
}

// summonTick replaces the regular mob AI for a summoned pet (the RouteType-5
// branch of StandingByProcessor, CMob.cpp:84-152): despawn when the owner is
// gone, tick the lifespan, fight the owner's fight within the leash, otherwise
// follow.
func (d *Dispatcher) summonTick(w *world.World, id int, e *world.Entity) {
	owner := w.Entity(e.Summoner)
	ownerGone := owner == nil || owner.HP <= 0
	if !ownerGone {
		if m, ok := w.SessionMode(e.Summoner); !ok || m != world.UserPlay {
			ownerGone = true // logout / back to char select / disconnect
		}
	}
	if !ownerGone && !petIsListed(w, id, e) {
		// Belt and braces: a pet whose party slot is gone has no owner bond left,
		// and nothing else would ever reap it (its lifespan ticks right here). The
		// legacy bails the same way when a summon's leader is cleared
		// (CMob.cpp:118-124). Keeps any future PartyList reset from leaking pets.
		ownerGone = true
	}
	if ownerGone {
		// The legacy clears summons via its timers on death/logout
		// (ProcessSecMinTimer.cpp:2381,2498); this lazy per-tick check covers
		// every exit path in one place. Owner-death despawn is UNVERIFIED
		// timing-wise (the original may keep the pet until the timer).
		d.despawnPet(w, id, e, 3)
		return
	}
	// Lifespan: the Type-24 affect counts down on the same 8s phase the player
	// sweep uses; at zero the pet vanishes (DeleteMob(idx,3), Server.cpp:5843).
	// Swept here rather than in sweepAffects so the 10k-mob scan stays free of
	// affect work for everything that isn't a summon.
	if d.tickCount%affectTickPeriod == id%affectTickPeriod {
		if af := &e.Affect[0]; af.Type == affectSummonLife && af.Time < affectInfiniteTime {
			if af.Time > 0 {
				af.Time--
			}
			if af.Time == 0 {
				d.despawnPet(w, id, e, 3)
				return
			}
		}
	}
	if e.Target != 0 {
		// Fight, but never further than the leash from the owner: past it the
		// pet drops the fight and returns (CMob.cpp:268-274).
		if mobDistance(e.X, e.Y, owner.X, owner.Y) >= summonLeash {
			e.Target = 0
			clearEnemyList(e)
			e.Mode = world.MobIdle
		} else {
			d.mobBattle(w, id, e)
			return
		}
	}
	dis := mobDistance(e.X, e.Y, owner.X, owner.Y)
	switch {
	case dis >= summonTeleport:
		// Warp to the owner's side (the recall jump: MSG_Action Effect=8 Speed=6,
		// Server.cpp:3008-3020) and re-reveal for players who never saw it.
		x, y, ok := d.freeCellNear(w, owner.X, owner.Y)
		if !ok {
			return
		}
		oldX, oldY := e.X, e.Y
		w.SetEntityPos(id, x, y)
		body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Speed: 6, TargetX: x, TargetY: y, Effect: 8}
		d.moveMulticast(w, id, oldX, oldY, protocol.MsgAction, body.Encode())
	case dis > summonFollowMin:
		d.stepToward(w, id, e, owner.X, owner.Y)
	}
}

// commandSummons points the owner's idle pets at target — the stand-in for the
// legacy EnemyList propagation that made pets assist (attack what the owner
// attacks, defend when the owner is struck). Same ±23 engage box as the mob
// group drag (setGroupBattle).
func (d *Dispatcher) commandSummons(w *world.World, ownerID int, target *world.Entity) {
	owner := w.Entity(ownerID)
	if owner == nil || target == nil {
		return
	}
	leaderID := owner.Leader
	if leaderID == 0 {
		leaderID = ownerID
	}
	le := w.Entity(leaderID)
	if le == nil {
		return
	}
	for _, m := range le.PartyList {
		if m < world.MaxUser {
			continue // empty slot or player member
		}
		pet := w.Entity(m)
		// No `pet.Target != 0` here, on purpose. The legacy calls SetBattle on
		// EVERY live party member whatever it is doing (Server.cpp:9964-9985 when
		// a mob strikes the owner, _MSG_Attack.cpp:1690-1720 when the owner
		// strikes), and SetBattle only ADDS to the EnemyList — the pick is left to
		// target selection, which takes the nearest.
		//
		// Skipping a busy pet is what made the set look deaf on defence: once the
		// pets have something to hit they always have a target, so the owner's
		// attacker never reached their list and nobody ever turned around. Idle
		// pets defended, which is why the test caught nothing — it only ever had
		// idle ones.
		if pet == nil || pet.Summoner != ownerID || pet.HP <= 0 {
			continue
		}
		if abs16(pet.X-target.X) <= battleDragBox && abs16(pet.Y-target.Y) <= battleDragBox {
			setBattle(w, m, pet, target)
		}
	}
}

// despawnPet takes a summon out of the world AND out of the party rows its
// owner's client is drawing.
//
// DespawnMob alone is not enough. It frees the server-side PartyList slot and
// sends MsgRemoveMob, but the client keeps the row in the group panel — the
// legacy gets the removal from DeleteMob → RemoveParty (Server.cpp:7840), which
// this port only ever did on the despawnSummons path.
//
// Every other way a pet leaves — its lifespan running out, the owner walking
// off, a monster killing it — went straight to DespawnMob, so the rows piled up.
// A player saw creatures that were already gone, the panel filled to its twelve
// slots, and both halves read as bugs in the summon itself: "they never expire"
// and "they keep accumulating". Server-side the count was right the whole time,
// which is why nothing in the tests ever caught it — the tests read the world,
// and the lie was only ever on the screen.
func (d *Dispatcher) despawnPet(w *world.World, id int, pet *world.Entity, removeType int32) {
	if petsNoPainelDeGrupo && pet != nil && pet.Summoner != 0 {
		leaderID := pet.Leader
		if leaderID == 0 {
			leaderID = pet.Summoner
		}
		d.sendRemoveParty(w, pet.Summoner, id)
		if leader := w.Entity(leaderID); leader != nil {
			d.sendRemoveParty(w, leaderID, id)
			for _, memberID := range leader.PartyList {
				if memberID > 0 && world.IsPlayer(memberID) {
					d.sendRemoveParty(w, memberID, id)
				}
			}
		}
	}
	w.DespawnMob(id, removeType)
}
