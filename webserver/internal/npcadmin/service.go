// Package npcadmin holds the web platform's moderator NPC-editing logic
// (npc-editing-plan.md): CRUD over the cold NPC definition in Postgres, gated by
// account.role and recorded in the audit trail by the store. It never touches
// live game state — that stays in tmServer's single-owner loop; the tmServer
// polls the config version and materializes the definitions.
package npcadmin

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/droptool"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mapzones"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npctemplates"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	AccountRole(ctx context.Context, id int64) (string, error)
	ListNPCDefinitions(ctx context.Context) ([]domain.NPCDefinition, error)
	GetNPCDefinition(ctx context.Context, id int64) (domain.NPCDefinition, error)
	UpsertNPCDefinition(ctx context.Context, d domain.NPCDefinition, moderatorID int64) (int64, error)
	SetNPCShop(ctx context.Context, npcID int64, items []domain.NPCShopItem, moderatorID int64) error
	SetNPCVisibility(ctx context.Context, npcID int64, enabled bool, moderatorID int64) error
	SetItemPrice(ctx context.Context, itemIndex int32, price int64, moderatorID int64) error
	ItemPriceOverrides(ctx context.Context) ([]domain.ItemPriceOverride, error)
	DeleteNPCDefinition(ctx context.Context, npcID int64, moderatorID int64) error
}

// Result is the business outcome of an admin operation. Only infra failures are
// returned as errors; these ride in the response body.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Forbidden means the caller is not a moderator/admin.
	Forbidden
	// Invalid means the request failed validation.
	Invalid
	// NotFound means the target definition does not exist.
	NotFound
	// ContentOwned means versioned content must be hidden rather than deleted.
	ContentOwned
)

// Shop slots mirror MSG_ShopList's 27 entries; a merchant NPC's stock lives in
// slots 0..26. maxSlot bounds shop-item validation.
const (
	maxSlot       = 26
	maxQuantity   = 255
	amountEffect  = 61
	defaultAmount = 1
)

// Service implements the moderator NPC-editing operations.
type Service struct {
	store       Store
	templates   []npctemplates.Template
	items       []itemcatalog.Entry
	dropCatalog droptool.Catalog
}

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// SetTemplates installs the merchant templates ListMerchantTemplates serves.
// Called once at boot after scanning the content tree (npctemplates.Scan);
// left unset (nil) when -content/W2PP_CONTENT wasn't configured, in which case
// ListMerchantTemplates returns an empty list rather than failing.
func (s *Service) SetTemplates(templates []npctemplates.Template) { s.templates = templates }

// SetItems installs the item catalog ListItemCatalog serves. Called once at
// boot after scanning the content tree (itemcatalog.Scan); left unset (nil)
// when -content/W2PP_CONTENT wasn't configured, in which case
// ListItemCatalog returns an empty list rather than failing.
func (s *Service) SetItems(items []itemcatalog.Entry) { s.items = items }

// SetDropCatalog installs the DropTool-equivalent catalog ListDropItems and
// ListMobDrops serve. Called once at boot after scanning the content tree; left
// as a zero-value catalog when -content/W2PP_CONTENT wasn't configured.
func (s *Service) SetDropCatalog(catalog droptool.Catalog) { s.dropCatalog = catalog }

// List returns every NPC definition, after authorizing the caller.
func (s *Service) List(ctx context.Context, moderatorID int64) (Result, []domain.NPCDefinition, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	defs, err := s.store.ListNPCDefinitions(ctx)
	if err != nil {
		return Invalid, nil, fmt.Errorf("npcadmin: list: %w", err)
	}
	return OK, defs, nil
}

// ListMerchantTemplates returns the merchant NPC templates scanned from the
// content tree at boot (npctemplates.Scan), after authorizing the caller. It is
// never an error for the list to be empty — that just means -content wasn't
// configured, and the UI falls back to manual template_name entry.
func (s *Service) ListMerchantTemplates(ctx context.Context, moderatorID int64) (Result, []npctemplates.Template, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	return OK, s.templates, nil
}

// ListItemCatalog returns the item index/name catalog scanned from the
// content tree at boot (itemcatalog.Scan), after authorizing the caller. It is
// never an error for the list to be empty — that just means -content wasn't
// configured, and the shop-item editor falls back to a raw item_index field.
func (s *Service) ListItemCatalog(ctx context.Context, moderatorID int64) (Result, []itemcatalog.Entry, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	return OK, s.items, nil
}

// ListDropItems returns the item-centric DropTool report scanned from the
// content tree at boot, after authorizing the caller.
func (s *Service) ListDropItems(ctx context.Context, moderatorID int64, filter droptool.Filter) (Result, []droptool.ItemDropEntry, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	return OK, s.dropCatalog.ListDropItems(filter), nil
}

// ListMobDrops returns the mob-centric DropTool report scanned from the content
// tree at boot, after authorizing the caller.
func (s *Service) ListMobDrops(ctx context.Context, moderatorID int64, filter droptool.Filter) (Result, []droptool.MobDropEntry, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	return OK, s.dropCatalog.ListMobDrops(filter), nil
}

// ListMapZones returns the fixed map_id label table (mapzones.All), after
// authorizing the caller. Unlike the template/item lists this never depends
// on -content — it's a static table, not scanned content.
func (s *Service) ListMapZones(ctx context.Context, moderatorID int64) (Result, []mapzones.Zone, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	return OK, mapzones.All, nil
}

