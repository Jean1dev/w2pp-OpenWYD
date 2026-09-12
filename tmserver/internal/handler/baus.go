package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The baús: a consumable that is opened into one item drawn from its table
// (_MSG_UseItem.cpp:5886-6027 and :6309-6685). The legacy writes each one as its
// own block of `rand()%100` thresholds; here they are data, one table per baú,
// and one path opens them all.
//
// They are dispatched by sIndex, before the EF_VOLATILE switch, because the
// volatile does not identify them: the legacy tests `Vol == 210 && sIndex ==`,
// and the Bau de Runas at 5750 carries Vol 200 — the Kappa buff potion's class —
// so going by volatile would drink it.
//
// Two deliberate divergences:
//
//   - A full bag refuses, and the baú stays. The legacy spends the baú first and
//     then PutItem drops the prize on the floor of a full inventory, so the
//     player loses both (Recusar em vez de apagar).
//   - The Baú do Âmago Especial gives the Svadilfari's and the Sleipnir's âmago
//     (2417/2418). The legacy gave 2400 and 2411, which fed those two mounts only
//     because it shared the Andaluz N's and the Unicórnio's âmago with them — a
//     sharing this server dropped (amago.go, mountAmagoSlot). Same odds, same
//     stack sizes, the mounts the baú was meant for.
//
// 4007 and 5750 are also named Baú de Runas in the catalog and have no code in
// the legacy at all; they open like 3219.

// chestPrize is one line of a baú's table: a roll of rand()%100 below upTo
// gives item.
type chestPrize struct {
	upTo int
	item world.Item
}

const (
	itemBauPedraSecreta = 3212
	itemBauRunas        = 3219
	itemBauRunas2       = 4007
	itemBauRunas3       = 5750
	itemAmagoSvadilfari = 2417
	itemAmagoSleipnir   = 2418
	itemSvadilfari      = 2387
	itemSleipnir        = 2388
)

func plainPrize(upTo int, index int16) chestPrize {
	return chestPrize{upTo: upTo, item: world.Item{Index: index}}
}

func stackPrize(upTo int, index int16, amount uint8) chestPrize {
	it := world.Item{Index: index}
	it.Effects[0] = world.Effect{Effect: efAmount, Value: amount}
	return chestPrize{upTo: upTo, item: it}
}

// mountPrize is the adult mount the Baú da Montaria gives, as the legacy builds
// it (:6610-6626): HP 26652 in the first effect, level 1, ration 100. The
// vitality is rolled at open time (rollAdultVitality), not the legacy's flat 20,
// so a mount from a baú is born like every other adult.
func mountPrize(upTo int, index int16) chestPrize {
	it := world.Item{Index: index}
	it.Effects[0] = world.Effect{Effect: 28, Value: 104}
	it.Effects[1] = world.Effect{Effect: 1}
	it.Effects[2] = world.Effect{Effect: 100, Value: 1}
	return chestPrize{upTo: upTo, item: it}
}

// runeTable is the Baú de Runas (:5889-6011): seven runes of each element, 3%
// each except Isa's 6%, and 13% of three more baús.
var runeTable = []chestPrize{
	plainPrize(3, 5110), plainPrize(6, 5124), plainPrize(9, 5117), plainPrize(12, 5129), // Sol: Ansuz, Elhaz, Gebo, Mannaz
	plainPrize(15, 5114), plainPrize(18, 5125), plainPrize(21, 5128), // Sol: Raidho, Sowilo, Tiwaz
	plainPrize(24, 5131), plainPrize(27, 5113), plainPrize(30, 5115), plainPrize(33, 5116), // Terra: Dagaz, Fehu, Kenaz, Naudhiz
	plainPrize(36, 5125), plainPrize(39, 5112), plainPrize(42, 5114), // Terra: Sowilo, Thurisaz, Raidho
	plainPrize(45, 5126), plainPrize(48, 5127), plainPrize(51, 5121), plainPrize(54, 5114), // Água: Berkano, Ehwaz, Jara, Raidho
	plainPrize(57, 5125), plainPrize(60, 5111), plainPrize(63, 5118), // Água: Sowilo, Uraz, Wunjo
	plainPrize(66, 5122), plainPrize(69, 5119), plainPrize(72, 5132), plainPrize(78, 5120), // Vento: Eihwaz, Hagalaz, Ing, Isa
	plainPrize(81, 5130), plainPrize(84, 5133), plainPrize(87, 5123), // Vento: Laguz, Othel, Perthro
	stackPrize(100, itemBauRunas, 3), // "tenta de novo x3"
}

