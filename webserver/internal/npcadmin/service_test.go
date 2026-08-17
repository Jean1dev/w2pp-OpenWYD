package npcadmin

import (
	"bytes"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/droptool"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mapzones"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npctemplates"
)

// fakeStore is an in-memory Store for exercising the service's authorization and
// validation without a database.
type fakeStore struct {
	roles      map[int64]string
	defs       map[int64]domain.NPCDefinition
	upsertErr  error
	lastShop   []domain.NPCShopItem
	lastPrice  *int64
	deletedID  int64
	visibility *bool
	itemPrices []domain.ItemPriceOverride
}

func (f *fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	r, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return r, nil
}
func (f *fakeStore) ListNPCDefinitions(context.Context) ([]domain.NPCDefinition, error) {
	out := make([]domain.NPCDefinition, 0, len(f.defs))
	for _, d := range f.defs {
		out = append(out, d)
	}
	return out, nil
}
func (f *fakeStore) GetNPCDefinition(_ context.Context, id int64) (domain.NPCDefinition, error) {
	d, ok := f.defs[id]
	if !ok {
		return domain.NPCDefinition{}, store.ErrNotFound
	}
	return d, nil
}
func (f *fakeStore) UpsertNPCDefinition(_ context.Context, _ domain.NPCDefinition, _ int64) (int64, error) {
	if f.upsertErr != nil {
		return 0, f.upsertErr
	}
	return 42, nil
}
func (f *fakeStore) SetNPCShop(_ context.Context, id int64, items []domain.NPCShopItem, _ int64) error {
	if _, ok := f.defs[id]; !ok {
		return store.ErrNotFound
	}
	f.lastShop = items
	return nil
}
func (f *fakeStore) SetNPCVisibility(_ context.Context, id int64, enabled bool, _ int64) error {
	if _, ok := f.defs[id]; !ok {
		return store.ErrNotFound
	}
	f.visibility = &enabled
	return nil
}
func (f *fakeStore) SetItemPrice(_ context.Context, _ int32, price int64, _ int64) error {
	f.lastPrice = &price
	return nil
}
func (f *fakeStore) ItemPriceOverrides(context.Context) ([]domain.ItemPriceOverride, error) {
	return f.itemPrices, nil
}
func (f *fakeStore) DeleteNPCDefinition(_ context.Context, id int64, _ int64) error {
	d, ok := f.defs[id]
	if !ok {
		return store.ErrNotFound
	}
	// Mirrors the real store: versioned content is hidden, never deleted.
	if d.Origin == "content" {
		return store.ErrContentOwned
	}
	f.deletedID = id
	return nil
}

func newFake() *fakeStore {
	return &fakeStore{
		roles: map[int64]string{1: "moderator", 2: "admin", 3: "player"},
		defs: map[int64]domain.NPCDefinition{
			10: {ID: 10, Slug: "shop-10", Merchant: 1},
			11: {ID: 11, Slug: "Set_BM-6079", Origin: "content"},
		},
	}
}

