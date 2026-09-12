package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// /status is the sheet the character window does not have: what the equipment
// is actually worth in a fight.
//
// The C window shows Ataque, Defesa, Atq Mágico, Vel Ataque and Crítico —
// STRUCT_SCORE, and nothing else, because that is all the client is sent. The
// numbers that decide a duel are computed server-side and never leave it: how
// often you hit, how often you are hit, how much damage walks past armour, and
// how much is taken off every blow you receive.
//
// Those four lead, in that order. Everything after them — the Defesa de
// Evolução, the mount's share, the drop bonus — is context, printed only when
// the character actually has it.

// espelhoDoAlvo names what the hit/dodge percentages are measured against. There
// is no such thing as "your accuracy" as a single number: the parry roll is
// always your accuracy MINUS their dodge, so a percentage needs an opponent.
// Measuring against a copy of the character is the one choice that needs no
// invented stat block and that a player can reason about — "against someone
// exactly like me".
const espelhoDoAlvo = "contra alguém igual a você"

// estadoStatus is what /status reports, read off the world so the wording can be
// tested on its own.
type estadoStatus struct {
	// Esquiva is the defender's whole half of combat.ParryRate, in thousandths,
	// before any attacker's accuracy is subtracted. Precisao is the attacker
	// half — what this character takes off the dodge of whoever it swings at.
	// EsquivaEspelho is the roll that survives when the two meet: this
	// character dodging an attacker with this same accuracy, already clamped to
	// the [1, 650] the roll enforces.
	Esquiva        int
	Precisao       int
	EsquivaEspelho int

	Perfuracao int32 // EquipForceDamage: flat damage that lands past armour
	Reflect    int   // flat damage taken off every blow from a player
	AtaquePvP  int   // EF_HWORDGUILD percentage
	DefesaPvP  int   // EF_LWORDGUILD percentage

	Defesa int32 // effectiveAC: the same number the C window shows
	// Tier is the character's ClassMaster, which decides how much damage each
	// attacking tier keeps against it (tierdefense.go).
	Tier uint8

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
	// Both halves come from the functions the attack path calls, never restated
	// here: precisaoDe is the attacker side of parryRateWith, and ParryRate with
	// a zero attacker is the defender side on its own.
	precisao := precisaoDe(e, int(effectiveDex(e)))
	st := estadoStatus{
		Esquiva:        combat.ParryRate(int(effectiveDex(e)), e.Parry, 0, 0),
		Precisao:       precisao,
		EsquivaEspelho: combat.ParryRate(int(effectiveDex(e)), e.Parry, precisao, int(e.Rsv)),
		Perfuracao:     e.EquipForceDamage,
		Reflect:        d.reflectDamage(e),
		AtaquePvP:      d.pvpAttackPct(e),
		DefesaPvP:      d.pvpDefensePct(e),
		Defesa:         effectiveAC(e),
		Tier:           e.ClassMaster,
		AbsHp:          e.AffHpAbs,
		DropBonus:      e.EquipDropBonus,
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
	// The roll is rand()%1000+1 < parry, so the dodge is already in tenths of a
	// percent; the hit is simply what is left of the thousand.
	linhas := []string{fmt.Sprintf("Acerto %s · Esquiva %s — %s.",
		pctMilesimos(1000-st.EsquivaEspelho), pctMilesimos(st.EsquivaEspelho), espelhoDoAlvo)}

	// The raw halves, because the percentage above moves when the opponent
	// changes and these do not: they are what the character carries into any
	// fight.
	linhas = append(linhas, fmt.Sprintf(
		"Precisão %d (tira da esquiva do alvo) · a sua esquiva %d em 1000, teto 650.",
		st.Precisao, st.Esquiva))

	if partes := partesGolpe(st); len(partes) > 0 {
		linhas = append(linhas, juntarPartes("Contra jogador: ", partes, linhaPainelMax)...)
	}

	linhas = append(linhas, fmt.Sprintf(
		"Defesa %d · contra jogador vale %d, porque a Defesa conta 3x em PvP.", st.Defesa, st.Defesa*3))
	linhas = append(linhas, linhasEvolucao(st.Tier)...)

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
	}
	return append(linhas, "Bônus de XP: digite /xp.")
}

// pctMilesimos renders a thousandths roll as the percentage a player reads,
// keeping the tenth: 385 is 38.5%, and dropping it would round two very
// different builds onto the same number.
func pctMilesimos(v int) string {
	return fmt.Sprintf("%d.%d%%", v/10, v%10)
}

// partesGolpe is what the gear does to a blow — perfuração first, because it is
// the one number that ignores armour entirely, then what comes off blows the
// character receives.
func partesGolpe(st estadoStatus) []string {
	var partes []string
	if st.Perfuracao > 0 {
		// "passa pela defesa" is the part a damage number cannot express: these
		// points land after the target's armour has taken its cut. The Defesa de
		// Evolução does still reduce them (tierdefense.go).
		partes = append(partes, fmt.Sprintf("perfuração +%d que passa pela defesa", st.Perfuracao))
	}
	if st.Reflect > 0 {
		partes = append(partes, fmt.Sprintf("absorve %d de cada golpe", st.Reflect))
	}
	if st.DefesaPvP > 0 {
		partes = append(partes, fmt.Sprintf("e mais %d%% do que sobrou", st.DefesaPvP))
	}
	if st.AtaquePvP > 0 {
		partes = append(partes, fmt.Sprintf("você bate +%d%%", st.AtaquePvP))
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
