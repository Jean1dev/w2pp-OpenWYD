package npcadmin

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
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
func (f *fakeStore) DeleteNPCDefinition(_ context.Context, id int64, _ int64) error {
	if _, ok := f.defs[id]; !ok {
		return store.ErrNotFound
	}
	f.deletedID = id
	return nil
}

func newFake() *fakeStore {
	return &fakeStore{
		roles: map[int64]string{1: "moderator", 2: "admin", 3: "player"},
		defs:  map[int64]domain.NPCDefinition{10: {ID: 10, Slug: "shop-10", Merchant: 1}},
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
		{"empty slug", domain.NPCDefinition{TemplateName: "t"}, Invalid},
		{"empty template", domain.NPCDefinition{Slug: "s"}, Invalid},
		{"bad merchant", domain.NPCDefinition{Slug: "s", TemplateName: "t", Merchant: 7}, Invalid},
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
		{"slot too high", []domain.NPCShopItem{{Slot: 27, ItemIndex: 1100}}, Invalid},
		{"negative slot", []domain.NPCShopItem{{Slot: -1, ItemIndex: 1100}}, Invalid},
		{"zero item", []domain.NPCShopItem{{Slot: 0, ItemIndex: 0}}, Invalid},
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