// chestTables is every baú this server opens.
var chestTables = map[int16][]chestPrize{
	// Baú da Pedra Secreta (:6309): Água, Terra, Sol, Vento.
	itemBauPedraSecreta: {plainPrize(25, 5334), plainPrize(50, 5335), plainPrize(75, 5336), plainPrize(100, 5337)},
	// Baús de peça (:6368-6603): Flamejante, Guardiã, Destruição, Rake.
	3213: {plainPrize(25, 1221), plainPrize(50, 1356), plainPrize(75, 1506), plainPrize(100, 1656)}, // peito
	3214: {plainPrize(25, 1222), plainPrize(50, 1357), plainPrize(75, 1507), plainPrize(100, 1657)}, // calça
	3215: {plainPrize(25, 1223), plainPrize(50, 1358), plainPrize(75, 1508), plainPrize(100, 1658)}, // luva
	3216: {plainPrize(25, 1224), plainPrize(50, 1359), plainPrize(75, 1509), plainPrize(100, 1659)}, // bota
	// Baú da Montaria (:6604): Sleipnir or Svadilfari.
	3217: {mountPrize(50, itemSleipnir), mountPrize(100, itemSvadilfari)},
	// Baú do Âmago Especial (:6643), with the mounts' own âmago — see above.
	3218: {
		stackPrize(15, itemAmagoSvadilfari, 20), stackPrize(30, itemAmagoSleipnir, 20),
		stackPrize(65, itemAmagoSvadilfari, 10), stackPrize(100, itemAmagoSleipnir, 10),
	},
	itemBauRunas:  runeTable,
	itemBauRunas2: runeTable,
	itemBauRunas3: runeTable,
}

// drawChest picks the prize for a roll in [0,100).
func drawChest(table []chestPrize, roll int) world.Item {
	for _, p := range table {
		if roll < p.upTo {
			return p.item
		}
	}
	return table[len(table)-1].item
}

// msgFullCarry is _NN_FUll_CARRY (Language.txt:452).
const msgFullCarry = "Seu inventário está cheio."

// openChest opens the baú in carry slot src, if it is one. It reports whether
// the item was a baú, so the caller stops there either way.
func (d *Dispatcher) openChest(w *world.World, s *world.Session, e *world.Entity, src int) bool {
	chest := e.Carry[src].Index
	table, ok := chestTables[chest]
	if !ok {
		return false
	}
	// The prize takes the baú's own slot when this was the last one of the
	// stack; otherwise it needs a free slot, and without one nothing is spent.
	dst := src
	if itemAmount(e.Carry[src]) > 1 {
		dst = firstEmptyAccessibleCarry(e)
		if dst < 0 {
			sendClientMessage(w, s, msgFullCarry)
			d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
			return true
		}
	}

	prize := drawChest(table, w.Rand().Intn(100))
	if prize.Index == itemSleipnir || prize.Index == itemSvadilfari {
		prize.Effects[1].Value = rollAdultVitality(w)
	}
	consumeOneItem(&e.Carry[src])

	// With a merging fairy a stackable prize goes onto the pile it belongs to
	// (carry.go) — the whole point when a chest hands out Âmagos or gems one at
	// a time. The slot reserved above simply goes unused then.
	//
	// Everyone else keeps the old placement, which is deliberate: the prize
	// appearing in the chest's own slot is how a player follows what happened.
	// That placement is also why the chest's slot is pushed only AFTER the prize
	// is written — when dst is src, the one SendItem for that slot has to carry
	// the prize, not the emptied chest.
	if fadaJuntaPilhas(e) && isSplittable(prize.Index) {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		if d.putCarryItem(w, e, prize) < 0 {
			sendClientMessage(w, s, msgFullCarry)
			d.log.Warn("prêmio do baú perdido: bolsa cheia",
				"conn", s.Conn, "bau", chest, "premio", prize.Index)
			return true
		}
	} else {
		e.Carry[dst] = prize
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		if dst != src {
			d.sendSlot(w, s, world.ItemPlaceCarry, dst, e.Carry[dst])
		}
	}
	sendClientMessage(w, s, fmt.Sprintf("!Chegou o item %s", d.itemName(prize.Index)))
	d.log.Info("baú aberto", "conn", s.Conn, "account", s.AccountName, "bau", chest, "premio", prize.Index)
	return true
}
