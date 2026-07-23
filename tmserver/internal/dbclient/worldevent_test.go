package dbclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

type fakeWorldEventAPI struct {
	versionReq       bool
	snapshotResp     *dbv1.GetWorldEventConfigResponse
	progressVersion  int64
	progressIndex    int32
	progressResponse bool
}

func (f *fakeWorldEventAPI) WorldEventConfigVersion(context.Context, *dbv1.WorldEventConfigVersionRequest, ...grpc.CallOption) (*dbv1.WorldEventConfigVersionResponse, error) {
	f.versionReq = true
	return &dbv1.WorldEventConfigVersionResponse{Version: 5}, nil
}
func (f *fakeWorldEventAPI) GetWorldEventConfig(context.Context, *dbv1.GetWorldEventConfigRequest, ...grpc.CallOption) (*dbv1.GetWorldEventConfigResponse, error) {
	return f.snapshotResp, nil
}
func (f *fakeWorldEventAPI) UpdateWorldEventProgress(_ context.Context, req *dbv1.UpdateWorldEventProgressRequest, _ ...grpc.CallOption) (*dbv1.UpdateWorldEventProgressResponse, error) {
	f.progressVersion, f.progressIndex = req.GetExpectedVersion(), req.GetCurrentIndex()
	return &dbv1.UpdateWorldEventProgressResponse{Applied: f.progressResponse}, nil
}

func TestWorldEventConfigClientMapsSnapshotAndProgress(t *testing.T) {
	api := &fakeWorldEventAPI{
		progressResponse: true,
		snapshotResp: &dbv1.GetWorldEventConfigResponse{
			Version: 5,
			Config: &dbv1.WorldEventConfig{
				Enabled: true, ItemIndex: 777, Rate: 2,
				StartIndex: 10, CurrentIndex: 11, EndIndex: 20,
				Indexed: true, NoticeEnabled: true, DoubleExpEnabled: true, NewbieEventEnabled: true,
			},
		},
	}
	c := &WorldEventConfig{api: api}

	v, err := c.Version(context.Background())
	if err != nil || v != 5 || !api.versionReq {
		t.Fatalf("Version = (%d,%v), req=%v, want 5 nil true", v, err, api.versionReq)
	}
	snap, err := c.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Version != 5 || !snap.Event.Enabled || snap.Event.ItemIndex != 777 ||
		snap.Event.CurrentIndex != 11 || !snap.Event.DoubleExpEnabled || !snap.Event.NewbieEventEnabled {
		t.Errorf("snapshot = %+v, want mapped event", snap)
	}
	applied, err := c.UpdateProgress(context.Background(), 5, 12)
	if err != nil || !applied || api.progressVersion != 5 || api.progressIndex != 12 {
		t.Errorf("UpdateProgress = (%v,%v), version/index=%d/%d; want true nil 5/12",
			applied, err, api.progressVersion, api.progressIndex)
	}
}

func TestWorldEventConfigClientNilSnapshotDefaultsNotice(t *testing.T) {
	c := &WorldEventConfig{api: &fakeWorldEventAPI{snapshotResp: &dbv1.GetWorldEventConfigResponse{Version: 1}}}
	snap, err := c.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !snap.Event.NoticeEnabled {
		t.Errorf("NoticeEnabled = false, want true default for nil config")
	}
}
