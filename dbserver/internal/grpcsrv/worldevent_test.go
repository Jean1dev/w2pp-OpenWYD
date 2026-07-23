package grpcsrv

import (
	"context"
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeWorldEventStore struct {
	version         int64
	cfg             domain.WorldEventConfig
	progressVersion int64
	progressIndex   int32
	progressApplied bool
}

func (f *fakeWorldEventStore) WorldEventConfigVersion(context.Context) (int64, error) {
	return f.version, nil
}
func (f *fakeWorldEventStore) WorldEventConfig(context.Context) (domain.WorldEventConfig, error) {
	return f.cfg, nil
}
func (f *fakeWorldEventStore) UpdateWorldEventProgress(_ context.Context, expectedVersion int64, currentIndex int32) (bool, error) {
	f.progressVersion, f.progressIndex = expectedVersion, currentIndex
	return f.progressApplied, nil
}

func TestWorldEventConfigServerMapsSnapshot(t *testing.T) {
	st := &fakeWorldEventStore{
		version: 12,
		cfg: domain.WorldEventConfig{
			Enabled: true, ItemIndex: 777, Rate: 5,
			StartIndex: 100, CurrentIndex: 101, EndIndex: 200,
			Indexed: true, NoticeEnabled: true, DoubleExpEnabled: true, NewbieEventEnabled: true,
		},
	}
	s := NewWorldEventConfig(st)

	version, err := s.WorldEventConfigVersion(context.Background(), &dbv1.WorldEventConfigVersionRequest{})
	if err != nil || version.GetVersion() != 12 {
		t.Fatalf("WorldEventConfigVersion = (%v,%v), want 12", version, err)
	}
	resp, err := s.GetWorldEventConfig(context.Background(), &dbv1.GetWorldEventConfigRequest{})
	if err != nil {
		t.Fatalf("GetWorldEventConfig: %v", err)
	}
	cfg := resp.GetConfig()
	if resp.GetVersion() != 12 || !cfg.GetEnabled() || cfg.GetItemIndex() != 777 ||
		cfg.GetCurrentIndex() != 101 || !cfg.GetDoubleExpEnabled() || !cfg.GetNewbieEventEnabled() {
		t.Errorf("snapshot = version %d cfg %+v, want mapped config", resp.GetVersion(), cfg)
	}
}

func TestWorldEventProgressMapsRequest(t *testing.T) {
	st := &fakeWorldEventStore{progressApplied: true}
	s := NewWorldEventConfig(st)

	resp, err := s.UpdateWorldEventProgress(context.Background(), &dbv1.UpdateWorldEventProgressRequest{
		ExpectedVersion: 3,
		CurrentIndex:    44,
	})
	if err != nil {
		t.Fatalf("UpdateWorldEventProgress: %v", err)
	}
	if !resp.GetApplied() || st.progressVersion != 3 || st.progressIndex != 44 {
		t.Errorf("progress applied=%v version=%d index=%d, want true/3/44", resp.GetApplied(), st.progressVersion, st.progressIndex)
	}
}
