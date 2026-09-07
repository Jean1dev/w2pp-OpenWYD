package audit

import (
	"reflect"
	"strings"
	"testing"
)

// TestTodaAcaoTemRotulo is the test that keeps the audit page readable as the
// panel grows. A new action without a label renders as an upper-case English
// constant among Portuguese phrases, and nobody notices until they are reading
// the log to find out what went wrong — the worst possible moment.
func TestTodaAcaoTemRotulo(t *testing.T) {
	// Every exported Action* constant, found by reflection over this package is
	// not possible for constants, so the list is the map's own keys checked
	// against the constants named in the file. Instead: assert the count and
	// that each known constant resolves to something other than itself.
	acoes := []string{
		ActionSetRole, ActionSetBlocked, ActionSetVip, ActionSetPassword,
		ActionSetItemPrice, ActionSetNpcShop, ActionSetNpc, ActionDeleteNpc,
		ActionSetMobStat, ActionClearMobStat, ActionSetItemStat, ActionClearItemStat,
		ActionDeliverItem, ActionCancelDelivery, ActionKick, ActionBroadcast, ActionUnstuck, ActionSetWorldEvent, ActionHandleReport,
		ActionRestartGame, ActionSafeRestart, ActionStopGame, ActionStartGame,
		ActionCreateAccount, ActionSetXPRule, ActionClearXPRule,
		ActionSetMountGrowth, ActionClearMountGrowth,
		ActionSetMountAbsorb, ActionClearMountAbsorb,
	}
	for _, a := range acoes {
		e := Entry{Action: a}
		if got := e.Rotulo(); got == a {
			t.Errorf("a ação %q não tem rótulo — aparece crua na tela", a)
		}
	}
	if len(rotulos) != len(acoes) {
		t.Errorf("o mapa tem %d rótulos para %d ações listadas aqui; uma das duas ficou para trás",
			len(rotulos), len(acoes))
	}
}

func TestAcaoDesconhecidaApareceCruaEmVezDeSumir(t *testing.T) {
	// A row written by a version of the panel this one does not know about is
	// exactly the row somebody is hunting for. Showing it untranslated beats
	// showing a dash.
	e := Entry{Action: "ALGO_QUE_AINDA_NAO_EXISTE"}
	if got := e.Rotulo(); got != "ALGO_QUE_AINDA_NAO_EXISTE" {
		t.Errorf("Rotulo = %q, want a própria ação", got)
	}
	if e.Rotulo() == "" {
		t.Error("uma ação desconhecida sumiu da tela")
	}
}

func TestRotulosSaoFrasesEmPortugues(t *testing.T) {
	// The point is that a person reads them. A label that is still a constant —
	// upper case with underscores — would pass the "different from the action"
	// check above while being just as unreadable.
	for acao, rot := range rotulos {
		if strings.Contains(rot, "_") || rot == strings.ToUpper(rot) {
			t.Errorf("o rótulo de %s ainda parece constante: %q", acao, rot)
		}
	}
}

func TestNenhumRotuloRepetido(t *testing.T) {
	// Two actions sharing a phrase makes the log ambiguous exactly where it has
	// to be precise: "Reiniciou o servidor" and "Reiniciou com segurança" are
	// different events with different consequences.
	visto := map[string]string{}
	for acao, rot := range rotulos {
		if antes, ok := visto[rot]; ok {
			t.Errorf("%s e %s compartilham o rótulo %q", antes, acao, rot)
		}
		visto[rot] = acao
	}
	if len(visto) != len(rotulos) {
		t.Errorf("rótulos distintos = %d, ações = %d", len(visto), len(rotulos))
	}
	_ = reflect.TypeOf(Entry{}) // guards the import if the checks above change
}
