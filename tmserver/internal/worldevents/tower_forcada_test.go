package worldevents

import (
	"testing"
	"time"
)

// TestTorreForcadaIgnoraHoraEInterruptor: às 15h, com a guerra desligada no
// painel, o GM testa assim mesmo — e ela segue os prazos dele, não os do relógio.
func TestTorreForcadaIgnoraHoraEInterruptor(t *testing.T) {
	now := time.Date(2026, time.September, 11, 15, 0, 0, 0, time.Local)
	tower := NewTower(20)
	if got := tower.ForceAnnounce(now, time.Minute, 10*time.Minute); got != TowerAnnounce {
		t.Fatalf("ForceAnnounce = %v, want announce", got)
	}
	steps := []struct {
		at   time.Duration
		want TowerAction
	}{
		{30 * time.Second, TowerNone}, // ainda no aviso
		{time.Minute, TowerStart},     // abre no prazo do GM
		{5 * time.Minute, TowerNone},  // 15h, desligada: uma guerra agendada já teria caído
		{11 * time.Minute, TowerEnd},
		{12 * time.Minute, TowerNone}, // de volta ao relógio: 15h e desligada, nada
	}
	for _, s := range steps {
		if got := tower.Step(now.Add(s.at), false); got != s.want {
			t.Fatalf("Step(+%v) = %v, want %v", s.at, got, s.want)
		}
	}
	if tower.Forced() || tower.Phase() != TowerIdle {
		t.Errorf("depois do fim: forçada %v fase %v, want não forçada e parada", tower.Forced(), tower.Phase())
	}
}

// TestTorreForcadaPrazos: o aviso diz quando abre e o lembrete quanto falta.
func TestTorreForcadaPrazos(t *testing.T) {
	now := time.Date(2026, time.September, 11, 15, 0, 0, 0, time.Local)
	tower := NewTower(20)
	tower.ForceAnnounce(now, 3*time.Minute, 10*time.Minute)
	if got, want := tower.OpensAt(now), now.Add(3*time.Minute); !got.Equal(want) {
		t.Errorf("OpensAt = %v, want %v", got, want)
	}
	if got, want := tower.EndsAt(now), now.Add(13*time.Minute); !got.Equal(want) {
		t.Errorf("EndsAt = %v, want %v", got, want)
	}
	agendada := NewTower(20)
	if got := agendada.EndsAt(now); got.Hour() != 20 || got.Minute() != 30 {
		t.Errorf("agendada termina %v, want 20:30", got)
	}
}

// TestTorreForceEnd: cancelar o aviso não paga ninguém (nada nasceu); encerrar a
// guerra aberta é o fim normal, com prêmio.
func TestTorreForceEnd(t *testing.T) {
	now := time.Date(2026, time.September, 11, 15, 0, 0, 0, time.Local)
	tower := NewTower(20)
	tower.ForceAnnounce(now, time.Minute, time.Minute)
	if got := tower.ForceEnd(); got != TowerNone || tower.Phase() != TowerIdle {
		t.Errorf("fim no aviso = %v fase %v, want nada e parada", got, tower.Phase())
	}
	tower.ForceOpen(now, time.Minute)
	if got := tower.ForceOpen(now, time.Minute); got != TowerNone {
		t.Errorf("abrir de novo = %v, want nada", got)
	}
	if got := tower.ForceEnd(); got != TowerEnd {
		t.Errorf("fim com a guerra aberta = %v, want end", got)
	}
}
