// Package dungeon names the instanced dungeons' doors, so the game, the
// database and the panel all mean the same thing by "Pesadelo Místico".
//
// It lives at the repo root rather than inside tmserver because the identity is
// shared: tmServer enforces a door, dbServer stores it, and the panel opens and
// closes it. A second enumeration on either side could only disagree with the
// first, and a disagreement here means the wrong dungeon is shut.
package dungeon

// Gate is one door that can be opened or closed independently.
//
// The numbering is the storage key, so entries are only ever APPENDED: renaming
// is free, reordering would silently move every saved row to another dungeon.
// The Cubo da Maldade is deliberately absent — it has no entry handler at all,
// and a door onto a dungeon nobody can enter would be a control that does
// nothing.
type Gate uint8

const (
	PesadeloN Gate = iota
	PesadeloM
	PesadeloA
	AguaN
	AguaM
	AguaA
	Carta
)

// Kind groups the gates that belong to one dungeon, for the screen's tabs.
type Kind uint8

const (
	KindPesadelo Kind = iota
	KindAgua
	KindCarta
)

type meta struct {
	name string
	kind Kind
	// tier is the progression label the door carries, empty for a dungeon that
	// has only one.
	tier string
	// kindName is the dungeon's own name, repeated per gate so the tabs do not
	// need a second table.
	kindName string
}

var gates = [...]meta{
	PesadeloN: {"Pesadelo Normal", KindPesadelo, "Normal", "Pesadelo"},
	PesadeloM: {"Pesadelo Místico", KindPesadelo, "Místico", "Pesadelo"},
	PesadeloA: {"Pesadelo Arcano", KindPesadelo, "Arcano", "Pesadelo"},
	AguaN:     {"Água Normal", KindAgua, "Normal", "Pergaminho da Água"},
	AguaM:     {"Água Místico", KindAgua, "Místico", "Pergaminho da Água"},
	AguaA:     {"Água Arcano", KindAgua, "Arcano", "Pergaminho da Água"},
	Carta:     {"Carta de Duelo", KindCarta, "", "Carta de Duelo"},
}

// Gates lists every door, in display order.
func Gates() []Gate {
	out := make([]Gate, len(gates))
	for i := range gates {
		out[i] = Gate(i)
	}
	return out
}

// Valid reports whether a number read from storage or a form names a door.
func Valid(g int32) bool { return g >= 0 && int(g) < len(gates) }

// Name is the door's full name.
func (g Gate) Name() string {
	if int(g) >= len(gates) {
		return "desconhecida"
	}
	return gates[g].name
}

// Tier is the progression label, empty for a dungeon with a single door.
func (g Gate) Tier() string {
	if int(g) >= len(gates) {
		return ""
	}
	return gates[g].tier
}

// Kind is the dungeon this door belongs to.
func (g Gate) Kind() Kind {
	if int(g) >= len(gates) {
		return KindPesadelo
	}
	return gates[g].kind
}

// DungeonName is the dungeon's own name.
func (g Gate) DungeonName() string {
	if int(g) >= len(gates) {
		return ""
	}
	return gates[g].kindName
}

// State is one door's setting.
//
// The zero value is deliberately NOT the default: a door with no row is open and
// announced, because that is how the server behaved before any of this existed
// and a fresh database must not silently shut the game. Config.Of applies that.
type State struct {
	Open bool
	// Announce controls the automatic "opens in one minute" notice. It is
	// separate from Open because a door can be open and quiet — an event the
	// staff wants running without drawing the whole server to it.
	Announce bool
}

// Config is the whole door configuration as the game reads it.
type Config struct {
	Version int64
	// States holds only the doors somebody has touched. Absence means the
	// default, which is why this is a map and not an array.
	States map[Gate]State
}

// Of returns a door's setting, defaulting to open and announced.
func (c Config) Of(g Gate) State {
	if st, ok := c.States[g]; ok {
		return st
	}
	return State{Open: true, Announce: true}
}

// IsOpen is the question the entry handlers ask.
func (c Config) IsOpen(g Gate) bool { return c.Of(g).Open }

// Announces is the question the minute timer asks.
func (c Config) Announces(g Gate) bool { return c.Of(g).Announce }
