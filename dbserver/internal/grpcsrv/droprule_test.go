package grpcsrv

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
)

type fakeDropRuleStore struct {
	version int64
	cfg     droprule.Config
	err     error
}

func (f *fakeDropRuleStore) DropRuleVersion(context.Context) (int64, error) { return f.version, f.err }
func (f *fakeDropRuleStore) DropRules(context.Context) (droprule.Config, error) {
	return f.cfg, f.err
}

func TestDropRuleServerCarriesEveryRule(t *testing.T) {
	regras := []droprule.Rule{
		{Mob: droprule.AllMobs, Item: 2405, Chance: 0},
		{Mob: "Dark_Shadow_", Item: 2316, Chance: 800},
	}
	s := NewDropRule(&fakeDropRuleStore{version: 7, cfg: droprule.Config{Version: 7, Rules: regras}})
	resp, err := s.ListDropRules(context.Background(), &dbv1.ListDropRulesRequest{})
	if err != nil {
		t.Fatalf("ListDropRules: %v", err)
	}
	if resp.GetVersion() != 7 || len(resp.GetRules()) != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	for i, r := range resp.GetRules() {
		got := droprule.Rule{Mob: r.GetMob(), Item: int16(r.GetItem()), Chance: r.GetChance()}
		if got != regras[i] {
			t.Errorf("regra %d chegou como %+v, want %+v", i, got, regras[i])
		}
	}
	v, err := s.DropRuleVersion(context.Background(), &dbv1.DropRuleVersionRequest{})
	if err != nil || v.GetVersion() != 7 {
		t.Errorf("DropRuleVersion = %v, %v", v, err)
	}
}

func TestDropRuleServerStoreError(t *testing.T) {
	s := NewDropRule(&fakeDropRuleStore{err: errors.New("banco caiu")})
	if _, err := s.ListDropRules(context.Background(), &dbv1.ListDropRulesRequest{}); status.Code(err) != codes.Internal {
		t.Errorf("ListDropRules err = %v, want Internal", err)
	}
	if _, err := s.DropRuleVersion(context.Background(), &dbv1.DropRuleVersionRequest{}); status.Code(err) != codes.Internal {
		t.Errorf("DropRuleVersion err = %v, want Internal", err)
	}
}
