package handler

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// GM commands over the NPCGener blocks and the mobs they raise — what the
// legacy had as "generate", "create" and "reloadnpc" (imple.cpp:663-686, 842),
// plus a way to find a block's number and to switch it off for good:
//
//	/gm npc [raio]          lists the blocks around you, with their numbers
//	/gm npc off <bloco>     switches a block off: its mobs go, and it stops
//	/gm npc on <bloco>      switches it back on and raises it now
//	/gm gerar <bloco> [aqui] raises the block (at its place, or around you)
//	/gm criar <nome>        creates one mob by template name, around you
//	/gm matar [raio]        kills the monsters around you (default 3)
//	/gm matar bloco <bloco> kills every live mob of a block, wherever it is
//	/gm recarregar          re-reads the database switches and tops up every block
//
// A block's number is its position in NPCGener.txt (Entity.GenIndex); /gm npc
// is how a GM learns it without opening the file.

const (
	gmNPCListRadius    = 10
	gmNPCListMaxRadius = 40
	gmNPCListMaxLines  = 12
	gmKillRadius       = 3
	gmKillMaxRadius    = 40
)

func (d *Dispatcher) gmNPC(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)
	if len(fields) > 0 {
		switch strings.ToLower(fields[0]) {
		case "off", "desligar":
			d.gmNPCSwitch(w, s, fields[1:], true)
			return
		case "on", "ligar":
			d.gmNPCSwitch(w, s, fields[1:], false)
			return
		case "perto":
			fields = fields[1:]
		}
	}
	radius := gmNPCListRadius
	if len(fields) > 0 {
		r, err := strconv.Atoi(fields[0])
		if err != nil || r <= 0 {
			sendClientMessage(w, s, "Uso: /gm npc [raio] | /gm npc off <bloco> | /gm npc on <bloco>")
			return
		}
		radius = min(r, gmNPCListMaxRadius)
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	lines := d.nearbyBlocks(w, e.X, e.Y, radius)
	if len(lines) == 0 {
		sendClientMessage(w, s, fmt.Sprintf("Nenhum mob ou bloco num raio de %d.", radius))
		return
	}
	for i, l := range lines {
		if i == gmNPCListMaxLines {
			sendClientMessage(w, s, fmt.Sprintf("... e mais %d. Diminua o raio.", len(lines)-i))
			break
		}
		sendClientMessage(w, s, l)
	}
}

// nearbyBlocks describes what stands within radius of (x, y), one line per block
// and nearest first: live mobs grouped by the block that raised them, blocks
// switched off whose start is in range (so the GM can find them to switch on),
// and mobs raised by no block, which only /gm matar can remove.
func (d *Dispatcher) nearbyBlocks(w *world.World, x, y int16, radius int) []string {
	type bloco struct {
		idx, n, dist int
		name         string
		x, y         int16
	}
	byIdx := map[int]*bloco{}
	var loose []bloco
	w.ForEachMob(func(id int, m *world.Entity) {
		dist := chebyshev(x, y, m.X, m.Y)
		if dist > radius {
			return
		}
		if m.GenIndex < 0 {
			loose = append(loose, bloco{idx: -id, n: 1, dist: dist, name: m.Name, x: m.X, y: m.Y})
			return
		}
		b := byIdx[int(m.GenIndex)]
		if b == nil {
			b = &bloco{idx: int(m.GenIndex), dist: dist, name: m.Name, x: m.X, y: m.Y}
			byIdx[b.idx] = b
		}
		b.n++
		if dist < b.dist {
			b.dist, b.x, b.y = dist, m.X, m.Y
		}
	})
	all := make([]bloco, 0, len(byIdx)+len(loose))
	for _, b := range byIdx {
		all = append(all, *b)
	}
	for i := 0; i < w.GeneratorCount(); i++ {
		g := w.GeneratorAt(i)
		if g == nil || !g.Off || byIdx[i] != nil {
			continue
		}
		if dist := chebyshev(x, y, g.SegX[0], g.SegY[0]); g.SegX[0] != 0 && dist <= radius {
			all = append(all, bloco{idx: i, dist: dist, name: g.Name, x: g.SegX[0], y: g.SegY[0]})
		}
	}
	all = append(all, loose...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].dist != all[j].dist {
			return all[i].dist < all[j].dist
		}
		return all[i].idx < all[j].idx
	})
	out := make([]string, 0, len(all))
	for _, b := range all {
		switch {
		case b.idx < 0:
			out = append(out, fmt.Sprintf("sem bloco: %s (%d,%d) id %d", b.name, b.x, b.y, -b.idx))
		case b.n == 0:
			out = append(out, fmt.Sprintf("#%d %s (%d,%d) [desligado]", b.idx, b.name, b.x, b.y))
		default:
			tag := ""
			if g := w.GeneratorAt(b.idx); g != nil && g.Off {
				tag = " [desligado]"
			}
			out = append(out, fmt.Sprintf("#%d %s (%d,%d) x%d%s", b.idx, b.name, b.x, b.y, b.n, tag))
		}
	}
	return out
}

