package handler

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Commands over the NPCGener blocks and the mobs they raise — what the legacy
// had as "generate", "create" and "reloadnpc" (imple.cpp:663-686, 842), plus a
// way to find a block's number and to switch it off for good:
//
//	npc [raio]            lists the blocks around you, with their numbers
//	npc off <bloco>       switches a block off: its mobs go, and it stops
//	npc on <bloco>        switches it back on and raises it now
//	gerar <bloco> [aqui]  raises the block (at its place, or around you)
//	criar <nome>          creates one mob by template name, around you
//	matar [raio]          kills the monsters around you (default 3)
//	matar bloco <bloco>   kills every live mob of a block, wherever it is
//	recarregar            re-reads the database switches and tops up every block
//
// The same commands run from two places: typed in game after "/gm", and sent by
// the staff panel through the control API (RunBlockCommand). Each one answers
// with lines of text; the game shows them to the GM, the panel on the page.
// "Around you" is the GM's tile in game and the coordinates typed in the panel.
//
// A block's number is its position in NPCGener.txt (Entity.GenIndex).

const (
	gmNPCListRadius    = 10
	gmNPCListMaxRadius = 40
	gmNPCListMaxLines  = 12
	gmKillRadius       = 3
	gmKillMaxRadius    = 40
)

// blocoOrigem is who runs a block command and where "around you" is.
type blocoOrigem struct {
	by   string
	x, y int16
	s    *world.Session // the GM's session in game; nil from the panel
}

// gmBloco runs a block command typed in game and shows the answer to the GM.
func (d *Dispatcher) gmBloco(w *world.World, s *world.Session, sub, rest string) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	for _, l := range d.blocoCmd(w, blocoOrigem{by: s.AccountName, x: e.X, y: e.Y, s: s}, sub+" "+rest) {
		sendClientMessage(w, s, l)
	}
}

// RunBlockCommand runs a block command for the staff panel, around (x, y), and
// returns its answer. Called INSIDE the game loop (control.BlockCommand).
func (d *Dispatcher) RunBlockCommand(w *world.World, by string, x, y int16, line string) []string {
	return d.blocoCmd(w, blocoOrigem{by: by, x: x, y: y}, line)
}

func (d *Dispatcher) blocoCmd(w *world.World, o blocoOrigem, line string) []string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	sub, args := strings.ToLower(fields[0]), fields[1:]
	d.log.Info("block command", "by", o.by, "in_game", o.s != nil, "line", line, "x", o.x, "y", o.y)
	switch sub {
	case "npc":
		return d.npcCmd(w, o, args)
	case "gerar", "generate":
		return d.gerarCmd(w, o, args)
	case "criar", "create":
		return d.criarCmd(w, o, args)
	case "matar", "kill":
		return d.matarCmd(w, o, args)
	case "recarregar", "reloadnpc":
		return d.recarregarCmd(w)
	}
	return []string{fmt.Sprintf("Comando %q não existe.", sub)}
}

func (d *Dispatcher) npcCmd(w *world.World, o blocoOrigem, args []string) []string {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "off", "desligar":
			return d.npcSwitchCmd(w, o, args[1:], true)
		case "on", "ligar":
			return d.npcSwitchCmd(w, o, args[1:], false)
		case "perto":
			args = args[1:]
		}
	}
	radius := gmNPCListRadius
	if len(args) > 0 {
		r, err := strconv.Atoi(args[0])
		if err != nil || r <= 0 {
			return []string{"Uso: npc [raio] | npc off <bloco> | npc on <bloco>"}
		}
		radius = min(r, gmNPCListMaxRadius)
	}
	lines := d.nearbyBlocks(w, o.x, o.y, radius)
	if len(lines) == 0 {
		return []string{fmt.Sprintf("Nenhum mob ou bloco num raio de %d.", radius)}
	}
	if len(lines) > gmNPCListMaxLines {
		extra := len(lines) - gmNPCListMaxLines
		lines = append(lines[:gmNPCListMaxLines], fmt.Sprintf("... e mais %d. Diminua o raio.", extra))
	}
	return lines
}

