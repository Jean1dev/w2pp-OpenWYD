package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
)

// fonteDeTaxas is a CombineRateSource that answers from memory.
type fonteDeTaxas struct {
	versao  int64
	cfg     combine.RateConfig
	verErr  error
	lerErr  error
	chamada int
}

func (f *fonteDeTaxas) Version(context.Context) (int64, error) {
	if f.verErr != nil {
		return 0, f.verErr
	}
	return f.versao, nil
}

func (f *fonteDeTaxas) Fetch(context.Context) (combine.RateConfig, error) {
	f.chamada++
	if f.lerErr != nil {
		return combine.RateConfig{}, f.lerErr
	}
	return f.cfg, nil
}

func taxasDeTeste(versao int64, taxa int32) combine.RateConfig {
	return combine.NewRateConfig(versao,
		[]combine.RateRow{{Family: "Ailyn", Key: "Refino", Rate: taxa}}, nil)
}

// TestMesaDasMaquinasChegaAoJogoNoBoot is the gap this closes: the screen, the
// table and the gRPC source all existed, but nothing ever assigned the config to
// the dispatcher — so every machine kept reading CompRate.txt no matter what a
// moderator saved, with no error anywhere to say so.
func TestMesaDasMaquinasChegaAoJogoNoBoot(t *testing.T) {
	fonte := &fonteDeTaxas{versao: 3, cfg: taxasDeTeste(3, 42)}
	d := &Dispatcher{combineRateSource: fonte, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	d.ApplyCombineRatesBoot()

	if got, ok := d.combineRates.Rate("Ailyn", "Refino"); !ok || got != 42 {
		t.Fatalf("taxa após o boot = %d (ok=%v), esperado 42", got, ok)
	}
	if d.combineRates.Version != 3 {
		t.Errorf("versão = %d, esperado 3", d.combineRates.Version)
	}
}

// TestBootSemBancoDeixaOArquivoNoLugar: without dbServer there is no source, and
// the zero config is exactly the behaviour the server had before this existed.
func TestBootSemBancoDeixaOArquivoNoLugar(t *testing.T) {
	d := &Dispatcher{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	d.ApplyCombineRatesBoot()
	if _, ok := d.combineRates.Rate("Ailyn", "Refino"); ok {
		t.Error("sem fonte, alguma taxa apareceu; o esperado é cair no CompRate.txt")
	}
}

// TestBootQueFalhaNaoDerrubaAMaquina: a dbServer that is slow or down at boot
// must leave the machines running on the file, not refuse to combine.
func TestBootQueFalhaNaoDerrubaAMaquina(t *testing.T) {
	fonte := &fonteDeTaxas{lerErr: errors.New("dbserver indisponível")}
	d := &Dispatcher{combineRateSource: fonte, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	d.ApplyCombineRatesBoot()
	if _, ok := d.combineRates.Rate("Ailyn", "Refino"); ok {
		t.Error("uma leitura falha deixou taxa aplicada")
	}
}

// TestContagemSeparaVazioDeNaoCarregado is what the log line rests on: an
// operator reading the boot line has no other way to tell a table that loaded
// empty from one that never loaded, and in game the two look the same.
func TestContagemSeparaVazioDeNaoCarregado(t *testing.T) {
	vazia := combine.RateConfig{}
	if vazia.RateCount() != 0 || vazia.BandCount() != 0 {
		t.Fatalf("config vazia contou %d taxas e %d faixas", vazia.RateCount(), vazia.BandCount())
	}
	cheia := combine.NewRateConfig(1,
		[]combine.RateRow{{Family: "Ailyn", Key: "Refino", Rate: 10}},
		[]combine.Band{
			{SlotKind: combine.SlotWeapon, ReqLvlMin: 0, ReqLvlMax: 99, Label: "Armas D", MultPct: 180},
			{SlotKind: combine.SlotArmour, ReqLvlMin: 0, ReqLvlMax: 99, Label: "Armaduras D", MultPct: 120},
		})
	if cheia.RateCount() != 1 {
		t.Errorf("RateCount = %d, esperado 1", cheia.RateCount())
	}
	if cheia.BandCount() != 2 {
		t.Errorf("BandCount = %d, esperado 2 (as duas espécies somadas)", cheia.BandCount())
	}
}
