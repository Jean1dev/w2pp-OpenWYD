package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// O Kefra é chefe semanal. No original (ProcessSecMinTimer.cpp:808-816), toda
// terça ao meio-dia, se ele tinha morrido, o servidor devolve o Kefra (bloco
// KEFRA_BOSS, 396) e os guardas (KEFRA_MOB_INITIAL..END, 397-400). O original
// rodava no relógio do Brasil; o do contêiner é UTC, então o meio-dia vai em
// UTC: 15:00. O laço do boot do original usa "<" e pula o guarda 400; aqui ele
// volta junto, como na terça.
//
// Fica de fora, de propósito: a morte do Kefra no original liga o KefraLive — a
// XP inteira no servidor todo — até a terça seguinte, e dá +100 de fama à guilda
// que matou (MobKilled.cpp:1451-1492). A chave continua no painel, desligada: a
// Mesa de XP foi calibrada com ela desligada, e o prêmio vai para a guilda, que
// depende da cidadania, ainda não portada. No boot ele nasce sempre: guardar a
// morte exigiria o banco, e sem o prêmio não há o que proteger.

const (
	kefraDia     = time.Tuesday
	kefraHoraUTC = 15
)

// tickKefraSemanal devolve o Kefra e os guardas uma vez por terça, às 15h UTC.
// GenerateMob respeita o teto de população de cada bloco, então quem está vivo
// não nasce de novo.
func (d *Dispatcher) tickKefraSemanal(w *world.World) {
	if d.tickCount%minutoTicks != 0 {
		return
	}
	agora := d.now().UTC()
	if agora.Weekday() != kefraDia || agora.Hour() != kefraHoraUTC {
		return
	}
	dia := agora.Format(time.DateOnly)
	if d.kefraVolta == dia {
		return
	}
	d.kefraVolta = dia
	n := 0
	for idx := world.KefraBossGenIndex; idx <= world.KefraGuardLast; idx++ {
		ids := w.GenerateMob(idx)
		n += len(ids)
		d.revealSpawned(w, ids)
	}
	d.log.Info("kefra semanal: chefe e guardas de volta", "monstros", n, "dia", dia)
}