// nearbyBlocks describes what stands within radius of (x, y), one line per block
// and nearest first: live mobs grouped by the block that raised them, blocks
// switched off whose start is in range (so the GM can find them to switch on),
// and mobs raised by no block, which only matar can remove.
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

// npcSwitchCmd switches a block off or on here and now, then writes it to the
// database so it holds after a restart and on every other server.
func (d *Dispatcher) npcSwitchCmd(w *world.World, o blocoOrigem, args []string, off bool) []string {
	verb := "on"
	if off {
		verb = "off"
	}
	if len(args) == 0 {
		return []string{"Uso: npc " + verb + " <bloco>  (o número vem da lista npc)"}
	}
	idx, err := strconv.Atoi(args[0])
	g := w.GeneratorAt(idx)
	if err != nil || g == nil {
		return []string{fmt.Sprintf("Bloco %q não existe (0..%d).", args[0], w.GeneratorCount()-1)}
	}
	d.genOffEpoch++
	var out []string
	if off {
		d.switchGeneratorOff(w, idx)
		out = append(out, fmt.Sprintf("Bloco #%d %s desligado.", idx, g.Name))
	} else {
		d.switchGeneratorOn(w, idx, true)
		out = append(out, fmt.Sprintf("Bloco #%d %s ligado.", idx, g.Name))
	}
	d.log.Info("npc "+verb, "by", o.by, "index", idx, "leader", g.Name)
	if d.genOffSource == nil {
		return append(out, "Sem banco: vale só até reiniciar.")
	}
	src, by := d.genOffSource, o.by
	persist := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), gmCommandTimeout)
		defer cancel()
		return src.SetOff(ctx, int32(idx), off, by)
	}
	if o.s != nil {
		w.Go(o.s, func() func(*world.World, *world.Session) {
			err := persist()
			return func(w *world.World, s *world.Session) {
				if err != nil {
					d.log.Error("npc "+verb+" not persisted", "by", by, "index", idx, "err", err)
					sendClientMessage(w, s, "Não gravei no banco: vale só até reiniciar. Tente de novo.")
				}
			}
		})
		return out
	}
	// From the panel there is no session to tell later; the page reloads the
	// list, which reads the database state through the game's poll.
	w.GoDetached(func() func(*world.World) {
		err := persist()
		return func(*world.World) {
			if err != nil {
				d.log.Error("npc "+verb+" not persisted", "by", by, "index", idx, "err", err)
			}
		}
	})
	return out
}

// gerarCmd raises one group of a block — the legacy "generate". The block's cap
// still holds: a boss that is alive is not raised twice.
func (d *Dispatcher) gerarCmd(w *world.World, o blocoOrigem, args []string) []string {
	if len(args) == 0 {
		return []string{"Uso: gerar <bloco> [aqui]"}
	}
	idx, err := strconv.Atoi(args[0])
	g := w.GeneratorAt(idx)
	switch {
	case err != nil || g == nil:
		return []string{fmt.Sprintf("Bloco %q não existe (0..%d).", args[0], w.GeneratorCount()-1)}
	case g.Off:
		return []string{fmt.Sprintf("Bloco #%d está desligado. Ligue com npc on %d.", idx, idx)}
	case g.DBManaged:
		// Raised here it would stand without the panel's shop.
		return []string{fmt.Sprintf("Bloco #%d é NPC do painel; ele volta sozinho quando ligado.", idx)}
	}
	var ids []int
	aqui := len(args) > 1 && strings.EqualFold(args[1], "aqui")
	if aqui {
		ids = w.GenerateMobNear(idx, o.x, o.y)
	} else {
		ids = w.GenerateMob(idx)
	}
	if len(ids) == 0 {
		limite := g.MaxNumMob
		if limite < 0 {
			limite = 1 // GenerateMob reads a negative cap as one
		}
		return []string{fmt.Sprintf("Nada gerado: #%d tem %d de %d vivos. Use matar bloco %d antes.",
			idx, g.CurrentNumMob, limite, idx)}
	}
	d.revealSpawned(w, ids)
	lead := w.Entity(ids[0])
	d.log.Info("generate", "by", o.by, "index", idx, "mobs", len(ids), "aqui", aqui)
	return []string{fmt.Sprintf("Gerados %d de #%d %s em (%d,%d).", len(ids), idx, g.Name, lead.X, lead.Y)}
}