// TestAuthorization checks every operation refuses non-moderators. A player, a
// missing account, and a zero id all yield Forbidden; a moderator/admin passes.
func TestAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		moderator  int64
		wantResult Result
	}{
		{"moderator ok", 1, OK},
		{"admin ok", 2, OK},
		{"player forbidden", 3, Forbidden},
		{"missing account forbidden", 99, Forbidden},
		{"zero id forbidden", 0, Forbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(newFake())
			got, _, err := s.List(context.Background(), tt.moderator)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if got != tt.wantResult {
				t.Errorf("List result = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestUpsertValidation(t *testing.T) {
	tests := []struct {
		name string
		def  domain.NPCDefinition
		want Result
	}{
		{"ok", domain.NPCDefinition{Slug: "s", TemplateName: "t", Merchant: 1}, OK},
		{"king ok", domain.NPCDefinition{Slug: "king", TemplateName: "Rei_Harabard", Merchant: 111}, OK},
		{"empty slug", domain.NPCDefinition{TemplateName: "t"}, Invalid},
		{"empty template", domain.NPCDefinition{Slug: "s"}, Invalid},
		{"legacy quest merchant", domain.NPCDefinition{Slug: "s", TemplateName: "t", Merchant: 7}, OK},
		{"legacy event merchant", domain.NPCDefinition{Slug: "s", TemplateName: "t", Merchant: 96}, OK},
		{"merchant outside byte range", domain.NPCDefinition{Slug: "s", TemplateName: "t", Merchant: 256}, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(newFake())
			got, _, err := s.Upsert(context.Background(), 1, tt.def)
			if err != nil {
				t.Fatalf("Upsert: %v", err)
			}
			if got != tt.want {
				t.Errorf("Upsert result = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetShopValidation(t *testing.T) {
	tests := []struct {
		name  string
		items []domain.NPCShopItem
		want  Result
	}{
		{"ok", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100}, {Slot: 1, ItemIndex: 1101}}, OK},
		{"quantity ok", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100, Quantity: 120}}, OK},
		{"slot too high", []domain.NPCShopItem{{Slot: 27, ItemIndex: 1100}}, Invalid},
		{"negative slot", []domain.NPCShopItem{{Slot: -1, ItemIndex: 1100}}, Invalid},
		{"zero item", []domain.NPCShopItem{{Slot: 0, ItemIndex: 0}}, Invalid},
		{"quantity too low", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100, Quantity: -1}}, Invalid},
		{"quantity too high", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100, Quantity: 256}}, Invalid},
		{"raw amount rejected", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100, Eff1: 61, EffV1: 120}}, Invalid},
		{"quantity needs empty effect slot", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100, Quantity: 2, Eff1: 1, Eff2: 2, Eff3: 3}}, Invalid},
		{"duplicate slot", []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100}, {Slot: 0, ItemIndex: 1101}}, Invalid},
		{"empty shop ok", nil, OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(newFake())
			got, err := s.SetShop(context.Background(), 1, 10, tt.items)
			if err != nil {
				t.Fatalf("SetShop: %v", err)
			}
			if got != tt.want {
				t.Errorf("SetShop result = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSetShopNotFound checks a write to a missing definition surfaces NotFound.
func TestSetShopNotFound(t *testing.T) {
	s := New(newFake())
	got, err := s.SetShop(context.Background(), 1, 999, []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100}})
	if err != nil {
		t.Fatalf("SetShop: %v", err)
	}
	if got != NotFound {
		t.Errorf("SetShop result = %v, want NotFound", got)
	}
}

func TestSetShopDefaultsQuantity(t *testing.T) {
	fs := newFake()
	s := New(fs)
	got, err := s.SetShop(context.Background(), 1, 10, []domain.NPCShopItem{{Slot: 0, ItemIndex: 1100}})
	if err != nil {
		t.Fatalf("SetShop: %v", err)
	}
	if got != OK {
		t.Fatalf("SetShop result = %v, want OK", got)
	}
	if fs.lastShop[0].Quantity != 1 {
		t.Errorf("stored quantity = %d, want default 1", fs.lastShop[0].Quantity)
	}
}

// TestDeleteNotFound checks deleting a missing definition surfaces NotFound.
func TestDeleteNotFound(t *testing.T) {
	s := New(newFake())
	got, err := s.Delete(context.Background(), 1, 999)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got != NotFound {
		t.Errorf("Delete result = %v, want NotFound", got)
	}
}

// TestDeleteContentOwned checks a definition that came from NPCGener.txt is
// refused rather than deleted, and that the refusal is logged: the store rejects
// before writing its audit row, so the log is the only server-side trace (#257).
func TestDeleteContentOwned(t *testing.T) {
	fs := newFake()
	s := New(fs)
	var logs bytes.Buffer
	s.SetLogger(slog.New(slog.NewTextHandler(&logs, nil)))

	got, err := s.Delete(context.Background(), 1, 11)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got != ContentOwned {
		t.Errorf("Delete result = %v, want ContentOwned", got)
	}
	if fs.deletedID != 0 {
		t.Errorf("deleted id = %d, want none deleted", fs.deletedID)
	}
	if !strings.Contains(logs.String(), "content-owned") || !strings.Contains(logs.String(), "npc_id=11") {
		t.Errorf("log = %q, want a content-owned refusal naming npc_id=11", logs.String())
	}
}

// TestDeleteContentOwnedWithoutLogger checks the logger is optional — the
// refusal still classifies correctly when SetLogger was never called.
func TestDeleteContentOwnedWithoutLogger(t *testing.T) {
	s := New(newFake())
	got, err := s.Delete(context.Background(), 1, 11)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got != ContentOwned {
		t.Errorf("Delete result = %v, want ContentOwned", got)
	}
}

// TestListMerchantTemplates checks the authorization gate and that it returns
// whatever SetTemplates installed (or an empty list when unset — -content not
// configured, so the UI falls back to manual entry).
func TestListMerchantTemplates(t *testing.T) {
	want := []npctemplates.Template{
		{TemplateName: "AkeMerchant", DisplayName: "Ake", Merchant: 1},
		{TemplateName: "HekalMerchant", DisplayName: "Hekal", Merchant: 19},
	}

	s := New(newFake())
	s.SetTemplates(want)

	got, tmpls, err := s.ListMerchantTemplates(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListMerchantTemplates: %v", err)
	}
	if got != OK {
		t.Fatalf("result = %v, want OK", got)
	}
	if !reflect.DeepEqual(tmpls, want) {
		t.Errorf("templates = %+v, want %+v", tmpls, want)
	}

	if got, _, err := s.ListMerchantTemplates(context.Background(), 3); err != nil || got != Forbidden {
		t.Errorf("player ListMerchantTemplates result = %v, err %v, want Forbidden, nil", got, err)
	}
}

// TestListMerchantTemplatesEmptyWhenUnset checks a service with no SetTemplates
// call (content dir not configured) returns an empty, non-error list.
func TestListMerchantTemplatesEmptyWhenUnset(t *testing.T) {
	s := New(newFake())
	got, tmpls, err := s.ListMerchantTemplates(context.Background(), 1)
	if err != nil || got != OK {
		t.Fatalf("result = %v, err %v, want OK, nil", got, err)
	}
	if len(tmpls) != 0 {
		t.Errorf("templates = %+v, want empty", tmpls)
	}
}

// TestListItemCatalog checks the authorization gate and that it returns
// whatever SetItems installed (or an empty list when unset).
func TestListItemCatalog(t *testing.T) {
	want := []itemcatalog.Entry{
		{Index: 1100, Name: "Espada Curta"},
		{Index: 1101, Name: "Adaga"},
	}

	s := New(newFake())
	s.SetItems(want)

	got, items, err := s.ListItemCatalog(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListItemCatalog: %v", err)
	}
	if got != OK {
		t.Fatalf("result = %v, want OK", got)
	}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("items = %+v, want %+v", items, want)
	}

	if got, _, err := s.ListItemCatalog(context.Background(), 3); err != nil || got != Forbidden {
		t.Errorf("player ListItemCatalog result = %v, err %v, want Forbidden, nil", got, err)
	}
}

