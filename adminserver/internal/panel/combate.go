package panel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Combate is the combat-rule store, satisfied by *store.Store.
//
// The knobs that decide how hard a blow lands on this server: how much Magic a
// weapon draws from INT, whether the damage buffs reach spells, how much a
// monster's resistance bends a spell, and how much of a skill or melee blow on
// another player survives the legacy quarter. Until this screen they could only
// change in code, which meant a deploy for every turn of a dial that is tuned by
// watching a fight.
type Combate interface {
	CombatRule(ctx context.Context) (combatrule.Config, error)
	SetCombatRule(ctx context.Context, r combatrule.Rules, moderatorID int64) (combatrule.Config, error)
	ClearCombatRule(ctx context.Context, moderatorID int64) (combatrule.Config, error)
}

// combateBotao is one knob as the screen shows it: the value in force next to
// the decided default and to Kersef, so a departure from either is visible
// without opening another page.
type combateBotao struct {
	Nome    string
	Explica string
	Agora   string
	Padrao  string
	Kersef  string
	// Mudado marks a knob off the default, which is the question somebody
	// scanning this table is actually asking.
	Mudado bool
}

// combateView is the page.
type combateView struct {
	Configurada bool
	Versao      int64
	Regra       combatrule.Rules
	Botoes      []combateBotao
	Kersef      combatrule.Rules
	MinArma     int32
	MaxArma     int32
	MinResist   int32
	MaxResist   int32
	MinPvP      int32
	MaxPvP      int32
	MinPrecisao int32
	MaxPrecisao int32
	MinErros    int32
	MaxErros    int32
	MinArmaFis  int32
	MaxArmaFis  int32
	MinCritDup  int32
	MaxCritDup  int32
}

func pctTexto(v int32) string { return fmt.Sprintf("%d%%", v) }

func ligadoTexto(v bool) string {
	if v {
		return "ligado"
	}
	return "desligado"
}

func baseTexto(v int32) string { return strconv.Itoa(int(v)) }

// vezesTexto says how many evolutions add the weapon term: "1×", "3×".
func vezesTexto(v int32) string { return fmt.Sprintf("%d×", v) }

// errosTexto says the miss streak the way the screen explains it: 0 is not "zero
// misses allowed" but the feature switched off.
func errosTexto(v int32) string {
	if v == 0 {
		return "desligado"
	}
	return strconv.Itoa(int(v))
}

// pvpExplica is what the two PvP knobs share: both scale a blow on a player on
// top of the legacy quarter, and differ only in which blow.
const pvpExplica = "No legado todo golpe em jogador já é dividido por 4 (a Perfuração). " +
	"Este ajuste vale por cima disso: 100% é o legado, 50% dá metade. " +
	"Serve para decidir quantos golpes uma luta entre iguais deve levar."