// criarCmd creates one mob from any template a block uses, around the caller —
// the legacy "create" (imple.cpp:675). It belongs to no block and does not come
// back when it dies: this is a one-off for an event, not world population.
func (d *Dispatcher) criarCmd(w *world.World, o blocoOrigem, args []string) []string {
	if len(args) == 0 {
		return []string{"Uso: criar <nome do template>"}
	}
	name := args[0]
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
		return []string{msg}
	}
	x, y, ok := w.EmptyCellNear(o.x, o.y)
	if !ok {
		return []string{fmt.Sprintf("Não há espaço livre em (%d,%d).", o.x, o.y)}
	}
	id := w.SpawnMobAt(world.MobSpawn{Template: tmpl, TemplateName: tmplName, X: x, Y: y, GenIndex: -1})
	if id < 0 {
		return []string{"O mundo está cheio."}
	}
	// The template is kept only for the respawn queue; without it the mob dies
	// for good, which is what a one-off must do.
	w.Entity(id).Template = nil
	d.revealSpawned(w, []int{id})
	d.log.Info("create", "by", o.by, "template", name, "mob", id)
	return []string{fmt.Sprintf("Criado %s em (%d,%d).", w.Entity(id).Name, x, y)}
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// matarCmd kills monsters: those around the caller, or every live mob of one
// block. It is a death (removeType 1), so the block's own rules decide whether
// and when they return — to stop that, switch the block off.
//
// Around the caller it spares service NPCs and city guards (those leave with
// npc off) and anybody's summons; naming a block kills what the block raised,
// whatever it is, because the caller asked for exactly that.
func (d *Dispatcher) matarCmd(w *world.World, o blocoOrigem, args []string) []string {
	if len(args) >= 1 && strings.EqualFold(args[0], "bloco") {
		if len(args) < 2 {
			return []string{"Uso: matar bloco <bloco>"}
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || w.GeneratorAt(idx) == nil {
			return []string{fmt.Sprintf("Bloco %q não existe (0..%d).", args[1], w.GeneratorCount()-1)}
		}
		n := 0
		w.ForEachMob(func(id int, m *world.Entity) {
			if int(m.GenIndex) == idx {
				w.DespawnMob(id, 1)
				n++
			}
		})
		d.log.Info("kill block", "by", o.by, "index", idx, "mobs", n)
		return []string{fmt.Sprintf("Mortos %d do bloco #%d.", n, idx)}
	}
	radius := gmKillRadius
	if len(args) >= 1 {
		r, err := strconv.Atoi(args[0])
		if err != nil || r < 0 {
			return []string{"Uso: matar [raio] | matar bloco <bloco>"}
		}
		radius = min(r, gmKillMaxRadius)
	}
	n := 0
	w.ForEachMob(func(id int, m *world.Entity) {
		if chebyshev(o.x, o.y, m.X, m.Y) > radius || m.Summoner != 0 || m.NonCombatNPC || m.Merchant != 0 {
			return
		}
		w.DespawnMob(id, 1)
		n++
	})
	d.log.Info("kill", "by", o.by, "radius", radius, "mobs", n)
	return []string{fmt.Sprintf("Mortos %d num raio de %d.", n, radius)}
}

// recarregarCmd is the legacy "reloadnpc" as this server can do it. The file
// itself ships with the deploy, so there is nothing on disk to re-read; what it
// does is re-read the database now (block switches and the NPC panel) and top
// every block up to its cap, bringing back whatever is missing.
func (d *Dispatcher) recarregarCmd(w *world.World) []string {
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
	return []string{fmt.Sprintf("Recarregado: %d mobs repostos; o banco é relido no próximo segundo.", n)}
}
