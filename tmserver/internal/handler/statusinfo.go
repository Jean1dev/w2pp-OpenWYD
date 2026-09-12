package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// /status answers the other half of the question /xp answers: the numbers that
// decide how much damage a character TAKES and DEALS in PvP, and how much loot
// it sees — none of which the character window shows.
//
// The client's C window is built from STRUCT_SCORE, so it can only draw what
// that structure carries: Ataque, Defesa, Atq Mágico, Vel Ataque, Crítico. Every
// rule below is computed server-side and never reaches the client at all — the
// ×3 the Defesa is worth against a player, the Defesa de Evolução, Ataque and
// Defesa PvP off the gear, the flat reflect, perfuração, the mount's share of
// each blow, the drop bonus.
//
// Each line is printed only when the character HAS the thing. A screen of zeroes
// is how people learn to stop reading a screen.

// estadoStatus is what /status reports, read off the world so the wording can be
// tested on its own.
type estadoStatus struct {
	Defesa int32 // effectiveAC: the same number the C window shows

	// Tier is the character's ClassMaster, which decides how much damage each
	// attacking tier keeps against it (tierdefense.go).
	Tier uint8

	// The PvP block off the gear (pvp.go): AtaquePvP and DefesaPvP are
	// percentages from EF_HWORDGUILD/EF_LWORDGUILD, Reflect the flat number
	// taken off every blow another player lands, Perfuracao the flat damage the
	// Esmeralda gem adds after the target's defence came off.
	AtaquePvP  int
	DefesaPvP  int
	Reflect    int
	Perfuracao int32

	// MontariaPvP / MontariaPvE are the share an adult mount eats of a blow
	// from a player and from a monster. TemMontaria is false when there is no
	// adult mount equipped, or it is down — a mount at zero HP absorbs nothing.
	TemMontaria bool
	MontariaPvP int
	MontariaPvE int

	AbsHp     int32 // AffHpAbs: the Jóia da Absorção lifesteal, in percent
	DropBonus int32 // EquipDropBonus
}

// showStatus backs /status.
func (d *Dispatcher) showStatus(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	st := estadoStatus{
		Defesa:     effectiveAC(e),
		Tier:       e.ClassMaster,
		AtaquePvP:  d.pvpAttackPct(e),
		DefesaPvP:  d.pvpDefensePct(e),
		Reflect:    d.reflectDamage(e),
		Perfuracao: e.EquipForceDamage,
		AbsHp:      e.AffHpAbs,
		DropBonus:  e.EquipDropBonus,
	}
	// The same conditions absorbBlow checks, in the same order: an adult
	// lineage, still standing, and then its configured pair.
	if mount := e.Equip[mountEquipSlot]; mountrate.IsAdultMount(mount.Index) && mountHP(mount) > 0 {
		st.TemMontaria = true
		st.MontariaPvP, st.MontariaPvE = defaultMountAbsorb, defaultMountAbsorb
		if p, ok := d.mountAbsorb.Percent(mount.Index, true); ok {
			st.MontariaPvP = p
		}
		if p, ok := d.mountAbsorb.Percent(mount.Index, false); ok {
			st.MontariaPvE = p
		}
	}
	for _, linha := range textoStatus(st) {
		sendClientMessage(w, s, linha)
	}
}

// textoStatus builds the answer, one string per panel line.
func textoStatus(st estadoStatus) []string {
	linhas := []string{fmt.Sprintf(
		"Defesa %d · contra jogador vale %d, porque a Defesa conta 3x em PvP.", st.Defesa, st.Defesa*3)}

	linhas = append(linhas, linhasEvolucao(st.Tier)...)

	if partes := partesPvP(st); len(partes) > 0 {
		linhas = append(linhas, juntarPartes("Em PvP: ", partes, linhaPainelMax)...)
	}
	if st.TemMontaria {
		linhas = append(linhas, fmt.Sprintf(
			"A sua montaria come %d%% do golpe de jogador e %d%% do de monstro.",
			st.MontariaPvP, st.MontariaPvE))
	}
	if st.AbsHp > 0 {
		linhas = append(linhas, fmt.Sprintf(
			"Jóia da Absorção: metade dos golpes devolve %d%% do dano em vida, até 350.", st.AbsHp))
	}
	if st.DropBonus > 0 {
		linhas = append(linhas, fmt.Sprintf("Bônus de drop dos seus itens: +%d%%", st.DropBonus))
	} else {
		linhas = append(linhas, "Bônus de drop dos seus itens: nenhum.")
	}
	return append(linhas, "Bônus de XP: digite /xp.")
}

// partesPvP is the gear-side PvP block, each piece named by what it does to a
// blow rather than by the effect id it came from.
func partesPvP(st estadoStatus) []string {
	var partes []string
	if st.AtaquePvP > 0 {
		partes = append(partes, fmt.Sprintf("ataque +%d%%", st.AtaquePvP))
	}
	if st.DefesaPvP > 0 {
		partes = append(partes, fmt.Sprintf("defesa +%d%%", st.DefesaPvP))
	}
	if st.Reflect > 0 {
		partes = append(partes, fmt.Sprintf("absorve %d por golpe", st.Reflect))
	}
	if st.Perfuracao > 0 {
		// "passa pela defesa" is the part a damage number cannot express: these
		// points are added after the target's armour has taken its cut, so they
		// arrive whole.
		partes = append(partes, fmt.Sprintf("perfuração +%d que passa pela defesa", st.Perfuracao))
	}
	return partes
}

// linhasEvolucao reports the Defesa de Evolução from the DEFENDER's side: how
// much damage each lower tier keeps when it hits this character.
//
// Written as "quem te bate" rather than as a percentage of reduction because
// the rule scales the ATTACKER's damage, and "you take 80% less" and "they keep
// 20%" are the same number said two ways — one of which invites the reader to
// subtract it from something.
func linhasEvolucao(tier uint8) []string {
	var out []string
	for _, atacante := range []struct {
		classMaster uint8
		nome        string
	}{
		{classMasterMortal, "Mortal"},
		{classMasterArch, "Arch"},
	} {
		if pct := tierDamagePct(atacante.classMaster, tier); pct < tierDamageFull {
			out = append(out, fmt.Sprintf(
				"Defesa de Evolução: um %s te acerta com %d%% do dano dele.", atacante.nome, pct))
		}
	}
	return out
}