// combateBotoes lays the knobs out for the table, in the order the form asks
// for them.
func combateBotoes(r combatrule.Rules) []combateBotao {
	p, k := combatrule.Default(), combatrule.Kersef()
	return []combateBotao{
		{
			Nome: "Magia da arma por INT",
			Explica: "Com a oitava skill aprendida, o Kersef soma à Magia um termo da arma, " +
				"(DES×k + INT×k)/100. 0% = a Magia vem só do equipamento, montaria e buffs; " +
				"100% = como o Kersef, em que o INT sozinho colava toda FM no teto já no +11 " +
				"e o refino deixava de valer para mago.",
			Agora: pctTexto(r.WeaponIntMagicPct), Padrao: pctTexto(p.WeaponIntMagicPct),
			Kersef: pctTexto(k.WeaponIntMagicPct), Mudado: r.WeaponIntMagicPct != p.WeaponIntMagicPct,
		},
		{
			Nome: "Multiplicador de dano na magia",
			Explica: "Se os buffs de porcentagem de dano — poções, Assalto, Meditação, " +
				"transformações — multiplicam também a magia. Desligado = como o legado: " +
				"só o golpe físico, e o golpe da magia bate com o Atq Mágico da janela. " +
				"Ligado = como o Kersef.",
			Agora: ligadoTexto(r.SpellDamageMulti), Padrao: ligadoTexto(p.SpellDamageMulti),
			Kersef: ligadoTexto(k.SpellDamageMulti), Mudado: r.SpellDamageMulti != p.SpellDamageMulti,
		},
		{
			Nome: "Resistência de monstro à magia",
			Explica: "A base da escala de resistência da magia contra MONSTRO: o golpe sai a " +
				"(base − resistência/2)%. 100 = monstro sem resistência leva o golpe cheio; " +
				"150 = o legado, que dava +50% contra qualquer monstro de resistência baixa. " +
				"Contra jogador vale sempre 150, diga isto o que disser.",
			Agora: baseTexto(r.MobResistBase), Padrao: baseTexto(p.MobResistBase),
			Kersef: baseTexto(k.MobResistBase), Mudado: r.MobResistBase != p.MobResistBase,
		},
		{
			Nome:    "Dano de skill em jogador (%)",
			Explica: "Quanto sobra do golpe de uma skill em outro jogador. " + pvpExplica,
			Agora:   pctTexto(r.PvPSkillPct), Padrao: pctTexto(p.PvPSkillPct),
			Kersef: pctTexto(k.PvPSkillPct), Mudado: r.PvPSkillPct != p.PvPSkillPct,
		},
		{
			Nome:    "Dano de golpe físico em jogador (%)",
			Explica: "Quanto sobra do golpe físico em outro jogador. " + pvpExplica,
			Agora:   pctTexto(r.PvPMeleePct), Padrao: pctTexto(p.PvPMeleePct),
			Kersef: pctTexto(k.PvPMeleePct), Mudado: r.PvPMeleePct != p.PvPMeleePct,
		},
		{
			Nome: "Precisão da magia pela INT (%)",
			Explica: "Na esquiva, o legado só olha a DES de quem ataca, então um mago de INT " +
				"cheia acerta como um personagem de DES 12 — uma FM com INT 3.148 errava ~43% " +
				"das magias numa TK de DES 700. Com 50%, metade da INT conta como DES (vale o " +
				"maior entre DES e INT×%), e o erro cai para ~12%. 0% é o legado.",
			Agora: pctTexto(r.SpellIntAccuracyPct), Padrao: pctTexto(p.SpellIntAccuracyPct),
			Kersef: pctTexto(k.SpellIntAccuracyPct), Mudado: r.SpellIntAccuracyPct != p.SpellIntAccuracyPct,
		},
		{
			Nome: "Máximo de erros seguidos no mesmo alvo",
			Explica: "Depois desse número de esquivas seguidas no mesmo alvo, a próxima magia " +
				"acerta. 0 desliga (legado: cada sorteio vale sozinho).",
			Agora: errosTexto(r.MaxMissStreak), Padrao: errosTexto(p.MaxMissStreak),
			Kersef: errosTexto(k.MaxMissStreak), Mudado: r.MaxMissStreak != p.MaxMissStreak,
		},
		{
			Nome: "Bônus de arma no golpe físico",
			Explica: "Cada classe soma ao Ataque um bônus da arma, DES×a + FOR×b conforme o tipo. " +
				"O legado soma esse bônus uma vez POR EVOLUÇÃO aprendida (no TK: Confiança, Trans, " +
				"Espada Mágica), então quem tem as três leva três vezes — numa TK +11 com FOR 2.802 " +
				"eram ~6.200 de um Ataque de 12.630. 1× = conta uma vez, como a magia; 3× = o legado. " +
				"Vale para TK, FM e BM (a HT só tem uma evolução que soma).",
			Agora: vezesTexto(r.WeaponDamageGrants), Padrao: vezesTexto(p.WeaponDamageGrants),
			Kersef: vezesTexto(k.WeaponDamageGrants), Mudado: r.WeaponDamageGrants != p.WeaponDamageGrants,
		},
		{
			Nome: "Chance máxima do crítico duplo (%)",
			Explica: "O crítico duplo dobra o golpe físico. No legado a chance é 10% por ponto de " +
				"velocidade de ataque acima de 50, e a velocidade leva DES/5: DES 100 dá 20%, DES 250 " +
				"dá 50%, e com DES 500 ou mais TODO golpe sai dobrado. Este teto corta a chance; " +
				"100% = o legado, 0% desliga o crítico duplo.",
			Agora: pctTexto(r.DoubleCriticalMaxPct), Padrao: pctTexto(p.DoubleCriticalMaxPct),
			Kersef: pctTexto(k.DoubleCriticalMaxPct), Mudado: r.DoubleCriticalMaxPct != p.DoubleCriticalMaxPct,
		},
	}
}

