package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

// /gm guerra drives the wars by hand, so they can be tested without waiting for
// their hour:
//
//	/gm guerra torre aviso [min]   announce now; opens after min (default 1)
//	/gm guerra torre abrir [min]   open now for min minutes (default 24)
//	/gm guerra torre fim           end now; the holder is paid, as at :30
//	/gm guerra torre estado        phase, holder, deadlines and schedule
//
// A forced war ignores the panel's hour and switch until it ends, and goes
// through the same transitions as the scheduled one: the same notices, the
// same area clear, the same reward.

const (
	gmTowerLeadDefault = time.Minute
	gmTowerOpenDefault = 24 * time.Minute // :06 to :30, the scheduled length
	gmTowerMaxMinutes  = 180
)

func (d *Dispatcher) gmGuerra(w *world.World, s *world.Session, rest string) {
	war := strings.ToLower(firstToken(rest))
	args := strings.TrimSpace(strings.TrimPrefix(rest, firstToken(rest)))
	switch war {
	case "torre", "tower":
		d.gmGuerraTorre(w, s, args)
	case "cidade", "cidades", "city", "noatum", "noatun":
		sendClientMessage(w, s, "Essa guerra ainda não existe no servidor; por enquanto, só a torre.")
	default:
		sendClientMessage(w, s, "uso: /gm guerra torre <aviso|abrir|fim|estado> [minutos]")
	}
}

func (d *Dispatcher) gmGuerraTorre(w *world.World, s *world.Session, args string) {
	op := strings.ToLower(firstToken(args))
	n, hasN, ok := gmMinutes(strings.TrimSpace(strings.TrimPrefix(args, firstToken(args))))
	if !ok {
		sendClientMessage(w, s, fmt.Sprintf("minutos inválidos (1 a %d)", gmTowerMaxMinutes))
		return
	}
	now := d.now()
	t := &d.events.tower
	var act worldevents.TowerAction
	switch op {
	case "aviso", "anunciar", "announce":
		lead := gmTowerLeadDefault
		if hasN {
			lead = n
		}
		if act = t.ForceAnnounce(now, lead, gmTowerOpenDefault); act == worldevents.TowerNone {
			sendClientMessage(w, s, "Já há uma Guerra de Torres em andamento.")
			return
		}
	case "abrir", "iniciar", "start":
		dur := gmTowerOpenDefault
		if hasN {
			dur = n
		}
		wasIdle := t.Phase() == worldevents.TowerIdle
		if act = t.ForceOpen(now, dur); act == worldevents.TowerNone {
			sendClientMessage(w, s, "A Guerra de Torres já está aberta.")
			return
		}
		if wasIdle {
			// Skipping the announce skips its owner reset too; a war never
			// starts with yesterday's holder.
			d.setTowerOwner(w, 0)
		}
	case "fim", "encerrar", "end":
		if t.Phase() == worldevents.TowerIdle {
			sendClientMessage(w, s, "Não há Guerra de Torres em andamento.")
			return
		}
		if act = t.ForceEnd(); act == worldevents.TowerNone {
			d.towerNotice(w, "A Guerra de Torres foi cancelada.")
		}
	case "estado", "status", "":
		sendClientMessage(w, s, d.gmTowerStatus(w, now))
		return
	default:
		sendClientMessage(w, s, "uso: /gm guerra torre <aviso|abrir|fim|estado> [minutos]")
		return
	}
	d.log.Info("gm tower war", "account", s.AccountName, "op", op, "minutes", int(n/time.Minute))
	d.applyTowerAction(w, act)
	sendClientMessage(w, s, d.gmTowerStatus(w, now))
}

// gmMinutes parses the optional minutes argument.
func gmMinutes(arg string) (dur time.Duration, given, ok bool) {
	arg = firstToken(arg)
	if arg == "" {
		return 0, false, true
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > gmTowerMaxMinutes {
		return 0, true, false
	}
	return time.Duration(n) * time.Minute, true, true
}

func (d *Dispatcher) gmTowerStatus(w *world.World, now time.Time) string {
	t := &d.events.tower
	holder := "sem dono"
	if owner := d.events.towerOwner; owner != 0 {
		holder = "[" + d.guildLabel(w, owner) + "]"
	}
	mode := "horário"
	if t.Forced() {
		mode = "GM"
	}
	switch t.Phase() {
	case worldevents.TowerAnnounced:
		return fmt.Sprintf("Torre (%s): aviso, abre %s, fecha %s.",
			mode, t.OpensAt(now).Format("15:04"), t.EndsAt(now).Format("15:04"))
	case worldevents.TowerOpen:
		return fmt.Sprintf("Torre (%s): aberta até %s, %s.", mode, t.EndsAt(now).Format("15:04"), holder)
	}
	sched := "desligada no painel"
	if t.Enabled {
		sched = fmt.Sprintf("todo dia às %02d:00", t.Hour)
	}
	return "Torre: parada, " + sched + "."
}
