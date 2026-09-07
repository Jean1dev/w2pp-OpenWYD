package level

import "testing"

// Adding the desert zones must not move a single point of experience. They are
// copies of the field branch under new names, so until a moderator edits one,
// every kill in the desert has to pay exactly what it paid when the whole belt
// was ZoneField. This is the test that makes the change safe to deploy: a
// silent shift here is a week of levelling nobody can undo.
func TestDesertoPagaIgualAoCampoAteAlguemEditar(t *testing.T) {
	desertos := []Zone{
		ZoneDesertoPilar, ZoneDesertoManticora, ZoneDesertoLugefer,
		ZoneDesertoBaixo, ZoneDesertoReino,
	}
	tiers := []struct {
		nome string
		t    Tier
	}{
		{"mortal", Tier{ClassMaster: classMortal}},
		{"arch", Tier{ClassMaster: classArch, ArchLv355: true, ArchLv370: true}},
		{"celestial", Tier{ClassMaster: classCelestial, CelLv40: true, CelLv90: true}},
	}
	for _, tr := range tiers {
		for _, nivel := range []int32{50, 200, 300, 370, 395} {
			in := ExpRewardInput{
				Zone: ZoneField, MobExp: 500_000, KillerLevel: nivel, MobLevel: 399, Tier: tr.t,
			}
			campo := ExpReward(in)
			if campo <= 0 {
				t.Fatalf("%s nível %d: campo pagou %d — o caso não testa nada", tr.nome, nivel, campo)
			}
			for _, z := range desertos {
				in.Zone = z
				if got := ExpReward(in); got != campo {
					t.Errorf("%s nível %d em %s pagou %d, campo paga %d — o deserto tem que "+
						"começar idêntico ao campo", tr.nome, nivel, z.Name(), got, campo)
				}
			}
		}
	}
}

// The rectangles have to select the zones they claim to. Tauron is the reason
// the user asked for this: its mobs spawn around 1300-1382, 1796-1898.
func TestRetangulosDoDeserto(t *testing.T) {
	casos := []struct {
		nome string
		x, y int32
		want Zone
	}{
		{"onde nascem os Tauron", 1300, 1802, ZoneDesertoLugefer},
		{"outro ponto de Tauron", 1382, 1814, ZoneDesertoLugefer},
		{"Pilar", 1200, 1700, ZoneDesertoPilar},
		{"Manticora", 1350, 1700, ZoneDesertoManticora},
		{"Baixo", 1450, 1700, ZoneDesertoBaixo},
		{"Reino", 1600, 1700, ZoneDesertoReino},
		// Just outside the belt: still the open field.
		{"acima do deserto", 1300, 1500, ZoneField},
		{"a leste do deserto", 1800, 1700, ZoneField},
	}
	for _, c := range casos {
		if got := ZoneForTile(c.x, c.y); got != c.want {
			t.Errorf("%s (%d,%d) = %s, want %s", c.nome, c.x, c.y, got.Name(), c.want.Name())
		}
	}
}

// The legacy blocks must win over any rectangle. A desert rectangle that
// swallowed a Pesadelo or Água kill would silently change a dungeon's pay,
// which is the one thing this whole file is not allowed to do.
func TestBlocoDoLegadoVenceORetangulo(t *testing.T) {
	casos := []struct {
		x, y int32
		want Zone
	}{
		{1250, 3608, ZoneAguaMistico},
		{1342, 3517, ZoneAguaArcano},
		{1090, 315, ZonePesadeloMistico},
		{1310, 330, ZonePesadeloNormal},
		{1216, 192, ZonePesadeloArcano},
	}
	for _, c := range casos {
		if got := ZoneForTile(c.x, c.y); got != c.want {
			t.Errorf("(%d,%d) = %s, want %s — um retângulo roubou uma zona do legado",
				c.x, c.y, got.Name(), c.want.Name())
		}
	}
}

// Every zone needs a usable name and rule, including the new ones: the panel
// builds its tabs straight off Zones(), so a missing entry renders as an
// unnamed tab that saves to a branch nobody can identify.
func TestTodaZonaTemNomeERegra(t *testing.T) {
	vistos := map[string]bool{}
	for _, z := range Zones() {
		nome := z.Name()
		if nome == "" {
			t.Errorf("zona %d ficou sem nome", z)
			continue
		}
		if vistos[nome] {
			t.Errorf("duas zonas se chamam %q — as abas do painel ficariam ambíguas", nome)
		}
		vistos[nome] = true
	}
	if len(Zones()) != 12 {
		t.Errorf("%d zonas, want 12 (7 do legado + 5 do deserto)", len(Zones()))
	}
}

// A kill reaching across a zone boundary falls back to the field, exactly as
// the legacy's shared-block guard does. Otherwise standing in Pilar and killing
// into Manticora would pay on a table neither of them agreed to.
func TestMatarAtravessandoAFronteiraCaiNoCampo(t *testing.T) {
	// Mob in Lugefer, killer in Baixo.
	if got := ZoneForKill(1300, 1802, 1450, 1700); got != ZoneField {
		t.Errorf("kill atravessando fronteira = %s, want Campo", got.Name())
	}
	// Both inside Lugefer: the zone holds.
	if got := ZoneForKill(1300, 1802, 1320, 1810); got != ZoneDesertoLugefer {
		t.Errorf("kill dentro do Lugefer = %s, want Deserto Lugefer", got.Name())
	}
}
