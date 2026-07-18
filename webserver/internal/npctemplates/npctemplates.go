// Package npctemplates scans the read-only content tree for supported NPC
// templates, so the moderator UI can offer a searchable picker for
// NpcAdminService.UpsertNpc's template_name instead of free text (a typo there
// leaves the definition in Postgres but the tmServer silently skips the spawn —
// npc-editing-plan.md). The picker is intentionally curated: the raw content
// tree has legacy quest variants that the panel cannot use safely yet.
//
// STRUCT_MOB.Name is ISO-8859-1 (same source tree, same encoding as
// itemcatalog's ItemList.csv) — see cString for the transcoding, needed here
// too since DisplayName is surfaced to the moderator UI.
package npctemplates

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

const (
	merchantShop      = 1
	merchantCargo     = 2
	merchantShopType3 = 19
	merchantQuest     = 100
	merchantKing      = 111

	efGrade0 = 100
)

var canonicalKingTemplates = map[string]struct{}{
	"Rei_Harabard": {},
	"Rei_Glantuar": {},
}

// Template is one supported STRUCT_MOB template found under
// Release/TMsrv/run/npc/.
type Template struct {
	TemplateName string
	DisplayName  string
	Merchant     int16
}

// Scan reads every file in <contentDir>/TMsrv/run/npc/, decodes it as a
// STRUCT_MOB and keeps only the templates supported by the admin panel today,
// sorted by TemplateName. A file that isn't a valid 816-byte STRUCT_MOB is
// logged and skipped rather than failing the whole scan - the npc/ directory can
// contain non-template files.
func Scan(contentDir string, logger *slog.Logger) ([]Template, error) {
	dir := filepath.Join(contentDir, "TMsrv", "run", "npc")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("npctemplates: read %s: %w", dir, err)
	}

	var out []Template
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			logger.Warn("npc template read failed", "name", name, "err", err)
			continue
		}
		mob, err := savefmt.DecodeMob(b)
		if err != nil {
			logger.Warn("npc template decode failed", "name", name, "err", err)
			continue
		}
		if !supportedTemplate(name, mob) {
			continue
		}
		out = append(out, Template{
			TemplateName: name,
			DisplayName:  cString(mob.Name[:]),
			Merchant:     int16(mob.CurrentScore.Merchant),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].TemplateName < out[j].TemplateName })
	return out, nil
}

func supportedTemplate(name string, mob savefmt.Mob) bool {
	switch mob.CurrentScore.Merchant {
	case merchantShop, merchantCargo, merchantShopType3:
		return true
	case merchantQuest:
		grade := questGrade(mob)
		return grade >= 7 && grade <= 9
	case merchantKing:
		_, ok := canonicalKingTemplates[name]
		return ok
	default:
		return false
	}
}

func questGrade(mob savefmt.Mob) uint8 {
	for _, ef := range mob.Equip[0].Effects {
		if ef.Effect == efGrade0 {
			return ef.Value
		}
	}
	return 0
}

// cString trims a fixed-size name field at the first NUL byte and converts it
// from ISO-8859-1 to UTF-8 (each Latin-1 byte's value IS its Unicode code
// point, by definition of that encoding) - without this, accented names come
// out mojibake over gRPC/JSON, same bug class as itemcatalog.latin1ToUTF8.
func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			b = b[:i]
			break
		}
	}
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}
