package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// gmPools reports and repairs the stored HP/MP pools:
//
//	/gm pools [nome]              — mostra os valores
//	/gm pools [nome] <hp> <mp>    — soma um delta aos dois
//
// It exists because BaseMaxHP is STORED, not derived. refreshScore builds the
// live MaxHP as BaseMaxHP + equipment and never recomputes it from CON, so a
// wrong pool stays wrong forever — no login, no level, no re-equip corrects it.
// That is what made the Retorno da Habilidade leak durable: the refund handed the
// points back and left the HP they had bought, and nothing downstream noticed.
//
// The command adjusts by DELTA rather than recalculating, on purpose. A canonical
// rebuild (class base + level×IncHP + 2×invested) is only right for a Mortal: the
// Arch rebirth subtracts IncHP per level lost (arch.go:71) and each crystal adds
// 80 or 60 (archcrystal.go:65-75), and neither is reconstructible from the
// character's current state. Recomputing would quietly rob an Arch of its
// crystals. A delta touches only what the operator asked for.
func (d *Dispatcher) gmPools(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)

	// The name is optional and only recognised when it is not a number, so
	// "/gm pools -2000 0" edits the caller and "/gm pools Fulano -2000 0" does not
	// have to repeat it.
	target, targetSess := w.Entity(s.Conn), s
	if len(fields) > 0 {
		if _, err := strconv.Atoi(fields[0]); err != nil {
			ts, te := w.SessionByName(fields[0])
			if te == nil {
				d.notify(w, s, NoticeNotConnected)
				return
			}
			target, targetSess = te, ts
			fields = fields[1:]
		}
	}
	if target == nil {
		return
	}

	switch len(fields) {
	case 0:
		d.sayPools(w, s, target)
	case 2:
		dhp, err1 := strconv.Atoi(fields[0])
		dmp, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			sendClientMessage(w, s, "Uso: /gm pools [nome] <delta_hp> <delta_mp>")
			return
		}
		beforeHP, beforeMP := target.BaseMaxHP, target.BaseMaxMP
		target.BaseMaxHP = clampPool(target.BaseMaxHP+int32(dhp), level.MaxHPCap)
		target.BaseMaxMP = clampPool(target.BaseMaxMP+int32(dmp), level.MaxMPCap)
		d.refreshScore(target)
		// The live pools shrank, so the current values may now sit above them.
		if m := effectiveMaxHP(target); target.HP > m {
			target.HP = m
		}
		if m := effectiveMaxMP(target); target.MP > m {
			target.MP = m
		}
		d.sendScore(w, targetSess, target)
		d.log.Info("gm pools",
			"account", s.AccountName, "target", target.Name,
			"hp_before", beforeHP, "hp_after", target.BaseMaxHP,
			"mp_before", beforeMP, "mp_after", target.BaseMaxMP)
		d.sayPools(w, s, target)
	default:
		sendClientMessage(w, s, "Uso: /gm pools [nome] <delta_hp> <delta_mp>")
	}
}

// sayPools prints the stored pools next to what a MORTAL of this build would
// have. The canonical figure is a REFERENCE, not a target: for an Arch or a
// Celestial it legitimately differs by the rebirth and crystal grants, so the
// line says which tier it is and lets the operator judge.
func (d *Dispatcher) sayPools(w *world.World, s *world.Session, e *world.Entity) {
	base := level.BaseAttributes(e.Class)
	investedCon := int32(e.BaseCon) - base[3]
	investedInt := int32(e.BaseInt) - base[1]
	mortalHP := level.ClassBaseHP(e.Class) + (e.Level-1)*level.IncHP(e.Class) + 2*investedCon
	mortalMP := level.ClassBaseMP(e.Class) + (e.Level-1)*level.IncMP(e.Class) + 2*investedInt

	sendClientMessage(w, s, fmt.Sprintf("%s nv%d: HP base %d (mortal %d), MP base %d (mortal %d)",
		e.Name, e.Level, e.BaseMaxHP, mortalHP, e.BaseMaxMP, mortalMP))
	sendClientMessage(w, s, fmt.Sprintf("CON %d (+%d), INT %d (+%d), tier %d",
		e.BaseCon, investedCon, e.BaseInt, investedInt, e.ClassMaster))
}

func clampPool(v, teto int32) int32 {
	if v < 1 {
		return 1
	}
	if v > teto {
		return teto
	}
	return v
}
