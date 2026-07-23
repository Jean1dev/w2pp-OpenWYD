package mobtemplateadmin

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore is an in-memory Store for exercising the service's authorization
// and validation without a database.
type fakeStore struct {
	roles map[int64]string
	stats map[string]domain.MobTemplateStat
	equip map[string][]domain.MobTemplateEquipItem
}

func (f *fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	r, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return r, nil
}
func (f *fakeStore) ListMobTemplateStats(context.Context) ([]domain.MobTemplateStat, error) {
	out := make([]domain.MobTemplateStat, 0, len(f.stats))
	for _, st := range f.stats {
		out = append(out, st)
	}
	return out, nil
}
func (f *fakeStore) GetMobTemplateStat(_ context.Context, templateName string) (domain.MobTemplateStat, error) {
	st, ok := f.stats[templateName]
	if !ok {
		return domain.MobTemplateStat{}, store.ErrNotFound
	}
	return st, nil
}
func (f *fakeStore) UpsertMobTemplateStat(_ context.Context, st domain.MobTemplateStat, _ int64) error {
	if f.stats == nil {
		f.stats = map[string]domain.MobTemplateStat{}
	}
	f.stats[st.TemplateName] = st
	return nil
}
func (f *fakeStore) SetMobTemplateEquip(_ context.Context, templateName string, items []domain.MobTemplateEquipItem, _ int64) error {
	if _, ok := f.stats[templateName]; !ok {
		return store.ErrNotFound
	}
	if f.equip == nil {
		f.equip = map[string][]domain.MobTemplateEquipItem{}
	}
	f.equip[templateName] = items
	return nil
}
func (f *fakeStore) DeleteMobTemplateStat(_ context.Context, templateName string, _ int64) error {
	if _, ok := f.stats[templateName]; !ok {
		return store.ErrNotFound
	}
	delete(f.stats, templateName)
	return nil
}

func newFake() *fakeStore {
	return &fakeStore{
		roles: map[int64]string{1: "moderator", 2: "admin", 3: "player"},
		stats: map[string]domain.MobTemplateStat{
			"Karkarian": {TemplateName: "Karkarian", Level: 80},
		},
	}
}

// TestAuthorization checks every operation refuses non-moderators.
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
			got, _, err := s.ListTemplates(context.Background(), tt.moderator)
			if err != nil {
				t.Fatalf("ListTemplates: %v", err)
			}
			if got != tt.wantResult {
				t.Errorf("ListTemplates result = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestUpsertValidation(t *testing.T) {
	tests := []struct {
		name string
		st   domain.MobTemplateStat
		want Result
	}{
		{"ok", domain.MobTemplateStat{TemplateName: "Karkarian", Level: 90}, OK},
		{"empty template name", domain.MobTemplateStat{Level: 90}, Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(newFake())
			got, err := s.Upsert(context.Background(), 1, tt.st)
			if err != nil {
				t.Fatalf("Upsert: %v", err)
			}
			if got != tt.want {
				t.Errorf("Upsert result = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetEquipValidation(t *testing.T) {
	tests := []struct {
		name  string
		items []domain.MobTemplateEquipItem
		want  Result
	}{
		{"ok", []domain.MobTemplateEquipItem{{Slot: 0, ItemIndex: 1100}, {Slot: 15, ItemIndex: 1101}}, OK},
		{"slot too high", []domain.MobTemplateEquipItem{{Slot: 16, ItemIndex: 1100}}, Invalid},
		{"negative slot", []domain.MobTemplateEquipItem{{Slot: -1, ItemIndex: 1100}}, Invalid},
		{"zero item", []domain.MobTemplateEquipItem{{Slot: 0, ItemIndex: 0}}, Invalid},
		{"duplicate slot", []domain.MobTemplateEquipItem{{Slot: 0, ItemIndex: 1100}, {Slot: 0, ItemIndex: 1101}}, Invalid},
		{"empty ok", nil, OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(newFake())
			got, err := s.SetEquip(context.Background(), 1, "Karkarian", tt.items)
			if err != nil {
				t.Fatalf("SetEquip: %v", err)
			}
			if got != tt.want {
				t.Errorf("SetEquip result = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetEquipNotFound(t *testing.T) {
	s := New(newFake())
	got, err := s.SetEquip(context.Background(), 1, "NoSuchTemplate", []domain.MobTemplateEquipItem{{Slot: 0, ItemIndex: 1100}})
	if err != nil {
		t.Fatalf("SetEquip: %v", err)
	}
	if got != NotFound {
		t.Errorf("SetEquip result = %v, want NotFound", got)
	}
}

func TestDelete(t *testing.T) {
	fs := newFake()
	s := New(fs)
	got, err := s.Delete(context.Background(), 1, "Karkarian")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got != OK {
		t.Fatalf("Delete result = %v, want OK", got)
	}
	if _, ok := fs.stats["Karkarian"]; ok {
		t.Error("Karkarian still present after delete")
	}

	got, err = s.Delete(context.Background(), 1, "Karkarian")
	if err != nil {
		t.Fatalf("Delete (again): %v", err)
	}
	if got != NotFound {
		t.Errorf("Delete (again) result = %v, want NotFound", got)
	}
}

// TestGetReadThrough checks that when no override exists, Get falls back to
// TemplateReader and reports Overridden=false; with no reader configured it's
// NotFound instead.
func TestGetReadThrough(t *testing.T) {
	s := New(newFake())

	// Existing override: OK, overridden=true, no reader involved.
	res, st, overridden, err := s.Get(context.Background(), 1, "Karkarian")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res != OK || !overridden || st.Level != 80 {
		t.Fatalf("Get(Karkarian) = %v, %+v, overridden=%v, want OK/true/Level=80", res, st, overridden)
	}

	// No override, no reader configured: NotFound.
	res, _, _, err = s.Get(context.Background(), 1, "SomeMonster")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res != NotFound {
		t.Errorf("Get(SomeMonster) without reader = %v, want NotFound", res)
	}

	// No override, reader configured: OK, overridden=false, values from the
	// "file".
	raw := make([]byte, 816)
	copy(raw[0:16], "SomeMonster")
	raw[44] = 42 // BaseScore.Level (also mirrored at CurrentScore@92, but Get reads CurrentScore)
	raw[92] = 42 // CurrentScore.Level
	s.SetTemplateReader(func(name string) ([]byte, error) {
		if name != "SomeMonster" {
			return nil, store.ErrNotFound
		}
		return raw, nil
	})
	res, st, overridden, err = s.Get(context.Background(), 1, "SomeMonster")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res != OK || overridden || st.Level != 42 {
		t.Fatalf("Get(SomeMonster) with reader = %v, %+v, overridden=%v, want OK/false/Level=42", res, st, overridden)
	}
}

func TestGetEmptyTemplateNameInvalid(t *testing.T) {
	s := New(newFake())
	res, _, _, err := s.Get(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res != Invalid {
		t.Errorf("Get(\"\") = %v, want Invalid", res)
	}
}