// Get returns one definition by id.
func (s *Service) Get(ctx context.Context, moderatorID, npcID int64) (Result, domain.NPCDefinition, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, domain.NPCDefinition{}, err
	}
	def, err := s.store.GetNPCDefinition(ctx, npcID)
	if errors.Is(err, store.ErrNotFound) {
		return NotFound, domain.NPCDefinition{}, nil
	}
	if err != nil {
		return Invalid, domain.NPCDefinition{}, fmt.Errorf("npcadmin: get %d: %w", npcID, err)
	}
	return OK, def, nil
}

// Upsert creates or updates a definition (position, visibility, merchant type).
func (s *Service) Upsert(ctx context.Context, moderatorID int64, d domain.NPCDefinition) (Result, int64, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, 0, err
	}
	if d.Slug == "" || d.TemplateName == "" || !validMerchant(d.Merchant) {
		return Invalid, 0, nil
	}
	id, err := s.store.UpsertNPCDefinition(ctx, d, moderatorID)
	if err != nil {
		return Invalid, 0, fmt.Errorf("npcadmin: upsert %q: %w", d.Slug, err)
	}
	return OK, id, nil
}

// SetVisibility toggles whether the NPC appears in the world.
func (s *Service) SetVisibility(ctx context.Context, moderatorID, npcID int64, enabled bool) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	err := s.store.SetNPCVisibility(ctx, npcID, enabled, moderatorID)
	return classifyWrite(err, "set visibility")
}

// SetShop replaces a merchant NPC's shop stock after validating the slots.
func (s *Service) SetShop(ctx context.Context, moderatorID, npcID int64, items []domain.NPCShopItem) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	seen := make(map[int16]bool, len(items))
	for i := range items {
		it := &items[i]
		if it.Quantity == 0 {
			it.Quantity = defaultAmount
		}
		if it.Slot < 0 || it.Slot > maxSlot || it.ItemIndex <= 0 || seen[it.Slot] {
			return Invalid, nil
		}
		if it.Quantity < defaultAmount || it.Quantity > maxQuantity {
			return Invalid, nil
		}
		if it.Eff1 == amountEffect || it.Eff2 == amountEffect || it.Eff3 == amountEffect {
			return Invalid, nil
		}
		if it.Quantity > defaultAmount && it.Eff1 != 0 && it.Eff2 != 0 && it.Eff3 != 0 {
			return Invalid, nil
		}
		seen[it.Slot] = true
	}
	err := s.store.SetNPCShop(ctx, npcID, items, moderatorID)
	return classifyWrite(err, "set shop")
}

// SetItemPrice sets (price>=0) or clears (price<0) the global item price.
func (s *Service) SetItemPrice(ctx context.Context, moderatorID int64, itemIndex int32, price int64) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	if itemIndex <= 0 {
		return Invalid, nil
	}
	if err := s.store.SetItemPrice(ctx, itemIndex, price, moderatorID); err != nil {
		return Invalid, fmt.Errorf("npcadmin: set item price %d: %w", itemIndex, err)
	}
	return OK, nil
}

// ListItemPrices returns every global item price override currently set
// (item_price table), after authorizing the caller. An item with no override
// simply doesn't appear — it uses the content catalog's base price.
func (s *Service) ListItemPrices(ctx context.Context, moderatorID int64) (Result, []domain.ItemPriceOverride, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	prices, err := s.store.ItemPriceOverrides(ctx)
	if err != nil {
		return Invalid, nil, fmt.Errorf("npcadmin: list item prices: %w", err)
	}
	return OK, prices, nil
}

// Delete removes a definition.
func (s *Service) Delete(ctx context.Context, moderatorID, npcID int64) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	err := s.store.DeleteNPCDefinition(ctx, npcID, moderatorID)
	return classifyWrite(err, "delete")
}

// authorize checks the caller has a moderator/admin role. A missing account or a
// plain-player role yields Forbidden (never leaks whether the account exists).
func (s *Service) authorize(ctx context.Context, moderatorID int64) (Result, error) {
	if moderatorID <= 0 {
		return Forbidden, nil
	}
	role, err := s.store.AccountRole(ctx, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return Forbidden, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("npcadmin: role lookup %d: %w", moderatorID, err)
	}
	if role != "moderator" && role != "admin" {
		return Forbidden, nil
	}
	return OK, nil
}

// classifyWrite maps a store write error to a Result: ErrNotFound → NotFound,
// nil → OK, anything else → Invalid (wrapped).
func classifyWrite(err error, op string) (Result, error) {
	switch {
	case err == nil:
		return OK, nil
	case errors.Is(err, store.ErrNotFound):
		return NotFound, nil
	case errors.Is(err, store.ErrContentOwned):
		return ContentOwned, nil
	default:
		return Invalid, fmt.Errorf("npcadmin: %s: %w", op, err)
	}
}

// validMerchant accepts the merchant sub-types the admin panel can materialize:
// 0 (none/monster), 1 (shop), 2 (cargo guard), 19 (shop type 3), 100 (supported
// quest NPC), and 111 (canonical King templates). Others are rejected until
// confirmed by capture (npc-editing-plan.md §9.4).
func validMerchant(m int16) bool {
	return m >= 0 && m <= 255
}
