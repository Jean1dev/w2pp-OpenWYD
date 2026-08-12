// Package mobtemplateadmin holds the web platform's moderator mob/NPC
// template STAT-editing logic (mob-template-editing-plan.md) — the
// equivalent-tool successor to the legacy Win32 EDITAPPMOB. It is the sibling
// of webserver/internal/npcadmin: that package edits spawn position/
// visibility/shop for the DB-managed merchant subset; this one edits the
// combat/attribute stats of ANY npc/<template_name> STRUCT_MOB file, gated by
// account.role and recorded in the audit trail by the store. It never touches
// live game state — the tmServer only reads these overrides, once at boot.
package mobtemplateadmin

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/mobtemplates"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	AccountRole(ctx context.Context, id int64) (string, error)
	ListMobTemplateStats(ctx context.Context) ([]domain.MobTemplateStat, error)
	GetMobTemplateStat(ctx context.Context, templateName string) (domain.MobTemplateStat, error)
	UpsertMobTemplateStat(ctx context.Context, st domain.MobTemplateStat, moderatorID int64) error
	SetMobTemplateEquip(ctx context.Context, templateName string, items []domain.MobTemplateEquipItem, moderatorID int64) error
	DeleteMobTemplateStat(ctx context.Context, templateName string, moderatorID int64) error
}

// TemplateReader resolves a template_name to its raw STRUCT_MOB bytes, read
// verbatim from the content tree — any layout savefmt knows, including the
// legacy 756/920-byte ones (data-formats.md §1.4.1). Used by Get's read-through
// fallback when no override exists yet, so opening the editor shows the file's
// real current values instead of zeros. nil when -content/W2PP_CONTENT wasn't
// configured.
type TemplateReader func(templateName string) ([]byte, error)

// Result is the business outcome of an admin operation. Only infra failures
// are returned as errors; these ride in the response body.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Forbidden means the caller is not a moderator/admin.
	Forbidden
	// Invalid means the request failed validation.
	Invalid
	// NotFound means the target template has no override AND (for Get) could
	// not be read through from the content tree either.
	NotFound
)

// maxEquipSlot bounds slot validation (STRUCT_MOB.Equip[savefmt.MaxEquip], 0-indexed).
const maxEquipSlot = savefmt.MaxEquip - 1

// Service implements the moderator mob-template-stat-editing operations.
type Service struct {
	store        Store
	templates    []mobtemplates.File
	readTemplate TemplateReader
}

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// SetTemplates installs the mob template list ListTemplates serves. Called
// once at boot after scanning the content tree (mobtemplates.Scan); left
// unset (nil) when -content/W2PP_CONTENT wasn't configured, in which case
// ListTemplates returns an empty list rather than failing.
func (s *Service) SetTemplates(templates []mobtemplates.File) { s.templates = templates }

// SetTemplateReader installs the read-through template reader Get uses when
// no override exists yet. Left unset (nil) when -content/W2PP_CONTENT wasn't
// configured, in which case Get returns NotFound for un-overridden templates.
func (s *Service) SetTemplateReader(r TemplateReader) { s.readTemplate = r }

// ListTemplates returns every mob template file scanned from the content tree
// at boot (mobtemplates.Scan), after authorizing the caller. Unlike
// npcadmin's ListMerchantTemplates this is NOT filtered to merchants.
func (s *Service) ListTemplates(ctx context.Context, moderatorID int64) (Result, []mobtemplates.File, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, nil, err
	}
	return OK, s.templates, nil
}

// Get returns the stat override for a template if one exists; otherwise it
// reads the raw template file's current values through TemplateReader
// (Overridden=false on the result) — mirroring EDITAPPMOB's own
// open-and-see-current-values behavior without needing a bulk seed import.
func (s *Service) Get(ctx context.Context, moderatorID int64, templateName string) (Result, domain.MobTemplateStat, bool, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, domain.MobTemplateStat{}, false, err
	}
	if templateName == "" {
		return Invalid, domain.MobTemplateStat{}, false, nil
	}
	st, err := s.store.GetMobTemplateStat(ctx, templateName)
	if err == nil {
		return OK, st, true, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return Invalid, domain.MobTemplateStat{}, false, fmt.Errorf("mobtemplateadmin: get %q: %w", templateName, err)
	}
	if s.readTemplate == nil {
		return NotFound, domain.MobTemplateStat{}, false, nil
	}
	raw, rerr := s.readTemplate(templateName)
	if rerr != nil {
		return NotFound, domain.MobTemplateStat{}, false, nil
	}
	st, derr := statFromRawTemplate(templateName, raw)
	if derr != nil {
		return NotFound, domain.MobTemplateStat{}, false, nil
	}
	return OK, st, false, nil
}

// Upsert creates or replaces the full stat override for a template_name,
// including st.Equip (the store now persists it in the same write — see
// store.UpsertMobTemplateStat), so it needs the same slot validation SetEquip
// applies; otherwise this path could write equip data SetEquip would reject.
func (s *Service) Upsert(ctx context.Context, moderatorID int64, st domain.MobTemplateStat) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	if st.TemplateName == "" || !validEquip(st.Equip) {
		return Invalid, nil
	}
	if err := s.store.UpsertMobTemplateStat(ctx, st, moderatorID); err != nil {
		return Invalid, fmt.Errorf("mobtemplateadmin: upsert %q: %w", st.TemplateName, err)
	}
	return OK, nil
}

// SetEquip replaces a template's Equip[] slot overrides after validating the
// slots. Requires a stat override to already exist for template_name.
func (s *Service) SetEquip(ctx context.Context, moderatorID int64, templateName string, items []domain.MobTemplateEquipItem) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	if !validEquip(items) {
		return Invalid, nil
	}
	err := s.store.SetMobTemplateEquip(ctx, templateName, items, moderatorID)
	return classifyWrite(err, "set equip")
}

// validEquip checks Equip[] slot bounds, positive item indices and no
// duplicate slots — shared by Upsert and SetEquip so both write paths
// enforce the same rule.
func validEquip(items []domain.MobTemplateEquipItem) bool {
	seen := make(map[int16]bool, len(items))
	for _, it := range items {
		if it.Slot < 0 || it.Slot > maxEquipSlot || it.ItemIndex <= 0 || seen[it.Slot] {
			return false
		}
		seen[it.Slot] = true
	}
	return true
}

// Delete removes the override, reverting the template to its raw file
// defaults (never touches the underlying file — Release/ is read-only in
// production).
func (s *Service) Delete(ctx context.Context, moderatorID int64, templateName string) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	err := s.store.DeleteMobTemplateStat(ctx, templateName, moderatorID)
	return classifyWrite(err, "delete")
}

// authorize checks the caller has a moderator/admin role. A missing account or
// a plain-player role yields Forbidden (never leaks whether the account exists).
func (s *Service) authorize(ctx context.Context, moderatorID int64) (Result, error) {
	if moderatorID <= 0 {
		return Forbidden, nil
	}
	role, err := s.store.AccountRole(ctx, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return Forbidden, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("mobtemplateadmin: role lookup %d: %w", moderatorID, err)
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
	default:
		return Invalid, fmt.Errorf("mobtemplateadmin: %s: %w", op, err)
	}
}
