package handler

import (
	"fmt"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// /gm dano [nome] shows how the "Ataque" of the character window is built, part
// by part. It exists because balancing the physical attack by estimate went
// wrong: cutting the class weapon term to one grant was supposed to take a +11
// TK from 12.6K to ~7.4K, and the window still read ~11K — the rest of the
// number lives in parts nobody could see from outside (each piece of gear, the
// mount, the buffs). This prints them from the same functions the score uses.
//
// The window shows effectiveDamage (computeScore), which is
//
//	(Damage + AffDamage) × AffDamageMultiPct% (+20% Divina) + weaponDamage
//
// and Damage is BaseDamage + gear + mount + class weapon term + skill flat +
// attributes (refreshScore). Each term below is recomputed on its own and the
// sum is checked against the stored Damage; a gap is printed, never hidden.

// danoItem is one piece of gear's share of the flat Damage.
type danoItem struct {
	nome string
	dano int32
}

func (d *Dispatcher) gmDano(w *world.World, s *world.Session, rest string) {
	target := w.Entity(s.Conn)
	if name := firstToken(rest); name != "" {
		_, te := w.SessionByName(name)
		if te == nil {
			// Answered in words, not through notify: a report that goes silent
			// is indistinguishable from a server that does not know the command.
			sendClientMessage(w, s, fmt.Sprintf("Não achei %s em jogo.", name))
			return
		}
		target = te
	}
	if target == nil {
		sendClientMessage(w, s, "Sem personagem para medir.")
		return
	}
	d.log.Info("gm dano", "account", s.AccountName, "target", target.Name)
	for _, linha := range d.danoLinhas(target) {
		sendClientMessage(w, s, linha)
	}
}

// danoLinhas builds the report. Every line fits the 94 characters of the
// notice line.
func (d *Dispatcher) danoLinhas(e *world.Entity) []string {
	var itens []danoItem
	var somaItens, montaria int32
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		if slot == mountEquipSlot {
			if mb, ok := d.mountBonusFor(it); ok {
				montaria = mb.damage
			}
			continue
		}
		// The same two effects equipBonus folds into Damage: EF_DAMAGE outside
		// the weapon hands (their damage is the separate weaponDamage) and
		// EF_DAMAGEADD on jewels only.
		var v int32
		if slot != weaponSlotR && slot != weaponSlotL {
			v += d.itemAbilityRefined(it, efDamage)
		}
		if u := d.itemUnique[int(it.Index)]; u >= 41 && u <= 50 {
			v += d.itemAbilityRefined(it, efDamageAdd)
		}
		if v != 0 {
			itens = append(itens, danoItem{nome: d.danoNomeItem(it), dano: v})
			somaItens += v
		}
	}
	sort.SliceStable(itens, func(i, j int) bool { return itens[i].dano > itens[j].dano })

	// The face gate of the legado (Basedef.cpp:4651, `if (face < 4)`): a
	// character whose Equip[0] is out of the player range gets NEITHER the
	// attribute term NOR the class weapon term. isPlayerMob is that gate, and
	// refreshScore skips both terms for such a character.
	jogador := isPlayerMob(e)
	var armaClasse, vezes, skill, forca, destreza, maestria, nivel, atributos int32
	if jogador {
		armaClasse = d.classWeaponDamage(e)
		vezes = d.danoVezesArma(e, armaClasse)
		skill = skillFlatDamage(e)
		forca = int32(effectiveStr(e)) / 2
		destreza = int32(effectiveDex(e)) / 3
		maestria = int32(effectiveSpecial(e, 0))
		nivel = attributeDamageLevelTerm(e)
		atributos = forca + destreza + maestria + nivel
	}
	conta := e.BaseDamage + somaItens + montaria + armaClasse + skill + atributos

	linhas := []string{
		fmt.Sprintf("Ataque de %s na janela: %d", e.Name, d.effectiveDamage(e)),
		fmt.Sprintf("Itens: %d | Montaria: %d | Base: %d", somaItens, montaria, e.BaseDamage),
	}
	for _, it := range itens {
		linhas = append(linhas, fmt.Sprintf("  %s: %d", it.nome, it.dano))
	}
	if armaClasse > 0 {
		linhas = append(linhas, fmt.Sprintf("Bônus de arma da classe: %d (%d× de %d)", armaClasse, vezes, armaClasse/max(vezes, 1)))
	} else {
		linhas = append(linhas, "Bônus de arma da classe: 0 (arma fora da tabela ou sem evolução)")
	}
	if skill != 0 {
		linhas = append(linhas, fmt.Sprintf("Bônus fixo de skill: %d", skill))
	}
	linhas = append(linhas,
		fmt.Sprintf("Atributos: FOR/2 %d + DES/3 %d + Aprender %d + nível %d = %d", forca, destreza, maestria, nivel, atributos))
	if !jogador {
		linhas = append(linhas, fmt.Sprintf("Rosto %d (Equip[0] %d): atributos e bônus de arma NÃO entram (legado: face<4)",
			e.Equip[0].Index/10, e.Equip[0].Index))
	}
	if gap := e.Damage - conta; gap != 0 {
		linhas = append(linhas, fmt.Sprintf("Parte não identificada: %d (Damage %d, conta %d)", gap, e.Damage, conta))
	}
	divina := ""
	if e.HasAffect(world.AffectDivine) {
		divina = ", +20% Divina"
	}
	linhas = append(linhas,
		fmt.Sprintf("Subtotal %d + buffs %d, ×%d%%%s", e.Damage, e.AffDamage, e.AffDamageMultiPct, divina),
		fmt.Sprintf("Dano da arma (mãos + refino, fora do multiplicador): %d", d.weaponDamage(e)))
	if pct := d.combatRules.PhysicalDamagePct; pct != 100 {
		linhas = append(linhas, fmt.Sprintf("Regra do painel: ataque físico ×%d%% (aplicado no fim, sobre tudo acima)", pct))
	}
	return linhas
}

// danoVezesArma is how many times the class weapon term was counted: one per
// learned evolution skill, up to the panel's cap.
func (d *Dispatcher) danoVezesArma(e *world.Entity, armaClasse int32) int32 {
	if armaClasse == 0 {
		return 0
	}
	var n int32
	for _, bit := range classSkillBits(e.Class) {
		if e.LearnedSkill&(1<<bit) != 0 {
			n++
		}
	}
	return min(n, d.combatRules.WeaponDamageGrants)
}

func (d *Dispatcher) danoNomeItem(it world.Item) string {
	nome := d.itemNames[int(it.Index)]
	if nome == "" {
		nome = fmt.Sprintf("item %d", it.Index)
	}
	if sanc := itemSanc(it); sanc > 0 {
		nome = fmt.Sprintf("%s +%d", nome, sanc)
	}
	return nome
}
