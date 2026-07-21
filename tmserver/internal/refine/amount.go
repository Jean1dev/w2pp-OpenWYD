package refine

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// efAmount is EF_AMOUNT (ItemEffect.h): a stack size, when an item carries
// one. Shared by the handler and combine packages, which both already depend
// on refine for the sanc encoding.
const efAmount = 61

// Amount returns an item's stack size (BASE_GetItemAmount), or 1 for an item
// with no EF_AMOUNT pair.
func Amount(it world.Item) int {
	for _, ef := range it.Effects {
		if ef.Effect == efAmount {
			return int(ef.Value)
		}
	}
	return 1
}
