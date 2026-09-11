package world

import (
	"math"
	"testing"
)

// TestKefraNaoEntraNaFila: o Kefra (396) e os guardas (397-400) voltam na terça
// (handler/kefra.go). Se entrassem na fila de 15 s, o chefe semanal voltaria
// antes de o corpo esfriar. O bloco vizinho, fora da faixa, continua na fila.
func TestKefraNaoEntraNaFila(t *testing.T) {
	now := uint32(1000)
	w := New(Config{GridDim: 16, Now: func() uint32 { return now }}, slogDiscard(), nil, nil)
	for i, gen := range []int16{KefraBossGenIndex, KefraBossGenIndex + 1, KefraGuardLast} {
		id := w.SpawnMobAt(MobSpawn{Template: make([]byte, structMobTemplateSize), X: int16(2 + i), Y: 2, GenIndex: gen})
		if id < MaxUser {
			t.Fatalf("SpawnMobAt(bloco %d) = %d, want a mob id", gen, id)
		}
		w.DespawnMob(id, 1)
	}
	if len(w.respawnQueue) != 0 {
		t.Fatalf("respawnQueue len = %d, want o Kefra e os guardas fora da fila", len(w.respawnQueue))
	}
	id := w.SpawnMobAt(MobSpawn{Template: make([]byte, structMobTemplateSize), X: 9, Y: 2, GenIndex: KefraBossGenIndex - 1})
	w.DespawnMob(id, 1)
	if len(w.respawnQueue) != 1 {
		t.Fatalf("respawnQueue len = %d, want o bloco 395 na fila de sempre", len(w.respawnQueue))
	}
}

// TestFilaAguentaAViradaDoRelogio: o relógio do mundo é de 32 bits em ms e vira
// a cada ~49,7 dias. Um chefe posto na fila por 24 h logo antes da virada não
// pode voltar na hora, nem antes do prazo, e tem de voltar no prazo.
func TestFilaAguentaAViradaDoRelogio(t *testing.T) {
	const dia = 24 * 3600 * 1000
	now := uint32(math.MaxUint32 - 1000)
	w := New(Config{GridDim: 16, Now: func() uint32 { return now }}, slogDiscard(), nil, nil)
	w.SetRespawnDelayFor(func(int32) uint32 { return dia })

	id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	w.DespawnMob(id, 1)
	if ids := w.SpawnDueRespawns(now); len(ids) != 0 {
		t.Fatalf("voltou na hora, antes da virada: %v", ids)
	}
	if ids := w.SpawnDueRespawns(now + dia - 1); len(ids) != 0 {
		t.Fatalf("voltou 1 ms antes do prazo, já depois da virada: %v", ids)
	}
	if ids := w.SpawnDueRespawns(now + dia); len(ids) != 1 {
		t.Fatalf("não voltou no prazo, depois da virada: %v", ids)
	}
}