func TestListItemCatalogEmptyWhenUnset(t *testing.T) {
	s := New(newFake())
	got, items, err := s.ListItemCatalog(context.Background(), 1)
	if err != nil || got != OK {
		t.Fatalf("result = %v, err %v, want OK, nil", got, err)
	}
	if len(items) != 0 {
		t.Errorf("items = %+v, want empty", items)
	}
}

func TestListDropItems(t *testing.T) {
	catalog := droptool.Catalog{
		Items: []droptool.ItemEntry{{Index: 1000, Name: "Adaga"}},
		Mobs:  []droptool.MobEntry{{TemplateName: "Alpha", DisplayName: "Alpha", Level: 37}},
		Drops: []droptool.DropEntry{{
			TemplateName: "Alpha",
			MobName:      "Alpha",
			MobLevel:     37,
			Slot:         0,
			ItemIndex:    1000,
			ItemName:     "Adaga",
			RateDivisor:  900,
		}},
	}

	s := New(newFake())
	s.SetDropCatalog(catalog)

	got, items, err := s.ListDropItems(context.Background(), 1, droptool.Filter{ItemQuery: "ada"})
	if err != nil {
		t.Fatalf("ListDropItems: %v", err)
	}
	if got != OK {
		t.Fatalf("result = %v, want OK", got)
	}
	if len(items) != 1 || items[0].ItemIndex != 1000 || len(items[0].Mobs) != 1 {
		t.Fatalf("items = %+v, want one item drop entry", items)
	}

	if got, _, err := s.ListDropItems(context.Background(), 3, droptool.Filter{}); err != nil || got != Forbidden {
		t.Errorf("player ListDropItems result = %v, err %v, want Forbidden, nil", got, err)
	}
}

