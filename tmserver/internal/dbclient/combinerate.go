package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
)

// CombineRateSource fetches the Mesa das Máquinas — the panel-managed combine
// rates — from dbServer's CombineRateService.
//
// Polled rather than read once at boot, for the same reason the Mesa de XP is
// (xpconfig.go): these are the dials an operator turns repeatedly while watching
// whether the economy came out right, and restart-to-apply would charge a
// disconnect for everyone online per turn. The unfairness it avoids — two
// players combining at different rates seconds apart — is smaller than the one
// it would create.
type CombineRateSource struct {
	api dbv1.CombineRateServiceClient
}

// NewCombineRateSource wraps a gRPC connection as a CombineRateSource.
func NewCombineRateSource(conn grpc.ClientConnInterface) *CombineRateSource {
	return &CombineRateSource{api: dbv1.NewCombineRateServiceClient(conn)}
}

// Version returns the monotonic version, which is what the poll compares so a
// reload only costs a full read when something actually moved.
func (c *CombineRateSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.CombineRateVersion(ctx, &dbv1.CombineRateVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: combine rate version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Fetch returns the configuration ready for the combine handlers. A reply with
// no rows yields a zero RateConfig, which is the pure CompRate.txt behaviour.
func (c *CombineRateSource) Fetch(ctx context.Context) (combine.RateConfig, error) {
	resp, err := c.api.GetCombineRates(ctx, &dbv1.GetCombineRatesRequest{})
	if err != nil {
		return combine.RateConfig{}, fmt.Errorf("dbclient: get combine rates: %w", err)
	}
	rates := make([]combine.RateRow, 0, len(resp.GetRates()))
	for _, r := range resp.GetRates() {
		rates = append(rates, combine.RateRow{Family: r.GetFamily(), Key: r.GetKey(), Rate: r.GetRate()})
	}
	bands := make([]combine.Band, 0, len(resp.GetBands()))
	for _, b := range resp.GetBands() {
		bands = append(bands, combine.Band{
			SlotKind:  combine.SlotKind(b.GetSlotKind()),
			ReqLvlMin: b.GetReqLvlMin(),
			ReqLvlMax: b.GetReqLvlMax(),
			Label:     b.GetLabel(),
			MultPct:   b.GetMultPct(),
		})
	}
	return combine.NewRateConfig(resp.GetVersion(), rates, bands), nil
}
