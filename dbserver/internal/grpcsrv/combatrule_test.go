package grpcsrv

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

type fakeCombatRuleStore struct {
	version int64
	cfg     combatrule.Config
	err     error
}

func (f *fakeCombatRuleStore) CombatRuleVersion(context.Context) (int64, error) {
	return f.version, f.err
}

func (f *fakeCombatRuleStore) CombatRule(context.Context) (combatrule.Config, error) {
	return f.cfg, f.err
}

// TestCombatRuleServerCarriesEveryKnob: every knob crosses the wire, and
// configured with them — tmServer needs the flag to tell a saved rule from the
// default.
func TestCombatRuleServerCarriesEveryKnob(t *testing.T) {
	regra := combatrule.Kersef()
	// Os dois de PvP diferentes entre si e do legado, para um campo trocado com
	// o outro no mapeamento não passar.
	regra.PvPSkillPct, regra.PvPMeleePct = 60, 80
	s := NewCombatRule(&fakeCombatRuleStore{cfg: combatrule.Config{
		Version: 4, Configured: true, Rules: regra,
	}})
	resp, err := s.GetCombatRule(context.Background(), &dbv1.GetCombatRuleRequest{})
	if err != nil {
		t.Fatalf("GetCombatRule: %v", err)
	}
	if resp.GetVersion() != 4 || !resp.GetConfigured() {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.GetWeaponIntMagicPct() != 100 || !resp.GetSpellDamageMulti() || resp.GetMobResistBase() != 150 {
		t.Errorf("os botões de magia chegaram como %+v, quero o Kersef", resp)
	}
	if resp.GetPvpSkillPct() != 60 || resp.GetPvpMeleePct() != 80 {
		t.Errorf("PvP chegou como skill %d%% e golpe %d%%, quero 60%% e 80%%",
			resp.GetPvpSkillPct(), resp.GetPvpMeleePct())
	}
}

// TestCombatRuleServerUnconfigured: a fresh server says so plainly.
func TestCombatRuleServerUnconfigured(t *testing.T) {
	s := NewCombatRule(&fakeCombatRuleStore{cfg: combatrule.Unconfigured(0)})
	resp, err := s.GetCombatRule(context.Background(), &dbv1.GetCombatRuleRequest{})
	if err != nil {
		t.Fatalf("GetCombatRule: %v", err)
	}
	if resp.GetConfigured() || resp.GetVersion() != 0 {
		t.Fatalf("resp = %+v, want not configured at version 0", resp)
	}
}

func TestCombatRuleVersionIsServed(t *testing.T) {
	s := NewCombatRule(&fakeCombatRuleStore{version: 31})
	got, err := s.CombatRuleVersion(context.Background(), &dbv1.CombatRuleVersionRequest{})
	if err != nil || got.GetVersion() != 31 {
		t.Fatalf("CombatRuleVersion = (%v, %v), want 31", got, err)
	}
}

func TestCombatRuleStoreFailureIsInternal(t *testing.T) {
	s := NewCombatRule(&fakeCombatRuleStore{err: errors.New("sem banco")})
	if _, err := s.GetCombatRule(context.Background(), &dbv1.GetCombatRuleRequest{}); status.Code(err) != codes.Internal {
		t.Errorf("GetCombatRule err = %v, want Internal", err)
	}
	if _, err := s.CombatRuleVersion(context.Background(), &dbv1.CombatRuleVersionRequest{}); status.Code(err) != codes.Internal {
		t.Errorf("CombatRuleVersion err = %v, want Internal", err)
	}
}