// combate renders the combat-rule page.
func (h *Handler) combate(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Combate.CombatRule(r.Context())
	if err != nil {
		// A failed read refuses the page rather than drawing the default: a
		// screen claiming the server runs the default when nobody knows is how
		// somebody "fixes" a rule that was never broken.
		h.cfg.Logger.Error("combat rule read failed", "err", err)
		http.Error(w, "Erro ao ler a regra de combate.", http.StatusInternalServerError)
		return
	}
	h.render(w, "combate.html", struct {
		page
		Aba       string
		Aviso     string
		Combate   combateView
		Historico []audit.Entry
	}{
		page:  h.pageFor(r, "rates"),
		Aba:   "combate",
		Aviso: r.URL.Query().Get("aviso"),
		Combate: combateView{
			Configurada: cfg.Configured, Versao: cfg.Version, Regra: cfg.Rules,
			Botoes: combateBotoes(cfg.Rules), Kersef: combatrule.Kersef(),
			MinArma: combatrule.MinWeaponIntMagicPct, MaxArma: combatrule.MaxWeaponIntMagicPct,
			MinResist: combatrule.MinMobResistBase, MaxResist: combatrule.MaxMobResistBase,
			MinPvP: combatrule.MinPvPPct, MaxPvP: combatrule.MaxPvPPct,
			MinPrecisao: combatrule.MinSpellIntAccuracy, MaxPrecisao: combatrule.MaxSpellIntAccuracy,
			MinErros: combatrule.MinMissStreak, MaxErros: combatrule.MaxMissStreak,
			MinArmaFis: combatrule.MinWeaponDamageGrants, MaxArmaFis: combatrule.MaxWeaponDamageGrants,
			MinCritDup: combatrule.MinDoubleCriticalPct, MaxCritDup: combatrule.MaxDoubleCriticalPct,
		},
		Historico: h.combateHistorico(r.Context()),
	})
}

