package dbclient

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/npccfg"
)

// TemplateLoader resolves an NPC template name to its raw 816-byte STRUCT_MOB.
// The dbServer stores only the template NAME (not the bytes), so the tmServer
// resolves it against its local content tree at snapshot time.
type TemplateLoader func(name string) ([]byte, error)

// NpcConfig adapts the dbServer NpcConfigService to the npccfg.Source port. It
// fetches the moderator-edited definitions over gRPC and resolves each template
// name to bytes so the loop can materialize the NPC directly.
type NpcConfig struct {
	api      dbv1.NpcConfigServiceClient
	template TemplateLoader
	log      *slog.Logger
}

// NewNpcConfig wraps a gRPC connection + template loader as an npccfg.Source. log
// receives a warning whenever a template fails to resolve (nil defaults to the
// slog default), so a missing/corrupt NPC file is never silently dropped.
func NewNpcConfig(conn grpc.ClientConnInterface, loader TemplateLoader, log *slog.Logger) *NpcConfig {
	if log == nil {
		log = slog.Default()
	}
	return &NpcConfig{api: dbv1.NewNpcConfigServiceClient(conn), template: loader, log: log}
}

var _ npccfg.Source = (*NpcConfig)(nil)

// Version returns the current config version (cheap poll).
func (c *NpcConfig) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.NpcConfigVersion(ctx, &dbv1.NpcConfigVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: npc config version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Snapshot fetches the full definition set + price overrides and resolves each
// definition's template bytes. A template that fails to load leaves Template nil
// (the applier skips it) rather than failing the whole reload.
func (c *NpcConfig) Snapshot(ctx context.Context) (npccfg.Snapshot, error) {
	resp, err := c.api.ListNpcDefinitions(ctx, &dbv1.ListNpcDefinitionsRequest{})
	if err != nil {
		return npccfg.Snapshot{}, fmt.Errorf("dbclient: list npc definitions: %w", err)
	}
	snap := npccfg.Snapshot{Version: resp.GetVersion()}
	cache := make(map[string][]byte)
	for _, d := range resp.GetDefinitions() {
		tmpl, ok := cache[d.GetTemplateName()]
		if !ok {
			b, lerr := c.template(d.GetTemplateName())
			if lerr != nil {
				// Don't fail the whole reload for one bad template; leave it nil (the
				// applier skips + warns) but surface why here.
				c.log.Warn("npc template resolve failed", "template", d.GetTemplateName(), "slug", d.GetSlug(), "err", lerr)
			} else {
				tmpl = b
			}
			cache[d.GetTemplateName()] = tmpl
		}
		def := npccfg.Definition{
			Slug:      d.GetSlug(),
			Template:  tmpl,
			Enabled:   d.GetEnabled(),
			X:         int16(d.GetPosX()),
			Y:         int16(d.GetPosY()),
			RouteType: uint8(d.GetRouteType()),
			Merchant:  uint8(d.GetMerchant()),
		}
		for _, it := range d.GetShop() {
			def.Shop = append(def.Shop, npccfg.ShopItem{
				Slot:  int(it.GetSlot()),
				Index: uint16(it.GetItemIndex()),
				Eff: [3][2]uint8{
					{uint8(it.GetEff1()), uint8(it.GetEffv1())},
					{uint8(it.GetEff2()), uint8(it.GetEffv2())},
					{uint8(it.GetEff3()), uint8(it.GetEffv3())},
				},
			})
		}
		snap.Defs = append(snap.Defs, def)
	}
	if prices := resp.GetPriceOverrides(); len(prices) > 0 {
		snap.PriceOverrides = make(map[int]int32, len(prices))
		for _, p := range prices {
			snap.PriceOverrides[int(p.GetItemIndex())] = int32(p.GetPrice())
		}
	}
	return snap, nil
}
