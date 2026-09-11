package grpcsrv

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeGenOffStore struct {
	cfg  domain.GeneratorOffConfig
	sets []domain.GeneratorOff
}

func (f *fakeGenOffStore) GeneratorOffVersion(context.Context) (int64, error) {
	return f.cfg.Version, nil
}
func (f *fakeGenOffStore) GeneratorsOff(context.Context) (domain.GeneratorOffConfig, error) {
	return f.cfg, nil
}
func (f *fakeGenOffStore) SetGeneratorOff(_ context.Context, index int32, off bool, by string) error {
	if off {
		by += ":off"
	}
	f.sets = append(f.sets, domain.GeneratorOff{Index: index, By: by})
	return nil
}

func TestNpcGeneratorServerLeEGrava(t *testing.T) {
	st := &fakeGenOffStore{cfg: domain.GeneratorOffConfig{Version: 7, Off: []domain.GeneratorOff{{Index: 6077, By: "gm"}}}}
	s := NewNpcGenerator(st)

	resp, err := s.GetGeneratorsOff(context.Background(), &dbv1.GetGeneratorsOffRequest{})
	if err != nil {
		t.Fatalf("GetGeneratorsOff: %v", err)
	}
	if resp.GetVersion() != 7 || len(resp.GetOff()) != 1 || resp.GetOff()[0].GetIndex() != 6077 || resp.GetOff()[0].GetBy() != "gm" {
		t.Fatalf("resp = %+v", resp)
	}

	if _, err := s.SetGeneratorOff(context.Background(), &dbv1.SetGeneratorOffRequest{Index: 23, Off: true, By: "mod"}); err != nil {
		t.Fatalf("SetGeneratorOff: %v", err)
	}
	if len(st.sets) != 1 || st.sets[0].Index != 23 || st.sets[0].By != "mod:off" {
		t.Errorf("store recebeu %+v", st.sets)
	}

	_, err = s.SetGeneratorOff(context.Background(), &dbv1.SetGeneratorOffRequest{Index: -1, Off: true})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("índice negativo: %v, want InvalidArgument", err)
	}
}