func TestListMobDrops(t *testing.T) {
	catalog := droptool.Catalog{
		Items: []droptool.ItemEntry{{Index: 1000, Name: "Adaga"}},
		Mobs:  []droptool.MobEntry{{TemplateName: "Alpha", DisplayName: "Alpha", Level: 37}},
		Drops: []droptool.DropEntry{{
			TemplateName: "Alpha",
			MobName:      "Alpha",
			MobLevel:     37,
			Slot:         0,
			ItemIndex:    1000,
			ItemName:     "Adaga",
			RateDivisor:  900,
		}},
	}

	s := New(newFake())
	s.SetDropCatalog(catalog)

	got, mobs, err := s.ListMobDrops(context.Background(), 1, droptool.Filter{MobQuery: "alp"})
	if err != nil {
		t.Fatalf("ListMobDrops: %v", err)
	}
	if got != OK {
		t.Fatalf("result = %v, want OK", got)
	}
	if len(mobs) != 1 || mobs[0].TemplateName != "Alpha" || len(mobs[0].Items) != 1 {
		t.Fatalf("mobs = %+v, want one mob drop entry", mobs)
	}

	if got, _, err := s.ListMobDrops(context.Background(), 3, droptool.Filter{}); err != nil || got != Forbidden {
		t.Errorf("player ListMobDrops result = %v, err %v, want Forbidden, nil", got, err)
	}
}

func TestDropCatalogEmptyWhenUnset(t *testing.T) {
	s := New(newFake())
	got, items, err := s.ListDropItems(context.Background(), 1, droptool.Filter{})
	if err != nil || got != OK {
		t.Fatalf("ListDropItems result = %v, err %v, want OK, nil", got, err)
	}
	if len(items) != 0 {
		t.Errorf("items = %+v, want empty", items)
	}

	got, mobs, err := s.ListMobDrops(context.Background(), 1, droptool.Filter{})
	if err != nil || got != OK {
		t.Fatalf("ListMobDrops result = %v, err %v, want OK, nil", got, err)
	}
	if len(mobs) != 0 {
		t.Errorf("mobs = %+v, want empty", mobs)
	}
}

// TestListItemPrices checks the authorization gate and that it returns
// whatever the store's ItemPriceOverrides yields.
func TestListItemPrices(t *testing.T) {
	want := []domain.ItemPriceOverride{
		{ItemIndex: 1100, Price: 500},
		{ItemIndex: 1101, Price: 1200},
	}

	f := newFake()
	f.itemPrices = want
	s := New(f)

	got, prices, err := s.ListItemPrices(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListItemPrices: %v", err)
	}
	if got != OK {
		t.Fatalf("result = %v, want OK", got)
	}
	if !reflect.DeepEqual(prices, want) {
		t.Errorf("prices = %+v, want %+v", prices, want)
	}

	if got, _, err := s.ListItemPrices(context.Background(), 3); err != nil || got != Forbidden {
		t.Errorf("player ListItemPrices result = %v, err %v, want Forbidden, nil", got, err)
	}
}

// TestListMapZones checks the authorization gate and that it always returns
// mapzones.All (a static table, not dependent on -content).
func TestListMapZones(t *testing.T) {
	s := New(newFake())

	got, zones, err := s.ListMapZones(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListMapZones: %v", err)
	}
	if got != OK {
		t.Fatalf("result = %v, want OK", got)
	}
	if !reflect.DeepEqual(zones, mapzones.All) {
		t.Errorf("zones = %+v, want %+v", zones, mapzones.All)
	}

	if got, _, err := s.ListMapZones(context.Background(), 3); err != nil || got != Forbidden {
		t.Errorf("player ListMapZones result = %v, err %v, want Forbidden, nil", got, err)
	}
}