// setCombate saves every knob together.
//
// Together on purpose, like the table's single row: they are one decision about
// how hard a blow lands here, and the form always carries all of them, so a
// save never leaves a rule that nobody actually chose.
func (h *Handler) setCombate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	regra, msg := combateDoForm(r)
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	antes, err := h.cfg.Combate.SetCombatRule(r.Context(), regra, sess.AccountID)
	if errors.Is(err, store.ErrInvalidCombatRule) {
		// combateDoForm already checks the ranges; this is the store saying so
		// again, and it is still the operator's input that is wrong.
		http.Error(w, "A regra tem um valor fora da faixa.", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.cfg.Logger.Error("combat rule save failed", "err", err)
		http.Error(w, "Erro ao gravar a regra de combate.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetCombatRule,
		Old:    combateParaAudit(antes),
		New:    combateParaAudit(combatrule.Config{Configured: true, Rules: regra}),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaCombate(w, r, fmt.Sprintf(
		"Regra gravada: magia da arma por INT %d%%, multiplicador na magia %s, "+
			"resistência de monstro %d, skill em jogador %d%%, golpe físico em jogador %d%%, "+
			"precisão da magia pela INT %d%%, máximo de erros seguidos %s, bônus de arma %s, crítico duplo até %d%%. "+
			"O jogo passa a usar em até 15 segundos.",
		regra.WeaponIntMagicPct, ligadoTexto(regra.SpellDamageMulti), regra.MobResistBase,
		regra.PvPSkillPct, regra.PvPMeleePct, regra.SpellIntAccuracyPct, errosTexto(regra.MaxMissStreak), vezesTexto(regra.WeaponDamageGrants), regra.DoubleCriticalMaxPct))
}

// limparCombate drops the row, back to the decided default.
func (h *Handler) limparCombate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	antes, err := h.cfg.Combate.ClearCombatRule(r.Context(), sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("combat rule clear failed", "err", err)
		http.Error(w, "Erro ao voltar a regra de combate ao padrão.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearCombatRule, Old: combateParaAudit(antes),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaCombate(w, r,
		"A regra de combate voltou ao padrão. O jogo passa a usar em até 15 segundos.")
}

// combateDoForm reads every knob. It returns the message to show when one is
// missing or out of range; an empty message means the rule is good.
//
// The multiplier is a pair of radio buttons with explicit values rather than a
// checkbox: an unticked checkbox sends nothing at all, and "nothing" would read
// as "desligado" — a form that lost the field would switch the multiplier off
// in silence.
func combateDoForm(r *http.Request) (combatrule.Rules, string) {
	arma, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("arma")))
	if err != nil || arma < combatrule.MinWeaponIntMagicPct || arma > combatrule.MaxWeaponIntMagicPct {
		return combatrule.Rules{}, fmt.Sprintf("A magia da arma por INT precisa ser um número entre %d e %d por cento.",
			combatrule.MinWeaponIntMagicPct, combatrule.MaxWeaponIntMagicPct)
	}
	var multi bool
	switch r.PostFormValue("multi") {
	case "1":
		multi = true
	case "0":
		multi = false
	default:
		return combatrule.Rules{}, "Escolha se o multiplicador de dano vale na magia: ligado ou desligado."
	}
	resist, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("resist")))
	if err != nil || resist < combatrule.MinMobResistBase || resist > combatrule.MaxMobResistBase {
		return combatrule.Rules{}, fmt.Sprintf("A resistência de monstro precisa ser um número entre %d e %d.",
			combatrule.MinMobResistBase, combatrule.MaxMobResistBase)
	}
	pvpSkill, ok := faixaDoForm(r, "pvp_skill", combatrule.MinPvPPct, combatrule.MaxPvPPct)
	if !ok {
		return combatrule.Rules{}, fmt.Sprintf("O dano de skill em jogador precisa ser um número entre %d e %d por cento.",
			combatrule.MinPvPPct, combatrule.MaxPvPPct)
	}
	pvpMelee, ok := faixaDoForm(r, "pvp_melee", combatrule.MinPvPPct, combatrule.MaxPvPPct)
	if !ok {
		return combatrule.Rules{}, fmt.Sprintf("O dano de golpe físico em jogador precisa ser um número entre %d e %d por cento.",
			combatrule.MinPvPPct, combatrule.MaxPvPPct)
	}
	precisao, ok := faixaDoForm(r, "precisao", combatrule.MinSpellIntAccuracy, combatrule.MaxSpellIntAccuracy)
	if !ok {
		return combatrule.Rules{}, fmt.Sprintf("A precisão da magia pela INT precisa ser um número entre %d e %d por cento.",
			combatrule.MinSpellIntAccuracy, combatrule.MaxSpellIntAccuracy)
	}
	erros, ok := faixaDoForm(r, "erros", combatrule.MinMissStreak, combatrule.MaxMissStreak)
	if !ok {
		return combatrule.Rules{}, fmt.Sprintf("O máximo de erros seguidos precisa ser um número entre %d e %d.",
			combatrule.MinMissStreak, combatrule.MaxMissStreak)
	}
	armaFis, ok := faixaDoForm(r, "arma_fisico", combatrule.MinWeaponDamageGrants, combatrule.MaxWeaponDamageGrants)
	if !ok {
		return combatrule.Rules{}, fmt.Sprintf("O bônus de arma no golpe físico precisa ser um número entre %d e %d.",
			combatrule.MinWeaponDamageGrants, combatrule.MaxWeaponDamageGrants)
	}
	critDup, ok := faixaDoForm(r, "critico_duplo", combatrule.MinDoubleCriticalPct, combatrule.MaxDoubleCriticalPct)
	if !ok {
		return combatrule.Rules{}, fmt.Sprintf("A chance máxima do crítico duplo precisa ser um número entre %d e %d por cento.",
			combatrule.MinDoubleCriticalPct, combatrule.MaxDoubleCriticalPct)
	}
	return combatrule.Rules{
		WeaponIntMagicPct: int32(arma), SpellDamageMulti: multi, MobResistBase: int32(resist),
		PvPSkillPct: pvpSkill, PvPMeleePct: pvpMelee,
		SpellIntAccuracyPct: precisao, MaxMissStreak: erros,
		WeaponDamageGrants:   armaFis,
		DoubleCriticalMaxPct: critDup,
	}, ""
}

