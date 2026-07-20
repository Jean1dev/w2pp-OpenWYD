package dbclient

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

type fakeNpcConfigAPI struct {
	version int64
	list    *dbv1.ListNpcDefinitionsResponse
}

func (f *fakeNpcConfigAPI) NpcConfigVersion(context.Context, *dbv1.NpcConfigVersionRequest, ...grpc.CallOption) (*dbv1.NpcConfigVersionResponse, error) {
	return &dbv1.NpcConfigVersionResponse{Version: f.version}, nil
}

func (f *fakeNpcConfigAPI) ListNpcDefinitions(context.Context, *dbv1.ListNpcDefinitionsRequest, ...grpc.CallOption) (*dbv1.ListNpcDefinitionsResponse, error) {
	return f.list, nil
}

func (f *fakeNpcConfigAPI) ListMobTemplateStats(context.Context, *dbv1.ListMobTemplateStatsRequest, ...grpc.CallOption) (*dbv1.ListMobTemplateStatsResponse, error) {
	return &dbv1.ListMobTemplateStatsResponse{}, nil
}

func TestNpcConfigSnapshotMapsDisplayName(t *testing.T) {
	template := merchantTemplateBytes("Default")
	src := &NpcConfig{
		api: &fakeNpcConfigAPI{list: &dbv1.ListNpcDefinitionsResponse{
			Version: 7,
			Definitions: []*dbv1.NpcDefinition{{
				Slug: "shop-1", TemplateName: "Keeper", DisplayName: "Ferreiro",
				Enabled: true, PosX: 8, PosY: 9, RouteType: 2, Merchant: 1,
			}},
		}},
		template: func(name string) ([]byte, error) {
			if name != "Keeper" {
				t.Fatalf("template name = %q, want Keeper", name)
			}
			return template, nil
		},
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	snap, err := src.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Version != 7 || len(snap.Defs) != 1 {
		t.Fatalf("snapshot = version %d defs %d, want version 7 defs 1", snap.Version, len(snap.Defs))
	}
	def := snap.Defs[0]
	if def.DisplayName != "Ferreiro" {
		t.Fatalf("DisplayName = %q, want Ferreiro", def.DisplayName)
	}
	if def.Template == nil {
		t.Fatal("template not resolved")
	}
	if got := string(def.Template[:7]); got != "Default" {
		t.Fatalf("template name = %q, want Default", got)
	}
}

func merchantTemplateBytes(name string) []byte {
	tmpl := make([]byte, 816)
	copy(tmpl[:16], name)
	return tmpl
}
