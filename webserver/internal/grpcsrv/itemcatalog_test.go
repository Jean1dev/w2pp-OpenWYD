package grpcsrv

import (
	"context"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
)

func TestListItemsMapping(t *testing.T) {
	s := NewItemCatalog(itemcatalog.Catalog{
		Version: "deadbeefdeadbeef", IconPackVersion: "cafebabecafebabe",
		Items: []itemcatalog.Entry{
			{
				Index: 1188, Name: "Botas_Douradas(N)", DisplayName: "Botas Douradas(N)",
				Mesh: 10, Texture: 0, SlotMask: 32, Slots: []string{"boots"},
				Grade: 1, IconKey: "i0042", IconURL: "https://storage.example/i0042",
			},
			{
				Index: 3002, Name: "Cupom_da_Sorte", DisplayName: "Cupom da Sorte",
				Mesh: 2711,
			},
		},
	})

	resp, err := s.ListItems(context.Background(), &webv1.ListItemsRequest{})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if resp.GetCatalogVersion() != "deadbeefdeadbeef" {
		t.Errorf("catalog version = %q, want deadbeefdeadbeef", resp.GetCatalogVersion())
	}
	if resp.GetIconPackVersion() != "cafebabecafebabe" {
		t.Errorf("icon pack version = %q, want cafebabecafebabe", resp.GetIconPackVersion())
	}
	if len(resp.GetItems()) != 2 {
		t.Fatalf("items = %+v, want 2 entries", resp.GetItems())
	}

	boots := resp.GetItems()[0]
	if boots.GetItemIndex() != 1188 || boots.GetIconKey() != "i0042" {
		t.Errorf("items[0] = %d / %q, want 1188 / i0042", boots.GetItemIndex(), boots.GetIconKey())
	}
	if boots.GetIconUrl() != "https://storage.example/i0042" {
		t.Errorf("items[0] URL = %q", boots.GetIconUrl())
	}
	if boots.GetDisplayName() != "Botas Douradas(N)" || boots.GetGrade() != 1 {
		t.Errorf("items[0] = %q / grade %d, want Botas Douradas(N) / grade 1", boots.GetDisplayName(), boots.GetGrade())
	}
	if boots.GetSlotMask() != 32 || len(boots.GetSlots()) != 1 || boots.GetSlots()[0] != "boots" {
		t.Errorf("items[0] slots = %d / %v, want 32 / [boots]", boots.GetSlotMask(), boots.GetSlots())
	}

	// A non-equippable item carries an empty slot list, not a missing key: the
	// front still needs an icon for coupons and potions.
	coupon := resp.GetItems()[1]
	if coupon.GetSlotMask() != 0 || len(coupon.GetSlots()) != 0 {
		t.Errorf("items[1] slots = %d / %v, want 0 / []", coupon.GetSlotMask(), coupon.GetSlots())
	}
	if coupon.GetIconKey() != "" {
		t.Errorf("items[1] icon = %q, want fallback", coupon.GetIconKey())
	}
}

// TestListItemsWithoutContent pins the degraded mode: webserver started without
// -content/W2PP_CONTENT must serve an empty catalog rather than fail, matching
// how the moderator pickers already behave.
func TestListItemsWithoutContent(t *testing.T) {
	s := NewItemCatalog(itemcatalog.Catalog{})

	resp, err := s.ListItems(context.Background(), &webv1.ListItemsRequest{})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(resp.GetItems()) != 0 {
		t.Errorf("items = %+v, want empty", resp.GetItems())
	}
	if resp.GetCatalogVersion() != "" {
		t.Errorf("catalog version = %q, want empty", resp.GetCatalogVersion())
	}
}
