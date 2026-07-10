package grpcsrv

import (
	"context"
	"errors"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mapzones"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npcadmin"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npctemplates"
)

// fakeNpcAdmin is a scripted NpcAdmin for testing the gRPC mapping in
// isolation, mirroring fakeAccounts in server_test.go. Only the fields the
// tests below exercise are wired up; the rest return zero values.
type fakeNpcAdmin struct {
	listTemplatesRes npcadmin.Result
	listTemplates    []npctemplates.Template
	listTemplatesErr error
	lastModeratorID  int64

	listItemsRes npcadmin.Result
	listItems    []itemcatalog.Entry
	listItemsErr error

	listZonesRes npcadmin.Result
	listZones    []mapzones.Zone
	listZonesErr error

	setShopItems []domain.NPCShopItem
}

func (f *fakeNpcAdmin) List(context.Context, int64) (npcadmin.Result, []domain.NPCDefinition, error) {
	return npcadmin.OK, nil, nil
}
func (f *fakeNpcAdmin) Get(context.Context, int64, int64) (npcadmin.Result, domain.NPCDefinition, error) {
	return npcadmin.OK, domain.NPCDefinition{}, nil
}
func (f *fakeNpcAdmin) Upsert(context.Context, int64, domain.NPCDefinition) (npcadmin.Result, int64, error) {
	return npcadmin.OK, 0, nil
}
func (f *fakeNpcAdmin) SetVisibility(context.Context, int64, int64, bool) (npcadmin.Result, error) {
	return npcadmin.OK, nil
}
func (f *fakeNpcAdmin) SetShop(_ context.Context, _ int64, _ int64, items []domain.NPCShopItem) (npcadmin.Result, error) {
	f.setShopItems = items
	return npcadmin.OK, nil
}
func (f *fakeNpcAdmin) SetItemPrice(context.Context, int64, int32, int64) (npcadmin.Result, error) {
	return npcadmin.OK, nil
}
func (f *fakeNpcAdmin) Delete(context.Context, int64, int64) (npcadmin.Result, error) {
	return npcadmin.OK, nil
}
func (f *fakeNpcAdmin) ListMerchantTemplates(_ context.Context, moderatorID int64) (npcadmin.Result, []npctemplates.Template, error) {
	f.lastModeratorID = moderatorID
	return f.listTemplatesRes, f.listTemplates, f.listTemplatesErr
}
func (f *fakeNpcAdmin) ListItemCatalog(_ context.Context, moderatorID int64) (npcadmin.Result, []itemcatalog.Entry, error) {
	f.lastModeratorID = moderatorID
	return f.listItemsRes, f.listItems, f.listItemsErr
}
func (f *fakeNpcAdmin) ListMapZones(_ context.Context, moderatorID int64) (npcadmin.Result, []mapzones.Zone, error) {
	f.lastModeratorID = moderatorID
	return f.listZonesRes, f.listZones, f.listZonesErr
}

func TestListMerchantTemplatesMapping(t *testing.T) {
	fake := &fakeNpcAdmin{
		listTemplatesRes: npcadmin.OK,
		listTemplates: []npctemplates.Template{
			{TemplateName: "AkeMerchant", DisplayName: "Ake", Merchant: 1},
			{TemplateName: "HekalMerchant", DisplayName: "Hekal", Merchant: 19},
		},
	}
	s := NewNpcAdmin(fake)

	resp, err := s.ListMerchantTemplates(context.Background(), &webv1.ListMerchantTemplatesRequest{ModeratorId: 7})
	if err != nil {
		t.Fatalf("ListMerchantTemplates: %v", err)
	}
	if fake.lastModeratorID != 7 {
		t.Errorf("moderator id forwarded = %d, want 7", fake.lastModeratorID)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Fatalf("result = %v, want OK", resp.GetResult())
	}
	if len(resp.GetTemplates()) != 2 {
		t.Fatalf("templates = %+v, want 2 entries", resp.GetTemplates())
	}
	got := resp.GetTemplates()[0]
	if got.GetTemplateName() != "AkeMerchant" || got.GetDisplayName() != "Ake" || got.GetMerchant() != 1 {
		t.Errorf("templates[0] = %+v, want {AkeMerchant Ake 1}", got)
	}
}