// faixaDoForm reads one numeric knob and checks it against [lo, hi]. A missing
// field is refused like an out-of-range one, never filled in: a form that lost
// the field would otherwise put a tuned knob back on some fallback with nobody
// asking for it — and for the precision pair, where 0 is a real value (the
// legacy), an empty field read as 0 would switch the feature off in silence.
func faixaDoForm(r *http.Request, campo string, lo, hi int) (int32, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue(campo)))
	if err != nil || n < lo || n > hi {
		return 0, false
	}
	return int32(n), true
}

// combateParaAudit writes the log entry in words. An unconfigured rule is said
// as such rather than as its numbers: "estava no padrão" and "alguém gravou
// 0%/desligado/100" are different facts, even though the game ran the same.
func combateParaAudit(c combatrule.Config) map[string]any {
	if !c.Configured {
		return map[string]any{"estado": "padrão, nada gravado"}
	}
	return map[string]any{
		"magia_da_arma":          pctTexto(c.Rules.WeaponIntMagicPct),
		"multiplicador_na_magia": ligadoTexto(c.Rules.SpellDamageMulti),
		"resistencia_de_monstro": c.Rules.MobResistBase,
		"skill_em_jogador":       pctTexto(c.Rules.PvPSkillPct),
		"golpe_em_jogador":       pctTexto(c.Rules.PvPMeleePct),
		"precisao_pela_int":      pctTexto(c.Rules.SpellIntAccuracyPct),
		"erros_seguidos":         c.Rules.MaxMissStreak,
		"bonus_de_arma":          vezesTexto(c.Rules.WeaponDamageGrants),
		"critico_duplo_maximo":   pctTexto(c.Rules.DoubleCriticalMaxPct),
	}
}

func (h *Handler) voltarParaCombate(w http.ResponseWriter, r *http.Request, aviso string) {
	http.Redirect(w, r, "/rates/combate?"+url.Values{"aviso": {aviso}}.Encode(), http.StatusSeeOther)
}

// combateHistorico is the rule's own slice of the audit log.
func (h *Handler) combateHistorico(ctx context.Context) []audit.Entry {
	lista, err := h.cfg.Audit.ListActions(ctx,
		[]string{audit.ActionSetCombatRule, audit.ActionClearCombatRule})
	if err != nil {
		h.cfg.Logger.Error("combat rule history failed", "err", err)
		return nil
	}
	return lista
}
