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
// Three knobs that decide how strong a caster is on this server: how much Magic
// a weapon draws from INT, whether the damage buffs reach spells, and how much a
// monster's resistance bends a spell. Until this screen they could only change
// in code, which meant a deploy for every turn of a dial that is tuned by
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
}

func pctTexto(v int32) string { return fmt.Sprintf("%d%%", v) }

func ligadoTexto(v bool) string {
	if v {
		return "ligado"
	}
	return "desligado"
}

func baseTexto(v int32) string { return strconv.Itoa(int(v)) }

// combateBotoes lays the three knobs out for the table, in the order the form
// asks for them.
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
		},
		Historico: h.combateHistorico(r.Context()),
	})
}

// setCombate saves the three knobs together.
//
// Together on purpose, like the table's single row: they are one decision about
// what a spell is here, and the form always carries all three, so a save never
// leaves a rule that nobody actually chose.
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
			"resistência de monstro %d. O jogo passa a usar em até 15 segundos.",
		regra.WeaponIntMagicPct, ligadoTexto(regra.SpellDamageMulti), regra.MobResistBase))
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

// combateDoForm reads the three knobs. It returns the message to show when one
// is missing or out of range; an empty message means the rule is good.
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
	return combatrule.Rules{
		WeaponIntMagicPct: int32(arma), SpellDamageMulti: multi, MobResistBase: int32(resist),
	}, ""
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
