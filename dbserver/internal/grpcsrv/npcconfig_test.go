package grpcsrv

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func TestNPCDefinitionsToProtoMapsQuantity(t *testing.T) {
	defs := []domain.NPCDefinition{{
		ID: 1, Slug: "shop", TemplateName: "Keeper", Enabled: true,
		Shop: []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100, Quantity: 120, Eff1: 2, EffV1: 9}},
	}}

	got := npcDefinitionsToProto(defs)
	if len(got) != 1 || len(got[0].GetShop()) != 1 {
		t.Fatalf("proto defs = %+v, want one shop item", got)
	}
	item := got[0].GetShop()[0]
	if item.GetQuantity() != 120 || item.GetEff1() != 2 || item.GetEffv1() != 9 {
		t.Errorf("shop item = %+v, want quantity 120 and effect 2/9", item)
	}
}
