package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestGolpeUsaAContaDaJanela prova a ligação, não a fórmula: resolveSkillHit tem
// de entregar a SkillBaseDamage o tier, as skills aprendidas e a Magia com teto do
// personagem. Um campo esquecido faria o golpe cair calado no ramo antigo.
//
// O alvo não tem AC nem resistência e o mundo é novo (rng.New é determinístico),
// então o golpe é o bruto da skill passado pelo mesmo sorteio e pelo ×1,5 da
// resistência zero — o esperado sai das mesmas funções, do mesmo ponto do sorteio.
func TestGolpeUsaAContaDaJanela(t *testing.T) {
	fera := content.Spell{InstanceType: 2, InstanceValue: 25} // skill 48, Fera Flamejante
	golpe := func(e *world.Entity) int {
		d := New(Config{})
		w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
		target := &world.Entity{ID: 2}
		cast := castInfo{isSkill: true, spell: fera, special: 185}
		return d.resolveSkillHit(w, e, target, target.ID, 48, cast)
	}
	esperado := func(bruto int) int {
		dmg := combat.SkillDamage(rng.New(), bruto, 0, 0)
		return combat.SkillResistScale(dmg, 2, [4]int16{}, false)
	}
	bm := func(tier uint8, magic int16, learned int32) *world.Entity {
		return &world.Entity{ID: 1, Class: 2, Level: 349, Int: 2377, Magic: magic,
			ClassMaster: tier, LearnedSkill: learned}
	}

	tests := []struct {
		name  string
		e     *world.Entity
		bruto int
	}{
		// O número da janela da captura: 11.295.
		{"BM Mortal, Magia 155", bm(classMasterMortal, 155, 0), 11295},
		// Arch: maestria ×2 e nível inteiro (1615 × 7,2 × 5/4).
		{"BM Arch, Magia 155", bm(classMasterArch, 155, 0), 14535},
		// Magia 411 bate como 254: 1255 × (4×254+100)/100 = 14005; ×5/4 = 17506.
		// Sem o teto seria 1255 × 17,44 × 5/4 = 27358.
		{"BM Mortal, Magia acima do byte", bm(classMasterMortal, 411, 0), 17506},
		// Com a 8ª skill da Elemental (bit 7), a BM ganha +10% na árvore 1.
		{"BM Mortal com a 8ª da Elemental", bm(classMasterMortal, 155, 1<<7), 11295 * 110 / 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := golpe(tt.e), esperado(tt.bruto); got != want {
				t.Errorf("golpe = %d, want %d (bruto %d)", got, want, tt.bruto)
			}
		})
	}
}