// gmNPCSwitch switches a block off or on here and now, then writes it to the
// database so it holds after a restart and on every other server.
func (d *Dispatcher) gmNPCSwitch(w *world.World, s *world.Session, args []string, off bool) {
	verb := "on"
	if off {
		verb = "off"
	}
	if len(args) == 0 {
		sendClientMessage(w, s, "Uso: /gm npc "+verb+" <bloco>  (o número vem do /gm npc)")
		return
	}
	idx, err := strconv.Atoi(args[0])
	g := w.GeneratorAt(idx)
	if err != nil || g == nil {
		sendClientMessage(w, s, fmt.Sprintf("Bloco %q não existe (0..%d).", args[0], w.GeneratorCount()-1))
		return
	}
	d.genOffEpoch++
	if off {
		d.switchGeneratorOff(w, idx)
		sendClientMessage(w, s, fmt.Sprintf("Bloco #%d %s desligado.", idx, g.Name))
	} else {
		d.switchGeneratorOn(w, idx, true)
		sendClientMessage(w, s, fmt.Sprintf("Bloco #%d %s ligado.", idx, g.Name))
	}
	d.log.Info("gm npc "+verb, "account", s.AccountName, "index", idx, "leader", g.Name)
	if d.genOffSource == nil {
		sendClientMessage(w, s, "Sem banco: vale só até reiniciar.")
		return
	}
	src, by := d.genOffSource, s.AccountName
	w.Go(s, func() func(*world.World, *world.Session) {
		ctx, cancel := context.WithTimeout(context.Background(), gmCommandTimeout)
		defer cancel()
		err := src.SetOff(ctx, int32(idx), off, by)
		return func(w *world.World, s *world.Session) {
			if err != nil {
				d.log.Error("gm npc "+verb+" not persisted", "account", s.AccountName, "index", idx, "err", err)
				sendClientMessage(w, s, "Não gravei no banco: vale só até reiniciar. Tente de novo.")
			}
		}
	})
}

// gmGenerate raises one group of a block — the legacy "generate". The block's
// cap still holds: a boss that is alive is not raised twice.
func (d *Dispatcher) gmGenerate(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		sendClientMessage(w, s, "Uso: /gm gerar <bloco> [aqui]")
		return
	}
	idx, err := strconv.Atoi(fields[0])
	g := w.GeneratorAt(idx)
	switch {
	case err != nil || g == nil:
		sendClientMessage(w, s, fmt.Sprintf("Bloco %q não existe (0..%d).", fields[0], w.GeneratorCount()-1))
		return
	case g.Off:
		sendClientMessage(w, s, fmt.Sprintf("Bloco #%d está desligado. Ligue com /gm npc on %d.", idx, idx))
		return
	case g.DBManaged:
		// Raised here it would stand without the panel's shop.
		sendClientMessage(w, s, fmt.Sprintf("Bloco #%d é NPC do painel; ele volta sozinho quando ligado.", idx))
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	var ids []int
	aqui := len(fields) > 1 && strings.EqualFold(fields[1], "aqui")
	if aqui {
		ids = w.GenerateMobNear(idx, e.X, e.Y)
	} else {
		ids = w.GenerateMob(idx)
	}
	if len(ids) == 0 {
		limite := g.MaxNumMob
		if limite < 0 {
			limite = 1 // GenerateMob reads a negative cap as one
		}
		sendClientMessage(w, s, fmt.Sprintf("Nada gerado: #%d tem %d de %d vivos. Use /gm matar bloco %d antes.",
			idx, g.CurrentNumMob, limite, idx))
		return
	}
	d.revealSpawned(w, ids)
	lead := w.Entity(ids[0])
	sendClientMessage(w, s, fmt.Sprintf("Gerados %d de #%d %s em (%d,%d).", len(ids), idx, g.Name, lead.X, lead.Y))
	d.log.Info("gm generate", "account", s.AccountName, "index", idx, "mobs", len(ids), "aqui", aqui)
}