func TestListMerchantTemplatesForbidden(t *testing.T) {
	fake := &fakeNpcAdmin{listTemplatesRes: npcadmin.Forbidden}
	s := NewNpcAdmin(fake)

	resp, err := s.ListMerchantTemplates(context.Background(), &webv1.ListMerchantTemplatesRequest{ModeratorId: 3})
	if err != nil {
		t.Fatalf("ListMerchantTemplates: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN {
		t.Errorf("result = %v, want FORBIDDEN", resp.GetResult())
	}
}

func TestListMerchantTemplatesInfraError(t *testing.T) {
	fake := &fakeNpcAdmin{listTemplatesErr: errors.New("scan failed")}
	s := NewNpcAdmin(fake)

	if _, err := s.ListMerchantTemplates(context.Background(), &webv1.ListMerchantTemplatesRequest{}); err == nil {
		t.Fatal("expected gRPC error on infra failure")
	}
}

func TestListItemCatalogMapping(t *testing.T) {
	fake := &fakeNpcAdmin{
		listItemsRes: npcadmin.OK,
		listItems: []itemcatalog.Entry{
			{Index: 1100, Name: "Espada Curta"},
			{Index: 1101, Name: "Adaga"},
		},
	}
	s := NewNpcAdmin(fake)

	resp, err := s.ListItemCatalog(context.Background(), &webv1.ListItemCatalogRequest{ModeratorId: 7})
	if err != nil {
		t.Fatalf("ListItemCatalog: %v", err)
	}
	if fake.lastModeratorID != 7 {
		t.Errorf("moderator id forwarded = %d, want 7", fake.lastModeratorID)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Fatalf("result = %v, want OK", resp.GetResult())
	}
	if len(resp.GetItems()) != 2 {
		t.Fatalf("items = %+v, want 2 entries", resp.GetItems())
	}
	got := resp.GetItems()[0]
	if got.GetItemIndex() != 1100 || got.GetName() != "Espada Curta" {
		t.Errorf("items[0] = %+v, want {1100 Espada Curta}", got)
	}
}

func TestListItemCatalogForbidden(t *testing.T) {
	fake := &fakeNpcAdmin{listItemsRes: npcadmin.Forbidden}
	s := NewNpcAdmin(fake)

	resp, err := s.ListItemCatalog(context.Background(), &webv1.ListItemCatalogRequest{ModeratorId: 3})
	if err != nil {
		t.Fatalf("ListItemCatalog: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN {
		t.Errorf("result = %v, want FORBIDDEN", resp.GetResult())
	}
}

func TestListItemCatalogInfraError(t *testing.T) {
	fake := &fakeNpcAdmin{listItemsErr: errors.New("scan failed")}
	s := NewNpcAdmin(fake)

	if _, err := s.ListItemCatalog(context.Background(), &webv1.ListItemCatalogRequest{}); err == nil {
		t.Fatal("expected gRPC error on infra failure")
	}
}

func TestSetNpcShopMapsQuantity(t *testing.T) {
	fake := &fakeNpcAdmin{}
	s := NewNpcAdmin(fake)

	resp, err := s.SetNpcShop(context.Background(), &webv1.SetNpcShopRequest{
		ModeratorId: 7,
		NpcId:       10,
		Items: []*webv1.AdminNpcShopItem{{
			Slot: 0, ItemIndex: 1100, Quantity: 120,
			Eff1: 2, Effv1: 9,
		}},
	})
	if err != nil {
		t.Fatalf("SetNpcShop: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Fatalf("result = %v, want OK", resp.GetResult())
	}
	if len(fake.setShopItems) != 1 {
		t.Fatalf("items = %+v, want 1", fake.setShopItems)
	}
	got := fake.setShopItems[0]
	if got.Quantity != 120 || got.Eff1 != 2 || got.EffV1 != 9 {
		t.Errorf("mapped item = %+v, want quantity 120 and effect 2/9", got)
	}
}

func TestProtoQuantityRejectsOutOfRange(t *testing.T) {
	if got := protoQuantity(65537); got != -1 {
		t.Errorf("protoQuantity overflow = %d, want -1", got)
	}
	if got := protoQuantity(0); got != 0 {
		t.Errorf("protoQuantity default marker = %d, want 0", got)
	}
}

func TestListMapZonesMapping(t *testing.T) {
	fake := &fakeNpcAdmin{
		listZonesRes: npcadmin.OK,
		listZones:    []mapzones.Zone{{ID: 0, Name: "Armia"}, {ID: 1, Name: "Azran"}},
	}
	s := NewNpcAdmin(fake)

	resp, err := s.ListMapZones(context.Background(), &webv1.ListMapZonesRequest{ModeratorId: 7})
	if err != nil {
		t.Fatalf("ListMapZones: %v", err)
	}
	if fake.lastModeratorID != 7 {
		t.Errorf("moderator id forwarded = %d, want 7", fake.lastModeratorID)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Fatalf("result = %v, want OK", resp.GetResult())
	}
	if len(resp.GetZones()) != 2 {
		t.Fatalf("zones = %+v, want 2 entries", resp.GetZones())
	}
	got := resp.GetZones()[0]
	if got.GetId() != 0 || got.GetName() != "Armia" {
		t.Errorf("zones[0] = %+v, want {0 Armia}", got)
	}
}

func TestListMapZonesForbidden(t *testing.T) {
	fake := &fakeNpcAdmin{listZonesRes: npcadmin.Forbidden}
	s := NewNpcAdmin(fake)

	resp, err := s.ListMapZones(context.Background(), &webv1.ListMapZonesRequest{ModeratorId: 3})
	if err != nil {
		t.Fatalf("ListMapZones: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN {
		t.Errorf("result = %v, want FORBIDDEN", resp.GetResult())
	}
}

func TestListMapZonesInfraError(t *testing.T) {
	fake := &fakeNpcAdmin{listZonesErr: errors.New("boom")}
	s := NewNpcAdmin(fake)

	if _, err := s.ListMapZones(context.Background(), &webv1.ListMapZonesRequest{}); err == nil {
		t.Fatal("expected gRPC error on infra failure")
	}
}
