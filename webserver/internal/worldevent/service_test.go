package worldevent

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

type fakeStore struct {
	roles   map[int64]string
	version int64
	cfg     domain.WorldEventConfig
	setCfg  domain.WorldEventConfig
	setBy   int64
	set     bool
}

func (f *fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	r, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return r, nil
}
func (f *fakeStore) WorldEventConfigVersion(context.Context) (int64, error) {
	return f.version, nil
}
func (f *fakeStore) WorldEventConfig(context.Context) (domain.WorldEventConfig, error) {
	return f.cfg, nil
}
func (f *fakeStore) UpsertWorldEventConfig(_ context.Context, cfg domain.WorldEventConfig, moderatorID int64) error {
	f.setCfg, f.setBy, f.set = cfg, moderatorID, true
	return nil
}

func newFake() *fakeStore {
	return &fakeStore{
		roles:   map[int64]string{1: "moderator", 2: "admin", 3: "player"},
		version: 7,
		cfg:     domain.WorldEventConfig{NoticeEnabled: true, DoubleExpEnabled: true},
	}
}

func TestAuthorization(t *testing.T) {
	tests := []struct {
		name      string
		moderator int64
		want      Result
	}{
		{"moderator", 1, OK},
		{"admin", 2, OK},
		{"player", 3, Forbidden},
		{"missing", 99, Forbidden},
		{"zero", 0, Forbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(newFake())
			if r, _, _, err := s.Get(context.Background(), tc.moderator); err != nil || r != tc.want {
				t.Errorf("Get = (%v,%v), want %v", r, err, tc.want)
			}
			if r, err := s.Set(context.Background(), tc.moderator, domain.WorldEventConfig{}); err != nil || r != tc.want {
				t.Errorf("Set = (%v,%v), want %v", r, err, tc.want)
			}
		})
	}
}

func TestGetReturnsVersionAndConfig(t *testing.T) {
	s := New(newFake())
	res, version, cfg, err := s.Get(context.Background(), 1)
	if err != nil || res != OK {
		t.Fatalf("Get = (%v,%v), want OK", res, err)
	}
	if version != 7 || !cfg.NoticeEnabled || !cfg.DoubleExpEnabled {
		t.Errorf("Get version/config = %d/%+v, want version 7 with flags", version, cfg)
	}
}

func TestSetValidation(t *testing.T) {
	valid := domain.WorldEventConfig{
		Enabled: true, ItemIndex: 777, Rate: 10,
		StartIndex: 100, CurrentIndex: 100, EndIndex: 200,
		Indexed: true, NoticeEnabled: true,
	}
	bad := []domain.WorldEventConfig{
		{Enabled: true, ItemIndex: 0, Rate: 10, StartIndex: 1, CurrentIndex: 1, EndIndex: 2},
		{Enabled: true, ItemIndex: maxWorldEventItemIndex + 1, Rate: 10, StartIndex: 1, CurrentIndex: 1, EndIndex: 2},
		{Enabled: true, ItemIndex: 1, Rate: 0, StartIndex: 1, CurrentIndex: 1, EndIndex: 2},
		{Enabled: true, ItemIndex: 1, Rate: 1, StartIndex: 0, CurrentIndex: 0, EndIndex: 2},
		{Enabled: true, ItemIndex: 1, Rate: 1, StartIndex: 10, CurrentIndex: 9, EndIndex: 20},
		{Enabled: true, ItemIndex: 1, Rate: 1, StartIndex: 10, CurrentIndex: 21, EndIndex: 20},
	}

	st := newFake()
	s := New(st)
	if r, err := s.Set(context.Background(), 1, valid); err != nil || r != OK {
		t.Fatalf("Set(valid) = (%v,%v), want OK", r, err)
	}
	if !st.set || st.setBy != 1 || st.setCfg.ItemIndex != 777 {
		t.Fatalf("store set = %v by %d cfg %+v, want item 777 by moderator 1", st.set, st.setBy, st.setCfg)
	}
	for _, cfg := range bad {
		st.set = false
		if r, err := s.Set(context.Background(), 1, cfg); err != nil || r != Invalid {
			t.Errorf("Set(%+v) = (%v,%v), want Invalid", cfg, r, err)
		}
		if st.set {
			t.Errorf("Set(%+v) wrote to store despite Invalid", cfg)
		}
	}
}

func TestSetAllowsExpOnlyConfig(t *testing.T) {
	st := newFake()
	s := New(st)
	cfg := domain.WorldEventConfig{DoubleExpEnabled: true, NewbieEventEnabled: true}
	if r, err := s.Set(context.Background(), 1, cfg); err != nil || r != OK {
		t.Fatalf("Set(exp-only) = (%v,%v), want OK", r, err)
	}
	if !st.setCfg.DoubleExpEnabled || !st.setCfg.NewbieEventEnabled {
		t.Errorf("set cfg = %+v, want exp flags preserved", st.setCfg)
	}
}