// gmCreate creates one mob from any template a block uses, around the GM — the
// legacy "create" (imple.cpp:675). It belongs to no block and does not come back
// when it dies: this is a one-off for an event, not world population.
func (d *Dispatcher) gmCreate(w *world.World, s *world.Session, rest string) {
	name := firstToken(rest)
	if name == "" {
		sendClientMessage(w, s, "Uso: /gm criar <nome do template>")
		return
	}
	var tmpl []byte
	var tmplName string // the file name, so the one-off drops what the Mesa de Drops says
	var similar []string
	for i := 0; i < w.GeneratorCount(); i++ {
		g := w.GeneratorAt(i)
		if g == nil || g.LeaderTmpl == nil || g.DBManaged {
			continue
		}
		if strings.EqualFold(g.Name, name) {
			tmpl, tmplName = g.LeaderTmpl, g.LeaderName
			break
		}
		if len(similar) < 5 && strings.Contains(strings.ToLower(g.Name), strings.ToLower(name)) &&
			!containsFold(similar, g.Name) {
			similar = append(similar, g.Name)
		}
	}
	if tmpl == nil {
		msg := fmt.Sprintf("Não achei %q no NPCGener.", name)
		if len(similar) > 0 {
			msg += " Parecidos: " + strings.Join(similar, ", ")
		}
		sendClientMessage(w, s, msg)
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	x, y, ok := w.EmptyCellNear(e.X, e.Y)
	if !ok {
		sendClientMessage(w, s, "Não há espaço livre aqui.")
		return
	}
	id := w.SpawnMobAt(world.MobSpawn{Template: tmpl, TemplateName: tmplName, X: x, Y: y, GenIndex: -1})
	if id < 0 {
		sendClientMessage(w, s, "O mundo está cheio.")
		return
	}
	// The template is kept only for the respawn queue; without it the mob dies
	// for good, which is what a one-off from a GM must do.
	w.Entity(id).Template = nil
	d.revealSpawned(w, []int{id})
	sendClientMessage(w, s, fmt.Sprintf("Criado %s em (%d,%d).", w.Entity(id).Name, x, y))
	d.log.Info("gm create", "account", s.AccountName, "template", name, "mob", id)
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// gmKill kills monsters: those around the GM, or every live mob of one block.
// It is a death (removeType 1), so the block's own rules decide whether and when
// they return — to stop that, switch the block off.
//
// Around the GM it spares service NPCs and city guards (those leave with /gm npc
// off) and anybody's summons; naming a block kills what the block raised,
// whatever it is, because the GM asked for exactly that.
func (d *Dispatcher) gmKill(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)
	if len(fields) >= 1 && strings.EqualFold(fields[0], "bloco") {
		if len(fields) < 2 {
			sendClientMessage(w, s, "Uso: /gm matar bloco <bloco>")
			return
		}
		idx, err := strconv.Atoi(fields[1])
		if err != nil || w.GeneratorAt(idx) == nil {
			sendClientMessage(w, s, fmt.Sprintf("Bloco %q não existe (0..%d).", fields[1], w.GeneratorCount()-1))
			return
		}
		n := 0
		w.ForEachMob(func(id int, m *world.Entity) {
			if int(m.GenIndex) == idx {
				w.DespawnMob(id, 1)
				n++
			}
		})
		sendClientMessage(w, s, fmt.Sprintf("Mortos %d do bloco #%d.", n, idx))
		d.log.Info("gm kill block", "account", s.AccountName, "index", idx, "mobs", n)
		return
	}
	radius := gmKillRadius
	if len(fields) >= 1 {
		r, err := strconv.Atoi(fields[0])
		if err != nil || r < 0 {
			sendClientMessage(w, s, "Uso: /gm matar [raio] | /gm matar bloco <bloco>")
			return
		}
		radius = min(r, gmKillMaxRadius)
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	n := 0
	w.ForEachMob(func(id int, m *world.Entity) {
		if chebyshev(e.X, e.Y, m.X, m.Y) > radius || m.Summoner != 0 || m.NonCombatNPC || m.Merchant != 0 {
			return
		}
		w.DespawnMob(id, 1)
		n++
	})
	sendClientMessage(w, s, fmt.Sprintf("Mortos %d num raio de %d.", n, radius))
	d.log.Info("gm kill", "account", s.AccountName, "radius", radius, "mobs", n)
}

// gmReloadNPC is the legacy "reloadnpc" as this server can do it. The file
// itself ships with the deploy, so there is nothing on disk to re-read; what it
// does is re-read the database now (block switches and the NPC panel) and top
// every block up to its cap, bringing back whatever is missing.
func (d *Dispatcher) gmReloadNPC(w *world.World, s *world.Session) {
	d.forceGeneratorOffReload()
	d.forceNPCConfigReload()
	n := 0
	for i := 0; i < w.GeneratorCount(); i++ {
		g := w.GeneratorAt(i)
		if g == nil || g.Off || g.DBManaged || world.IsWaterDungeonGenerator(i) || world.IsEventOwnedGenerator(i) {
			continue
		}
		ids := w.GenerateMob(i)
		d.revealSpawned(w, ids)
		n += len(ids)
	}
	sendClientMessage(w, s, fmt.Sprintf("Recarregado: %d mobs repostos; o banco é relido no próximo segundo.", n))
	d.log.Info("gm reloadnpc", "account", s.AccountName, "mobs", n)
}
